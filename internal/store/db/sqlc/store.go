package db

import (
	"log"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/koliader/tellmi-users/internal/lib/config"
)

type Store struct {
	connPool *pgxpool.Pool
	*Queries
	config config.Config
}

func NewStore(connPool *pgxpool.Pool) Store {
	config, err := config.LoadConfig("../../../..")
	// config, err := config.LoadKuberConfig()
	if err != nil {
		log.Fatal("cannot load config:", err)
	}
	return Store{
		connPool: connPool,
		Queries:  New(connPool),
		config:   config,
	}
}
