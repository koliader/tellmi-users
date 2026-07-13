package main

import (
	"context"
	"fmt"
	"net"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
	users_server "github.com/koliader/tellmi-users/internal/app/grpc/users"
	"github.com/koliader/tellmi-users/internal/lib/config"
	"github.com/koliader/tellmi-users/internal/lib/logger"
	pb "github.com/koliader/tellmi-users/internal/pb"
	db "github.com/koliader/tellmi-users/internal/store/db/sqlc"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func main() {
	config, err := config.LoadConfig(".")
	// config, err := config.LoadKuberConfig()
	if err != nil {
		log.Fatal().Msg("cannot load config")
	}

	if config.Environment == "dev" || config.Environment == "docker" {
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	}

	connPool, err := pgxpool.New(context.Background(), config.DBSource)
	if err != nil {
		log.Fatal().Err(err).Msg("cannot connect to db")
	}

	defer connPool.Close()
	store := db.NewStore(connPool)

	runGrpcServer(config, store)
}

func runGrpcServer(config config.Config, store db.Store) {
	usersServer, err := users_server.NewServer(config, store)
	if err != nil {
		log.Fatal().Err(err).Msg(fmt.Sprintf("cannot create users service: %v", err))
	}
	listener, err := net.Listen("tcp", config.ServerAddress)
	if err != nil {
		log.Fatal().Err(err).Msg("cannot create listener")
	}

	// create logger
	grpcLogger := grpc.UnaryInterceptor(logger.GrpcLogger)
	// create server
	grpcServer := grpc.NewServer(grpcLogger)
	pb.RegisterUsersServer(grpcServer, usersServer)
	reflection.Register(grpcServer)

	log.Info().Msgf("start gRPC server at %s", listener.Addr().String())
	err = grpcServer.Serve(listener)
	if err != nil {
		log.Fatal().Err(err).Msg("cannot start gRPC server")
	}
}
