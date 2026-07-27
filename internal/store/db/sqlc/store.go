package db

import (
	"context"
	"log"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/koliader/tellmi-users/internal/lib/config"
)

type Store interface {
	CreateUser(ctx context.Context, arg CreateUserParams) (User, error)
	GetUserById(ctx context.Context, id int64) (User, error)
	GetUserByUsername(ctx context.Context, username string) (User, error)
	ListUsers(ctx context.Context) ([]User, error)
	UpdateUser(ctx context.Context, arg UpdateUserParams) (User, error)
	CreateRefreshToken(ctx context.Context, arg CreateRefreshTokenParams) (RefreshToken, error)
	GetRefreshToken(ctx context.Context, token string) (RefreshToken, error)
	DeleteRefreshToken(ctx context.Context, token string) error
	DeleteRefreshTokensByUsername(ctx context.Context, username string) error
}

type SQLStore struct {
	connPool *pgxpool.Pool
	*Queries
	config config.Config
}

func NewStore(connPool *pgxpool.Pool) Store {
	config, err := config.LoadConfig("../../../..")
	if err != nil {
		log.Fatal("cannot load config:", err)
	}
	return &SQLStore{
		connPool: connPool,
		Queries:  New(connPool),
		config:   config,
	}
}
