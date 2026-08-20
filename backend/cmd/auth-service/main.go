package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"time"

	bookstorev1 "github.com/thinhnguyenwilliam/book-store/backend/gen/bookstore/v1"
	"github.com/thinhnguyenwilliam/book-store/backend/internal/auth/adapter/outbox"
	"github.com/thinhnguyenwilliam/book-store/backend/internal/auth/adapter/postgres"
	"github.com/thinhnguyenwilliam/book-store/backend/internal/auth/adapter/security"
	"github.com/thinhnguyenwilliam/book-store/backend/internal/auth/application"
	authgrpc "github.com/thinhnguyenwilliam/book-store/backend/internal/auth/delivery/grpc"
	rabbitmqadapter "github.com/thinhnguyenwilliam/book-store/backend/internal/messaging/rabbitmq"
	"github.com/thinhnguyenwilliam/book-store/backend/internal/platform/config"
	"github.com/thinhnguyenwilliam/book-store/backend/internal/platform/database"
	"github.com/thinhnguyenwilliam/book-store/backend/internal/platform/grpcserver"
	"google.golang.org/grpc"
)

func main() {
	configPath := flag.String("config", "config/config.yml", "path to YAML configuration")
	flag.Parse()
	if err := run(*configPath); err != nil {
		slog.Error("auth service stopped", "error", err)
		os.Exit(1)
	}
}

func run(configPath string) error {
	cfg, err := config.Load(configPath)
	if err != nil {
		return err
	}
	ttl, err := time.ParseDuration(cfg.Auth.JWTTTL)
	if err != nil {
		return err
	}
	pollInterval, err := time.ParseDuration(cfg.Outbox.PollInterval)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	db, err := database.Open(ctx, cfg.Postgres.URL)
	if err != nil {
		return err
	}
	defer database.Close(db)

	repository := postgres.NewRepository(db)
	hasher := security.NewPasswordHasher()
	tokens := security.NewTokenManager(cfg.Auth.JWTSecret, cfg.Auth.JWTIssuer, ttl)
	service := application.NewService(repository, hasher, tokens)
	handler := authgrpc.NewHandler(service)

	publisher := rabbitmqadapter.NewPublisher(rabbitmqadapter.Config{
		URL:          cfg.RabbitMQ.URL,
		Exchange:     cfg.RabbitMQ.Exchange,
		Queue:        cfg.RabbitMQ.UserProfileQueue,
		RoutingKey:   cfg.RabbitMQ.AccountRegisteredRoutingKey,
		ConsumerName: cfg.RabbitMQ.ConsumerName,
	})
	defer publisher.Close()
	dispatcherCtx, stopDispatcher := context.WithCancel(context.Background())
	defer stopDispatcher()
	go outbox.NewDispatcher(db, publisher, pollInterval).Run(dispatcherCtx)

	return grpcserver.Run(cfg.GRPC.AuthListenAddress, func(server *grpc.Server) {
		bookstorev1.RegisterAuthServiceServer(server, handler)
	})
}
