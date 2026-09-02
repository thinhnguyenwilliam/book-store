package postgres

import (
	"context"
	"fmt"
	"time"

	analyticsapp "github.com/thinhnguyenwilliam/book-store/backend/internal/analytics/application"
	analyticsdomain "github.com/thinhnguyenwilliam/book-store/backend/internal/analytics/domain"
	customeractivity "github.com/thinhnguyenwilliam/book-store/backend/internal/events/customeractivity"
	orderevent "github.com/thinhnguyenwilliam/book-store/backend/internal/events/order"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Repository struct{ db *gorm.DB }

func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }

type inboxModel struct {
	EventID       string    `gorm:"type:uuid;primaryKey"`
	Topic         string    `gorm:"not null"`
	PartitionID   int32     `gorm:"not null"`
	MessageOffset int64     `gorm:"not null"`
	EventType     string    `gorm:"not null"`
	AggregateID   string    `gorm:"type:uuid;not null"`
	ReceivedAt    time.Time `gorm:"not null"`
	ProcessedAt   time.Time `gorm:"not null"`
}

type activityInboxModel struct {
	EventID       string    `gorm:"type:uuid;primaryKey"`
	Topic         string    `gorm:"not null"`
	PartitionID   int32     `gorm:"not null"`
	MessageOffset int64     `gorm:"not null"`
	ReceivedAt    time.Time `gorm:"not null"`
}

// orderReportSummary is a database projection, not a domain entity. Keeping it
// separate prevents GORM from interpreting OrderReport.Daily as an association
// while scanning the aggregate query.
type orderReportSummary struct {
	TotalOrders                int64
	ConfirmedOrders            int64
	CancelledOrders            int64
	PaymentAttempts            int64
	PaymentSucceeded           int64
	PaymentFailed              int64
	StockReservationFailed     int64
	AverageConfirmationSeconds float64
	LastEventAt                *time.Time
}

func (r *Repository) ApplyOrderEvent(
	ctx context.Context,
	event orderevent.Event,
	metadata analyticsapp.EventMetadata,
) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now := time.Now().UTC()
		inbox := inboxModel{
			EventID: event.EventID, Topic: metadata.Topic, PartitionID: metadata.Partition,
			MessageOffset: metadata.Offset, EventType: event.EventType,
			AggregateID: event.AggregateID, ReceivedAt: now, ProcessedAt: now,
		}
		result := tx.Table("analytics.kafka_inbox_events").Clauses(clause.OnConflict{DoNothing: true}).Create(&inbox)
		if result.Error != nil {
			return fmt.Errorf("insert Kafka inbox event: %w", result.Error)
		}
		if result.RowsAffected == 0 {
			return nil
		}

		stockReservedAt, paymentPendingAt, paymentSucceededAt := eventTimes(event)
		paymentFailedAt, confirmedAt, cancelledAt, compensationAt := terminalEventTimes(event)
		query := `
			INSERT INTO analytics.order_lifecycle (
				order_id, user_id, status, total_cents, currency, failure_stage, created_at,
				stock_reserved_at, payment_pending_at, payment_succeeded_at, payment_failed_at,
				confirmed_at, cancelled_at, compensation_pending_at, last_event_id, last_event_at, updated_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT (order_id) DO UPDATE SET
				user_id = CASE WHEN EXCLUDED.last_event_at >= analytics.order_lifecycle.last_event_at THEN EXCLUDED.user_id ELSE analytics.order_lifecycle.user_id END,
				status = CASE WHEN EXCLUDED.last_event_at >= analytics.order_lifecycle.last_event_at THEN EXCLUDED.status ELSE analytics.order_lifecycle.status END,
				total_cents = CASE WHEN EXCLUDED.last_event_at >= analytics.order_lifecycle.last_event_at THEN EXCLUDED.total_cents ELSE analytics.order_lifecycle.total_cents END,
				currency = CASE WHEN EXCLUDED.last_event_at >= analytics.order_lifecycle.last_event_at THEN EXCLUDED.currency ELSE analytics.order_lifecycle.currency END,
				failure_stage = CASE
					WHEN analytics.order_lifecycle.failure_stage <> '' THEN analytics.order_lifecycle.failure_stage
					WHEN EXCLUDED.last_event_at >= analytics.order_lifecycle.last_event_at THEN EXCLUDED.failure_stage
					ELSE analytics.order_lifecycle.failure_stage
				END,
				stock_reserved_at = COALESCE(analytics.order_lifecycle.stock_reserved_at, EXCLUDED.stock_reserved_at),
				payment_pending_at = COALESCE(analytics.order_lifecycle.payment_pending_at, EXCLUDED.payment_pending_at),
				payment_succeeded_at = COALESCE(analytics.order_lifecycle.payment_succeeded_at, EXCLUDED.payment_succeeded_at),
				payment_failed_at = COALESCE(analytics.order_lifecycle.payment_failed_at, EXCLUDED.payment_failed_at),
				confirmed_at = COALESCE(analytics.order_lifecycle.confirmed_at, EXCLUDED.confirmed_at),
				cancelled_at = COALESCE(analytics.order_lifecycle.cancelled_at, EXCLUDED.cancelled_at),
				compensation_pending_at = COALESCE(analytics.order_lifecycle.compensation_pending_at, EXCLUDED.compensation_pending_at),
				last_event_id = CASE WHEN EXCLUDED.last_event_at >= analytics.order_lifecycle.last_event_at THEN EXCLUDED.last_event_id ELSE analytics.order_lifecycle.last_event_id END,
				last_event_at = GREATEST(analytics.order_lifecycle.last_event_at, EXCLUDED.last_event_at),
				updated_at = NOW()`
		if err := tx.Exec(query,
			event.AggregateID, event.Order.UserID, event.Order.Status, event.Order.TotalCents,
			event.Order.Currency, event.FailureStage, event.OccurredAt,
			stockReservedAt, paymentPendingAt, paymentSucceededAt, paymentFailedAt,
			confirmedAt, cancelledAt, compensationAt, event.EventID, event.OccurredAt, now,
		).Error; err != nil {
			return fmt.Errorf("upsert order analytics read model: %w", err)
		}
		return nil
	})
}

