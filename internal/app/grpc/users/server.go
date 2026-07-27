package users_server

import (
	"github.com/koliader/tellmi-users/internal/lib/middleware"
	pb "github.com/koliader/tellmi-users/internal/pb"
)

type Server struct {
	pb.UnimplementedUsersServer
	users_service UserService
	middleware    middleware.Middleware
}

func NewServer(usersService UserService, middleware middleware.Middleware) *Server {
	return &Server{users_service: usersService, middleware: middleware}
}
