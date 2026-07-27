package users_server

import (
	"github.com/koliader/tellmi-sdk/middleware"
	pb "github.com/koliader/tellmi-sdk/proto/pb"
)

type Server struct {
	pb.UnimplementedUsersServer
	users_service UserService
	middleware    middleware.GRPCMiddleware
}

func NewServer(usersService UserService, mw middleware.GRPCMiddleware) *Server {
	return &Server{users_service: usersService, middleware: mw}
}
