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
	rabbitmqadapter "github.com/thinhnguyenwilliam/book-store/backend/internal/messaging/rabbitmq"
	paymentpostgres "github.com/thinhnguyenwilliam/book-store/backend/internal/payment/adapter/postgres"
	"github.com/thinhnguyenwilliam/book-store/backend/internal/payment/adapter/vnpay"
	"github.com/thinhnguyenwilliam/book-store/backend/internal/payment/application"
	paymentgrpc "github.com/thinhnguyenwilliam/book-store/backend/internal/payment/delivery/grpc"
	"github.com/thinhnguyenwilliam/book-store/backend/internal/platform/config"
	"github.com/thinhnguyenwilliam/book-store/backend/internal/platform/database"
	"github.com/thinhnguyenwilliam/book-store/backend/internal/platform/grpcserver"
	"github.com/thinhnguyenwilliam/book-store/backend/internal/platform/lifecycle"
	appLogger "github.com/thinhnguyenwilliam/book-store/backend/internal/platform/logger"
	platformoutbox "github.com/thinhnguyenwilliam/book-store/backend/internal/platform/outbox"
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
		slog.Error("load payment service config", "error", err)
		return 1
	}
	logManager, err := appLogger.New("paymentservice", cfg.Logging)
	if err != nil {
		slog.Error("initialize payment service logger", "error", err)
		return 1
	}
	slog.SetDefault(logManager.Logger())
	defer func() { _ = logManager.Close() }()

	if err := run(cfg); err != nil {
		slog.Error("payment service stopped", "error", err)
		return 1
	}
	return 0
}

func run(cfg config.Config) error {
	shutdownTimeout, err := time.ParseDuration(cfg.Shutdown.Timeout)
	if err != nil {
		return err
	}
	reconcileInterval, err := time.ParseDuration(cfg.Payment.ReconcileInterval)
	if err != nil {
		return err
	}
	reconcileGrace, err := time.ParseDuration(cfg.Payment.ReconcileGrace)
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

	repository := paymentpostgres.NewRepository(db)
	service := application.NewService(repository, application.Config{
		Currency: cfg.Payment.Currency, PlatformOwnerID: cfg.Payment.PlatformOwnerID,
		FundingOwnerID: cfg.Payment.FundingOwnerID, ClearingOwnerID: cfg.Payment.ClearingOwnerID,
		DefaultProvider: cfg.Payment.DefaultProvider, PlatformFeeBPS: cfg.Payment.PlatformFeeBPS,
		ReconcileGrace: reconcileGrace,
	})
	if cfg.Payment.VNPay.Enabled {
		gateway, gatewayErr := newVNPayGateway(cfg)
		if gatewayErr != nil {
			return gatewayErr
		}
		service.SetGateway(gateway)
	}
	handler := paymentgrpc.NewHandler(service)
	publisher := rabbitmqadapter.NewPublisher(rabbitmqadapter.Config{
		URL: cfg.RabbitMQ.URL, Exchange: cfg.RabbitMQ.Exchange,
		Queue: cfg.RabbitMQ.PaymentEventsQueue,
		RoutingKeys: []string{
			cfg.RabbitMQ.PaymentSucceededRoutingKey,
			cfg.RabbitMQ.PaymentFailedRoutingKey,
			cfg.RabbitMQ.PaymentRefundedRoutingKey,
		},
		ConsumerName: cfg.RabbitMQ.PaymentConsumerName,
	})
	defer func() { _ = publisher.Close() }()
	dispatcher, err := platformoutbox.NewDispatcher(db, publisher, "payments.outbox_events", pollInterval)
	if err != nil {
		return err
	}

	serverCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	backgroundCtx, stopBackground := context.WithCancel(serverCtx)
	workers := &sync.WaitGroup{}
	workers.Add(2)
	go func() {
		defer workers.Done()
		dispatcher.Run(backgroundCtx)
	}()
	go func() {
		defer workers.Done()
		runPaymentReconciler(backgroundCtx, service, reconcileInterval, cfg.Payment.ReconcileBatchSize)
	}()
	serverErr := grpcserver.Run(serverCtx, cfg.GRPC.PaymentListenAddress, shutdownTimeout, func(server *grpc.Server) {
		bookstorev1.RegisterPaymentServiceServer(server, handler)
	})
	stopBackground()
	waitCtx, cancelWait := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancelWait()
	if err := lifecycle.WaitGroup(waitCtx, workers); err != nil {
		return errors.Join(serverErr, fmt.Errorf("wait for payment background workers: %w", err))
	}
	return serverErr
}

func newVNPayGateway(cfg config.Config) (*vnpay.Gateway, error) {
	expireAfter, err := time.ParseDuration(cfg.Payment.VNPay.ExpireAfter)
	if err != nil {
		return nil, err
	}
	httpTimeout, err := time.ParseDuration(cfg.Payment.VNPay.HTTPTimeout)
	if err != nil {
		return nil, err
	}
	return vnpay.New(vnpay.Config{
		PayURL: cfg.Payment.VNPay.PayURL, APIURL: cfg.Payment.VNPay.APIURL,
		TMNCode: cfg.Payment.VNPay.TMNCode, HashSecret: cfg.Payment.VNPay.HashSecret,
		ReturnURL: cfg.Payment.VNPay.ReturnURL, ServerIP: cfg.Payment.VNPay.ServerIP,
		TimeZone: cfg.Payment.VNPay.TimeZone, ExpireAfter: expireAfter, HTTPTimeout: httpTimeout,
	})
}

func runPaymentReconciler(ctx context.Context, service *application.Service, interval time.Duration, batchSize int) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			reconcileCtx, cancel := context.WithTimeout(ctx, interval)
			summary, err := service.ReconcilePendingPayments(reconcileCtx, batchSize)
			if err != nil && !errors.Is(err, context.Canceled) {
				slog.WarnContext(reconcileCtx, "payment settlement reconciliation failed", "error", err)
			} else if summary.Checked > 0 {
				slog.InfoContext(reconcileCtx, "payment settlement reconciliation completed",
					"checked", summary.Checked, "updated", summary.Updated, "mismatched", summary.Mismatched)
			}
			cancel()
		}
	}
}
