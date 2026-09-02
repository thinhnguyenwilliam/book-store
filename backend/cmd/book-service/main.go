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
	"github.com/thinhnguyenwilliam/book-store/backend/internal/book/adapter/postgres"
	"github.com/thinhnguyenwilliam/book-store/backend/internal/book/application"
	bookgrpc "github.com/thinhnguyenwilliam/book-store/backend/internal/book/delivery/grpc"
	kafkaadapter "github.com/thinhnguyenwilliam/book-store/backend/internal/messaging/kafka"
	"github.com/thinhnguyenwilliam/book-store/backend/internal/platform/config"
	"github.com/thinhnguyenwilliam/book-store/backend/internal/platform/database"
	"github.com/thinhnguyenwilliam/book-store/backend/internal/platform/grpcserver"
	"github.com/thinhnguyenwilliam/book-store/backend/internal/platform/lifecycle"
	appLogger "github.com/thinhnguyenwilliam/book-store/backend/internal/platform/logger"
	platformoutbox "github.com/thinhnguyenwilliam/book-store/backend/internal/platform/outbox"
	"github.com/thinhnguyenwilliam/book-store/backend/internal/platform/rediscache"
	"google.golang.org/grpc"
)

func main() {
	os.Exit(execute())
}

func execute() int {
	configPath := flag.String("config", "config/config.yml", "path to YAML configuration")
	flag.Parse()
	cfg, err := config.Load(*configPath)
	if err != nil {
		slog.Error("load book service config", "error", err)
		return 1
	}
	logManager, err := appLogger.New("bookservice", cfg.Logging)
	if err != nil {
		slog.Error("initialize book service logger", "error", err)
		return 1
	}
	slog.SetDefault(logManager.Logger())
	defer func() { _ = logManager.Close() }()

	if err := run(cfg); err != nil {
		slog.Error("book service stopped", "error", err)
		return 1
	}
	return 0
}

func run(cfg config.Config) error {
	shutdownTimeout, err := time.ParseDuration(cfg.Shutdown.Timeout)
	if err != nil {
		return err
	}
	reservationTTL, err := time.ParseDuration(cfg.Commerce.StockReservationTTL)
	if err != nil {
		return err
	}
	pollInterval, err := time.ParseDuration(cfg.Outbox.PollInterval)
	if err != nil {
		return err
	}

	startupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	db, err := database.Open(startupCtx, database.Config{
		URL:                   cfg.Postgres.URL,
		MaxOpenConnections:    cfg.Postgres.MaxOpenConnections,
		MaxIdleConnections:    cfg.Postgres.MaxIdleConnections,
		ConnectionMaxLifetime: cfg.Postgres.ConnectionMaxLifetime,
		ConnectionMaxIdleTime: cfg.Postgres.ConnectionMaxIdleTime,
	})
	if err != nil {
		return err
	}
	defer func() { _ = database.Close(db) }()

	repository := postgres.NewRepository(db)
	service := application.NewService(repository)
	service.SetReservationTTL(reservationTTL)
	if cfg.Redis.Enabled {
		cacheConfig, cacheTTL, lockTTL, cacheErr := bookCacheConfig(cfg)
		if cacheErr != nil {
			return cacheErr
		}
		cacheStore, openErr := rediscache.Open(startupCtx, cacheConfig)
		if openErr != nil {
			slog.Warn("Redis unavailable; book cache disabled", "error", openErr)
		} else {
			defer func() { _ = cacheStore.Close() }()
			service.SetCache(cacheStore, cacheTTL, lockTTL)
			slog.Info("book Redis cache enabled", "ttl", cacheTTL)
		}
	}
	handler := bookgrpc.NewHandler(service)
	var catalogDispatcher *platformoutbox.Dispatcher
	if cfg.Kafka.Enabled {
		publisher, publisherErr := kafkaadapter.NewPublisher(
			cfg.Kafka.Brokers, cfg.Kafka.ClientID+"-catalog", cfg.Kafka.CatalogEventsTopic,
		)
		if publisherErr != nil {
			return publisherErr
		}
		defer publisher.Close()
		catalogDispatcher, err = platformoutbox.NewDispatcher(
			db, publisher, "catalog.outbox_events", pollInterval,
		)
		if err != nil {
			return err
		}
	}

	serverCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	backgroundCtx, stopBackground := context.WithCancel(serverCtx)
	workers := &sync.WaitGroup{}
	if catalogDispatcher != nil {
		workers.Add(1)
		go func() {
			defer workers.Done()
			catalogDispatcher.Run(backgroundCtx)
		}()
	}
	serverErr := grpcserver.Run(serverCtx, cfg.GRPC.BookListenAddress, shutdownTimeout, func(server *grpc.Server) {
		bookstorev1.RegisterBookServiceServer(server, handler)
	})
	stopBackground()
	waitCtx, cancelWait := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancelWait()
	if err := lifecycle.WaitGroup(waitCtx, workers); err != nil {
		return errors.Join(serverErr, fmt.Errorf("wait for catalog outbox shutdown: %w", err))
	}
	return serverErr
}

func bookCacheConfig(cfg config.Config) (rediscache.Config, time.Duration, time.Duration, error) {
	dialTimeout, err := time.ParseDuration(cfg.Redis.DialTimeout)
	if err != nil {
		return rediscache.Config{}, 0, 0, err
	}
	readTimeout, err := time.ParseDuration(cfg.Redis.ReadTimeout)
	if err != nil {
		return rediscache.Config{}, 0, 0, err
	}
	writeTimeout, err := time.ParseDuration(cfg.Redis.WriteTimeout)
	if err != nil {
		return rediscache.Config{}, 0, 0, err
	}
	cacheTTL, err := time.ParseDuration(cfg.Redis.BookTTL)
	if err != nil {
		return rediscache.Config{}, 0, 0, err
	}
	lockTTL, err := time.ParseDuration(cfg.Redis.LockTTL)
	if err != nil {
		return rediscache.Config{}, 0, 0, err
	}
	return rediscache.Config{
		Address: cfg.Redis.Address, Password: cfg.Redis.Password, Database: cfg.Redis.Database,
		Namespace: cfg.Redis.Namespace, DialTimeout: dialTimeout, ReadTimeout: readTimeout,
		WriteTimeout: writeTimeout, PoolSize: cfg.Redis.PoolSize,
	}, cacheTTL, lockTTL, nil
}
