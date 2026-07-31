package users_server

import (
	"context"

	pb "github.com/koliader/tellmi-sdk/proto/pb"
	"github.com/koliader/tellmi-sdk/token"
	db "github.com/koliader/tellmi-users/internal/store/db/sqlc"
)

type UserService interface {
	Register(ctx context.Context, req *pb.RegisterReq) (*pb.AuthRes, error)
	Login(ctx context.Context, req *pb.LoginReq) (*pb.AuthRes, error)
	Refresh(ctx context.Context, req *pb.RefreshReq) (*pb.RefreshRes, error)
	GetUserById(ctx context.Context, req *pb.IdReq) (*db.User, error)
	ListUsers(ctx context.Context) (*[]db.User, error)
	UpdateUser(ctx context.Context, req *pb.UpdateUserReq, payload *token.Payload) (*db.User, error)
}
