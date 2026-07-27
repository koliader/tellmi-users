package users_server

import (
	"context"

	db "github.com/koliader/tellmi-users/internal/store/db/sqlc"
	pb "github.com/koliader/tellmi-users/internal/pb"
)

type UserService interface {
	Register(ctx context.Context, req *pb.RegisterReq) (*pb.AuthRes, error)
	Login(ctx context.Context, req *pb.LoginReq) (*pb.AuthRes, error)
	Refresh(ctx context.Context, req *pb.RefreshReq) (*pb.RefreshRes, error)
	GetUserById(ctx context.Context, req *pb.IdReq) (*db.User, error)
	ListUsers(ctx context.Context) (*[]db.User, error)
	UpdateUser(ctx context.Context, req *pb.UpdateUserReq) (*db.User, error)
}
