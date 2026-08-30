package application

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/thinhnguyenwilliam/book-store/backend/internal/messaging"
	"github.com/thinhnguyenwilliam/book-store/backend/internal/notification/domain"
)

type repositoryStub struct {
	event         domain.Event
	delivery      *domain.EmailDelivery
	status        string
	retryable     []*domain.EmailDelivery
	pushRetryable []*domain.PushDelivery
	pushStatus    string
	disabled      bool
}

func (r *repositoryStub) ProcessEvent(_ context.Context, event domain.Event) (*domain.EmailDelivery, error) {
	r.event = event
	return r.delivery, nil
}
func (r *repositoryStub) MarkEmailSent(_ context.Context, _ string, _ time.Time) error {
	r.status = domain.EmailSent
	return nil
}
func (r *repositoryStub) MarkEmailFailed(_ context.Context, _ string, _ string, _ time.Time) error {
	r.status = domain.EmailFailed
	return nil
}
func (r *repositoryStub) MarkEmailSkipped(_ context.Context, _ string, _ time.Time) error {
	r.status = domain.EmailSkipped
	return nil
}
func (r *repositoryStub) ClaimRetryableEmails(context.Context, int, int, time.Time, time.Time) ([]*domain.EmailDelivery, error) {
	return r.retryable, nil
}
func (r *repositoryStub) RegisterDevice(_ context.Context, item domain.DeviceInstallation) (*domain.DeviceInstallation, error) {
	return &item, nil
}
func (r *repositoryStub) UnregisterDevice(context.Context, string, string, time.Time) (bool, error) {
	return true, nil
}
func (r *repositoryStub) ClaimRetryablePushes(context.Context, int, int, time.Time, time.Time) ([]*domain.PushDelivery, error) {
	return r.pushRetryable, nil
}
func (r *repositoryStub) MarkPushSent(context.Context, string, string, time.Time) error {
	r.pushStatus = domain.PushSent
	return nil
}
func (r *repositoryStub) MarkPushFailed(context.Context, string, string, time.Time) error {
	r.pushStatus = domain.PushFailed
	return nil
}
func (r *repositoryStub) MarkPushSkipped(context.Context, string, string, time.Time) error {
	r.pushStatus = domain.PushSkipped
	return nil
}
func (r *repositoryStub) DisableInstallation(context.Context, string, string, time.Time) error {
	r.disabled = true
	return nil
}
func (r *repositoryStub) List(context.Context, string, int32, *domain.Cursor) ([]*domain.Notification, error) {
	return nil, nil
}
func (r *repositoryStub) UnreadCount(context.Context, string) (int64, error) { return 0, nil }
func (r *repositoryStub) MarkRead(context.Context, string, string, time.Time) (*domain.Notification, error) {
	return nil, nil
}
func (r *repositoryStub) MarkAllRead(context.Context, string, time.Time) (int64, error) {
	return 0, nil
}

type resolverStub struct{ recipient domain.Recipient }

func (r resolverStub) Resolve(context.Context, string) (domain.Recipient, error) {
	return r.recipient, nil
}

type senderStub struct {
	err   error
	calls int
}

func (s *senderStub) Send(context.Context, domain.EmailDelivery) error { s.calls++; return s.err }

type pushSenderStub struct {
	err   error
	calls int
}

func (s *pushSenderStub) Send(context.Context, domain.PushDelivery) (string, error) {
	s.calls++
	return "projects/book-store/messages/123", s.err
}

func TestHandleAccountRegisteredCreatesNotificationAndEmail(t *testing.T) {
	userID := uuid.NewString()
	payload, _ := json.Marshal(messaging.AccountRegisteredPayload{UserID: userID, Email: "Reader@Example.com", DisplayName: "Thịnh"})
	delivery := &domain.EmailDelivery{ID: uuid.NewString(), Status: domain.EmailPending}
	repository := &repositoryStub{delivery: delivery, retryable: []*domain.EmailDelivery{delivery}}
	sender := &senderStub{}
	service := NewService(repository, nil, sender, nil)
	if err := service.HandleEvent(context.Background(), "event-1", messaging.EventAccountRegistered, payload); err != nil {
		t.Fatal(err)
	}
	if repository.event.Notification.UserID != userID || repository.event.Email == nil {
		t.Fatalf("unexpected event: %+v", repository.event)
	}
	if repository.event.Email.Recipient != "reader@example.com" {
		t.Fatalf("recipient = %q", repository.event.Email.Recipient)
	}
	if err := service.ProcessEmails(context.Background(), 10, 3, time.Second); err != nil {
		t.Fatal(err)
	}
	if sender.calls != 1 || repository.status != domain.EmailSent {
		t.Fatalf("calls=%d status=%q", sender.calls, repository.status)
	}
}

