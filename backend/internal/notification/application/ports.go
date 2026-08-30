package application

import (
	"context"
	"time"

	"github.com/thinhnguyenwilliam/book-store/backend/internal/notification/domain"
)

type Repository interface {
	ProcessEvent(ctx context.Context, event domain.Event) (*domain.EmailDelivery, error)
	MarkEmailSent(ctx context.Context, id string, now time.Time) error
	MarkEmailFailed(ctx context.Context, id, reason string, now time.Time) error
	MarkEmailSkipped(ctx context.Context, id string, now time.Time) error
	ClaimRetryableEmails(ctx context.Context, limit, maxAttempts int, retryBefore, now time.Time) ([]*domain.EmailDelivery, error)
	RegisterDevice(ctx context.Context, installation domain.DeviceInstallation) (*domain.DeviceInstallation, error)
	UnregisterDevice(ctx context.Context, userID, deviceID string, now time.Time) (bool, error)
	ClaimRetryablePushes(ctx context.Context, limit, maxAttempts int, retryBefore, now time.Time) ([]*domain.PushDelivery, error)
	MarkPushSent(ctx context.Context, id, providerMessageID string, now time.Time) error
	MarkPushFailed(ctx context.Context, id, reason string, now time.Time) error
	MarkPushSkipped(ctx context.Context, id, reason string, now time.Time) error
	DisableInstallation(ctx context.Context, installationID, reason string, now time.Time) error
	List(ctx context.Context, userID string, limit int32, cursor *domain.Cursor) ([]*domain.Notification, error)
	UnreadCount(ctx context.Context, userID string) (int64, error)
	MarkRead(ctx context.Context, userID, id string, now time.Time) (*domain.Notification, error)
	MarkAllRead(ctx context.Context, userID string, now time.Time) (int64, error)
}

type RecipientResolver interface {
	Resolve(ctx context.Context, userID string) (domain.Recipient, error)
}

type EmailSender interface {
	Send(ctx context.Context, delivery domain.EmailDelivery) error
}

type PushSender interface {
	Send(ctx context.Context, delivery domain.PushDelivery) (providerMessageID string, err error)
}
