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
	chatgrpcclient "github.com/thinhnguyenwilliam/book-store/backend/internal/chat/adapter/grpcclient"
	chatpostgres "github.com/thinhnguyenwilliam/book-store/backend/internal/chat/adapter/postgres"
	"github.com/thinhnguyenwilliam/book-store/backend/internal/chat/application"
	chatgrpc "github.com/thinhnguyenwilliam/book-store/backend/internal/chat/delivery/grpc"
	rabbitmqadapter "github.com/thinhnguyenwilliam/book-store/backend/internal/messaging/rabbitmq"
	"github.com/thinhnguyenwilliam/book-store/backend/internal/platform/config"
	"github.com/thinhnguyenwilliam/book-store/backend/internal/platform/database"
	"github.com/thinhnguyenwilliam/book-store/backend/internal/platform/grpcclient"
	"github.com/thinhnguyenwilliam/book-store/backend/internal/platform/grpcserver"
	"github.com/thinhnguyenwilliam/book-store/backend/internal/platform/lifecycle"
	appLogger "github.com/thinhnguyenwilliam/book-store/backend/internal/platform/logger"
	"github.com/thinhnguyenwilliam/book-store/backend/internal/platform/outbox"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() { os.Exit(execute()) }

func execute() int {
	configPath := flag.String("config", "config/config.yml", "path to YAML configuration")
	flag.Parse()
	cfg, err := config.Load(*configPath)
	if err != nil {
		slog.Error("load chat service config", "error", err)
		return 1
	}
	logManager, err := appLogger.New("chatservice", cfg.Logging)
	if err != nil {
		slog.Error("initialize chat service logger", "error", err)
		return 1
	}
	slog.SetDefault(logManager.Logger())
	defer func() { _ = logManager.Close() }()
	if err := run(cfg); err != nil {
		slog.Error("chat service stopped", "error", err)
		return 1
	}
	return 0
}

func run(cfg config.Config) error {
	shutdownTimeout, err := time.ParseDuration(cfg.Shutdown.Timeout)
	if err != nil {
		return err
	}
	callTimeout, err := time.ParseDuration(cfg.GRPC.CallTimeout)
	if err != nil {
		return err
	}
	pollInterval, err := time.ParseDuration(cfg.Outbox.PollInterval)
	if err != nil {
		return err
	}
	startupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	db, err := database.Open(startupCtx, database.Config{URL: cfg.Postgres.URL, MaxOpenConnections: cfg.Postgres.MaxOpenConnections, MaxIdleConnections: cfg.Postgres.MaxIdleConnections, ConnectionMaxLifetime: cfg.Postgres.ConnectionMaxLifetime, ConnectionMaxIdleTime: cfg.Postgres.ConnectionMaxIdleTime})
	if err != nil {
		return err
	}
	defer func() { _ = database.Close(db) }()
	userConnection, err := grpc.NewClient(cfg.GRPC.UserAddress, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithChainUnaryInterceptor(grpcclient.UnaryDeadlineInterceptor(callTimeout), grpcclient.UnaryLoggingInterceptor), grpc.WithChainStreamInterceptor(grpcclient.StreamLoggingInterceptor))
	if err != nil {
		return err
	}
	defer func() { _ = userConnection.Close() }()
	service := application.NewService(chatpostgres.NewRepository(db), chatgrpcclient.NewAuthorResolver(bookstorev1.NewUserServiceClient(userConnection)))
	handler := chatgrpc.NewHandler(service)
	publisher := rabbitmqadapter.NewPublisher(rabbitmqadapter.Config{URL: cfg.RabbitMQ.URL, Exchange: cfg.RabbitMQ.Exchange, Queue: cfg.RabbitMQ.NotificationEventsQueue, RoutingKeys: []string{cfg.RabbitMQ.AccountRegisteredRoutingKey, cfg.RabbitMQ.PaymentSucceededRoutingKey, cfg.RabbitMQ.PaymentFailedRoutingKey, cfg.RabbitMQ.PaymentRefundedRoutingKey, cfg.RabbitMQ.ChatMessageCreatedRoutingKey}, ConsumerName: "bookstore-chat-outbox"})
	defer func() { _ = publisher.Close() }()
	dispatcher, err := outbox.NewDispatcher(db, publisher, "chat.outbox_events", pollInterval)
	if err != nil {
		return err
	}
	serverCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	dispatcherCtx, stopDispatcher := context.WithCancel(serverCtx)
	workers := &sync.WaitGroup{}
	workers.Add(1)
	go func() { defer workers.Done(); dispatcher.Run(dispatcherCtx) }()
	serverErr := grpcserver.Run(serverCtx, cfg.GRPC.ChatListenAddress, shutdownTimeout, func(server *grpc.Server) { bookstorev1.RegisterChatServiceServer(server, handler) })
	stopDispatcher()
	waitCtx, cancelWait := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancelWait()
	if err := lifecycle.WaitGroup(waitCtx, workers); err != nil {
		return errors.Join(serverErr, fmt.Errorf("wait for chat outbox dispatcher shutdown: %w", err))
	}
	return serverErr
}
