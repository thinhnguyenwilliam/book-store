package kafka

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	apptrace "github.com/thinhnguyenwilliam/book-store/backend/internal/platform/trace"
	"github.com/twmb/franz-go/pkg/kgo"
)

type Record struct {
	Topic     string
	Partition int32
	Offset    int64
	Key       []byte
	Value     []byte
	Headers   map[string]string
}

type Handler func(context.Context, Record) error

type Consumer struct {
	client       *kgo.Client
	topic        string
	dlqTopic     string
	maxRetries   int
	retryBackoff time.Duration
}

func NewConsumer(
	brokers []string,
	clientID, group, topic, dlqTopic string,
	maxRetries int,
	retryBackoff time.Duration,
) (*Consumer, error) {
	if len(brokers) == 0 || clientID == "" || group == "" || topic == "" || dlqTopic == "" {
		return nil, fmt.Errorf("complete Kafka consumer configuration is required")
	}
	client, err := kgo.NewClient(
		kgo.SeedBrokers(brokers...),
		kgo.ClientID(clientID),
		kgo.ConsumerGroup(group),
		kgo.ConsumeTopics(topic),
		kgo.DisableAutoCommit(),
		kgo.BlockRebalanceOnPoll(),
		kgo.FetchMaxWait(500*time.Millisecond),
	)
	if err != nil {
		return nil, fmt.Errorf("create Kafka consumer: %w", err)
	}
	return &Consumer{
		client: client, topic: topic, dlqTopic: dlqTopic,
		maxRetries: maxRetries, retryBackoff: retryBackoff,
	}, nil
}

func (c *Consumer) Run(ctx context.Context, handler Handler) error {
	// Always release a partition callback blocked by BlockRebalanceOnPoll,
	// including when PollRecords returns because the shutdown context expired.
	defer c.client.AllowRebalance()
	for ctx.Err() == nil {
		fetches := c.client.PollRecords(ctx, 1)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if fetches.IsClientClosed() {
			return errors.New("kafka consumer closed unexpectedly")
		}
		if errs := fetches.Errors(); len(errs) > 0 {
			for _, fetchErr := range errs {
				slog.WarnContext(ctx, "poll Kafka record failed", "topic", fetchErr.Topic, "partition", fetchErr.Partition, "error", fetchErr.Err)
			}
			continue
		}
		fetches.EachRecord(func(record *kgo.Record) {
			if ctx.Err() != nil {
				return
			}
			mapped := mapRecord(record)
			handlerCtx := apptrace.ContextWithID(ctx, mapped.Headers["trace_id"])
			err := c.handleWithRetry(handlerCtx, handler, mapped)
			if err != nil {
				if dlqErr := c.publishDLQ(handlerCtx, record, err); dlqErr != nil {
					slog.ErrorContext(handlerCtx, "publish Kafka DLQ failed; offset not committed", "error", dlqErr)
					return
				}
				slog.ErrorContext(handlerCtx, "Kafka event moved to DLQ", "event_id", mapped.Headers["event_id"], "error", err)
			}
			if commitErr := c.client.CommitRecords(handlerCtx, record); commitErr != nil && !errors.Is(commitErr, context.Canceled) {
				slog.ErrorContext(handlerCtx, "commit Kafka offset failed", "error", commitErr)
			}
		})
		c.client.AllowRebalance()
	}
	return nil
}

func (c *Consumer) Close() {
	// The consumer uses BlockRebalanceOnPoll so a plain Close can wait forever
	// when shutdown interrupts a poll before Run reaches AllowRebalance.
	c.client.CloseAllowingRebalance()
}

func (c *Consumer) handleWithRetry(ctx context.Context, handler Handler, record Record) error {
	var lastErr error
	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		if attempt > 0 {
			delay := c.retryBackoff * time.Duration(attempt)
			timer := time.NewTimer(delay)
			select {
			case <-ctx.Done():
				timer.Stop()
				return ctx.Err()
			case <-timer.C:
			}
		}
		if err := handler(ctx, record); err != nil {
			lastErr = err
			continue
		}
		return nil
	}
	return lastErr
}

func (c *Consumer) publishDLQ(ctx context.Context, original *kgo.Record, processingErr error) error {
	headers := append([]kgo.RecordHeader{}, original.Headers...)
	headers = append(headers,
		kgo.RecordHeader{Key: "source_topic", Value: []byte(c.topic)},
		kgo.RecordHeader{Key: "processing_error", Value: []byte(processingErr.Error())},
	)
	record := &kgo.Record{Topic: c.dlqTopic, Key: original.Key, Value: original.Value, Headers: headers}
	return c.client.ProduceSync(ctx, record).FirstErr()
}

func mapRecord(record *kgo.Record) Record {
	headers := make(map[string]string, len(record.Headers))
	for _, header := range record.Headers {
		headers[header.Key] = string(header.Value)
	}
	return Record{
		Topic: record.Topic, Partition: record.Partition, Offset: record.Offset,
		Key: record.Key, Value: record.Value, Headers: headers,
	}
}
