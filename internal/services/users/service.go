package users_service

import (
	"github.com/koliader/tellmi-users/internal/lib/config"
	"github.com/koliader/tellmi-users/internal/lib/rabbitmq"
	"github.com/koliader/tellmi-users/internal/lib/token"
	db "github.com/koliader/tellmi-users/internal/store/db/sqlc"
)

type Service struct {
	tokenMaker     token.Maker
	config         config.Config
	store          db.Store
	RabbitmqClient *rabbitmq.Client
}

func NewService(tokenMaker token.Maker, config config.Config, store db.Store) (*Service, error) {
	rabbitmqClient, err := rabbitmq.NewRabbitmqClient(config)
	if err != nil {
		return nil, err
	}
	return &Service{tokenMaker, config, store, rabbitmqClient}, nil
}
