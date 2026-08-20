package rabbitmq

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

type Handler func(context.Context, []byte) error

type Consumer struct {
	config Config
}

func NewConsumer(config Config) *Consumer {
	return &Consumer{config: config}
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
	defer workers.Wait()

	for {
		select {
		case <-ctx.Done():
			return nil
		case delivery, ok := <-deliveries:
			if !ok {
				return errors.New("RabbitMQ delivery channel closed")
			}
			semaphore <- struct{}{}
			workers.Add(1)
			go func(delivery amqp.Delivery) {
				defer workers.Done()
				defer func() { <-semaphore }()

				if err := handler(ctx, delivery.Body); err != nil {
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
}
