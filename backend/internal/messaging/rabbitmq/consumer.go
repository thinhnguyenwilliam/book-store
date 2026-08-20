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
)

type Handler func(context.Context, []byte) error

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
	defer connection.Close()

	channel, err := connection.Channel()
	if err != nil {
		return fmt.Errorf("open RabbitMQ consumer channel: %w", err)
	}
	defer channel.Close()

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
				if err := handler(handlerCtx, delivery.Body); err != nil {
					slog.Error("process RabbitMQ message", "message_id", delivery.MessageId, "error", err)
					if nackErr := delivery.Nack(false, true); nackErr != nil {
						slog.Error("nack RabbitMQ message", "message_id", delivery.MessageId, "error", nackErr)
					}
					return
				}
				if ackErr := delivery.Ack(false); ackErr != nil {
					slog.Error("ack RabbitMQ message", "message_id", delivery.MessageId, "error", ackErr)
				}
			}(delivery)
		}
	}

	slog.Info("RabbitMQ consumer graceful shutdown started", "timeout", c.shutdownTimeout)
	shutdownCtx, cancel := context.WithTimeout(context.Background(), c.shutdownTimeout)
	defer cancel()
	if err := lifecycle.WaitGroup(shutdownCtx, &workers); err != nil {
		return errors.Join(consumeErr, fmt.Errorf("wait for RabbitMQ workers shutdown: %w", err))
	}
	slog.Info("RabbitMQ consumer graceful shutdown completed")
	return consumeErr
}
