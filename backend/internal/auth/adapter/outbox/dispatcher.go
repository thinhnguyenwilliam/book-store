package outbox

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"gorm.io/gorm"
)

const batchSize = 50

type event struct {
	ID          string    `gorm:"column:id"`
	EventType   string    `gorm:"column:event_type"`
	Payload     []byte    `gorm:"column:payload"`
	Attempts    int       `gorm:"column:attempts"`
	AvailableAt time.Time `gorm:"column:available_at"`
}

type Dispatcher struct {
	db        *gorm.DB
	publisher Publisher
	interval  time.Duration
}

type Publisher interface {
	Publish(ctx context.Context, eventID, eventType string, payload []byte) error
}

func NewDispatcher(db *gorm.DB, publisher Publisher, interval time.Duration) *Dispatcher {
	return &Dispatcher{db: db, publisher: publisher, interval: interval}
}

func (d *Dispatcher) Run(ctx context.Context) {
	d.dispatch(ctx)
	ticker := time.NewTicker(d.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			d.dispatch(ctx)
		}
	}
}

func (d *Dispatcher) dispatch(ctx context.Context) {
	events, err := d.claim(ctx)
	if err != nil {
		slog.Error("claim outbox events", "error", err)
		return
	}

	for _, event := range events {
		publishCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		err := d.publisher.Publish(publishCtx, event.ID, event.EventType, event.Payload)
		cancel()
		if err != nil {
			d.releaseWithError(ctx, event, err)
			continue
		}
		if err := d.markPublished(ctx, event.ID); err != nil {
			// The task may be delivered again if this update fails. Consumers must
			// therefore be idempotent; this is normal at-least-once delivery.
			slog.Error("mark outbox event published", "event_id", event.ID, "error", err)
		}
	}
}

func (d *Dispatcher) claim(ctx context.Context) ([]event, error) {
	claimed := make([]event, 0, batchSize)
	err := d.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		const query = `
			SELECT id, event_type, payload, attempts, available_at
			FROM auth.outbox_events
			WHERE published_at IS NULL
			  AND available_at <= NOW()
			  AND (processing_at IS NULL OR processing_at < NOW() - INTERVAL '1 minute')
			ORDER BY created_at
			FOR UPDATE SKIP LOCKED
			LIMIT ?`
		if err := tx.Raw(query, batchSize).Scan(&claimed).Error; err != nil {
			return err
		}
		if len(claimed) == 0 {
			return nil
		}

		ids := make([]string, 0, len(claimed))
		for _, item := range claimed {
			ids = append(ids, item.ID)
		}
		return tx.Table("auth.outbox_events").
			Where("id IN ?", ids).
			Update("processing_at", time.Now().UTC()).Error
	})
	if err != nil {
		return nil, fmt.Errorf("claim outbox batch: %w", err)
	}
	return claimed, nil
}

func (d *Dispatcher) markPublished(ctx context.Context, id string) error {
	now := time.Now().UTC()
	return d.db.WithContext(ctx).
		Table("auth.outbox_events").
		Where("id = ?", id).
		Updates(map[string]any{
			"published_at":  now,
			"processing_at": nil,
			"last_error":    "",
		}).Error
}

func (d *Dispatcher) releaseWithError(ctx context.Context, event event, publishErr error) {
	retryDelay := time.Duration(1<<min(event.Attempts, 6)) * time.Second
	err := d.db.WithContext(ctx).
		Table("auth.outbox_events").
		Where("id = ?", event.ID).
		Updates(map[string]any{
			"attempts":      gorm.Expr("attempts + 1"),
			"available_at":  time.Now().UTC().Add(retryDelay),
			"processing_at": nil,
			"last_error":    publishErr.Error(),
		}).Error
	if err != nil {
		slog.Error("release failed outbox event", "event_id", event.ID, "error", err)
		return
	}
	slog.Warn("publish outbox event failed; scheduled retry", "event_id", event.ID, "error", publishErr)
}
