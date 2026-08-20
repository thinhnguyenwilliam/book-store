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
	"github.com/thinhnguyenwilliam/book-store/backend/internal/platform/config"
	"github.com/thinhnguyenwilliam/book-store/backend/internal/platform/database"
	"github.com/thinhnguyenwilliam/book-store/backend/internal/platform/grpcserver"
	"github.com/thinhnguyenwilliam/book-store/backend/internal/user/adapter/postgres"
	"github.com/thinhnguyenwilliam/book-store/backend/internal/user/application"
	usergrpc "github.com/thinhnguyenwilliam/book-store/backend/internal/user/delivery/grpc"
	"google.golang.org/grpc"
)

func main() {
	configPath := flag.String("config", "config/config.yml", "path to YAML configuration")
	flag.Parse()
	if err := run(*configPath); err != nil {
		slog.Error("user service stopped", "error", err)
		os.Exit(1)
	}
}

func run(configPath string) error {
	cfg, err := config.Load(configPath)
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
	service := application.NewService(repository)
	handler := usergrpc.NewHandler(service)

	serverCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	return grpcserver.Run(serverCtx, cfg.GRPC.UserListenAddress, shutdownTimeout, func(server *grpc.Server) {
		bookstorev1.RegisterUserServiceServer(server, handler)
	})
}
