package users_server

import (
	"fmt"

	"github.com/koliader/tellmi-users/internal/lib/config"
	"github.com/koliader/tellmi-users/internal/lib/middleware"
	"github.com/koliader/tellmi-users/internal/lib/token"
	pb "github.com/koliader/tellmi-users/internal/pb"
	users_service "github.com/koliader/tellmi-users/internal/services/users"
	db "github.com/koliader/tellmi-users/internal/store/db/sqlc"
)

type Server struct {
	pb.UnimplementedUsersServer
	users_service users_service.Service
	middleware    middleware.Middleware
}

func NewServer(config config.Config, store db.Store) (*Server, error) {
	tokenMaker, err := token.NewJWTMaker(config.TokenKey)
	if err != nil {
		return nil, fmt.Errorf("error to create token maker: %v", err)
	}

	usersService, err := users_service.NewService(tokenMaker, config, store)
	if err != nil {
		return nil, fmt.Errorf("error to create users service: %v", err)
	}
	middleware := middleware.NewMiddleware(tokenMaker)

	server := Server{users_service: *usersService, middleware: *middleware}

	return &server, nil
}
