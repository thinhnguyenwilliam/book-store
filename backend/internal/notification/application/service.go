package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/thinhnguyenwilliam/book-store/backend/internal/messaging"
	"github.com/thinhnguyenwilliam/book-store/backend/internal/notification/domain"
)

const (
	eventPaymentSucceeded   = "payment.succeeded"
	eventPaymentFailed      = "payment.failed"
	eventPaymentRefunded    = "payment.refunded"
	eventChatMessageCreated = "chat.message.created"
)

type Service struct {
	repository  Repository
	resolver    RecipientResolver
	emailSender EmailSender
	pushSender  PushSender
	now         func() time.Time
}

func NewService(repository Repository, resolver RecipientResolver, emailSender EmailSender, pushSender PushSender) *Service {
	return &Service{repository: repository, resolver: resolver, emailSender: emailSender, pushSender: pushSender, now: time.Now}
}

type paymentPayload struct {
	PaymentID  string `json:"payment_id"`
	OrderID    string `json:"order_id"`
	BuyerID    string `json:"buyer_id"`
	Status     string `json:"status"`
	Provider   string `json:"provider"`
	Amount     int64  `json:"amount_cents"`
	Currency   string `json:"currency"`
	OccurredAt string `json:"occurred_at"`
}

type chatMessagePayload struct {
	MessageID      string `json:"message_id"`
	ConversationID string `json:"conversation_id"`
	SenderID       string `json:"sender_id"`
	SenderName     string `json:"sender_name"`
	RecipientID    string `json:"recipient_id"`
	Preview        string `json:"preview"`
}

func (s *Service) HandleEvent(ctx context.Context, eventID, eventType string, payload []byte) error {
	if strings.TrimSpace(eventID) == "" || len(eventID) > 128 || !json.Valid(payload) {
		return domain.ErrInvalidInput
	}
	event, supported, err := s.buildEvent(ctx, eventID, eventType, payload)
	if err != nil || !supported {
		return err
	}
	_, err = s.repository.ProcessEvent(ctx, event)
	return err
}

// ProcessEmails drains the durable email delivery table independently from
// RabbitMQ. An SMTP outage therefore never blocks or hot-loops domain events.
func (s *Service) ProcessEmails(ctx context.Context, limit, maxAttempts int, retryDelay time.Duration) error {
	if limit < 1 || maxAttempts < 1 || retryDelay <= 0 {
		return domain.ErrInvalidInput
	}
	now := s.now().UTC()
	items, err := s.repository.ClaimRetryableEmails(ctx, limit, maxAttempts, now.Add(-retryDelay), now)
	if err != nil {
		return err
	}
	var joined error
	for _, delivery := range items {
		now := s.now().UTC()
		if s.emailSender == nil {
			joined = errors.Join(joined, s.repository.MarkEmailSkipped(ctx, delivery.ID, now))
			continue
		}
		if err := s.emailSender.Send(ctx, *delivery); err != nil {
			markErr := s.repository.MarkEmailFailed(ctx, delivery.ID, safeError(err), s.now().UTC())
			joined = errors.Join(joined, fmt.Errorf("send notification email: %w", err), markErr)
			continue
		}
		joined = errors.Join(joined, s.repository.MarkEmailSent(ctx, delivery.ID, s.now().UTC()))
	}
	return joined
}

// ProcessPushes drains durable FCM deliveries independently from RabbitMQ.
// Provider outages therefore cannot NACK or hot-loop the original domain event.
func (s *Service) ProcessPushes(ctx context.Context, limit, maxAttempts int, retryDelay time.Duration) error {
	if limit < 1 || maxAttempts < 1 || retryDelay <= 0 {
		return domain.ErrInvalidInput
	}
	now := s.now().UTC()
	items, err := s.repository.ClaimRetryablePushes(ctx, limit, maxAttempts, now.Add(-retryDelay), now)
	if err != nil {
		return err
	}
	var joined error
	for _, delivery := range items {
		now := s.now().UTC()
		if s.pushSender == nil {
			joined = errors.Join(joined, s.repository.MarkPushSkipped(ctx, delivery.ID, "FCM disabled", now))
			continue
		}
		providerID, sendErr := s.pushSender.Send(ctx, *delivery)
		if errors.Is(sendErr, domain.ErrPushRegistrationInvalid) {
			joined = errors.Join(joined, s.repository.DisableInstallation(ctx, delivery.InstallationID, safeError(sendErr), now))
			continue
		}
		if sendErr != nil {
			markErr := s.repository.MarkPushFailed(ctx, delivery.ID, safeError(sendErr), now)
			joined = errors.Join(joined, fmt.Errorf("send push notification: %w", sendErr), markErr)
			continue
		}
		joined = errors.Join(joined, s.repository.MarkPushSent(ctx, delivery.ID, providerID, now))
	}
	return joined
}

