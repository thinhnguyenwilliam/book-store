package rabbitmq

import (
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"
)

type Config struct {
	URL          string
	Exchange     string
	Queue        string
	RoutingKeys  []string
	ConsumerName string
	Concurrency  int
	Prefetch     int
}

func (c Config) deadExchange() string {
	return c.Exchange + ".dead"
}

func (c Config) deadQueue() string {
	return c.Queue + ".dead"
}

func declareTopology(channel *amqp.Channel, cfg Config) error {
	if err := channel.ExchangeDeclare(
		cfg.Exchange,
		"topic",
		true,
		false,
		false,
		false,
		nil,
	); err != nil {
		return fmt.Errorf("declare event exchange: %w", err)
	}

	if err := channel.ExchangeDeclare(
		cfg.deadExchange(),
		"direct",
		true,
		false,
		false,
		false,
		nil,
	); err != nil {
		return fmt.Errorf("declare dead-letter exchange: %w", err)
	}

	if _, err := channel.QueueDeclare(
		cfg.deadQueue(),
		true,
		false,
		false,
		false,
		amqp.Table{"x-queue-type": "quorum"},
	); err != nil {
		return fmt.Errorf("declare dead-letter queue: %w", err)
	}
	if err := channel.QueueBind(
		cfg.deadQueue(),
		cfg.deadQueue(),
		cfg.deadExchange(),
		false,
		nil,
	); err != nil {
		return fmt.Errorf("bind dead-letter queue: %w", err)
	}

	queueArguments := amqp.Table{
		"x-queue-type":              "quorum",
		"x-dead-letter-exchange":    cfg.deadExchange(),
		"x-dead-letter-routing-key": cfg.deadQueue(),
	}
	if _, err := channel.QueueDeclare(
		cfg.Queue,
		true,
		false,
		false,
		false,
		queueArguments,
	); err != nil {
		return fmt.Errorf("declare user profile queue: %w", err)
	}
	for _, routingKey := range cfg.RoutingKeys {
		if err := channel.QueueBind(
			cfg.Queue,
			routingKey,
			cfg.Exchange,
			false,
			nil,
		); err != nil {
			return fmt.Errorf("bind user profile queue to %s: %w", routingKey, err)
		}
	}
	return nil
}
