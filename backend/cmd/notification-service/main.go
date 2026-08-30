package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	bookstorev1 "github.com/thinhnguyenwilliam/book-store/backend/gen/bookstore/v1"
	rabbitmqadapter "github.com/thinhnguyenwilliam/book-store/backend/internal/messaging/rabbitmq"
	notificationevents "github.com/thinhnguyenwilliam/book-store/backend/internal/notification/adapter/events"
	notificationfcm "github.com/thinhnguyenwilliam/book-store/backend/internal/notification/adapter/fcm"
	notificationgrpcclient "github.com/thinhnguyenwilliam/book-store/backend/internal/notification/adapter/grpcclient"
	notificationpostgres "github.com/thinhnguyenwilliam/book-store/backend/internal/notification/adapter/postgres"
	notificationsmtp "github.com/thinhnguyenwilliam/book-store/backend/internal/notification/adapter/smtp"
	"github.com/thinhnguyenwilliam/book-store/backend/internal/notification/application"
	notificationgrpc "github.com/thinhnguyenwilliam/book-store/backend/internal/notification/delivery/grpc"
	"github.com/thinhnguyenwilliam/book-store/backend/internal/platform/config"
	"github.com/thinhnguyenwilliam/book-store/backend/internal/platform/database"
	"github.com/thinhnguyenwilliam/book-store/backend/internal/platform/grpcclient"
	"github.com/thinhnguyenwilliam/book-store/backend/internal/platform/grpcserver"
	"github.com/thinhnguyenwilliam/book-store/backend/internal/platform/lifecycle"
	appLogger "github.com/thinhnguyenwilliam/book-store/backend/internal/platform/logger"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() { os.Exit(execute()) }

