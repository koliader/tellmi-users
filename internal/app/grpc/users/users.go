package users_server

import (
	"context"

	"github.com/koliader/tellmi-users/internal/lib/converter"
	pb "github.com/koliader/tellmi-users/internal/pb"
)

func (s *Server) Register(ctx context.Context, req *pb.RegisterReq) (*pb.AuthRes, error) {
	token, err := s.users_service.Register(ctx, req)
	if err != nil {
		return nil, err
	}
	res := pb.AuthRes{Token: *token}
	return &res, nil
}

func (s *Server) Login(ctx context.Context, req *pb.LoginReq) (*pb.AuthRes, error) {
	token, err := s.users_service.Login(ctx, req)
	if err != nil {
		return nil, err
	}
	res := pb.AuthRes{Token: *token}
	return &res, nil
}

func (s *Server) GetUserById(ctx context.Context, req *pb.IdReq) (*pb.UserRes, error) {
	user, err := s.users_service.GetUserById(ctx, req)
	if err != nil {
		return nil, err
	}
	res := pb.UserRes{User: converter.ConvertUser(*user)}
	return &res, nil
}

func (s *Server) ListUsers(ctx context.Context, req *pb.Empty) (*pb.ListUserRes, error) {
	users, err := s.users_service.ListUsers(ctx)
	if err != nil {
		return nil, err
	}

	res := pb.ListUserRes{Users: converter.ConvertUsers(*users)}
	return &res, nil
}
