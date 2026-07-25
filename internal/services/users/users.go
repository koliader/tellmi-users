package users_service

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"
	"google.golang.org/grpc/codes"

	db_err "github.com/koliader/tellmi-users/internal/lib/error/db"
	grpc_err "github.com/koliader/tellmi-users/internal/lib/error/service"
	"github.com/koliader/tellmi-users/internal/lib/password"
	"github.com/koliader/tellmi-users/internal/lib/rabbitmq"
	pb "github.com/koliader/tellmi-users/internal/pb"
	db "github.com/koliader/tellmi-users/internal/store/db/sqlc"
	"github.com/rs/zerolog/log"
)

const (
	authError     = "invalid login or password"
	userNotFound  = "user not found"
	alreadyExists = "user with this username already exists"
)

func (s *Service) Register(ctx context.Context, req *pb.RegisterReq) (*string, error) {
	hashPassword, err := password.HashPassword(req.GetPassword())
	if err != nil {
		return nil, grpc_err.ErrorResponse(codes.Internal, "error to hash password: %v", err)
	}

	arg := db.CreateUserParams{
		Password: hashPassword,
		Username: req.GetUsername(),
	}
	user, err := s.store.CreateUser(ctx, arg)
	if err != nil {
		if db_err.ErrorCode(err) == db_err.UniqueViolation {
			return nil, grpc_err.ErrorResponse(
				codes.AlreadyExists,
				alreadyExists,
			)
		}
		return nil, grpc_err.ErrorResponse(codes.Internal, "error to create a user: %v", err)
	}

	token, err := s.tokenMaker.CreateToken(user.Username, user.Role, s.config.AccessTokenDuration)
	if err != nil {
		return nil, grpc_err.ErrorResponse(codes.Internal, "error to generate token: %v", err)
	}

	msg := rabbitmq.UserCreated{
		Username: user.Username,
	}
	msgBody, err := json.Marshal(msg)
	if err != nil {
		log.Error().Err(err).Msg("error to marshal rabbitmq message")
	} else {
		err = s.RabbitmqClient.SendMessage(rabbitmq.UserCreatedQueue, msgBody)
		if err != nil {
			log.Error().Err(err).Msg("error to send rabbitmq message")
		}
	}

	return &token, nil
}

func (s *Service) Login(ctx context.Context, req *pb.LoginReq) (*string, error) {
	user, err := s.store.GetUserByUsername(ctx, req.GetUsername())
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, grpc_err.AuthError(fmt.Errorf("%v", authError))
		}
		return nil, grpc_err.ErrorResponse(codes.Internal, "error to get user: %v", err)
	}

	err = password.CheckPassword(user.Password, req.GetPassword())
	if err != nil {
		return nil, grpc_err.AuthError(fmt.Errorf("%v", authError))
	}
	token, err := s.tokenMaker.CreateToken(user.Username, user.Role, s.config.AccessTokenDuration)
	if err != nil {
		return nil, grpc_err.ErrorResponse(codes.Internal, "error to generate token: %v", err)
	}
	return &token, nil
}

func (s *Service) GetUserById(ctx context.Context, req *pb.IdReq) (*db.User, error) {
	user, err := s.store.GetUserById(ctx, req.GetId())
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, grpc_err.AuthError(fmt.Errorf("%v", authError))
		}
		return nil, grpc_err.ErrorResponse(codes.Internal, "error to get user: %v", err)
	}
	return &user, nil
}

func (s *Service) ListUsers(ctx context.Context) (*[]db.User, error) {
	users, err := s.store.ListUsers(ctx)
	if err != nil {
		return nil, grpc_err.ErrorResponse(codes.Internal, "error to lsit users: %v", err)
	}
	return &users, nil
}

func (s *Service) UpdateUser(ctx context.Context, req *pb.UpdateUserReq) (*db.User, error) {
	arg := db.UpdateUserParams{
		ID:       req.GetId(),
		Username: req.GetUsername(),
	}

	user, err := s.store.GetUserById(ctx, req.GetId())
	if err != nil {

		if err == pgx.ErrNoRows {
			return nil, grpc_err.ErrorResponse(codes.NotFound, userNotFound)
		}
	}
	updatedUser, err := s.store.UpdateUser(ctx, arg)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, grpc_err.ErrorResponse(codes.NotFound, userNotFound)
		}
		if db_err.ErrorCode(err) == db_err.UniqueViolation {
			return nil, grpc_err.ErrorResponse(codes.AlreadyExists, alreadyExists)
		}
		return nil, grpc_err.ErrorResponse(codes.Internal, "error to update user: %v", err)
	}
	msg := rabbitmq.UserUpdated{
		Username:    user.Username,
		NewUsername: req.GetUsername(),
	}
	msgBody, err := json.Marshal(msg)
	if err != nil {
		log.Error().Err(err).Msg("error to marshal rabbitmq message")
	} else {
		err = s.RabbitmqClient.SendMessage(rabbitmq.UserUpdatedQueue, msgBody)
		if err != nil {
			log.Error().Err(err).Msg("error to send rabbitmq message")
		}
	}
	log.Info().Msg("sent")
	return &updatedUser, nil
}
