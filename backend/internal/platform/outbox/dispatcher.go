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

type Publisher interface {
	Publish(ctx context.Context, eventID, eventType string, payload []byte) error
}

type event struct {
	ID          string
	EventType   string
	TraceID     string
	Payload     []byte
	Attempts    int
	AvailableAt time.Time
}

type Dispatcher struct {
	db        *gorm.DB
	publisher Publisher
	table     string
	interval  time.Duration
}

func NewDispatcher(db *gorm.DB, publisher Publisher, table string, interval time.Duration) (*Dispatcher, error) {
	if table != "auth.outbox_events" && table != "payments.outbox_events" {
		return nil, fmt.Errorf("unsupported outbox table %q", table)
	}
	return &Dispatcher{db: db, publisher: publisher, table: table, interval: interval}, nil
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
		slog.Error("claim outbox events", "table", d.table, "error", err)
		return
	}
	for _, item := range events {
		if ctx.Err() != nil {
			return
		}
		publishCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
		publishCtx = apptrace.ContextWithID(publishCtx, item.TraceID)
		if err := d.publisher.Publish(publishCtx, item.ID, item.EventType, item.Payload); err != nil {
			d.releaseWithError(publishCtx, item, err)
			cancel()
			continue
		}
		if err := d.markPublished(publishCtx, item.ID); err != nil {
			slog.ErrorContext(publishCtx, "mark outbox event published", "event_id", item.ID, "error", err)
		} else {
			slog.InfoContext(publishCtx, "outbox event published", "event_id", item.ID, "event_type", item.EventType)
		}
		cancel()
	}
}

func (d *Dispatcher) claim(ctx context.Context) ([]event, error) {
	claimed := make([]event, 0, batchSize)
	err := d.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		query := fmt.Sprintf(`
			SELECT id, event_type, trace_id, payload, attempts, available_at
			FROM %s
			WHERE published_at IS NULL
			  AND available_at <= NOW()
			  AND (processing_at IS NULL OR processing_at < NOW() - INTERVAL '1 minute')
			ORDER BY created_at
			FOR UPDATE SKIP LOCKED
			LIMIT ?`, d.table) // #nosec G201 -- table is allowlisted by NewDispatcher.
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
		return tx.Table(d.table).Where("id IN ?", ids).Update("processing_at", time.Now().UTC()).Error
	})
	if err != nil {
		return nil, fmt.Errorf("claim outbox batch: %w", err)
	}
	return claimed, nil
}

func (d *Dispatcher) markPublished(ctx context.Context, id string) error {
	return d.db.WithContext(ctx).Table(d.table).Where("id = ?", id).Updates(map[string]any{
		"published_at": time.Now().UTC(), "processing_at": nil, "last_error": "",
	}).Error
}

func (d *Dispatcher) releaseWithError(ctx context.Context, item event, publishErr error) {
	retryDelay := time.Duration(1<<min(item.Attempts, 6)) * time.Second
	err := d.db.WithContext(ctx).Table(d.table).Where("id = ?", item.ID).Updates(map[string]any{
		"attempts": gorm.Expr("attempts + 1"), "available_at": time.Now().UTC().Add(retryDelay),
		"processing_at": nil, "last_error": publishErr.Error(),
	}).Error
	if err != nil {
		slog.ErrorContext(ctx, "release failed outbox event", "event_id", item.ID, "error", err)
		return
	}
	slog.WarnContext(ctx, "publish outbox event failed; scheduled retry", "event_id", item.ID, "error", publishErr)
}
