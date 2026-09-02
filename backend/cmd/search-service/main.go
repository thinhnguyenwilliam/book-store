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
	kafkaadapter "github.com/thinhnguyenwilliam/book-store/backend/internal/messaging/kafka"
	"github.com/thinhnguyenwilliam/book-store/backend/internal/platform/config"
	"github.com/thinhnguyenwilliam/book-store/backend/internal/platform/grpcclient"
	"github.com/thinhnguyenwilliam/book-store/backend/internal/platform/grpcserver"
	"github.com/thinhnguyenwilliam/book-store/backend/internal/platform/lifecycle"
	appLogger "github.com/thinhnguyenwilliam/book-store/backend/internal/platform/logger"
	searchelastic "github.com/thinhnguyenwilliam/book-store/backend/internal/search/adapter/elasticsearch"
	searchevents "github.com/thinhnguyenwilliam/book-store/backend/internal/search/adapter/events"
	searchgrpcclient "github.com/thinhnguyenwilliam/book-store/backend/internal/search/adapter/grpcclient"
	searchapp "github.com/thinhnguyenwilliam/book-store/backend/internal/search/application"
	searchgrpc "github.com/thinhnguyenwilliam/book-store/backend/internal/search/delivery/grpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() { os.Exit(execute()) }

func execute() int {
	configPath := flag.String("config", "config/config.yml", "path to YAML configuration")
	forceReindex := flag.Bool("reindex", false, "reindex all PostgreSQL books before serving")
	flag.Parse()
	cfg, err := config.Load(*configPath)
	if err != nil {
		slog.Error("load search service config", "error", err)
		return 1
	}
	logManager, err := appLogger.New("searchservice", cfg.Logging)
	if err != nil {
		slog.Error("initialize search service logger", "error", err)
		return 1
	}
	slog.SetDefault(logManager.Logger())
	defer func() { _ = logManager.Close() }()
	if err := run(cfg, *forceReindex); err != nil {
		slog.Error("search service stopped", "error", err)
		return 1
	}
	return 0
}

func run(cfg config.Config, forceReindex bool) error {
	if !cfg.Elasticsearch.Enabled {
		return fmt.Errorf("elasticsearch must be enabled for search-service")
	}
	shutdownTimeout, err := time.ParseDuration(cfg.Shutdown.Timeout)
	if err != nil {
		return err
	}
	requestTimeout, err := time.ParseDuration(cfg.Elasticsearch.RequestTimeout)
	if err != nil {
		return err
	}
	callTimeout, err := time.ParseDuration(cfg.GRPC.CallTimeout)
	if err != nil {
		return err
	}
	index, err := searchelastic.Open(searchelastic.Config{
		Addresses: cfg.Elasticsearch.Addresses, Username: cfg.Elasticsearch.Username,
		Password: cfg.Elasticsearch.Password, Alias: cfg.Elasticsearch.IndexAlias, Timeout: requestTimeout,
	})
	if err != nil {
		return err
	}
	service := searchapp.NewService(index)
	startupCtx, cancelStartup := context.WithTimeout(context.Background(), 20*time.Second)
	created, err := service.Ensure(startupCtx)
	cancelStartup()
	if err != nil {
		return err
	}

	bookConnection, err := grpc.NewClient(
		cfg.GRPC.BookAddress,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithChainUnaryInterceptor(
			grpcclient.UnaryDeadlineInterceptor(callTimeout), grpcclient.UnaryLoggingInterceptor,
		),
		grpc.WithChainStreamInterceptor(grpcclient.StreamLoggingInterceptor),
	)
	if err != nil {
		return err
	}
	defer func() { _ = bookConnection.Close() }()
	if forceReindex || (created && cfg.Elasticsearch.BootstrapReindex) {
		reindexCtx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		count, reindexErr := service.Reindex(
			reindexCtx, searchgrpcclient.NewCatalog(bookstorev1.NewBookServiceClient(bookConnection)),
		)
		cancel()
		if reindexErr != nil {
			return reindexErr
		}
		slog.Info("catalog reindex completed", "books", count, "index", cfg.Elasticsearch.IndexAlias)
	}

	var consumer *kafkaadapter.Consumer
	if cfg.Kafka.Enabled {
		retryBackoff, parseErr := time.ParseDuration(cfg.Kafka.ConsumerRetryBackoff)
		if parseErr != nil {
			return parseErr
		}
		consumer, err = kafkaadapter.NewConsumer(
			cfg.Kafka.Brokers, cfg.Kafka.ClientID+"-search", cfg.Kafka.SearchConsumerGroup,
			cfg.Kafka.CatalogEventsTopic, cfg.Kafka.CatalogEventsDLQTopic,
			cfg.Kafka.ConsumerMaxRetries, retryBackoff,
		)
		if err != nil {
			return err
		}
		defer consumer.Close()
	}

	handler := searchgrpc.NewHandler(service)
	eventHandler := searchevents.NewCatalogHandler(service)
	serverCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	backgroundCtx, stopBackground := context.WithCancel(serverCtx)
	workers := &sync.WaitGroup{}
	if consumer != nil {
		workers.Add(1)
		go func() {
			defer workers.Done()
			if consumeErr := consumer.Run(backgroundCtx, eventHandler.Handle); consumeErr != nil && !errors.Is(consumeErr, context.Canceled) {
				slog.Error("Kafka catalog search consumer stopped", "error", consumeErr)
			}
		}()
	}
	serverErr := grpcserver.Run(serverCtx, cfg.GRPC.SearchListenAddress, shutdownTimeout, func(server *grpc.Server) {
		bookstorev1.RegisterSearchServiceServer(server, handler)
	})
	stopBackground()
	waitCtx, cancelWait := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancelWait()
	if err := lifecycle.WaitGroup(waitCtx, workers); err != nil {
		return errors.Join(serverErr, fmt.Errorf("wait for search consumer shutdown: %w", err))
	}
	return serverErr
}
