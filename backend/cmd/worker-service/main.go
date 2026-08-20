package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	bookstorev1 "github.com/thinhnguyenwilliam/book-store/backend/gen/bookstore/v1"
	rabbitmqadapter "github.com/thinhnguyenwilliam/book-store/backend/internal/messaging/rabbitmq"
	"github.com/thinhnguyenwilliam/book-store/backend/internal/platform/config"
	"github.com/thinhnguyenwilliam/book-store/backend/internal/platform/grpcclient"
	appLogger "github.com/thinhnguyenwilliam/book-store/backend/internal/platform/logger"
	"github.com/thinhnguyenwilliam/book-store/backend/internal/worker"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	configPath := flag.String("config", "config/config.yml", "path to YAML configuration")
	flag.Parse()
	cfg, err := config.Load(*configPath)
	if err != nil {
		slog.Error("load worker service config", "error", err)
		os.Exit(1)
	}
	logManager, err := appLogger.New("workerservice", cfg.Logging)
	if err != nil {
		slog.Error("initialize worker service logger", "error", err)
		os.Exit(1)
	}
	slog.SetDefault(logManager.Logger())
	defer func() { _ = logManager.Close() }()

	if err := run(cfg); err != nil {
		slog.Error("worker service stopped", "error", err)
		_ = logManager.Close()
		os.Exit(1)
	}
}

func run(cfg config.Config) error {
	shutdownTimeout, err := time.ParseDuration(cfg.Shutdown.Timeout)
	if err != nil {
		return err
	}

	userConnection, err := grpc.NewClient(
		cfg.GRPC.UserAddress,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithChainUnaryInterceptor(grpcclient.UnaryLoggingInterceptor),
		grpc.WithChainStreamInterceptor(grpcclient.StreamLoggingInterceptor),
	)
	if err != nil {
		return err
	}
	defer userConnection.Close()

	consumer := rabbitmqadapter.NewConsumer(rabbitmqadapter.Config{
		URL:      cfg.RabbitMQ.URL,
		Exchange: cfg.RabbitMQ.Exchange,
		Queue:    cfg.RabbitMQ.UserProfileQueue,
		RoutingKeys: []string{
			cfg.RabbitMQ.AccountRegisteredRoutingKey,
			cfg.RabbitMQ.AccountDeletedRoutingKey,
		},
		ConsumerName: cfg.RabbitMQ.ConsumerName,
		Concurrency:  cfg.RabbitMQ.ConsumerConcurrency,
		Prefetch:     cfg.RabbitMQ.Prefetch,
	}, shutdownTimeout)
	profileHandler := worker.NewProfileHandler(bookstorev1.NewUserServiceClient(userConnection))

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	slog.Info("RabbitMQ worker started", "queue", cfg.RabbitMQ.UserProfileQueue)
	return consumer.Run(ctx, profileHandler.Handle)
}