func execute() int {
	configPath := flag.String("config", "config/config.yml", "path to YAML configuration")
	secretsPath := flag.String("secrets", "", "optional partial secret YAML configuration")
	flag.Parse()
	cfg, err := config.LoadWithOverride(*configPath, *secretsPath)
	if err != nil {
		slog.Error("load notification service config", "error", err)
		return 1
	}
	logManager, err := appLogger.New("notificationservice", cfg.Logging)
	if err != nil {
		slog.Error("initialize notification service logger", "error", err)
		return 1
	}
	slog.SetDefault(logManager.Logger())
	defer func() { _ = logManager.Close() }()
	if err := run(cfg); err != nil {
		slog.Error("notification service stopped", "error", err)
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
	smtpTimeout, err := time.ParseDuration(cfg.Notification.SMTP.Timeout)
	if err != nil {
		return err
	}
	emailPollInterval, err := time.ParseDuration(cfg.Notification.EmailPollInterval)
	if err != nil {
		return err
	}
	emailRetryDelay, err := time.ParseDuration(cfg.Notification.EmailRetryDelay)
	if err != nil {
		return err
	}
	pushPollInterval, err := time.ParseDuration(cfg.Notification.PushPollInterval)
	if err != nil {
		return err
	}
	pushRetryDelay, err := time.ParseDuration(cfg.Notification.PushRetryDelay)
	if err != nil {
		return err
	}
	fcmTimeout, err := time.ParseDuration(cfg.Notification.Firebase.HTTPTimeout)
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
	userConnection, err := grpc.NewClient(cfg.GRPC.UserAddress,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithChainUnaryInterceptor(grpcclient.UnaryDeadlineInterceptor(callTimeout), grpcclient.UnaryLoggingInterceptor),
		grpc.WithChainStreamInterceptor(grpcclient.StreamLoggingInterceptor),
	)
	if err != nil {
		return err
	}
	defer func() { _ = userConnection.Close() }()
	repository := notificationpostgres.NewRepository(db)
	resolver := notificationgrpcclient.NewRecipientResolver(bookstorev1.NewUserServiceClient(userConnection))
	var emailSender application.EmailSender
	if cfg.Notification.EmailEnabled {
		emailSender = notificationsmtp.NewSender(notificationsmtp.Config{Host: cfg.Notification.SMTP.Host, Port: cfg.Notification.SMTP.Port, Username: cfg.Notification.SMTP.Username, Password: cfg.Notification.SMTP.Password, FromAddress: cfg.Notification.SMTP.FromAddress, FromName: cfg.Notification.SMTP.FromName, StartTLS: cfg.Notification.SMTP.StartTLS, Timeout: smtpTimeout})
	}
	var pushSender application.PushSender
	if cfg.Notification.PushEnabled {
		pushSender, err = notificationfcm.NewSender(startupCtx, notificationfcm.Config{
			ProjectID: cfg.Notification.Firebase.ProjectID, CredentialsFile: cfg.Notification.Firebase.CredentialsFile,
			StorefrontURL: cfg.Notification.Firebase.StorefrontURL, AdminURL: cfg.Notification.Firebase.AdminURL, Timeout: fcmTimeout,
		})
		if err != nil {
			return err
		}
	}
	service := application.NewService(repository, resolver, emailSender, pushSender)
	handler := notificationgrpc.NewHandler(service)
	eventHandler := notificationevents.NewHandler(service)
	consumer := rabbitmqadapter.NewConsumer(rabbitmqadapter.Config{
		URL: cfg.RabbitMQ.URL, Exchange: cfg.RabbitMQ.Exchange, Queue: cfg.RabbitMQ.NotificationEventsQueue,
		RoutingKeys:  []string{cfg.RabbitMQ.AccountRegisteredRoutingKey, cfg.RabbitMQ.PaymentSucceededRoutingKey, cfg.RabbitMQ.PaymentFailedRoutingKey, cfg.RabbitMQ.PaymentRefundedRoutingKey, cfg.RabbitMQ.ChatMessageCreatedRoutingKey},
		ConsumerName: cfg.RabbitMQ.NotificationConsumerName, Concurrency: cfg.RabbitMQ.ConsumerConcurrency, Prefetch: cfg.RabbitMQ.Prefetch,
	}, shutdownTimeout)
	serverCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	consumerErr := make(chan error, 1)
	var workers sync.WaitGroup
	workers.Add(3)
	go func() { defer workers.Done(); consumerErr <- consumer.Run(serverCtx, eventHandler.Handle) }()
	go func() {
		defer workers.Done()
		runEmailWorker(serverCtx, service, emailPollInterval, emailRetryDelay, cfg.Notification.EmailBatchSize, cfg.Notification.EmailMaxAttempts)
	}()
	go func() {
		defer workers.Done()
		runPushWorker(serverCtx, service, pushPollInterval, pushRetryDelay, cfg.Notification.PushBatchSize, cfg.Notification.PushMaxAttempts)
	}()
	serverErr := grpcserver.Run(serverCtx, cfg.GRPC.NotificationListenAddress, shutdownTimeout, func(server *grpc.Server) { bookstorev1.RegisterNotificationServiceServer(server, handler) })
	stop()
	waitCtx, cancelWait := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancelWait()
	if waitErr := lifecycle.WaitGroup(waitCtx, &workers); waitErr != nil {
		return errors.Join(serverErr, waitErr)
	}
	err = <-consumerErr
	if errors.Is(err, context.Canceled) {
		err = nil
	}
	return errors.Join(serverErr, err)
}

func runPushWorker(ctx context.Context, service *application.Service, pollInterval, retryDelay time.Duration, batchSize, maxAttempts int) {
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()
	for {
		if err := service.ProcessPushes(ctx, batchSize, maxAttempts, retryDelay); err != nil && !errors.Is(err, context.Canceled) {
			slog.WarnContext(ctx, "process push notifications", "error", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func runEmailWorker(ctx context.Context, service *application.Service, pollInterval, retryDelay time.Duration, batchSize, maxAttempts int) {
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()
	for {
		if err := service.ProcessEmails(ctx, batchSize, maxAttempts, retryDelay); err != nil && !errors.Is(err, context.Canceled) {
			slog.WarnContext(ctx, "process notification emails", "error", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
