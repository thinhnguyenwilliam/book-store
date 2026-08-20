package rabbitmq

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/thinhnguyenwilliam/book-store/backend/internal/platform/lifecycle"
	apptrace "github.com/thinhnguyenwilliam/book-store/backend/internal/platform/trace"
)

type Handler func(context.Context, string, []byte) error

const maxDeliveryAttempts = 20

type Consumer struct {
	config          Config
	shutdownTimeout time.Duration
}

func NewConsumer(config Config, shutdownTimeout time.Duration) *Consumer {
	return &Consumer{config: config, shutdownTimeout: shutdownTimeout}
}

func (c *Consumer) Run(ctx context.Context, handler Handler) error {
	connection, err := amqp.DialConfig(c.config.URL, amqp.Config{
		Heartbeat: 10 * time.Second,
		Locale:    "en_US",
		Properties: amqp.Table{
			"connection_name": c.config.ConsumerName,
		},
	})
	if err != nil {
		return fmt.Errorf("connect RabbitMQ consumer: %w", err)
	}
	defer func() { _ = connection.Close() }()

	channel, err := connection.Channel()
	if err != nil {
		return fmt.Errorf("open RabbitMQ consumer channel: %w", err)
	}
	defer func() { _ = channel.Close() }()

	if err := declareTopology(channel, c.config); err != nil {
		return err
	}
	if err := channel.Qos(c.config.Prefetch, 0, false); err != nil {
		return fmt.Errorf("configure RabbitMQ prefetch: %w", err)
	}

	deliveries, err := channel.ConsumeWithContext(
		ctx,
		c.config.Queue,
		c.config.ConsumerName,
		false,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return fmt.Errorf("start RabbitMQ consumer: %w", err)
	}

	semaphore := make(chan struct{}, c.config.Concurrency)
	var workers sync.WaitGroup

	var consumeErr error
consumeLoop:
	for {
		select {
		case <-ctx.Done():
			break consumeLoop
		case delivery, ok := <-deliveries:
			if !ok {
				if ctx.Err() == nil {
					consumeErr = errors.New("RabbitMQ delivery channel closed")
				}
				break consumeLoop
			}
			select {
			case semaphore <- struct{}{}:
			case <-ctx.Done():
				break consumeLoop
			}
			workers.Add(1)
			go func(delivery amqp.Delivery) {
				defer workers.Done()
				defer func() { <-semaphore }()

				handlerCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), c.shutdownTimeout)
				defer cancel()
				handlerCtx = contextWithDeliveryTraceID(handlerCtx, delivery)
				if err := handler(handlerCtx, delivery.Type, delivery.Body); err != nil {
					attempt := deliveryAttempt(delivery)
					requeue := attempt < maxDeliveryAttempts
					retryIn := retryDelay(attempt)
					slog.ErrorContext(handlerCtx, "process RabbitMQ message",
						"message_id", delivery.MessageId,
						"delivery_attempt", attempt,
						"retry_in", retryIn,
						"dead_letter", !requeue,
						"error", err,
					)
					if requeue {
						waitForRetry(handlerCtx, retryIn)
					}
					if nackErr := delivery.Nack(false, requeue); nackErr != nil {
						slog.ErrorContext(handlerCtx, "nack RabbitMQ message", "message_id", delivery.MessageId, "error", nackErr)
					}
					return
				}
				if ackErr := delivery.Ack(false); ackErr != nil {
					slog.ErrorContext(handlerCtx, "ack RabbitMQ message", "message_id", delivery.MessageId, "error", ackErr)
					return
				}
				slog.InfoContext(handlerCtx, "RabbitMQ message processed", "message_id", delivery.MessageId, "event_type", delivery.Type)
			}(delivery)
		}
	}

	slog.Info("RabbitMQ consumer graceful shutdown started", "timeout", c.shutdownTimeout)
	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), c.shutdownTimeout)
	defer cancel()
	if err := lifecycle.WaitGroup(shutdownCtx, &workers); err != nil {
		return errors.Join(consumeErr, fmt.Errorf("wait for RabbitMQ workers shutdown: %w", err))
	}
	slog.Info("RabbitMQ consumer graceful shutdown completed")
	return consumeErr
}

func deliveryAttempt(delivery amqp.Delivery) int {
	value := delivery.Headers["x-delivery-count"]
	count := 0
	switch typed := value.(type) {
	case int64:
		count = int(typed)
	case int32:
		count = int(typed)
	case int:
		count = typed
	}
	// RabbitMQ's x-delivery-count starts at zero. Logs and retry policy use a
	// human-friendly one-based attempt number.
	return count + 1
}

func retryDelay(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	delay := 500 * time.Millisecond * time.Duration(1<<min(attempt-1, 4))
	return min(delay, 5*time.Second)
}

func waitForRetry(ctx context.Context, delay time.Duration) {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
	case <-timer.C:
	}
}

func contextWithDeliveryTraceID(ctx context.Context, delivery amqp.Delivery) context.Context {
	if value, ok := delivery.Headers["trace_id"].(string); ok {
		if traceID := apptrace.Normalize(value); traceID != "" {
			return apptrace.ContextWithID(ctx, traceID)
		}
	}
	traceID, err := apptrace.NewID()
	if err != nil {
		return ctx
	}
	return apptrace.ContextWithID(ctx, traceID)
}
