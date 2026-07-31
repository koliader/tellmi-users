package users_service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/koliader/tellmi-sdk/errors/db"
	"github.com/koliader/tellmi-sdk/errors/service"
	pb "github.com/koliader/tellmi-sdk/proto/pb"
	"github.com/koliader/tellmi-sdk/rabbitmq"
	"github.com/koliader/tellmi-sdk/token"
	"github.com/koliader/tellmi-users/internal/lib/password"
	db_store "github.com/koliader/tellmi-users/internal/store/db/sqlc"
)

const (
	authError     = "invalid login or password"
	userNotFound  = "user not found"
	alreadyExists = "user with this username already exists"

	aggregateTypeUser = "user"
	eventTypeCreated  = "userCreated"
	eventTypeUpdated  = "userUpdated"
)

type tokenPair struct {
	accessToken  string
	refreshToken string
}

func (s *Service) createTokenPair(ctx context.Context, q *db_store.Queries, user db_store.User) (*tokenPair, error) {
	accessToken, err := s.tokenMaker.CreateToken(user.ID, user.Role, s.accessTokenDuration)
	if err != nil {
		return nil, errsvc.ErrorResponse(codes.Internal, "error to generate access token: %v", err)
	}

	refreshToken, err := s.tokenMaker.CreateToken(user.ID, user.Role, s.refreshTokenDuration)
	if err != nil {
		return nil, errsvc.ErrorResponse(codes.Internal, "error to generate refresh token: %v", err)
	}

	_, err = q.CreateRefreshToken(ctx, db_store.CreateRefreshTokenParams{
		Token:     refreshToken,
		Username:  user.Username,
		ExpiresAt: time.Now().Add(s.refreshTokenDuration),
	})
	if err != nil {
		return nil, errsvc.ErrorResponse(codes.Internal, "error to save refresh token: %v", err)
	}

	return &tokenPair{accessToken: accessToken, refreshToken: refreshToken}, nil
}

func (s *Service) Register(ctx context.Context, req *pb.RegisterReq) (*pb.AuthRes, error) {
	hashPassword, err := password.HashPassword(req.GetPassword())
	if err != nil {
		return nil, errsvc.ErrorResponse(codes.Internal, "error to hash password: %v", err)
	}

	arg := db_store.CreateUserParams{
		Password: hashPassword,
		Username: req.GetUsername(),
	}

	var (
		user   db_store.User
		tokens *tokenPair
	)

	err = s.store.ExecTx(ctx, func(q *db_store.Queries) error {
		user, err = q.CreateUser(ctx, arg)
		if err != nil {
			return err
		}

		tokens, err = s.createTokenPair(ctx, q, user)
		if err != nil {
			return err
		}

		msgBody, err := json.Marshal(rabbitmq.UserCreated{ID: user.ID, Username: user.Username})
		if err != nil {
			return err
		}

		_, err = q.InsertOutboxEvent(ctx, db_store.InsertOutboxEventParams{
			AggregateType: aggregateTypeUser,
			AggregateID:   user.ID,
			EventType:     eventTypeCreated,
			Payload:       msgBody,
		})
		return err
	})
	if err != nil {
		if errdb.ErrorCode(err) == errdb.UniqueViolation {
			return nil, errsvc.ErrorResponse(codes.AlreadyExists, alreadyExists)
		}
		if _, ok := status.FromError(err); ok {
			return nil, err
		}
		return nil, errsvc.ErrorResponse(codes.Internal, "error to create a user: %v", err)
	}

	return &pb.AuthRes{
		AccessToken:  tokens.accessToken,
		RefreshToken: tokens.refreshToken,
	}, nil
}

func (s *Service) Login(ctx context.Context, req *pb.LoginReq) (*pb.AuthRes, error) {
	user, err := s.store.GetUserByUsername(ctx, req.GetUsername())
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, errsvc.AuthError(fmt.Errorf("%v", authError))
		}
		return nil, errsvc.ErrorResponse(codes.Internal, "error to get user: %v", err)
	}

	err = password.CheckPassword(user.Password, req.GetPassword())
	if err != nil {
		return nil, errsvc.AuthError(fmt.Errorf("%v", authError))
	}

	var tokens *tokenPair
	err = s.store.ExecTx(ctx, func(q *db_store.Queries) error {
		tokens, err = s.createTokenPair(ctx, q, user)
		return err
	})
	if err != nil {
		if _, ok := status.FromError(err); ok {
			return nil, err
		}
		return nil, errsvc.ErrorResponse(codes.Internal, "error to generate tokens: %v", err)
	}

	return &pb.AuthRes{
		AccessToken:  tokens.accessToken,
		RefreshToken: tokens.refreshToken,
	}, nil
}

