package kafka

import (
	"context"
	"fmt"
	"strings"
	"time"

	apptrace "github.com/thinhnguyenwilliam/book-store/backend/internal/platform/trace"
	"github.com/twmb/franz-go/pkg/kgo"
)

type Publisher struct {
	client *kgo.Client
	topic  string
}

func NewPublisher(brokers []string, clientID, topic string) (*Publisher, error) {
	if len(brokers) == 0 || strings.TrimSpace(clientID) == "" || strings.TrimSpace(topic) == "" {
		return nil, fmt.Errorf("kafka brokers, client ID, and topic are required")
	}
	client, err := kgo.NewClient(
		kgo.SeedBrokers(brokers...),
		kgo.ClientID(clientID),
		kgo.RequiredAcks(kgo.AllISRAcks()),
		kgo.RecordRetries(5),
		kgo.ProduceRequestTimeout(5*time.Second),
	)
	if err != nil {
		return nil, fmt.Errorf("create Kafka producer: %w", err)
	}
	return &Publisher{client: client, topic: topic}, nil
}

func (p *Publisher) Publish(
	ctx context.Context,
	eventID, eventType, aggregateID string,
	payload []byte,
) error {
	if aggregateID == "" {
		return fmt.Errorf("kafka aggregate ID is required")
	}
	record := &kgo.Record{
		Topic: p.topic,
		Key:   []byte(aggregateID), // Same aggregate/actor => same partition => ordered events.
		Value: payload,
		Headers: []kgo.RecordHeader{
			{Key: "event_id", Value: []byte(eventID)},
			{Key: "event_type", Value: []byte(eventType)},
			{Key: "trace_id", Value: []byte(apptrace.IDFromContext(ctx))},
			{Key: "schema_version", Value: []byte("1")},
		},
	}
	if err := p.client.ProduceSync(ctx, record).FirstErr(); err != nil {
		return fmt.Errorf("publish Kafka event: %w", err)
	}
	return nil
}

func (p *Publisher) Close() { p.client.Close() }
