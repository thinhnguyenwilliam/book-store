package order

import "time"

const (
	SchemaVersion = 1

	EventCreated             = "order.created"
	EventStockReserved       = "order.stock_reserved"
	EventPaymentPending      = "order.payment_pending"
	EventConfirmed           = "order.confirmed"
	EventCancelled           = "order.cancelled"
	EventCompensationPending = "order.compensation_pending"
)

// Event is the versioned integration contract published to bookstore.order-events.
// It deliberately contains a snapshot so analytics consumers do not call Order Service.
type Event struct {
	EventID        string    `json:"event_id"`
	EventType      string    `json:"event_type"`
	SchemaVersion  int       `json:"schema_version"`
	AggregateType  string    `json:"aggregate_type"`
	AggregateID    string    `json:"aggregate_id"`
	OccurredAt     time.Time `json:"occurred_at"`
	TraceID        string    `json:"trace_id,omitempty"`
	PreviousStatus string    `json:"previous_status,omitempty"`
	FailureStage   string    `json:"failure_stage,omitempty"`
	Order          Snapshot  `json:"order"`
}

type Snapshot struct {
	ID            string `json:"id"`
	UserID        string `json:"user_id"`
	Status        string `json:"status"`
	TotalCents    int64  `json:"total_cents"`
	Currency      string `json:"currency"`
	PaymentID     string `json:"payment_id,omitempty"`
	FailureReason string `json:"failure_reason,omitempty"`
}
