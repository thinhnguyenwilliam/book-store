package fcm

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/thinhnguyenwilliam/book-store/backend/internal/notification/domain"
)

func TestSendWebPush(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		var payload sendRequest
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		if payload.Message.Token != "registration-token-value" || payload.Message.WebPush.FCMOptions.Link != "https://admin.example.com/tro-chuyen" {
			t.Fatalf("unexpected payload: %+v", payload)
		}
		return response(http.StatusOK, `{"name":"projects/book-store/messages/123"}`), nil
	})}

	sender := &Sender{client: client, endpoint: "https://fcm.example.test/messages:send", storefrontURL: "https://store.example.com", adminURL: "https://admin.example.com"}
	providerID, err := sender.Send(context.Background(), domain.PushDelivery{
		NotificationID: "notification-1", NotificationType: "chat.message.created", Application: "admin",
		Platform: "web", RegistrationToken: "registration-token-value", Title: "Tin nhắn mới", Body: "Xin chào",
		Data: json.RawMessage(`{"conversation_id":"conversation-1","unread":2}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	if providerID != "projects/book-store/messages/123" {
		t.Fatalf("provider id = %q", providerID)
	}
}

func TestSendDisablesUnregisteredInstallation(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return response(http.StatusNotFound, `{"error":{"code":404,"status":"NOT_FOUND","details":[{"errorCode":"UNREGISTERED"}]}}`), nil
	})}

	sender := &Sender{client: client, endpoint: "https://fcm.example.test/messages:send"}
	_, err := sender.Send(context.Background(), domain.PushDelivery{RegistrationToken: "registration-token-value", Title: "Title", Body: "Body"})
	if !errors.Is(err, domain.ErrPushRegistrationInvalid) {
		t.Fatalf("Send() error = %v", err)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (function roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}

func response(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}
