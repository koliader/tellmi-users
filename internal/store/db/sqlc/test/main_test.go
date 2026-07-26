package db_tests

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/koliader/tellmi-users/internal/lib/config"
	db "github.com/koliader/tellmi-users/internal/store/db/sqlc"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

var testStore db.Store

func TestMain(m *testing.M) {
	cfg, err := config.LoadConfig("../../../../..")
	if err != nil {
		log.Fatal().Err(err).Msg("cannot load config")
	}

	connPool, err := pgxpool.New(context.Background(), cfg.DBSource)
	if err != nil {
		log.Fatal().Err(err).Msg("cannot connect to db")
	}
	defer connPool.Close()

	if cfg.Environment == "dev" {
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	}

	testStore = db.NewStore(connPool)
	os.Exit(m.Run())
}
