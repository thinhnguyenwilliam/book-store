package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"
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
	"github.com/thinhnguyenwilliam/book-store/backend/internal/platform/lifecycle"
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
	ttl, err := time.ParseDuration(cfg.Auth.AccessTokenTTL)
	if err != nil {
		return err
	}
	refreshTTL, err := time.ParseDuration(cfg.Auth.RefreshTokenTTL)
	if err != nil {
		return err
	}
	pollInterval, err := time.ParseDuration(cfg.Outbox.PollInterval)
	if err != nil {
		return err
	}
	shutdownTimeout, err := time.ParseDuration(cfg.Shutdown.Timeout)
	if err != nil {
		return err
	}

	startupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	db, err := database.Open(startupCtx, cfg.Postgres.URL)
	if err != nil {
		return err
	}
	defer database.Close(db)

	repository := postgres.NewRepository(db)
	hasher := security.NewPasswordHasher()
	accessTokens := security.NewTokenManager(cfg.Auth.JWTSecret, cfg.Auth.JWTIssuer, ttl)
	refreshTokens := security.NewRefreshTokenManager()
	service := application.NewService(repository, hasher, accessTokens, refreshTokens, refreshTTL)
	handler := authgrpc.NewHandler(service)

	publisher := rabbitmqadapter.NewPublisher(rabbitmqadapter.Config{
		URL:          cfg.RabbitMQ.URL,
		Exchange:     cfg.RabbitMQ.Exchange,
		Queue:        cfg.RabbitMQ.UserProfileQueue,
		RoutingKey:   cfg.RabbitMQ.AccountRegisteredRoutingKey,
		ConsumerName: cfg.RabbitMQ.ConsumerName,
	})
	defer publisher.Close()
	serverCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	dispatcherCtx, stopDispatcher := context.WithCancel(serverCtx)
	dispatcherWorkers := &sync.WaitGroup{}
	dispatcherWorkers.Add(1)
	go func() {
		defer dispatcherWorkers.Done()
		outbox.NewDispatcher(db, publisher, pollInterval).Run(dispatcherCtx)
	}()

	serverErr := grpcserver.Run(serverCtx, cfg.GRPC.AuthListenAddress, shutdownTimeout, func(server *grpc.Server) {
		bookstorev1.RegisterAuthServiceServer(server, handler)
	})
	slog.Info("outbox dispatcher graceful shutdown started", "timeout", shutdownTimeout)
	stopDispatcher()

	waitCtx, cancelWait := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancelWait()
	if err := lifecycle.WaitGroup(waitCtx, dispatcherWorkers); err != nil {
		return errors.Join(serverErr, fmt.Errorf("wait for outbox dispatcher shutdown: %w", err))
	}
	slog.Info("outbox dispatcher graceful shutdown completed")
	return serverErr
}