func (s *Service) Refresh(ctx context.Context, req *pb.RefreshReq) (*pb.RefreshRes, error) {
	refreshPayload, err := s.tokenMaker.VerifyToken(req.GetRefreshToken())
	if err != nil {
		return nil, errsvc.AuthError(fmt.Errorf("invalid refresh token"))
	}

	refreshToken, err := s.store.GetRefreshToken(ctx, req.GetRefreshToken())
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, errsvc.AuthError(fmt.Errorf("refresh token not found"))
		}
		return nil, errsvc.ErrorResponse(codes.Internal, "error to get refresh token: %v", err)
	}

	if time.Now().After(refreshToken.ExpiresAt) {
		s.store.DeleteRefreshToken(ctx, req.GetRefreshToken())
		return nil, errsvc.AuthError(fmt.Errorf("refresh token expired"))
	}

	user, err := s.store.GetUserById(ctx, refreshPayload.ID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, errsvc.AuthError(fmt.Errorf("user not found"))
		}
		return nil, errsvc.ErrorResponse(codes.Internal, "error to get user: %v", err)
	}

	var tokens *tokenPair
	err = s.store.ExecTx(ctx, func(q *db_store.Queries) error {
		if err := q.DeleteRefreshToken(ctx, req.GetRefreshToken()); err != nil {
			return err
		}
		tokens, err = s.createTokenPair(ctx, q, user)
		return err
	})
	if err != nil {
		if _, ok := status.FromError(err); ok {
			return nil, err
		}
		return nil, errsvc.ErrorResponse(codes.Internal, "error to refresh tokens: %v", err)
	}

	return &pb.RefreshRes{
		AccessToken:  tokens.accessToken,
		RefreshToken: tokens.refreshToken,
	}, nil
}

func (s *Service) GetUserById(ctx context.Context, req *pb.IdReq) (*db_store.User, error) {
	id, err := uuid.Parse(req.GetId())
	if err != nil {
		return nil, errsvc.ErrorResponse(codes.InvalidArgument, "invalid UUID")
	}
	user, err := s.store.GetUserById(ctx, id)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, errsvc.AuthError(fmt.Errorf("%v", userNotFound))
		}
		return nil, errsvc.ErrorResponse(codes.Internal, "error to get user: %v", err)
	}
	return &user, nil
}

func (s *Service) ListUsers(ctx context.Context) (*[]db_store.User, error) {
	users, err := s.store.ListUsers(ctx)
	if err != nil {
		return nil, errsvc.ErrorResponse(codes.Internal, "error to list users: %v", err)
	}
	return &users, nil
}

func (s *Service) UpdateUser(ctx context.Context, req *pb.UpdateUserReq, payload *token.Payload) (*db_store.User, error) {
	arg := db_store.UpdateUserParams{
		ID:       payload.ID,
		Username: req.GetUsername(),
	}

	var updatedUser db_store.User
	err := s.store.ExecTx(ctx, func(q *db_store.Queries) error {
		// actual update user
		updated, err := q.UpdateUser(ctx, arg)
		if err != nil {
			return err
		}
		updatedUser = updated

		msgBody, err := json.Marshal(rabbitmq.UserUpdated{ID: payload.ID, NewUsername: req.GetUsername()})
		if err != nil {
			return err
		}

		// insert event
		_, err = q.InsertOutboxEvent(ctx, db_store.InsertOutboxEventParams{
			AggregateType: aggregateTypeUser,
			AggregateID:   payload.ID,
			EventType:     eventTypeUpdated,
			Payload:       msgBody,
		})
		return err
	})
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, errsvc.ErrorResponse(codes.NotFound, userNotFound)
		}
		if errdb.ErrorCode(err) == errdb.UniqueViolation {
			return nil, errsvc.ErrorResponse(codes.AlreadyExists, alreadyExists)
		}
		return nil, errsvc.ErrorResponse(codes.Internal, "error to update user: %v", err)
	}

	return &updatedUser, nil
}
