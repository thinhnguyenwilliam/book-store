package outbox

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	apptrace "github.com/thinhnguyenwilliam/book-store/backend/internal/platform/trace"
	"gorm.io/gorm"
)

const batchSize = 50

type event struct {
	ID          string    `gorm:"column:id"`
	EventType   string    `gorm:"column:event_type"`
	TraceID     string    `gorm:"column:trace_id"`
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
		if ctx.Err() != nil {
			return
		}

		// Once an event is being published, allow its publisher confirm and
		// database bookkeeping to finish even when process shutdown starts.
		publishCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
		publishCtx = apptrace.ContextWithID(publishCtx, event.TraceID)
		err := d.publisher.Publish(publishCtx, event.ID, event.EventType, event.Payload)
		if err != nil {
			d.releaseWithError(publishCtx, event, err)
			cancel()
			continue
		}
		if err := d.markPublished(publishCtx, event.ID); err != nil {
			// The task may be delivered again if this update fails. Consumers must
			// therefore be idempotent; this is normal at-least-once delivery.
			slog.ErrorContext(publishCtx, "mark outbox event published", "event_id", event.ID, "error", err)
		} else {
			slog.InfoContext(publishCtx, "outbox event published", "event_id", event.ID, "event_type", event.EventType)
		}
		cancel()
	}
}

func (d *Dispatcher) claim(ctx context.Context) ([]event, error) {
	claimed := make([]event, 0, batchSize)
	err := d.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		const query = `
			SELECT id, event_type, trace_id, payload, attempts, available_at
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
		slog.ErrorContext(ctx, "release failed outbox event", "event_id", event.ID, "error", err)
		return
	}
	slog.WarnContext(ctx, "publish outbox event failed; scheduled retry", "event_id", event.ID, "error", publishErr)
}
