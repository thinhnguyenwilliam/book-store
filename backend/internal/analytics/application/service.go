package application

import (
	"context"
	"errors"
	"time"

	analyticsdomain "github.com/thinhnguyenwilliam/book-store/backend/internal/analytics/domain"
	customeractivity "github.com/thinhnguyenwilliam/book-store/backend/internal/events/customeractivity"
	orderevent "github.com/thinhnguyenwilliam/book-store/backend/internal/events/order"
)

var ErrInvalidRange = errors.New("invalid analytics time range")

type Service struct {
	repository Repository
	now        func() time.Time
}

func NewService(repository Repository) *Service {
	return &Service{repository: repository, now: time.Now}
}

func (s *Service) ApplyOrderEvent(ctx context.Context, event orderevent.Event, metadata EventMetadata) error {
	return s.repository.ApplyOrderEvent(ctx, event, metadata)
}

func (s *Service) ApplyCustomerActivity(ctx context.Context, event customeractivity.Event, metadata EventMetadata) error {
	return s.repository.ApplyCustomerActivity(ctx, event, metadata)
}

func (s *Service) CustomerActivityReport(ctx context.Context, from, to time.Time, limit int) (analyticsdomain.CustomerActivityReport, error) {
	from, to, err := s.rangeWithDefault(from, to)
	if err != nil {
		return analyticsdomain.CustomerActivityReport{}, err
	}
	limit = normalizedLimit(limit)
	if limit < 1 || limit > 100 {
		return analyticsdomain.CustomerActivityReport{}, ErrInvalidRange
	}
	return s.repository.CustomerActivityReport(ctx, from, to, limit)
}

func (s *Service) TrendingBooks(ctx context.Context, from, to time.Time, limit int) ([]analyticsdomain.BookActivityMetric, error) {
	limit = normalizedLimit(limit)
	from, to, err := s.rangeWithDefault(from, to)
	if err != nil || limit < 1 || limit > 100 {
		return nil, ErrInvalidRange
	}
	return s.repository.TrendingBooks(ctx, from, to, limit)
}

func (s *Service) RelatedBooks(ctx context.Context, bookID string, from, to time.Time, limit int) ([]analyticsdomain.RelatedBook, error) {
	limit = normalizedLimit(limit)
	from, to, err := s.rangeWithDefault(from, to)
	if err != nil || limit < 1 || limit > 100 || bookID == "" {
		return nil, ErrInvalidRange
	}
	return s.repository.RelatedBooks(ctx, bookID, from, to, limit)
}

func normalizedLimit(limit int) int {
	if limit == 0 {
		return 10
	}
	return limit
}

func (s *Service) rangeWithDefault(from, to time.Time) (time.Time, time.Time, error) {
	if from.IsZero() && to.IsZero() {
		to = s.now().UTC()
		from = to.AddDate(0, 0, -30)
	}
	if from.IsZero() || to.IsZero() || !from.Before(to) || to.Sub(from) > 366*24*time.Hour {
		return time.Time{}, time.Time{}, ErrInvalidRange
	}
	return from.UTC(), to.UTC(), nil
}

func (s *Service) OrderReport(ctx context.Context, from, to time.Time) (analyticsdomain.OrderReport, error) {
	from, to, err := s.rangeWithDefault(from, to)
	if err != nil {
		return analyticsdomain.OrderReport{}, ErrInvalidRange
	}
	return s.repository.OrderReport(ctx, from, to)
}