func (s *Service) buildEvent(
	ctx context.Context,
	eventID, eventType string,
	payload []byte,
) (domain.Event, bool, error) {
	now := s.now().UTC()
	switch eventType {
	case messaging.EventAccountRegistered:
		var item messaging.AccountRegisteredPayload
		if err := json.Unmarshal(payload, &item); err != nil || !validUser(item.UserID) || strings.TrimSpace(item.Email) == "" {
			return domain.Event{}, true, domain.ErrInvalidInput
		}
		recipient := domain.Recipient{UserID: item.UserID, Email: item.Email, DisplayName: item.DisplayName}
		return newEvent(eventID, eventType, payload, recipient,
			"Chào mừng bạn đến Book Store",
			"Tài khoản của bạn đã được tạo thành công.",
			map[string]any{"user_id": item.UserID}, now), true, nil
	case eventPaymentSucceeded, eventPaymentFailed, eventPaymentRefunded:
		var item paymentPayload
		if err := json.Unmarshal(payload, &item); err != nil || !validUser(item.BuyerID) || item.PaymentID == "" || item.OrderID == "" {
			return domain.Event{}, true, domain.ErrInvalidInput
		}
		if s.resolver == nil {
			return domain.Event{}, true, errors.New("notification recipient resolver is not configured")
		}
		recipient, err := s.resolver.Resolve(ctx, item.BuyerID)
		if err != nil {
			return domain.Event{}, true, err
		}
		title, body := paymentMessage(eventType, item)
		return newEvent(eventID, eventType, payload, recipient, title, body, map[string]any{
			"payment_id": item.PaymentID, "order_id": item.OrderID, "status": item.Status,
			"provider": item.Provider, "amount_cents": item.Amount, "currency": item.Currency,
		}, now), true, nil
	case eventChatMessageCreated:
		var item chatMessagePayload
		if err := json.Unmarshal(payload, &item); err != nil || !validUser(item.RecipientID) || item.MessageID == "" || item.ConversationID == "" {
			return domain.Event{}, true, domain.ErrInvalidInput
		}
		if s.resolver == nil {
			return domain.Event{}, true, errors.New("notification recipient resolver is not configured")
		}
		recipient, err := s.resolver.Resolve(ctx, item.RecipientID)
		if err != nil {
			return domain.Event{}, true, err
		}
		title := "Tin nhắn mới từ " + strings.TrimSpace(item.SenderName)
		if strings.TrimSpace(item.SenderName) == "" {
			title = "Bạn có tin nhắn mới"
		}
		event := newEvent(eventID, eventType, payload, recipient, title, item.Preview, map[string]any{
			"message_id": item.MessageID, "conversation_id": item.ConversationID, "sender_id": item.SenderID,
		}, now)
		// Chat is high-frequency. Keep an in-app notification, but do not send
		// one email per message; a digest/push policy can be added separately.
		event.Email = nil
		return event, true, nil
	default:
		return domain.Event{}, false, nil
	}
}

func newEvent(
	eventID, eventType string,
	payload []byte,
	recipient domain.Recipient,
	title, body string,
	data map[string]any,
	now time.Time,
) domain.Event {
	dataJSON, _ := json.Marshal(data)
	notification := domain.Notification{
		ID: uuid.NewString(), UserID: recipient.UserID, EventID: eventID, Type: eventType,
		Title: title, Body: body, Data: dataJSON, CreatedAt: now,
	}
	var email *domain.EmailDelivery
	if strings.TrimSpace(recipient.Email) != "" {
		email = &domain.EmailDelivery{
			ID: uuid.NewString(), EventID: eventID, UserID: recipient.UserID,
			Recipient: strings.ToLower(strings.TrimSpace(recipient.Email)), Subject: title,
			Body:   greeting(recipient.DisplayName) + "\n\n" + body + "\n\nBook Store",
			Status: domain.EmailPending, CreatedAt: now, UpdatedAt: now,
		}
	}
	return domain.Event{
		ID: eventID, Type: eventType, Payload: append([]byte(nil), payload...),
		Notification: notification, Email: email,
		Push: &domain.PushMessage{Title: title, Body: body, Data: dataJSON}, ReceivedAt: now,
	}
}

