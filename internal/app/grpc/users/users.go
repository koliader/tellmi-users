package users_server

import (
	"context"

	"github.com/koliader/tellmi-users/internal/lib/converter"
	pb "github.com/koliader/tellmi-users/internal/pb"
)

// TODO add middleware using
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
	user, err := s.users_service.GetUserById(ctx, req)
	if err != nil {
		return nil, err
	}
	return &pb.UserRes{User: converter.ConvertUser(*user)}, nil
}

func (s *Server) ListUsers(ctx context.Context, req *pb.Empty) (*pb.ListUserRes, error) {
	users, err := s.users_service.ListUsers(ctx)
	if err != nil {
		return nil, err
	}
	return &pb.ListUserRes{Users: converter.ConvertUsers(*users)}, nil
}

func (s *Server) UpdateUser(ctx context.Context, req *pb.UpdateUserReq) (*pb.UserRes, error) {
	user, err := s.users_service.UpdateUser(ctx, req)
	if err != nil {
		return nil, err
	}
	return &pb.UserRes{User: converter.ConvertUser(*user)}, nil
}
