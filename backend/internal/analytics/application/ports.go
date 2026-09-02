package application

import (
	"context"
	"time"

	analyticsdomain "github.com/thinhnguyenwilliam/book-store/backend/internal/analytics/domain"
	customeractivity "github.com/thinhnguyenwilliam/book-store/backend/internal/events/customeractivity"
	orderevent "github.com/thinhnguyenwilliam/book-store/backend/internal/events/order"
)

type EventMetadata struct {
	Topic     string
	Partition int32
	Offset    int64
}

type Repository interface {
	ApplyOrderEvent(context.Context, orderevent.Event, EventMetadata) error
	ApplyCustomerActivity(context.Context, customeractivity.Event, EventMetadata) error
	OrderReport(context.Context, time.Time, time.Time) (analyticsdomain.OrderReport, error)
	CustomerActivityReport(context.Context, time.Time, time.Time, int) (analyticsdomain.CustomerActivityReport, error)
	TrendingBooks(context.Context, time.Time, time.Time, int) ([]analyticsdomain.BookActivityMetric, error)
	RelatedBooks(context.Context, string, time.Time, time.Time, int) ([]analyticsdomain.RelatedBook, error)
}
