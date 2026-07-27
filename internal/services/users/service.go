package users_service

import (
	"github.com/koliader/tellmi-users/internal/lib/config"
	"github.com/koliader/tellmi-users/internal/lib/rabbitmq"
	"github.com/koliader/tellmi-users/internal/lib/token"
	db "github.com/koliader/tellmi-users/internal/store/db/sqlc"
)

type Service struct {
	tokenMaker    token.Maker
	config        config.Config
	store         db.Store
	messageSender rabbitmq.MessageSender
}

func NewService(tokenMaker token.Maker, config config.Config, store db.Store, sender rabbitmq.MessageSender) (*Service, error) {
	return &Service{tokenMaker, config, store, sender}, nil
}