func (r *Repository) ApplyCustomerActivity(
	ctx context.Context,
	event customeractivity.Event,
	metadata analyticsapp.EventMetadata,
) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now := time.Now().UTC()
		inbox := activityInboxModel{
			EventID: event.EventID, Topic: metadata.Topic, PartitionID: metadata.Partition,
			MessageOffset: metadata.Offset, ReceivedAt: now,
		}
		result := tx.Table("analytics.customer_activity_inbox").Clauses(clause.OnConflict{DoNothing: true}).Create(&inbox)
		if result.Error != nil {
			return fmt.Errorf("insert customer activity inbox: %w", result.Error)
		}
		if result.RowsAffected == 0 {
			return nil
		}
		if err := tx.Exec(`
			INSERT INTO analytics.customer_activity_events (
				event_id, event_type, schema_version, actor_id, user_id, anonymous_id,
				session_id, book_id, order_id, comment_id, search_query, quantity,
				source, trace_id, occurred_at, received_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT (event_id) DO NOTHING`,
			event.EventID, event.EventType, event.SchemaVersion, event.ActorID,
			nullableUUID(event.UserID), nullableUUID(event.AnonymousID), nullableUUID(event.SessionID),
			nullableUUID(event.BookID), nullableUUID(event.OrderID), nullableUUID(event.CommentID),
			event.Query, event.Quantity, event.Source, event.TraceID, event.OccurredAt, now,
		).Error; err != nil {
			return fmt.Errorf("insert customer activity event: %w", err)
		}
		return nil
	})
}

