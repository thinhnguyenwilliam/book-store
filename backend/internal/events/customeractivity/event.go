package customeractivity

import "time"

const (
	SchemaVersion = 1

	EventBookViewed          = "book.viewed"
	EventBookSearched        = "book.searched"
	EventBookAddedToCart     = "book.added_to_cart"
	EventBookRemovedFromCart = "book.removed_from_cart"
	EventCheckoutStarted     = "checkout.started"
	EventOrderConfirmed      = "order.confirmed"
	EventCommentCreated      = "comment.created"
)

// Event is the versioned integration contract published to
// bookstore.customer-activity. ActorID is the stable Kafka key so events for
// the same customer/session remain ordered within one partition.
type Event struct {
	EventID       string    `json:"event_id"`
	EventType     string    `json:"event_type"`
	SchemaVersion int       `json:"schema_version"`
	ActorID       string    `json:"actor_id"`
	UserID        string    `json:"user_id,omitempty"`
	AnonymousID   string    `json:"anonymous_id,omitempty"`
	SessionID     string    `json:"session_id,omitempty"`
	BookID        string    `json:"book_id,omitempty"`
	OrderID       string    `json:"order_id,omitempty"`
	CommentID     string    `json:"comment_id,omitempty"`
	Query         string    `json:"query,omitempty"`
	Quantity      int32     `json:"quantity,omitempty"`
	Source        string    `json:"source"`
	OccurredAt    time.Time `json:"occurred_at"`
	TraceID       string    `json:"trace_id,omitempty"`
}

func KnownEventType(eventType string) bool {
	switch eventType {
	case EventBookViewed, EventBookSearched, EventBookAddedToCart,
		EventBookRemovedFromCart, EventCheckoutStarted, EventOrderConfirmed,
		EventCommentCreated:
		return true
	default:
		return false
	}
}

func ClientEventType(eventType string) bool {
	switch eventType {
	case EventBookViewed, EventBookSearched, EventBookAddedToCart,
		EventBookRemovedFromCart, EventCheckoutStarted:
		return true
	default:
		return false
	}
}
