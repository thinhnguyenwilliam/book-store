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
	"github.com/thinhnguyenwilliam/book-store/backend/internal/book/adapter/postgres"
	"github.com/thinhnguyenwilliam/book-store/backend/internal/book/application"
	bookgrpc "github.com/thinhnguyenwilliam/book-store/backend/internal/book/delivery/grpc"
	"github.com/thinhnguyenwilliam/book-store/backend/internal/platform/config"
	"github.com/thinhnguyenwilliam/book-store/backend/internal/platform/database"
	"github.com/thinhnguyenwilliam/book-store/backend/internal/platform/grpcserver"
	appLogger "github.com/thinhnguyenwilliam/book-store/backend/internal/platform/logger"
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
	handler := bookgrpc.NewHandler(service)

	serverCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	return grpcserver.Run(serverCtx, cfg.GRPC.BookListenAddress, shutdownTimeout, func(server *grpc.Server) {
		bookstorev1.RegisterBookServiceServer(server, handler)
	})
}
