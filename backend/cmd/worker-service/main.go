package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	bookstorev1 "github.com/thinhnguyenwilliam/book-store/backend/gen/bookstore/v1"
	rabbitmqadapter "github.com/thinhnguyenwilliam/book-store/backend/internal/messaging/rabbitmq"
	"github.com/thinhnguyenwilliam/book-store/backend/internal/platform/config"
	"github.com/thinhnguyenwilliam/book-store/backend/internal/worker"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	configPath := flag.String("config", "config/config.yml", "path to YAML configuration")
	flag.Parse()
	if err := run(*configPath); err != nil {
		slog.Error("worker service stopped", "error", err)
		os.Exit(1)
	}
}

func run(configPath string) error {
	cfg, err := config.Load(configPath)
	if err != nil {
		return err
	}

	userConnection, err := grpc.NewClient(
		cfg.GRPC.UserAddress,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return err
	}
	defer userConnection.Close()

	consumer := rabbitmqadapter.NewConsumer(rabbitmqadapter.Config{
		URL:          cfg.RabbitMQ.URL,
		Exchange:     cfg.RabbitMQ.Exchange,
		Queue:        cfg.RabbitMQ.UserProfileQueue,
		RoutingKey:   cfg.RabbitMQ.AccountRegisteredRoutingKey,
		ConsumerName: cfg.RabbitMQ.ConsumerName,
		Concurrency:  cfg.RabbitMQ.ConsumerConcurrency,
		Prefetch:     cfg.RabbitMQ.Prefetch,
	})
	profileHandler := worker.NewProfileHandler(bookstorev1.NewUserServiceClient(userConnection))

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	slog.Info("RabbitMQ worker started", "queue", cfg.RabbitMQ.UserProfileQueue)
	return consumer.Run(ctx, profileHandler.Handle)
}
