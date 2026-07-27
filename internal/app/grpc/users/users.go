package users_server

import (
	"context"

	"github.com/koliader/tellmi-sdk/errors/service"
	"github.com/koliader/tellmi-sdk/proto/pb"
	"github.com/koliader/tellmi-users/internal/lib/converter"
	"google.golang.org/grpc/codes"
)

func (s *Server) Register(ctx context.Context, req *pb.RegisterReq) (*pb.AuthRes, error) {
	return s.users_service.Register(ctx, req)
}

func (s *Server) Login(ctx context.Context, req *pb.LoginReq) (*pb.AuthRes, error) {
	return s.users_service.Login(ctx, req)
}

func (s *Server) Refresh(ctx context.Context, req *pb.RefreshReq) (*pb.RefreshRes, error) {
	return s.users_service.Refresh(ctx, req)
}

func (s *Server) GetUserById(ctx context.Context, req *pb.IdReq) (*pb.UserRes, error) {
	_, err := s.middleware.AuthorizeAdmin(ctx)
	if err != nil {
		return nil, errsvc.ErrorResponse(codes.Unauthenticated, "error to authorize admin: %v", err)
	}

	user, err := s.users_service.GetUserById(ctx, req)
	if err != nil {
		return nil, err
	}
	return &pb.UserRes{User: converter.ConvertUser(*user)}, nil
}

func (s *Server) ListUsers(ctx context.Context, req *pb.Empty) (*pb.ListUserRes, error) {
	_, err := s.middleware.AuthorizeUser(ctx)
	if err != nil {
		return nil, errsvc.ErrorResponse(codes.Unauthenticated, "error to authorize user: %v", err)
	}

	users, err := s.users_service.ListUsers(ctx)
	if err != nil {
		return nil, err
	}
	return &pb.ListUserRes{Users: converter.ConvertUsers(*users)}, nil
}

func (s *Server) UpdateUser(ctx context.Context, req *pb.UpdateUserReq) (*pb.UserRes, error) {
	_, err := s.middleware.AuthorizeUser(ctx)
	if err != nil {
		return nil, errsvc.ErrorResponse(codes.Unauthenticated, "error to authorize user: %v", err)
	}

	user, err := s.users_service.UpdateUser(ctx, req)
	if err != nil {
		return nil, err
	}
	return &pb.UserRes{User: converter.ConvertUser(*user)}, nil
}
