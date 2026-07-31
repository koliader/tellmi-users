package users_service

import (
	"time"

	"github.com/koliader/tellmi-sdk/rabbitmq"
	"github.com/koliader/tellmi-sdk/token"
	db "github.com/koliader/tellmi-users/internal/store/db/sqlc"
)

type Service struct {
	tokenMaker           token.Maker
	accessTokenDuration  time.Duration
	refreshTokenDuration time.Duration
	store               db.Store
	messageSender       rabbitmq.MessageSender
}

func NewService(tokenMaker token.Maker, accessDuration, refreshDuration time.Duration, store db.Store, sender rabbitmq.MessageSender) (*Service, error) {
	return &Service{tokenMaker, accessDuration, refreshDuration, store, sender}, nil
}
