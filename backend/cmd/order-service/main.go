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
	rabbitmqadapter "github.com/thinhnguyenwilliam/book-store/backend/internal/messaging/rabbitmq"
	orderevents "github.com/thinhnguyenwilliam/book-store/backend/internal/order/adapter/events"
	ordergrpcclient "github.com/thinhnguyenwilliam/book-store/backend/internal/order/adapter/grpcclient"
	orderpostgres "github.com/thinhnguyenwilliam/book-store/backend/internal/order/adapter/postgres"
	"github.com/thinhnguyenwilliam/book-store/backend/internal/order/application"
	ordergrpc "github.com/thinhnguyenwilliam/book-store/backend/internal/order/delivery/grpc"
	"github.com/thinhnguyenwilliam/book-store/backend/internal/platform/config"
	"github.com/thinhnguyenwilliam/book-store/backend/internal/platform/database"
	"github.com/thinhnguyenwilliam/book-store/backend/internal/platform/grpcclient"
	"github.com/thinhnguyenwilliam/book-store/backend/internal/platform/grpcserver"
	"github.com/thinhnguyenwilliam/book-store/backend/internal/platform/lifecycle"
	appLogger "github.com/thinhnguyenwilliam/book-store/backend/internal/platform/logger"
	platformoutbox "github.com/thinhnguyenwilliam/book-store/backend/internal/platform/outbox"
	"github.com/thinhnguyenwilliam/book-store/backend/internal/platform/rediscache"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	os.Exit(execute())
}

