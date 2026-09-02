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
	analyticsevents "github.com/thinhnguyenwilliam/book-store/backend/internal/analytics/adapter/events"
	analyticspostgres "github.com/thinhnguyenwilliam/book-store/backend/internal/analytics/adapter/postgres"
	"github.com/thinhnguyenwilliam/book-store/backend/internal/analytics/application"
	analyticsgrpc "github.com/thinhnguyenwilliam/book-store/backend/internal/analytics/delivery/grpc"
	kafkaadapter "github.com/thinhnguyenwilliam/book-store/backend/internal/messaging/kafka"
	"github.com/thinhnguyenwilliam/book-store/backend/internal/platform/config"
	"github.com/thinhnguyenwilliam/book-store/backend/internal/platform/database"
	"github.com/thinhnguyenwilliam/book-store/backend/internal/platform/grpcserver"
	"github.com/thinhnguyenwilliam/book-store/backend/internal/platform/lifecycle"
	appLogger "github.com/thinhnguyenwilliam/book-store/backend/internal/platform/logger"
	"google.golang.org/grpc"
)

func main() { os.Exit(execute()) }

func execute() int {
	configPath := flag.String("config", "config/config.yml", "path to YAML configuration")
	flag.Parse()
	cfg, err := config.Load(*configPath)
	if err != nil {
		slog.Error("load analytics service config", "error", err)
		return 1
	}
	logManager, err := appLogger.New("analyticsservice", cfg.Logging)
	if err != nil {
		slog.Error("initialize analytics service logger", "error", err)
		return 1
	}
	slog.SetDefault(logManager.Logger())
	defer func() { _ = logManager.Close() }()
	if err := run(cfg); err != nil {
		slog.Error("analytics service stopped", "error", err)
		return 1
	}
	return 0
}

func run(cfg config.Config) error {
	shutdownTimeout, err := time.ParseDuration(cfg.Shutdown.Timeout)
	if err != nil {
		return err
	}
	startupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	db, err := database.Open(startupCtx, database.Config{
		URL: cfg.Postgres.URL, MaxOpenConnections: cfg.Postgres.MaxOpenConnections,
		MaxIdleConnections:    cfg.Postgres.MaxIdleConnections,
		ConnectionMaxLifetime: cfg.Postgres.ConnectionMaxLifetime,
		ConnectionMaxIdleTime: cfg.Postgres.ConnectionMaxIdleTime,
	})
	if err != nil {
		return err
	}
	defer func() { _ = database.Close(db) }()

	service := application.NewService(analyticspostgres.NewRepository(db))
	handler := analyticsgrpc.NewHandler(service)
	var orderConsumer *kafkaadapter.Consumer
	var activityConsumer *kafkaadapter.Consumer
	if cfg.Kafka.Enabled {
		retryBackoff, parseErr := time.ParseDuration(cfg.Kafka.ConsumerRetryBackoff)
		if parseErr != nil {
			return parseErr
		}
		orderConsumer, err = kafkaadapter.NewConsumer(
			cfg.Kafka.Brokers, cfg.Kafka.ClientID+"-analytics", cfg.Kafka.AnalyticsConsumerGroup,
			cfg.Kafka.OrderEventsTopic, cfg.Kafka.OrderEventsDLQTopic,
			cfg.Kafka.ConsumerMaxRetries, retryBackoff,
		)
		if err != nil {
			return err
		}
		defer orderConsumer.Close()
		activityConsumer, err = kafkaadapter.NewConsumer(
			cfg.Kafka.Brokers, cfg.Kafka.ClientID+"-customer-activity", cfg.Kafka.ActivityConsumerGroup,
			cfg.Kafka.CustomerActivityTopic, cfg.Kafka.CustomerActivityDLQ,
			cfg.Kafka.ConsumerMaxRetries, retryBackoff,
		)
		if err != nil {
			return err
		}
		defer activityConsumer.Close()
	}
	eventHandler := analyticsevents.NewHandler(service)
	activityHandler := analyticsevents.NewCustomerActivityHandler(service)

	serverCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	backgroundCtx, stopBackground := context.WithCancel(serverCtx)
	workers := &sync.WaitGroup{}
	if orderConsumer != nil {
		workers.Add(1)
		go func() {
			defer workers.Done()
			if consumeErr := orderConsumer.Run(backgroundCtx, eventHandler.Handle); consumeErr != nil && !errors.Is(consumeErr, context.Canceled) {
				slog.Error("Kafka analytics consumer stopped", "error", consumeErr)
			}
		}()
	}
	if activityConsumer != nil {
		workers.Add(1)
		go func() {
			defer workers.Done()
			if consumeErr := activityConsumer.Run(backgroundCtx, activityHandler.Handle); consumeErr != nil && !errors.Is(consumeErr, context.Canceled) {
				slog.Error("Kafka customer activity consumer stopped", "error", consumeErr)
			}
		}()
	}
	serverErr := grpcserver.Run(serverCtx, cfg.GRPC.AnalyticsListenAddress, shutdownTimeout, func(server *grpc.Server) {
		bookstorev1.RegisterAnalyticsServiceServer(server, handler)
	})
	stopBackground()
	waitCtx, cancelWait := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancelWait()
	if err := lifecycle.WaitGroup(waitCtx, workers); err != nil {
		return errors.Join(serverErr, fmt.Errorf("wait for analytics consumer shutdown: %w", err))
	}
	return serverErr
}
