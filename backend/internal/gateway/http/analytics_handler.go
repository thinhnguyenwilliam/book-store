package http

import (
	"net/http"

	"github.com/labstack/echo/v4"
	bookstorev1 "github.com/thinhnguyenwilliam/book-store/backend/gen/bookstore/v1"
)

// getOrderAnalytics godoc
// @Summary Get order lifecycle analytics
// @Description Returns the Kafka-backed order read model. Dates accept RFC3339 or YYYY-MM-DD; an omitted range defaults to the last 30 days. Data is eventually consistent.
// @Tags Admin Analytics
// @Produce json
// @Security BearerAuth
// @Param from query string false "Inclusive start"
// @Param to query string false "Inclusive calendar end, or exclusive RFC3339 end"
// @Success 200 {object} OrderAnalyticsResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/admin/analytics/orders [get]
func (h *Handler) getOrderAnalytics(c echo.Context) error {
	response, err := h.analytics.GetOrderAnalytics(grpcContext(c), &bookstorev1.GetOrderAnalyticsRequest{
		From: c.QueryParam("from"), To: c.QueryParam("to"),
	})
	if err != nil {
		return errorResponse(c, err)
	}
	daily := make([]DailyOrderMetricResponse, 0, len(response.GetDaily()))
	for _, item := range response.GetDaily() {
		daily = append(daily, DailyOrderMetricResponse{
			Date: item.GetDate(), Created: item.GetCreated(), Confirmed: item.GetConfirmed(), Cancelled: item.GetCancelled(),
		})
	}
	return c.JSON(http.StatusOK, OrderAnalyticsResponse{
		From: response.GetFrom(), To: response.GetTo(), TotalOrders: response.GetTotalOrders(),
		ConfirmedOrders: response.GetConfirmedOrders(), CancelledOrders: response.GetCancelledOrders(),
		PaymentAttempts: response.GetPaymentAttempts(), PaymentSucceeded: response.GetPaymentSucceeded(),
		PaymentFailed: response.GetPaymentFailed(), StockReservationFailed: response.GetStockReservationFailed(),
		PaymentSuccessRate:         response.GetPaymentSuccessRate(),
		AverageConfirmationSeconds: response.GetAverageConfirmationSeconds(),
		Daily:                      daily, LastEventAt: response.GetLastEventAt(),
	})
}

// getCustomerActivityAnalytics godoc
// @Summary Get customer behavior analytics
// @Description Returns Kafka-backed event counts, conversion funnel, cart abandonment and trending books. Data is eventually consistent.
// @Tags Admin Analytics
// @Produce json
// @Security BearerAuth
// @Param from query string false "Inclusive start"
// @Param to query string false "Inclusive calendar end, or exclusive RFC3339 end"
// @Param limit query int false "Number of top books" default(10) minimum(1) maximum(100)
// @Success 200 {object} CustomerActivityAnalyticsResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Router /api/v1/admin/analytics/customer-activity [get]
func (h *Handler) getCustomerActivityAnalytics(c echo.Context) error {
	response, err := h.analytics.GetCustomerActivityAnalytics(grpcContext(c), &bookstorev1.GetCustomerActivityAnalyticsRequest{
		From: c.QueryParam("from"), To: c.QueryParam("to"), Limit: int32Query(c, "limit", 10),
	})
	if err != nil {
		return errorResponse(c, err)
	}
	counts := make([]EventCountResponse, 0, len(response.GetEventCounts()))
	for _, item := range response.GetEventCounts() {
		counts = append(counts, EventCountResponse{EventType: item.GetEventType(), Count: item.GetCount()})
	}
	return c.JSON(http.StatusOK, CustomerActivityAnalyticsResponse{
		From: response.GetFrom(), To: response.GetTo(), TotalEvents: response.GetTotalEvents(),
		UniqueActors: response.GetUniqueActors(), AbandonedCarts: response.GetAbandonedCarts(),
		ViewToCartRate: response.GetViewToCartRate(), CartToCheckoutRate: response.GetCartToCheckoutRate(),
		CheckoutToOrderRate: response.GetCheckoutToOrderRate(), EventCounts: counts,
		TopBooks: bookActivityMetricsJSON(response.GetTopBooks()), LastEventAt: response.GetLastEventAt(),
	})
}

// getTrendingBooks godoc
// @Summary Get trending book IDs
// @Description Ranks books from recent view, add-to-cart and comment activity. The default range is the last 30 days.
// @Tags Recommendations
// @Produce json
// @Param from query string false "Inclusive start"
// @Param to query string false "Inclusive calendar end, or exclusive RFC3339 end"
// @Param limit query int false "Result size" default(10) minimum(1) maximum(100)
// @Success 200 {object} BookActivityListResponse
// @Router /api/v1/recommendations/trending [get]
func (h *Handler) getTrendingBooks(c echo.Context) error {
	response, err := h.analytics.GetTrendingBooks(grpcContext(c), &bookstorev1.GetTrendingBooksRequest{
		From: c.QueryParam("from"), To: c.QueryParam("to"), Limit: int32Query(c, "limit", 10),
	})
	if err != nil {
		return errorResponse(c, err)
	}
	return c.JSON(http.StatusOK, BookActivityListResponse{Data: bookActivityMetricsJSON(response.GetBooks())})
}

// getRelatedBooks godoc
// @Summary Get behaviorally related book IDs
// @Description Finds books viewed, carted or commented on by actors who also interacted with the requested book.
// @Tags Recommendations
// @Produce json
// @Param id path string true "Book ID" format(uuid)
// @Param from query string false "Inclusive start"
// @Param to query string false "Inclusive calendar end, or exclusive RFC3339 end"
// @Param limit query int false "Result size" default(10) minimum(1) maximum(100)
// @Success 200 {object} RelatedBookListResponse
// @Router /api/v1/books/{id}/related [get]
func (h *Handler) getRelatedBooks(c echo.Context) error {
	response, err := h.analytics.GetRelatedBooks(grpcContext(c), &bookstorev1.GetRelatedBooksRequest{
		BookId: c.Param("id"), From: c.QueryParam("from"), To: c.QueryParam("to"),
		Limit: int32Query(c, "limit", 10),
	})
	if err != nil {
		return errorResponse(c, err)
	}
	items := make([]RelatedBookResponse, 0, len(response.GetBooks()))
	for _, item := range response.GetBooks() {
		items = append(items, RelatedBookResponse{BookID: item.GetBookId(), SharedActors: item.GetSharedActors(), Score: item.GetScore()})
	}
	return c.JSON(http.StatusOK, RelatedBookListResponse{Data: items})
}

func bookActivityMetricsJSON(items []*bookstorev1.BookActivityMetric) []BookActivityMetricResponse {
	result := make([]BookActivityMetricResponse, 0, len(items))
	for _, item := range items {
		result = append(result, BookActivityMetricResponse{
			BookID: item.GetBookId(), Views: item.GetViews(), CartAdds: item.GetCartAdds(),
			Comments: item.GetComments(), Score: item.GetScore(),
		})
	}
	return result
}
