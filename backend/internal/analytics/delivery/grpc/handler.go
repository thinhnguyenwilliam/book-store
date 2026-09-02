package grpc

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	bookstorev1 "github.com/thinhnguyenwilliam/book-store/backend/gen/bookstore/v1"
	"github.com/thinhnguyenwilliam/book-store/backend/internal/analytics/application"
	analyticsdomain "github.com/thinhnguyenwilliam/book-store/backend/internal/analytics/domain"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Handler struct {
	bookstorev1.UnimplementedAnalyticsServiceServer
	service *application.Service
}

func NewHandler(service *application.Service) *Handler { return &Handler{service: service} }

func (h *Handler) GetOrderAnalytics(
	ctx context.Context,
	request *bookstorev1.GetOrderAnalyticsRequest,
) (*bookstorev1.OrderAnalytics, error) {
	from, err := parseBoundary(request.GetFrom(), false)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "from must be RFC3339 or YYYY-MM-DD")
	}
	to, err := parseBoundary(request.GetTo(), true)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "to must be RFC3339 or YYYY-MM-DD")
	}
	report, err := h.service.OrderReport(ctx, from, to)
	if err != nil {
		if errors.Is(err, application.ErrInvalidRange) {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
		return nil, status.Error(codes.Internal, "query order analytics failed")
	}
	return reportProto(report), nil
}

func (h *Handler) GetCustomerActivityAnalytics(
	ctx context.Context,
	request *bookstorev1.GetCustomerActivityAnalyticsRequest,
) (*bookstorev1.CustomerActivityAnalytics, error) {
	from, to, err := parseRange(request.GetFrom(), request.GetTo())
	if err != nil {
		return nil, err
	}
	report, err := h.service.CustomerActivityReport(ctx, from, to, int(request.GetLimit()))
	if err != nil {
		return nil, analyticsError(err, "query customer activity analytics failed")
	}
	counts := make([]*bookstorev1.EventCount, 0, len(report.EventCounts))
	for _, item := range report.EventCounts {
		counts = append(counts, &bookstorev1.EventCount{EventType: item.EventType, Count: item.Count})
	}
	lastEventAt := ""
	if report.LastEventAt != nil {
		lastEventAt = report.LastEventAt.Format(time.RFC3339Nano)
	}
	return &bookstorev1.CustomerActivityAnalytics{
		From: report.From.Format(time.RFC3339), To: report.To.Format(time.RFC3339),
		TotalEvents: report.TotalEvents, UniqueActors: report.UniqueActors,
		AbandonedCarts: report.AbandonedCarts, ViewToCartRate: report.ViewToCartRate,
		CartToCheckoutRate: report.CartToCheckoutRate, CheckoutToOrderRate: report.CheckoutToOrderRate,
		EventCounts: counts, TopBooks: bookMetricsProto(report.TopBooks), LastEventAt: lastEventAt,
	}, nil
}

func (h *Handler) GetTrendingBooks(ctx context.Context, request *bookstorev1.GetTrendingBooksRequest) (*bookstorev1.BookActivityList, error) {
	from, to, err := parseRange(request.GetFrom(), request.GetTo())
	if err != nil {
		return nil, err
	}
	items, err := h.service.TrendingBooks(ctx, from, to, int(request.GetLimit()))
	if err != nil {
		return nil, analyticsError(err, "query trending books failed")
	}
	return &bookstorev1.BookActivityList{Books: bookMetricsProto(items)}, nil
}

func (h *Handler) GetRelatedBooks(ctx context.Context, request *bookstorev1.GetRelatedBooksRequest) (*bookstorev1.RelatedBookList, error) {
	if _, err := uuid.Parse(request.GetBookId()); err != nil {
		return nil, status.Error(codes.InvalidArgument, "book_id must be a UUID")
	}
	from, to, err := parseRange(request.GetFrom(), request.GetTo())
	if err != nil {
		return nil, err
	}
	items, err := h.service.RelatedBooks(ctx, request.GetBookId(), from, to, int(request.GetLimit()))
	if err != nil {
		return nil, analyticsError(err, "query related books failed")
	}
	result := make([]*bookstorev1.RelatedBook, 0, len(items))
	for _, item := range items {
		result = append(result, &bookstorev1.RelatedBook{BookId: item.BookID, SharedActors: item.SharedActors, Score: item.Score})
	}
	return &bookstorev1.RelatedBookList{Books: result}, nil
}

func parseRange(fromValue, toValue string) (time.Time, time.Time, error) {
	from, err := parseBoundary(fromValue, false)
	if err != nil {
		return time.Time{}, time.Time{}, status.Error(codes.InvalidArgument, "from must be RFC3339 or YYYY-MM-DD")
	}
	to, err := parseBoundary(toValue, true)
	if err != nil {
		return time.Time{}, time.Time{}, status.Error(codes.InvalidArgument, "to must be RFC3339 or YYYY-MM-DD")
	}
	return from, to, nil
}

func analyticsError(err error, fallback string) error {
	if errors.Is(err, application.ErrInvalidRange) {
		return status.Error(codes.InvalidArgument, err.Error())
	}
	return status.Error(codes.Internal, fallback)
}

func bookMetricsProto(items []analyticsdomain.BookActivityMetric) []*bookstorev1.BookActivityMetric {
	result := make([]*bookstorev1.BookActivityMetric, 0, len(items))
	for _, item := range items {
		result = append(result, &bookstorev1.BookActivityMetric{
			BookId: item.BookID, Views: item.Views, CartAdds: item.CartAdds,
			Comments: item.Comments, Score: item.Score,
		})
	}
	return result
}

func parseBoundary(value string, exclusiveEnd bool) (time.Time, error) {
	if value == "" {
		return time.Time{}, nil
	}
	if parsed, err := time.Parse(time.RFC3339, value); err == nil {
		return parsed, nil
	}
	parsed, err := time.Parse("2006-01-02", value)
	if err != nil {
		return time.Time{}, err
	}
	if exclusiveEnd {
		return parsed.Add(24 * time.Hour), nil
	}
	return parsed, nil
}

func reportProto(report analyticsdomain.OrderReport) *bookstorev1.OrderAnalytics {
	daily := make([]*bookstorev1.DailyOrderMetric, 0, len(report.Daily))
	for _, item := range report.Daily {
		daily = append(daily, &bookstorev1.DailyOrderMetric{
			Date: item.Date.Format("2006-01-02"), Created: item.Created,
			Confirmed: item.Confirmed, Cancelled: item.Cancelled,
		})
	}
	lastEventAt := ""
	if report.LastEventAt != nil {
		lastEventAt = report.LastEventAt.Format(time.RFC3339Nano)
	}
	return &bookstorev1.OrderAnalytics{
		From: report.From.Format(time.RFC3339), To: report.To.Format(time.RFC3339),
		TotalOrders: report.TotalOrders, ConfirmedOrders: report.ConfirmedOrders,
		CancelledOrders: report.CancelledOrders, PaymentAttempts: report.PaymentAttempts,
		PaymentSucceeded: report.PaymentSucceeded, PaymentFailed: report.PaymentFailed,
		StockReservationFailed:     report.StockReservationFailed,
		PaymentSuccessRate:         report.PaymentSuccessRate,
		AverageConfirmationSeconds: report.AverageConfirmationSeconds,
		Daily:                      daily, LastEventAt: lastEventAt,
	}
}
