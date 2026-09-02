package catalog

import "time"

const (
	SchemaVersion = 1

	EventBookUpserted = "catalog.book.upserted"
	EventBookDeleted  = "catalog.book.deleted"
)

// Event is the versioned integration contract for bookstore.catalog-events.
// BookID is also the Kafka message key, which preserves mutation order for one book.
type Event struct {
	EventID       string    `json:"event_id"`
	EventType     string    `json:"event_type"`
	SchemaVersion int       `json:"schema_version"`
	BookID        string    `json:"book_id"`
	Version       int64     `json:"version"`
	Book          *Book     `json:"book,omitempty"`
	OccurredAt    time.Time `json:"occurred_at"`
	TraceID       string    `json:"trace_id,omitempty"`
}

type Book struct {
	ID         string    `json:"id"`
	Title      string    `json:"title"`
	Author     string    `json:"author"`
	ISBN       string    `json:"isbn"`
	PriceCents int64     `json:"price_cents"`
	Stock      int32     `json:"stock"`
	SellerID   string    `json:"seller_id,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

func KnownEventType(eventType string) bool {
	return eventType == EventBookUpserted || eventType == EventBookDeleted
}
