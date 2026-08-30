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
	commentgrpcclient "github.com/thinhnguyenwilliam/book-store/backend/internal/comment/adapter/grpcclient"
	commentpostgres "github.com/thinhnguyenwilliam/book-store/backend/internal/comment/adapter/postgres"
	"github.com/thinhnguyenwilliam/book-store/backend/internal/comment/application"
	commentgrpc "github.com/thinhnguyenwilliam/book-store/backend/internal/comment/delivery/grpc"
	"github.com/thinhnguyenwilliam/book-store/backend/internal/platform/config"
	"github.com/thinhnguyenwilliam/book-store/backend/internal/platform/database"
	"github.com/thinhnguyenwilliam/book-store/backend/internal/platform/grpcclient"
	"github.com/thinhnguyenwilliam/book-store/backend/internal/platform/grpcserver"
	appLogger "github.com/thinhnguyenwilliam/book-store/backend/internal/platform/logger"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() { os.Exit(execute()) }
func execute() int {
	configPath := flag.String("config", "config/config.yml", "path to YAML configuration")
	flag.Parse()
	cfg, err := config.Load(*configPath)
	if err != nil {
		slog.Error("load comment service config", "error", err)
		return 1
	}
	logManager, err := appLogger.New("commentservice", cfg.Logging)
	if err != nil {
		slog.Error("initialize comment service logger", "error", err)
		return 1
	}
	slog.SetDefault(logManager.Logger())
	defer func() { _ = logManager.Close() }()
	if err := run(cfg); err != nil {
		slog.Error("comment service stopped", "error", err)
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
	startupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	db, err := database.Open(startupCtx, database.Config{URL: cfg.Postgres.URL, MaxOpenConnections: cfg.Postgres.MaxOpenConnections, MaxIdleConnections: cfg.Postgres.MaxIdleConnections, ConnectionMaxLifetime: cfg.Postgres.ConnectionMaxLifetime, ConnectionMaxIdleTime: cfg.Postgres.ConnectionMaxIdleTime})
	if err != nil {
		return err
	}
	defer func() { _ = database.Close(db) }()
	bookConnection, err := newClient(cfg.GRPC.BookAddress, callTimeout)
	if err != nil {
		return err
	}
	defer func() { _ = bookConnection.Close() }()
	userConnection, err := newClient(cfg.GRPC.UserAddress, callTimeout)
	if err != nil {
		return err
	}
	defer func() { _ = userConnection.Close() }()
	service := application.NewService(commentpostgres.NewRepository(db), commentgrpcclient.NewBookResolver(bookstorev1.NewBookServiceClient(bookConnection)), commentgrpcclient.NewAuthorResolver(bookstorev1.NewUserServiceClient(userConnection)))
	handler := commentgrpc.NewHandler(service)
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	return grpcserver.Run(ctx, cfg.GRPC.CommentListenAddress, shutdownTimeout, func(server *grpc.Server) { bookstorev1.RegisterCommentServiceServer(server, handler) })
}

func newClient(address string, timeout time.Duration) (*grpc.ClientConn, error) {
	return grpc.NewClient(address, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithChainUnaryInterceptor(grpcclient.UnaryDeadlineInterceptor(timeout), grpcclient.UnaryLoggingInterceptor), grpc.WithChainStreamInterceptor(grpcclient.StreamLoggingInterceptor))
}
