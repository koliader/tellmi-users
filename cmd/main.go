package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	users_server "github.com/koliader/tellmi-users/internal/app/grpc/users"
	"github.com/koliader/tellmi-users/internal/dispatcher"
	"github.com/koliader/tellmi-users/internal/lib/logger"
	users_service "github.com/koliader/tellmi-users/internal/services/users"
	db "github.com/koliader/tellmi-users/internal/store/db/sqlc"
	"github.com/koliader/tellmi-sdk/config"
	"github.com/koliader/tellmi-sdk/middleware"
	"github.com/koliader/tellmi-sdk/rabbitmq"
	"github.com/koliader/tellmi-sdk/token"
	pb "github.com/koliader/tellmi-sdk/proto/pb"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

type Config struct {
	DBSource             string        `mapstructure:"DB_SOURCE"`
	ServerAddress        string        `mapstructure:"SERVER_ADDRESS"`
	TokenKey             string        `mapstructure:"TOKEN_KEY"`
	AccessTokenDuration  time.Duration `mapstructure:"ACCESS_TOKEN_DURATION"`
	RefreshTokenDuration time.Duration `mapstructure:"REFRESH_TOKEN_DURATION"`
	Environment          string        `mapstructure:"ENVIRONMENT"`
	RbmUrl               string        `mapstructure:"RBM_URL"`
}

func main() {
	var cfg Config
	err := config.LoadConfig(".", &cfg)
	if err != nil {
		log.Fatal().Msg("cannot load config")
	}

	if cfg.Environment == "dev" || cfg.Environment == "docker" {
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	}

	connPool, err := pgxpool.New(context.Background(), cfg.DBSource)
	if err != nil {
		log.Fatal().Err(err).Msg("cannot connect to db")
	}

	defer connPool.Close()
	store := db.NewStore(connPool)

	runGrpcServer(cfg, store)
}

func runGrpcServer(cfg Config, store db.Store) {
	tokenMaker, err := token.NewJWTMaker(cfg.TokenKey)
	if err != nil {
		log.Fatal().Err(err).Msg("cannot create token maker")
	}

	rabbitmqClient, err := rabbitmq.NewClient(cfg.RbmUrl)
	if err != nil {
		log.Fatal().Err(err).Msg("cannot create rabbitmq client")
	}
	defer rabbitmqClient.Close()

	svc, err := users_service.NewService(tokenMaker, cfg.AccessTokenDuration, cfg.RefreshTokenDuration, store)
	if err != nil {
		log.Fatal().Err(err).Msg(fmt.Sprintf("cannot create users service: %v", err))
	}

	outboxDispatcher := dispatcher.New(store, rabbitmqClient, dispatcher.Config{})
	outboxDispatcher.Start(context.Background())

	grpcMiddleware := middleware.NewMiddleware(tokenMaker)

	usersServer := users_server.NewServer(svc, grpcMiddleware)
	listener, err := net.Listen("tcp", cfg.ServerAddress)
	if err != nil {
		log.Fatal().Err(err).Msg("cannot create listener")
	}

	grpcLogger := grpc.UnaryInterceptor(logger.GrpcLogger)
	grpcServer := grpc.NewServer(grpcLogger)
	pb.RegisterUsersServer(grpcServer, usersServer)
	reflection.Register(grpcServer)

	log.Info().Msgf("start gRPC server at %s", listener.Addr().String())
	err = grpcServer.Serve(listener)
	if err != nil {
		log.Fatal().Err(err).Msg("cannot start gRPC server")
	}
}