func execute() int {
	configPath := flag.String("config", "config/config.yml", "path to YAML configuration")
	flag.Parse()
	cfg, err := config.Load(*configPath)
	if err != nil {
		slog.Error("load order service config", "error", err)
		return 1
	}
	logManager, err := appLogger.New("orderservice", cfg.Logging)
	if err != nil {
		slog.Error("initialize order service logger", "error", err)
		return 1
	}
	slog.SetDefault(logManager.Logger())
	defer func() { _ = logManager.Close() }()
	if err := run(cfg); err != nil {
		slog.Error("order service stopped", "error", err)
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
	reservationTTL, err := time.ParseDuration(cfg.Commerce.StockReservationTTL)
	if err != nil {
		return err
	}
	reconcileInterval, err := time.ParseDuration(cfg.Commerce.ReconcileInterval)
	if err != nil {
		return err
	}
	paymentReconcileGrace, err := time.ParseDuration(cfg.Commerce.PaymentReconcileGrace)
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
		URL: cfg.Postgres.URL, MaxOpenConnections: cfg.Postgres.MaxOpenConnections,
		MaxIdleConnections:    cfg.Postgres.MaxIdleConnections,
		ConnectionMaxLifetime: cfg.Postgres.ConnectionMaxLifetime,
		ConnectionMaxIdleTime: cfg.Postgres.ConnectionMaxIdleTime,
	})
	if err != nil {
		return err
	}
	defer func() { _ = database.Close(db) }()

	bookConnection, err := newClient(cfg.GRPC.BookAddress, callTimeout)
	if err != nil {
		return err
	}
	defer func() { _ = bookConnection.Close() }()
	paymentConnection, err := newClient(cfg.GRPC.PaymentAddress, callTimeout)
	if err != nil {
		return err
	}
	defer func() { _ = paymentConnection.Close() }()

	repository := orderpostgres.NewRepository(db)
	service := application.NewService(
		repository,
		ordergrpcclient.NewBookClient(bookstorev1.NewBookServiceClient(bookConnection)),
		ordergrpcclient.NewPaymentClient(bookstorev1.NewPaymentServiceClient(paymentConnection)),
		application.Config{
			Currency: cfg.Payment.Currency, PlatformOwnerID: cfg.Payment.PlatformOwnerID,
			StockReservationTTL:   reservationTTL,
			PaymentReconcileGrace: paymentReconcileGrace,
		},
	)
	if cfg.Redis.Enabled {
		cacheConfig, cacheTTL, lockTTL, cacheErr := orderCacheConfig(cfg)
		if cacheErr != nil {
			return cacheErr
		}
		cacheStore, openErr := rediscache.Open(startupCtx, cacheConfig)
		if openErr != nil {
			slog.Warn("Redis unavailable; cart cache disabled", "error", openErr)
		} else {
			defer func() { _ = cacheStore.Close() }()
			service.SetCache(cacheStore, cacheTTL, lockTTL)
			slog.Info("cart Redis cache enabled", "ttl", cacheTTL)
		}
	}
	handler := ordergrpc.NewHandler(service)
	var orderEventDispatcher *platformoutbox.Dispatcher
	var activityEventDispatcher *platformoutbox.Dispatcher
	if cfg.Kafka.Enabled {
		orderPublisher, publisherErr := kafkaadapter.NewPublisher(
			cfg.Kafka.Brokers, cfg.Kafka.ClientID+"-order", cfg.Kafka.OrderEventsTopic,
		)
		if publisherErr != nil {
			return publisherErr
		}
		defer orderPublisher.Close()
		activityPublisher, activityPublisherErr := kafkaadapter.NewPublisher(
			cfg.Kafka.Brokers, cfg.Kafka.ClientID+"-order-activity", cfg.Kafka.CustomerActivityTopic,
		)
		if activityPublisherErr != nil {
			return activityPublisherErr
		}
		defer activityPublisher.Close()
		orderEventDispatcher, err = platformoutbox.NewDispatcher(
			db, orderPublisher, "orders.outbox_events", pollInterval,
		)
		if err != nil {
			return err
		}
		activityEventDispatcher, err = platformoutbox.NewDispatcher(
			db, activityPublisher, "orders.customer_activity_outbox_events", pollInterval,
		)
		if err != nil {
			return err
		}
	}

	serverCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	reconcileCtx, stopReconciler := context.WithCancel(serverCtx)
	reconcileWorkers := &sync.WaitGroup{}
	reconcileWorkers.Add(2)
	go func() {
		defer reconcileWorkers.Done()
		runReconciler(reconcileCtx, service, reconcileInterval, cfg.Commerce.ReconcileBatchSize)
	}()
	paymentConsumer := rabbitmqadapter.NewConsumer(rabbitmqadapter.Config{
		URL: cfg.RabbitMQ.URL, Exchange: cfg.RabbitMQ.Exchange,
		Queue: cfg.RabbitMQ.PaymentEventsQueue,
		RoutingKeys: []string{
			cfg.RabbitMQ.PaymentSucceededRoutingKey,
			cfg.RabbitMQ.PaymentFailedRoutingKey,
			cfg.RabbitMQ.PaymentRefundedRoutingKey,
		},
		ConsumerName: cfg.RabbitMQ.PaymentConsumerName,
		Concurrency:  cfg.RabbitMQ.ConsumerConcurrency, Prefetch: cfg.RabbitMQ.Prefetch,
	}, shutdownTimeout)
	paymentHandler := orderevents.NewPaymentHandler(service)
	go func() {
		defer reconcileWorkers.Done()
		if err := paymentConsumer.Run(reconcileCtx, paymentHandler.Handle); err != nil && !errors.Is(err, context.Canceled) {
			slog.Error("payment event consumer stopped", "error", err)
		}
	}()
	if orderEventDispatcher != nil {
		reconcileWorkers.Add(1)
		go func() {
			defer reconcileWorkers.Done()
			orderEventDispatcher.Run(reconcileCtx)
		}()
	}
	if activityEventDispatcher != nil {
		reconcileWorkers.Add(1)
		go func() {
			defer reconcileWorkers.Done()
			activityEventDispatcher.Run(reconcileCtx)
		}()
	}

	serverErr := grpcserver.Run(serverCtx, cfg.GRPC.OrderListenAddress, shutdownTimeout, func(server *grpc.Server) {
		bookstorev1.RegisterOrderServiceServer(server, handler)
	})
	stopReconciler()
	waitCtx, cancelWait := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancelWait()
	if err := lifecycle.WaitGroup(waitCtx, reconcileWorkers); err != nil {
		return errors.Join(serverErr, fmt.Errorf("wait for order reconciler shutdown: %w", err))
	}
	return serverErr
}

func orderCacheConfig(cfg config.Config) (rediscache.Config, time.Duration, time.Duration, error) {
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
	cacheTTL, err := time.ParseDuration(cfg.Redis.CartTTL)
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

func runReconciler(ctx context.Context, service *application.Service, interval time.Duration, batchSize int) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if ctx.Err() != nil {
				return
			}
			reconcileCtx, cancel := context.WithTimeout(ctx, interval)
			if err := service.Reconcile(reconcileCtx, batchSize); err != nil && !errors.Is(err, context.Canceled) {
				slog.WarnContext(reconcileCtx, "order saga reconciliation failed", "error", err)
			}
			cancel()
		}
	}
}

func newClient(address string, timeout time.Duration) (*grpc.ClientConn, error) {
	return grpc.NewClient(
		address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithChainUnaryInterceptor(
			grpcclient.UnaryDeadlineInterceptor(timeout), grpcclient.UnaryLoggingInterceptor,
		),
		grpc.WithChainStreamInterceptor(grpcclient.StreamLoggingInterceptor),
	)
}
