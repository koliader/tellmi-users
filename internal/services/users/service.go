package users_service

import (
	"time"

	"github.com/koliader/tellmi-sdk/token"
	db "github.com/koliader/tellmi-users/internal/store/db/sqlc"
)

type Service struct {
	tokenMaker           token.Maker
	accessTokenDuration  time.Duration
	refreshTokenDuration time.Duration
	store               db.Store
}

func NewService(tokenMaker token.Maker, accessDuration, refreshDuration time.Duration, store db.Store) (*Service, error) {
	return &Service{tokenMaker, accessDuration, refreshDuration, store}, nil
}