func (r *Repository) CustomerActivityReport(
	ctx context.Context, from, to time.Time, limit int,
) (analyticsdomain.CustomerActivityReport, error) {
	report := analyticsdomain.CustomerActivityReport{
		From: from, To: to, EventCounts: []analyticsdomain.EventCount{},
		TopBooks: []analyticsdomain.BookActivityMetric{},
	}
	var summary struct {
		TotalEvents  int64
		UniqueActors int64
		LastEventAt  *time.Time
	}
	if err := r.db.WithContext(ctx).Raw(`
		SELECT COUNT(*) AS total_events, COUNT(DISTINCT actor_id) AS unique_actors,
		       MAX(occurred_at) AS last_event_at
		FROM analytics.customer_activity_events
		WHERE occurred_at >= ? AND occurred_at < ?`, from, to).Scan(&summary).Error; err != nil {
		return report, fmt.Errorf("query customer activity summary: %w", err)
	}
	report.TotalEvents, report.UniqueActors, report.LastEventAt = summary.TotalEvents, summary.UniqueActors, summary.LastEventAt
	if err := r.db.WithContext(ctx).Raw(`
		SELECT event_type, COUNT(*) AS count
		FROM analytics.customer_activity_events
		WHERE occurred_at >= ? AND occurred_at < ?
		GROUP BY event_type ORDER BY event_type`, from, to).Scan(&report.EventCounts).Error; err != nil {
		return report, fmt.Errorf("query customer activity counts: %w", err)
	}
	var funnel struct {
		ViewedActors    int64
		CartActors      int64
		CheckoutActors  int64
		ConfirmedActors int64
		AbandonedCarts  int64
	}
	if err := r.db.WithContext(ctx).Raw(`
		WITH actor_steps AS (
			SELECT actor_id,
				BOOL_OR(event_type = 'book.viewed') AS viewed,
				BOOL_OR(event_type = 'book.added_to_cart') AS carted,
				BOOL_OR(event_type = 'checkout.started') AS checkout,
				BOOL_OR(event_type = 'order.confirmed') AS confirmed,
				MAX(occurred_at) FILTER (WHERE event_type = 'book.added_to_cart') AS last_cart_add,
				MAX(occurred_at) FILTER (WHERE event_type = 'checkout.started') AS last_checkout
			FROM analytics.customer_activity_events
			WHERE occurred_at >= ? AND occurred_at < ?
			GROUP BY actor_id
		)
		SELECT COUNT(*) FILTER (WHERE viewed) AS viewed_actors,
			COUNT(*) FILTER (WHERE carted) AS cart_actors,
			COUNT(*) FILTER (WHERE checkout) AS checkout_actors,
			COUNT(*) FILTER (WHERE confirmed) AS confirmed_actors,
			COUNT(*) FILTER (WHERE carted AND (last_checkout IS NULL OR last_checkout < last_cart_add)) AS abandoned_carts
		FROM actor_steps`, from, to).Scan(&funnel).Error; err != nil {
		return report, fmt.Errorf("query customer funnel: %w", err)
	}
	report.AbandonedCarts = funnel.AbandonedCarts
	if funnel.ViewedActors > 0 {
		report.ViewToCartRate = float64(funnel.CartActors) * 100 / float64(funnel.ViewedActors)
	}
	if funnel.CartActors > 0 {
		report.CartToCheckoutRate = float64(funnel.CheckoutActors) * 100 / float64(funnel.CartActors)
	}
	if funnel.CheckoutActors > 0 {
		report.CheckoutToOrderRate = float64(funnel.ConfirmedActors) * 100 / float64(funnel.CheckoutActors)
	}
	books, err := r.TrendingBooks(ctx, from, to, limit)
	if err != nil {
		return report, err
	}
	report.TopBooks = books
	return report, nil
}

func (r *Repository) TrendingBooks(
	ctx context.Context, from, to time.Time, limit int,
) ([]analyticsdomain.BookActivityMetric, error) {
	items := []analyticsdomain.BookActivityMetric{}
	err := r.db.WithContext(ctx).Raw(`
		SELECT book_id::text AS book_id,
			COUNT(*) FILTER (WHERE event_type = 'book.viewed') AS views,
			COUNT(*) FILTER (WHERE event_type = 'book.added_to_cart') AS cart_adds,
			COUNT(*) FILTER (WHERE event_type = 'comment.created') AS comments,
			(COUNT(*) FILTER (WHERE event_type = 'book.viewed') +
			 3 * COUNT(*) FILTER (WHERE event_type = 'book.added_to_cart') +
			 2 * COUNT(*) FILTER (WHERE event_type = 'comment.created'))::double precision AS score
		FROM analytics.customer_activity_events
		WHERE occurred_at >= ? AND occurred_at < ? AND book_id IS NOT NULL
		GROUP BY book_id
		ORDER BY score DESC, views DESC, book_id
		LIMIT ?`, from, to, limit).Scan(&items).Error
	if err != nil {
		return nil, fmt.Errorf("query trending books: %w", err)
	}
	return items, nil
}

func (r *Repository) RelatedBooks(
	ctx context.Context, bookID string, from, to time.Time, limit int,
) ([]analyticsdomain.RelatedBook, error) {
	items := []analyticsdomain.RelatedBook{}
	err := r.db.WithContext(ctx).Raw(`
		WITH target_actors AS (
			SELECT DISTINCT actor_id
			FROM analytics.customer_activity_events
			WHERE book_id = ? AND occurred_at >= ? AND occurred_at < ?
		), candidates AS (
			SELECT event.book_id, event.actor_id,
				MAX(CASE WHEN event.event_type = 'book.added_to_cart' THEN 3.0
				         WHEN event.event_type = 'comment.created' THEN 2.0 ELSE 1.0 END) AS actor_score
			FROM analytics.customer_activity_events event
			JOIN target_actors target ON target.actor_id = event.actor_id
			WHERE event.book_id IS NOT NULL AND event.book_id <> ?
			  AND event.occurred_at >= ? AND event.occurred_at < ?
			GROUP BY event.book_id, event.actor_id
		)
		SELECT book_id::text AS book_id, COUNT(*) AS shared_actors, SUM(actor_score) AS score
		FROM candidates GROUP BY book_id
		ORDER BY score DESC, shared_actors DESC, book_id LIMIT ?`,
		bookID, from, to, bookID, from, to, limit).Scan(&items).Error
	if err != nil {
		return nil, fmt.Errorf("query related books: %w", err)
	}
	return items, nil
}

