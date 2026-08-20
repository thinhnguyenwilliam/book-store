package rabbitmq

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

type Publisher struct {
	config  Config
	mu      sync.Mutex
	conn    *amqp.Connection
	channel *amqp.Channel
	returns <-chan amqp.Return
}

func NewPublisher(config Config) *Publisher {
	return &Publisher{config: config}
}

func (p *Publisher) Publish(ctx context.Context, eventID, eventType string, payload []byte) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if err := p.ensureChannel(); err != nil {
		return err
	}

	confirmation, err := p.channel.PublishWithDeferredConfirmWithContext(
		ctx,
		p.config.Exchange,
		eventType,
		true,
		false,
		amqp.Publishing{
			Headers:      amqp.Table{"event_id": eventID},
			ContentType:  "application/json",
			DeliveryMode: amqp.Persistent,
			MessageId:    eventID,
			Type:         eventType,
			Timestamp:    time.Now().UTC(),
			Body:         payload,
		},
	)
	if err != nil {
		p.invalidate()
		return fmt.Errorf("publish RabbitMQ event: %w", err)
	}

	acknowledged, err := confirmation.WaitContext(ctx)
	if err != nil {
		p.invalidate()
		return fmt.Errorf("wait for RabbitMQ publisher confirm: %w", err)
	}
	if !acknowledged {
		return errors.New("RabbitMQ negatively acknowledged event")
	}

	select {
	case returned := <-p.returns:
		return fmt.Errorf("RabbitMQ returned unroutable event: %s", returned.ReplyText)
	default:
		return nil
	}
}

func (p *Publisher) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.closeLocked()
}

func (p *Publisher) ensureChannel() error {
	if p.channel != nil && !p.channel.IsClosed() && p.conn != nil && !p.conn.IsClosed() {
		return nil
	}
	p.invalidate()

	connection, err := amqp.DialConfig(p.config.URL, amqp.Config{
		Heartbeat: 10 * time.Second,
		Locale:    "en_US",
		Properties: amqp.Table{
			"connection_name": "bookstore-outbox-publisher",
		},
	})
	if err != nil {
		return fmt.Errorf("connect RabbitMQ publisher: %w", err)
	}

	channel, err := connection.Channel()
	if err != nil {
		_ = connection.Close()
		return fmt.Errorf("open RabbitMQ publisher channel: %w", err)
	}
	if err := declareTopology(channel, p.config); err != nil {
		_ = channel.Close()
		_ = connection.Close()
		return err
	}
	if err := channel.Confirm(false); err != nil {
		_ = channel.Close()
		_ = connection.Close()
		return fmt.Errorf("enable RabbitMQ publisher confirms: %w", err)
	}

	p.conn = connection
	p.channel = channel
	p.returns = channel.NotifyReturn(make(chan amqp.Return, 1))
	return nil
}

func (p *Publisher) invalidate() {
	_ = p.closeLocked()
	p.conn = nil
	p.channel = nil
	p.returns = nil
}

func (p *Publisher) closeLocked() error {
	var result error
	if p.channel != nil && !p.channel.IsClosed() {
		result = p.channel.Close()
	}
	if p.conn != nil && !p.conn.IsClosed() {
		if err := p.conn.Close(); result == nil {
			result = err
		}
	}
	return result
}