func TestHandlePaymentFailureIsRetried(t *testing.T) {
	payload, _ := json.Marshal(paymentPayload{PaymentID: uuid.NewString(), OrderID: uuid.NewString(), BuyerID: uuid.NewString(), Status: "failed", Amount: 100000, Currency: "VND"})
	delivery := &domain.EmailDelivery{ID: uuid.NewString(), Status: domain.EmailPending}
	repository := &repositoryStub{delivery: delivery, retryable: []*domain.EmailDelivery{delivery}}
	sendErr := errors.New("SMTP unavailable")
	sender := &senderStub{err: sendErr}
	service := NewService(repository, resolverStub{recipient: domain.Recipient{UserID: uuid.NewString(), Email: "reader@example.com"}}, sender, nil)
	if err := service.HandleEvent(context.Background(), "event-2", eventPaymentFailed, payload); err != nil {
		t.Fatal(err)
	}
	err := service.ProcessEmails(context.Background(), 10, 3, time.Second)
	if !errors.Is(err, sendErr) {
		t.Fatalf("HandleEvent() error = %v", err)
	}
	if repository.status != domain.EmailFailed {
		t.Fatalf("status = %q", repository.status)
	}
}

func TestListRejectsInvalidCursor(t *testing.T) {
	service := NewService(&repositoryStub{}, nil, nil, nil)
	if _, err := service.List(context.Background(), uuid.NewString(), "not-a-cursor", 20); !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("List() error = %v", err)
	}
}

func TestHandleChatMessageCreatesInAppNotificationWithoutEmail(t *testing.T) {
	recipientID := uuid.NewString()
	payload, _ := json.Marshal(chatMessagePayload{MessageID: uuid.NewString(), ConversationID: uuid.NewString(), SenderID: uuid.NewString(), SenderName: "Nhân viên", RecipientID: recipientID, Preview: "Xin chào bạn"})
	repository := &repositoryStub{}
	service := NewService(repository, resolverStub{recipient: domain.Recipient{UserID: recipientID, Email: "reader@example.com"}}, nil, nil)
	if err := service.HandleEvent(context.Background(), "chat-event-1", eventChatMessageCreated, payload); err != nil {
		t.Fatal(err)
	}
	if repository.event.Notification.UserID != recipientID || repository.event.Notification.Type != eventChatMessageCreated {
		t.Fatalf("unexpected notification: %+v", repository.event.Notification)
	}
	if repository.event.Email != nil {
		t.Fatal("chat message must not create one email per message")
	}
	if repository.event.Push == nil {
		t.Fatal("chat message must create a push template")
	}
}

func TestProcessPushesMarksDeliverySent(t *testing.T) {
	delivery := &domain.PushDelivery{ID: uuid.NewString(), InstallationID: uuid.NewString()}
	repository := &repositoryStub{pushRetryable: []*domain.PushDelivery{delivery}}
	sender := &pushSenderStub{}
	service := NewService(repository, nil, nil, sender)
	if err := service.ProcessPushes(context.Background(), 10, 3, time.Second); err != nil {
		t.Fatal(err)
	}
	if sender.calls != 1 || repository.pushStatus != domain.PushSent {
		t.Fatalf("calls=%d status=%q", sender.calls, repository.pushStatus)
	}
}

func TestProcessPushesDisablesInvalidRegistration(t *testing.T) {
	delivery := &domain.PushDelivery{ID: uuid.NewString(), InstallationID: uuid.NewString()}
	repository := &repositoryStub{pushRetryable: []*domain.PushDelivery{delivery}}
	sender := &pushSenderStub{err: domain.ErrPushRegistrationInvalid}
	service := NewService(repository, nil, nil, sender)
	if err := service.ProcessPushes(context.Background(), 10, 3, time.Second); err != nil {
		t.Fatal(err)
	}
	if !repository.disabled {
		t.Fatal("invalid registration must disable the installation")
	}
}
