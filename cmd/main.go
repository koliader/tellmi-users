package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/exaring/otelpgx"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"

	"github.com/koliader/tellmi-sdk/config"
	"github.com/koliader/tellmi-sdk/health"
	"github.com/koliader/tellmi-sdk/middleware"
	"github.com/koliader/tellmi-sdk/otel"
	pb "github.com/koliader/tellmi-sdk/proto/pb"
	"github.com/koliader/tellmi-sdk/rabbitmq"
	"github.com/koliader/tellmi-sdk/token"
	users_server "github.com/koliader/tellmi-users/internal/app/grpc/users"
	"github.com/koliader/tellmi-users/internal/dispatcher"
	"github.com/koliader/tellmi-users/internal/lib/logger"
	users_service "github.com/koliader/tellmi-users/internal/services/users"
	db "github.com/koliader/tellmi-users/internal/store/db/sqlc"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
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
	HealthAddress        string        `mapstructure:"HEALTH_ADDRESS"`
}

func main() {
	var cfg Config
	err := config.Load(".", &cfg)
	if err != nil {
		log.Fatal().Msg("cannot load config")
	}

	if cfg.Environment == "dev" {
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr}).Hook(otel.NewZerologHook())
	} else {
		log.Logger = log.Logger.Hook(otel.NewZerologHook())
	}
	zerolog.DefaultContextLogger = &log.Logger

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	otelSDK, err := otel.Init(ctx, otel.Config{ServiceName: "users-service"})
	if err != nil {
		log.Fatal().Err(err).Msg("cannot init otel")
	}
	defer func() {
		if err := otelSDK.Shutdown(context.Background()); err != nil {
			log.Error().Err(err).Msg("otel shutdown")
		}
	}()

	pgxConfig, err := pgxpool.ParseConfig(cfg.DBSource)
	if err != nil {
		log.Fatal().Err(err).Msg("cannot parse db config")
	}
	pgxConfig.ConnConfig.Tracer = otelpgx.NewTracer()
	connPool, err := pgxpool.NewWithConfig(ctx, pgxConfig)
	if err != nil {
		log.Fatal().Err(err).Msg("cannot connect to db")
	}
	defer connPool.Close()

	store := db.NewStore(connPool)

	if cfg.HealthAddress != "" {
		healthServer := health.NewServer(cfg.HealthAddress,
			func(ctx context.Context) error {
				return connPool.Ping(ctx)
			},
		).WithHandler("/metrics", otel.MetricsHandler())
		go func() {
			log.Info().Msgf("start health server at %s", cfg.HealthAddress)
			if err := healthServer.Start(); err != nil {
				log.Error().Err(err).Msg("health server stopped")
			}
		}()
	}

	runGrpcServer(ctx, cfg, store, otelSDK.TracerProvider, otelSDK.MeterProvider)
}

func runGrpcServer(ctx context.Context, cfg Config, store db.Store, tracerProvider trace.TracerProvider, meterProvider metric.MeterProvider) {
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

	outboxDispatcher := dispatcher.New(store, rabbitmqClient, dispatcher.Config{
		MeterProvider: meterProvider,
	})
	outboxDispatcher.Start(ctx)

	grpcMiddleware := middleware.NewMiddleware(tokenMaker)

	usersServer := users_server.NewServer(svc, grpcMiddleware)
	listener, err := net.Listen("tcp", cfg.ServerAddress)
	if err != nil {
		log.Fatal().Err(err).Msg("cannot create listener")
	}

	grpcLogger := grpc.UnaryInterceptor(logger.GrpcLogger)
	grpcServer := grpc.NewServer(
		grpcLogger,
		grpc.StatsHandler(otelgrpc.NewServerHandler(
			otelgrpc.WithTracerProvider(tracerProvider),
			otelgrpc.WithMeterProvider(meterProvider),
		)),
	)
	pb.RegisterUsersServer(grpcServer, usersServer)
	reflection.Register(grpcServer)

	go func() {
		<-ctx.Done()
		grpcServer.GracefulStop()
	}()

	log.Info().Msgf("start gRPC server at %s", listener.Addr().String())
	err = grpcServer.Serve(listener)
	if err != nil {
		log.Fatal().Err(err).Msg("cannot start gRPC server")
	}
}
