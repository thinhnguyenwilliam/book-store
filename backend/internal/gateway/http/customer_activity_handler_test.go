package http

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	customeractivity "github.com/thinhnguyenwilliam/book-store/backend/internal/events/customeractivity"
)

func TestCustomerActivityEventAnonymous(t *testing.T) {
	e := echo.New()
	request := httptest.NewRequestWithContext(context.Background(), "POST", "/api/v1/customer-activity", nil)
	context := e.NewContext(request, httptest.NewRecorder())
	anonymousID, sessionID, bookID := uuid.NewString(), uuid.NewString(), uuid.NewString()
	event, err := customerActivityEvent(context, CustomerActivityRequest{
		EventType: customeractivity.EventBookViewed, AnonymousID: anonymousID,
		SessionID: sessionID, BookID: bookID,
	})
	if err != nil {
		t.Fatalf("customerActivityEvent() error = %v", err)
	}
	if event.ActorID != anonymousID || event.BookID != bookID || event.SchemaVersion != customeractivity.SchemaVersion {
		t.Fatalf("unexpected event: %+v", event)
	}
}

func TestCustomerActivityEventUsesAuthenticatedActor(t *testing.T) {
	e := echo.New()
	echoContext := e.NewContext(httptest.NewRequestWithContext(context.Background(), "POST", "/api/v1/customer-activity", nil), httptest.NewRecorder())
	userID := uuid.NewString()
	echoContext.Set(principalContextKey, Principal{UserID: userID})
	event, err := customerActivityEvent(echoContext, CustomerActivityRequest{
		EventType:   customeractivity.EventCheckoutStarted,
		AnonymousID: uuid.NewString(), SessionID: uuid.NewString(),
	})
	if err != nil {
		t.Fatalf("customerActivityEvent() error = %v", err)
	}
	if event.ActorID != userID || event.UserID != userID {
		t.Fatalf("authenticated actor not applied: %+v", event)
	}
}

func TestCustomerActivityEventRejectsAuthoritativeType(t *testing.T) {
	e := echo.New()
	echoContext := e.NewContext(httptest.NewRequestWithContext(context.Background(), "POST", "/api/v1/customer-activity", nil), httptest.NewRecorder())
	_, err := customerActivityEvent(echoContext, CustomerActivityRequest{
		EventType:   customeractivity.EventOrderConfirmed,
		AnonymousID: uuid.NewString(), SessionID: uuid.NewString(),
	})
	if err == nil {
		t.Fatal("customerActivityEvent() error = nil, want authoritative event rejection")
	}
}
