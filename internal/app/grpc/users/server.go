package users_server

import (
	pb "github.com/koliader/tellmi-users/internal/pb"
)

type Server struct {
	pb.UnimplementedUsersServer
	users_service UserService
}

func NewServer(usersService UserService) *Server {
	return &Server{users_service: usersService}
}