func nullableUUID(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func (r *Repository) OrderReport(ctx context.Context, from, to time.Time) (analyticsdomain.OrderReport, error) {
	report := analyticsdomain.OrderReport{From: from, To: to, Daily: []analyticsdomain.DailyOrderMetric{}}
	var summary orderReportSummary
	row := r.db.WithContext(ctx).Raw(`
		SELECT
			COUNT(*) AS total_orders,
			COUNT(*) FILTER (WHERE status = 'confirmed') AS confirmed_orders,
			COUNT(*) FILTER (WHERE status = 'cancelled') AS cancelled_orders,
			COUNT(*) FILTER (WHERE payment_pending_at IS NOT NULL) AS payment_attempts,
			COUNT(*) FILTER (WHERE payment_succeeded_at IS NOT NULL) AS payment_succeeded,
			COUNT(*) FILTER (WHERE payment_failed_at IS NOT NULL) AS payment_failed,
			COUNT(*) FILTER (WHERE failure_stage = 'stock_reservation') AS stock_reservation_failed,
			COALESCE(AVG(EXTRACT(EPOCH FROM (confirmed_at - created_at))) FILTER (WHERE confirmed_at IS NOT NULL), 0) AS average_confirmation_seconds,
			MAX(last_event_at) AS last_event_at
		FROM analytics.order_lifecycle
		WHERE created_at >= ? AND created_at < ?`, from, to).Scan(&summary)
	if row.Error != nil {
		return analyticsdomain.OrderReport{}, fmt.Errorf("query order analytics: %w", row.Error)
	}
	report.TotalOrders = summary.TotalOrders
	report.ConfirmedOrders = summary.ConfirmedOrders
	report.CancelledOrders = summary.CancelledOrders
	report.PaymentAttempts = summary.PaymentAttempts
	report.PaymentSucceeded = summary.PaymentSucceeded
	report.PaymentFailed = summary.PaymentFailed
	report.StockReservationFailed = summary.StockReservationFailed
	report.AverageConfirmationSeconds = summary.AverageConfirmationSeconds
	report.LastEventAt = summary.LastEventAt
	if report.PaymentAttempts > 0 {
		report.PaymentSuccessRate = float64(report.PaymentSucceeded) * 100 / float64(report.PaymentAttempts)
	}
	if err := r.db.WithContext(ctx).Raw(`
		SELECT DATE_TRUNC('day', created_at) AS date,
			COUNT(*) AS created,
			COUNT(*) FILTER (WHERE status = 'confirmed') AS confirmed,
			COUNT(*) FILTER (WHERE status = 'cancelled') AS cancelled
		FROM analytics.order_lifecycle
		WHERE created_at >= ? AND created_at < ?
		GROUP BY DATE_TRUNC('day', created_at)
		ORDER BY date`, from, to).Scan(&report.Daily).Error; err != nil {
		return analyticsdomain.OrderReport{}, fmt.Errorf("query daily order analytics: %w", err)
	}
	return report, nil
}

func eventTimes(event orderevent.Event) (stockReserved, paymentPending, paymentSucceeded *time.Time) {
	timestamp := event.OccurredAt
	switch event.EventType {
	case orderevent.EventStockReserved:
		stockReserved = &timestamp
	case orderevent.EventPaymentPending:
		paymentPending = &timestamp
	case orderevent.EventConfirmed, orderevent.EventCompensationPending:
		paymentSucceeded = &timestamp
	}
	return
}

func terminalEventTimes(event orderevent.Event) (paymentFailed, confirmed, cancelled, compensation *time.Time) {
	timestamp := event.OccurredAt
	switch event.EventType {
	case orderevent.EventConfirmed:
		confirmed = &timestamp
	case orderevent.EventCancelled:
		cancelled = &timestamp
		if event.FailureStage == "payment" {
			paymentFailed = &timestamp
		}
	case orderevent.EventCompensationPending:
		compensation = &timestamp
	}
	return
}
