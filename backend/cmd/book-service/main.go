package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"time"

	bookstorev1 "github.com/thinhnguyenwilliam/book-store/backend/gen/bookstore/v1"
	"github.com/thinhnguyenwilliam/book-store/backend/internal/book/adapter/postgres"
	"github.com/thinhnguyenwilliam/book-store/backend/internal/book/application"
	bookgrpc "github.com/thinhnguyenwilliam/book-store/backend/internal/book/delivery/grpc"
	"github.com/thinhnguyenwilliam/book-store/backend/internal/platform/config"
	"github.com/thinhnguyenwilliam/book-store/backend/internal/platform/database"
	"github.com/thinhnguyenwilliam/book-store/backend/internal/platform/grpcserver"
	"google.golang.org/grpc"
)

func main() {
	configPath := flag.String("config", "config/config.yml", "path to YAML configuration")
	flag.Parse()
	if err := run(*configPath); err != nil {
		slog.Error("book service stopped", "error", err)
		os.Exit(1)
	}
}

func run(configPath string) error {
	cfg, err := config.Load(configPath)
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
	service := application.NewService(repository)
	handler := bookgrpc.NewHandler(service)

	return grpcserver.Run(cfg.GRPC.BookListenAddress, func(server *grpc.Server) {
		bookstorev1.RegisterBookServiceServer(server, handler)
	})
}
