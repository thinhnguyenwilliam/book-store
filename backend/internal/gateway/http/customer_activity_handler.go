package http

import (
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	customeractivity "github.com/thinhnguyenwilliam/book-store/backend/internal/events/customeractivity"
	apptrace "github.com/thinhnguyenwilliam/book-store/backend/internal/platform/trace"
)

type ActivityRecorder interface {
	Record(customeractivity.Event) bool
}

type CustomerActivityRequest struct {
	EventType   string `json:"event_type" enums:"book.viewed,book.searched,book.added_to_cart,book.removed_from_cart,checkout.started"`
	AnonymousID string `json:"anonymous_id,omitempty" format:"uuid"`
	SessionID   string `json:"session_id" format:"uuid"`
	BookID      string `json:"book_id,omitempty" format:"uuid"`
	Query       string `json:"query,omitempty" maxLength:"200"`
	Quantity    int32  `json:"quantity,omitempty" minimum:"1" maximum:"100"`
}

type CustomerActivityAcceptedResponse struct {
	EventID string `json:"event_id" format:"uuid"`
	Status  string `json:"status" example:"accepted"`
}

func (h *Handler) SetActivityRecorder(recorder ActivityRecorder) { h.activity = recorder }

// trackCustomerActivity godoc
// @Summary Track storefront customer activity
// @Description Queues a non-critical browser activity event for Kafka. Authorization is optional; anonymous clients must provide stable anonymous_id and session_id UUIDs. Authoritative order/comment events are generated server-side and cannot be submitted here.
// @Tags Customer Activity
// @Accept json
// @Produce json
// @Param request body CustomerActivityRequest true "Customer activity"
// @Success 202 {object} CustomerActivityAcceptedResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 503 {object} ErrorResponse
// @Router /api/v1/customer-activity [post]
func (h *Handler) trackCustomerActivity(c echo.Context) error {
	if h.activity == nil {
		return c.JSON(http.StatusServiceUnavailable, errorBody("customer activity tracking is unavailable"))
	}
	var request CustomerActivityRequest
	if err := c.Bind(&request); err != nil {
		return errorResponse(c, err)
	}
	event, err := customerActivityEvent(c, request)
	if err != nil {
		return c.JSON(http.StatusBadRequest, errorBody(err.Error()))
	}
	if !h.activity.Record(event) {
		return c.JSON(http.StatusServiceUnavailable, errorBody("customer activity buffer is busy"))
	}
	return c.JSON(http.StatusAccepted, CustomerActivityAcceptedResponse{EventID: event.EventID, Status: "accepted"})
}

func customerActivityEvent(c echo.Context, request CustomerActivityRequest) (customeractivity.Event, error) {
	request.EventType = strings.TrimSpace(request.EventType)
	request.Query = strings.TrimSpace(request.Query)
	if !customeractivity.ClientEventType(request.EventType) {
		return customeractivity.Event{}, errInvalidActivity("unsupported customer activity event type")
	}
	if _, err := uuid.Parse(request.SessionID); err != nil {
		return customeractivity.Event{}, errInvalidActivity("session_id must be a UUID")
	}
	principal := principalFromContext(c)
	if request.AnonymousID != "" {
		if _, err := uuid.Parse(request.AnonymousID); err != nil {
			return customeractivity.Event{}, errInvalidActivity("anonymous_id must be a UUID")
		}
	}
	actorID := principal.UserID
	if actorID == "" {
		if _, err := uuid.Parse(request.AnonymousID); err != nil {
			return customeractivity.Event{}, errInvalidActivity("anonymous_id must be a UUID for anonymous activity")
		}
		actorID = request.AnonymousID
	}
	if err := validateActivityFields(request); err != nil {
		return customeractivity.Event{}, err
	}
	return customeractivity.Event{
		EventID: uuid.NewString(), EventType: request.EventType, SchemaVersion: customeractivity.SchemaVersion,
		ActorID: actorID, UserID: principal.UserID, AnonymousID: request.AnonymousID,
		SessionID: request.SessionID, BookID: request.BookID, Query: request.Query,
		Quantity: request.Quantity, Source: "storefront", OccurredAt: time.Now().UTC(),
		TraceID: apptrace.IDFromContext(c.Request().Context()),
	}, nil
}

func validateActivityFields(request CustomerActivityRequest) error {
	switch request.EventType {
	case customeractivity.EventBookViewed:
		if _, err := uuid.Parse(request.BookID); err != nil {
			return errInvalidActivity("book_id must be a UUID for book.viewed")
		}
	case customeractivity.EventBookSearched:
		if len(request.Query) < 2 || len(request.Query) > 200 {
			return errInvalidActivity("query must contain between 2 and 200 characters")
		}
	case customeractivity.EventBookAddedToCart, customeractivity.EventBookRemovedFromCart:
		if _, err := uuid.Parse(request.BookID); err != nil {
			return errInvalidActivity("book_id must be a UUID for cart activity")
		}
		if request.Quantity < 1 || request.Quantity > 100 {
			return errInvalidActivity("quantity must be between 1 and 100")
		}
	}
	return nil
}

type invalidActivity string

func (e invalidActivity) Error() string       { return string(e) }
func errInvalidActivity(message string) error { return invalidActivity(message) }

func (h *Handler) recordServerActivity(c echo.Context, eventType, bookID, orderID, commentID string, quantity int32) {
	if h.activity == nil {
		return
	}
	principal := principalFromContext(c)
	if principal.UserID == "" {
		return
	}
	event := customeractivity.Event{
		EventID: uuid.NewString(), EventType: eventType, SchemaVersion: customeractivity.SchemaVersion,
		ActorID: principal.UserID, UserID: principal.UserID, BookID: bookID, OrderID: orderID,
		CommentID: commentID, Quantity: quantity, Source: "gateway",
		OccurredAt: time.Now().UTC(), TraceID: apptrace.IDFromContext(c.Request().Context()),
	}
	if !h.activity.Record(event) {
		slog.WarnContext(c.Request().Context(), "server customer activity dropped", "event_type", eventType)
	}
}
