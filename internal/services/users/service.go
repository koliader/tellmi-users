package users_service

import (
	"github.com/koliader/tellmi-users/internal/lib/config"
	"github.com/koliader/tellmi-users/internal/lib/token"
	db "github.com/koliader/tellmi-users/internal/store/db/sqlc"
)

type Service struct {
	tokenMaker token.Maker
	config     config.Config
	store      db.Store
}

func NewService(tokenMaker token.Maker, config config.Config, store db.Store) *Service {
	return &Service{tokenMaker, config, store}
}