func paymentMessage(eventType string, item paymentPayload) (string, string) {
	amount := fmt.Sprintf("%d %s", item.Amount, strings.ToUpper(item.Currency))
	switch eventType {
	case eventPaymentSucceeded:
		return "Thanh toán thành công", fmt.Sprintf("Đơn hàng %s đã thanh toán thành công %s.", item.OrderID, amount)
	case eventPaymentRefunded:
		return "Hoàn tiền thành công", fmt.Sprintf("Khoản thanh toán %s của đơn hàng %s đã được hoàn.", item.PaymentID, item.OrderID)
	default:
		return "Thanh toán không thành công", fmt.Sprintf("Thanh toán cho đơn hàng %s chưa thành công. Bạn có thể thử lại.", item.OrderID)
	}
}

func greeting(displayName string) string {
	displayName = strings.TrimSpace(displayName)
	if displayName == "" {
		return "Xin chào bạn,"
	}
	return "Xin chào " + displayName + ","
}

func validUser(value string) bool {
	_, err := uuid.Parse(value)
	return err == nil
}

func safeError(err error) string {
	message := err.Error()
	if len(message) > 1000 {
		return message[:1000]
	}
	return message
}

func (s *Service) List(ctx context.Context, userID, rawCursor string, limit int32) (domain.Page, error) {
	if !validUser(userID) {
		return domain.Page{}, domain.ErrInvalidInput
	}
	if limit < 1 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	cursor, err := decodeCursor(rawCursor)
	if err != nil {
		return domain.Page{}, err
	}
	items, err := s.repository.List(ctx, userID, limit+1, cursor)
	if err != nil {
		return domain.Page{}, err
	}
	hasMore := len(items) > int(limit)
	if hasMore {
		items = items[:limit]
	}
	page := domain.Page{Notifications: items, HasMore: hasMore}
	if hasMore && len(items) > 0 {
		last := items[len(items)-1]
		page.NextCursor, err = encodeCursor(domain.Cursor{CreatedAt: last.CreatedAt, ID: last.ID})
	}
	return page, err
}

func (s *Service) UnreadCount(ctx context.Context, userID string) (int64, error) {
	if !validUser(userID) {
		return 0, domain.ErrInvalidInput
	}
	return s.repository.UnreadCount(ctx, userID)
}

func (s *Service) MarkRead(ctx context.Context, userID, id string) (*domain.Notification, error) {
	if !validUser(userID) || !validUser(id) {
		return nil, domain.ErrInvalidInput
	}
	return s.repository.MarkRead(ctx, userID, id, s.now().UTC())
}

func (s *Service) MarkAllRead(ctx context.Context, userID string) (int64, error) {
	if !validUser(userID) {
		return 0, domain.ErrInvalidInput
	}
	return s.repository.MarkAllRead(ctx, userID, s.now().UTC())
}

func (s *Service) RegisterDevice(ctx context.Context, userID, deviceID, applicationName, platform, token string) (*domain.DeviceInstallation, error) {
	token = strings.TrimSpace(token)
	applicationName = strings.ToLower(strings.TrimSpace(applicationName))
	platform = strings.ToLower(strings.TrimSpace(platform))
	if !validUser(userID) || !validUser(deviceID) || len(token) < 20 || len(token) > 4096 {
		return nil, domain.ErrInvalidInput
	}
	if applicationName != "storefront" && applicationName != "admin" {
		return nil, domain.ErrInvalidInput
	}
	if platform != "web" && platform != "android" && platform != "ios" {
		return nil, domain.ErrInvalidInput
	}
	now := s.now().UTC()
	return s.repository.RegisterDevice(ctx, domain.DeviceInstallation{
		ID: uuid.NewString(), DeviceID: deviceID, UserID: userID, Application: applicationName,
		Platform: platform, RegistrationToken: token, LastSeenAt: now, CreatedAt: now, UpdatedAt: now,
	})
}

func (s *Service) UnregisterDevice(ctx context.Context, userID, deviceID string) (bool, error) {
	if !validUser(userID) || !validUser(deviceID) {
		return false, domain.ErrInvalidInput
	}
	return s.repository.UnregisterDevice(ctx, userID, deviceID, s.now().UTC())
}
