package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/thinhnguyenwilliam/book-store/backend/internal/notification/domain"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Repository struct{ db *gorm.DB }

func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }

type inboxModel struct {
	EventID         string    `gorm:"primaryKey"`
	EventType       string    `gorm:"not null"`
	Payload         []byte    `gorm:"type:jsonb;not null"`
	ProcessingError string    `gorm:"not null"`
	ReceivedAt      time.Time `gorm:"not null"`
	ProcessedAt     *time.Time
}

type notificationModel struct {
	ID        string `gorm:"type:uuid;primaryKey"`
	UserID    string `gorm:"type:uuid;not null"`
	EventID   string `gorm:"not null"`
	Type      string `gorm:"not null"`
	Title     string `gorm:"not null"`
	Body      string `gorm:"not null"`
	Data      []byte `gorm:"type:jsonb;not null"`
	ReadAt    *time.Time
	CreatedAt time.Time `gorm:"not null"`
}

type emailDeliveryModel struct {
	ID        string `gorm:"type:uuid;primaryKey"`
	EventID   string `gorm:"not null"`
	UserID    string `gorm:"type:uuid;not null"`
	Recipient string `gorm:"not null"`
	Subject   string `gorm:"not null"`
	Body      string `gorm:"not null"`
	Status    string `gorm:"not null"`
	Attempts  int    `gorm:"not null"`
	LastError string `gorm:"not null"`
	SentAt    *time.Time
	CreatedAt time.Time `gorm:"not null"`
	UpdatedAt time.Time `gorm:"not null"`
}

type deviceInstallationModel struct {
	ID                string `gorm:"type:uuid;primaryKey"`
	DeviceID          string `gorm:"type:uuid;not null"`
	UserID            string `gorm:"type:uuid;not null"`
	Application       string `gorm:"not null"`
	Platform          string `gorm:"not null"`
	RegistrationToken string `gorm:"not null"`
	LastSeenAt        time.Time
	DisabledAt        *time.Time
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type pushDeliveryModel struct {
	ID                string `gorm:"type:uuid;primaryKey"`
	EventID           string
	NotificationID    string `gorm:"type:uuid"`
	UserID            string `gorm:"type:uuid"`
	InstallationID    *string
	Application       string `gorm:"->"`
	Platform          string `gorm:"->"`
	RegistrationToken string `gorm:"->"`
	NotificationType  string
	Title             string
	Body              string
	Data              []byte `gorm:"type:jsonb"`
	Status            string
	Attempts          int
	LastError         string
	ProviderMessageID string
	SentAt            *time.Time
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

func (r *Repository) ProcessEvent(ctx context.Context, event domain.Event) (*domain.EmailDelivery, error) {
	var delivery *emailDeliveryModel
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		inbox := inboxModel{EventID: event.ID, EventType: event.Type, Payload: event.Payload, ReceivedAt: event.ReceivedAt}
		created := tx.Table("notifications.inbox_events").Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "event_id"}}, DoNothing: true,
		}).Create(&inbox)
		if created.Error != nil {
			return created.Error
		}
		if created.RowsAffected == 0 {
			var existing emailDeliveryModel
			err := tx.Table("notifications.email_deliveries").Where("event_id = ?", event.ID).First(&existing).Error
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil
			}
			if err != nil {
				return err
			}
			delivery = &existing
			return nil
		}

		notification := notificationRecord(event.Notification)
		if err := tx.Table("notifications.notifications").Create(&notification).Error; err != nil {
			return err
		}
		if event.Email != nil {
			record := emailRecord(*event.Email)
			if err := tx.Table("notifications.email_deliveries").Create(&record).Error; err != nil {
				return err
			}
			delivery = &record
		}
		if event.Push != nil {
			var installations []deviceInstallationModel
			if err := tx.Table("notifications.device_installations").
				Where("user_id = ? AND disabled_at IS NULL", event.Notification.UserID).
				Find(&installations).Error; err != nil {
				return err
			}
			for _, installation := range installations {
				installationID := installation.ID
				push := pushDeliveryModel{
					ID: uuid.NewString(), EventID: event.ID, NotificationID: event.Notification.ID,
					UserID: event.Notification.UserID, InstallationID: &installationID,
					NotificationType: event.Type, Title: event.Push.Title, Body: event.Push.Body,
					Data: event.Push.Data, Status: domain.PushPending, CreatedAt: event.ReceivedAt, UpdatedAt: event.ReceivedAt,
				}
				if err := tx.Table("notifications.push_deliveries").Create(&push).Error; err != nil {
					return err
				}
			}
		}
		return tx.Table("notifications.inbox_events").Where("event_id = ?", event.ID).
			Updates(map[string]any{"processed_at": event.ReceivedAt, "processing_error": ""}).Error
	})
	if err != nil {
		return nil, fmt.Errorf("process notification event: %w", err)
	}
	if delivery == nil {
		return nil, nil
	}
	return emailDomain(*delivery), nil
}

func (r *Repository) RegisterDevice(ctx context.Context, installation domain.DeviceInstallation) (*domain.DeviceInstallation, error) {
	record := deviceRecord(installation)
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Table("notifications.device_installations").
			Where("registration_token = ? AND device_id <> ? AND disabled_at IS NULL", installation.RegistrationToken, installation.DeviceID).
			Updates(map[string]any{"disabled_at": installation.UpdatedAt, "updated_at": installation.UpdatedAt}).Error; err != nil {
			return err
		}
		return tx.Table("notifications.device_installations").Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "device_id"}},
			DoUpdates: clause.Assignments(map[string]any{
				"user_id": installation.UserID, "application": installation.Application,
				"platform": installation.Platform, "registration_token": installation.RegistrationToken,
				"last_seen_at": installation.LastSeenAt, "disabled_at": nil, "updated_at": installation.UpdatedAt,
			}),
		}).Create(&record).Error
	})
	if err != nil {
		return nil, fmt.Errorf("register push device: %w", err)
	}
	var saved deviceInstallationModel
	if err := r.db.WithContext(ctx).Table("notifications.device_installations").Where("device_id = ?", installation.DeviceID).First(&saved).Error; err != nil {
		return nil, fmt.Errorf("reload push device: %w", err)
	}
	return deviceDomain(saved), nil
}

func (r *Repository) UnregisterDevice(ctx context.Context, userID, deviceID string, now time.Time) (bool, error) {
	var removed bool
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var installation deviceInstallationModel
		err := tx.Table("notifications.device_installations").Where("device_id = ? AND user_id = ?", deviceID, userID).First(&installation).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		removed = installation.DisabledAt == nil
		if err := tx.Table("notifications.device_installations").Where("id = ?", installation.ID).
			Updates(map[string]any{"disabled_at": now, "updated_at": now}).Error; err != nil {
			return err
		}
		return tx.Table("notifications.push_deliveries").Where("installation_id = ? AND status IN ?", installation.ID, []string{domain.PushPending, domain.PushFailed, domain.PushSending}).
			Updates(map[string]any{"status": domain.PushSkipped, "last_error": "device unregistered", "updated_at": now}).Error
	})
	if err != nil {
		return false, fmt.Errorf("unregister push device: %w", err)
	}
	return removed, nil
}

func (r *Repository) ClaimRetryablePushes(ctx context.Context, limit, maxAttempts int, retryBefore, now time.Time) ([]*domain.PushDelivery, error) {
	var records []pushDeliveryModel
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Table("notifications.push_deliveries AS pd").
			Select("pd.*, di.registration_token, di.platform, di.application").
			Joins("JOIN notifications.device_installations AS di ON di.id = pd.installation_id AND di.disabled_at IS NULL").
			Clauses(clause.Locking{Strength: "UPDATE", Table: clause.Table{Name: "pd"}, Options: "SKIP LOCKED"}).
			Where("pd.attempts < ? AND ((pd.status = ?) OR (pd.status IN (?, ?) AND pd.updated_at <= ?))", maxAttempts, domain.PushPending, domain.PushFailed, domain.PushSending, retryBefore).
			Order("pd.updated_at ASC, pd.id ASC").Limit(limit).Find(&records).Error; err != nil {
			return err
		}
		if len(records) == 0 {
			return nil
		}
		ids := make([]string, 0, len(records))
		for _, record := range records {
			ids = append(ids, record.ID)
		}
		if err := tx.Table("notifications.push_deliveries").Where("id IN ?", ids).Updates(map[string]any{
			"status": domain.PushSending, "attempts": gorm.Expr("attempts + 1"), "last_error": "", "updated_at": now,
		}).Error; err != nil {
			return err
		}
		for index := range records {
			records[index].Status = domain.PushSending
			records[index].Attempts++
			records[index].UpdatedAt = now
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("claim retryable push deliveries: %w", err)
	}
	items := make([]*domain.PushDelivery, 0, len(records))
	for _, record := range records {
		items = append(items, pushDomain(record))
	}
	return items, nil
}

func (r *Repository) MarkPushSent(ctx context.Context, id, providerMessageID string, now time.Time) error {
	return r.updatePush(ctx, id, map[string]any{"status": domain.PushSent, "provider_message_id": providerMessageID, "last_error": "", "sent_at": now, "updated_at": now})
}

func (r *Repository) MarkPushFailed(ctx context.Context, id, reason string, now time.Time) error {
	return r.updatePush(ctx, id, map[string]any{"status": domain.PushFailed, "last_error": reason, "updated_at": now})
}

func (r *Repository) MarkPushSkipped(ctx context.Context, id, reason string, now time.Time) error {
	return r.updatePush(ctx, id, map[string]any{"status": domain.PushSkipped, "last_error": reason, "updated_at": now})
}

func (r *Repository) DisableInstallation(ctx context.Context, installationID, reason string, now time.Time) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Table("notifications.device_installations").Where("id = ?", installationID).
			Updates(map[string]any{"disabled_at": now, "updated_at": now}).Error; err != nil {
			return err
		}
		return tx.Table("notifications.push_deliveries").Where("installation_id = ? AND status IN ?", installationID, []string{domain.PushPending, domain.PushFailed, domain.PushSending}).
			Updates(map[string]any{"status": domain.PushSkipped, "last_error": reason, "updated_at": now}).Error
	})
}

func (r *Repository) updatePush(ctx context.Context, id string, updates map[string]any) error {
	result := r.db.WithContext(ctx).Table("notifications.push_deliveries").Where("id = ?", id).Updates(updates)
	if result.Error != nil {
		return fmt.Errorf("update push delivery: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return domain.ErrNotificationNotFound
	}
	return nil
}

func (r *Repository) MarkEmailSent(ctx context.Context, id string, now time.Time) error {
	return r.updateEmail(ctx, id, map[string]any{
		"status": domain.EmailSent, "last_error": "", "sent_at": now, "updated_at": now,
	})
}

func (r *Repository) MarkEmailFailed(ctx context.Context, id, reason string, now time.Time) error {
	return r.updateEmail(ctx, id, map[string]any{
		"status": domain.EmailFailed, "last_error": reason, "updated_at": now,
	})
}

func (r *Repository) MarkEmailSkipped(ctx context.Context, id string, now time.Time) error {
	return r.updateEmail(ctx, id, map[string]any{
		"status": domain.EmailSkipped, "last_error": "SMTP disabled", "updated_at": now,
	})
}

func (r *Repository) ClaimRetryableEmails(ctx context.Context, limit, maxAttempts int, retryBefore, now time.Time) ([]*domain.EmailDelivery, error) {
	var records []emailDeliveryModel
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Table("notifications.email_deliveries").Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
			Where("attempts < ? AND ((status = ?) OR (status IN (?, ?) AND updated_at <= ?))", maxAttempts, domain.EmailPending, domain.EmailFailed, domain.EmailSending, retryBefore).
			Order("updated_at ASC, id ASC").Limit(limit).Find(&records).Error; err != nil {
			return err
		}
		if len(records) == 0 {
			return nil
		}
		ids := make([]string, 0, len(records))
		for _, record := range records {
			ids = append(ids, record.ID)
		}
		if err := tx.Table("notifications.email_deliveries").Where("id IN ?", ids).Updates(map[string]any{
			"status": domain.EmailSending, "attempts": gorm.Expr("attempts + 1"), "last_error": "", "updated_at": now,
		}).Error; err != nil {
			return err
		}
		for index := range records {
			records[index].Status = domain.EmailSending
			records[index].Attempts++
			records[index].UpdatedAt = now
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("claim retryable email deliveries: %w", err)
	}
	items := make([]*domain.EmailDelivery, 0, len(records))
	for _, record := range records {
		items = append(items, emailDomain(record))
	}
	return items, nil
}

func (r *Repository) updateEmail(ctx context.Context, id string, updates map[string]any) error {
	result := r.db.WithContext(ctx).Table("notifications.email_deliveries").Where("id = ?", id).Updates(updates)
	if result.Error != nil {
		return fmt.Errorf("update email delivery: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return domain.ErrNotificationNotFound
	}
	return nil
}

func (r *Repository) List(ctx context.Context, userID string, limit int32, cursor *domain.Cursor) ([]*domain.Notification, error) {
	db := r.db.WithContext(ctx).Table("notifications.notifications").
		Where("user_id = ?", userID).Order("created_at DESC, id DESC").Limit(int(limit))
	if cursor != nil {
		db = db.Where("(created_at, id) < (?, ?)", cursor.CreatedAt, cursor.ID)
	}
	var records []notificationModel
	if err := db.Find(&records).Error; err != nil {
		return nil, fmt.Errorf("list notifications: %w", err)
	}
	items := make([]*domain.Notification, 0, len(records))
	for _, record := range records {
		items = append(items, notificationDomain(record))
	}
	return items, nil
}

func (r *Repository) UnreadCount(ctx context.Context, userID string) (int64, error) {
	var count int64
	if err := r.db.WithContext(ctx).Table("notifications.notifications").
		Where("user_id = ? AND read_at IS NULL", userID).Count(&count).Error; err != nil {
		return 0, fmt.Errorf("count unread notifications: %w", err)
	}
	return count, nil
}

func (r *Repository) MarkRead(ctx context.Context, userID, id string, now time.Time) (*domain.Notification, error) {
	result := r.db.WithContext(ctx).Table("notifications.notifications").
		Where("id = ? AND user_id = ? AND read_at IS NULL", id, userID).Update("read_at", now)
	if result.Error != nil {
		return nil, fmt.Errorf("mark notification read: %w", result.Error)
	}
	var record notificationModel
	err := r.db.WithContext(ctx).Table("notifications.notifications").
		Where("id = ? AND user_id = ?", id, userID).First(&record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrNotificationNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("reload notification: %w", err)
	}
	return notificationDomain(record), nil
}

func (r *Repository) MarkAllRead(ctx context.Context, userID string, now time.Time) (int64, error) {
	result := r.db.WithContext(ctx).Table("notifications.notifications").
		Where("user_id = ? AND read_at IS NULL", userID).Update("read_at", now)
	if result.Error != nil {
		return 0, fmt.Errorf("mark all notifications read: %w", result.Error)
	}
	return result.RowsAffected, nil
}

func notificationRecord(item domain.Notification) notificationModel {
	return notificationModel{
		ID: item.ID, UserID: item.UserID, EventID: item.EventID, Type: item.Type,
		Title: item.Title, Body: item.Body, Data: item.Data, CreatedAt: item.CreatedAt,
	}
}

func notificationDomain(record notificationModel) *domain.Notification {
	item := &domain.Notification{
		ID: record.ID, UserID: record.UserID, EventID: record.EventID, Type: record.Type,
		Title: record.Title, Body: record.Body, Data: record.Data, CreatedAt: record.CreatedAt,
	}
	if record.ReadAt != nil {
		item.ReadAt = *record.ReadAt
	}
	return item
}

func emailRecord(item domain.EmailDelivery) emailDeliveryModel {
	return emailDeliveryModel{
		ID: item.ID, EventID: item.EventID, UserID: item.UserID, Recipient: item.Recipient,
		Subject: item.Subject, Body: item.Body, Status: item.Status, Attempts: item.Attempts,
		LastError: item.LastError, CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt,
	}
}

func emailDomain(record emailDeliveryModel) *domain.EmailDelivery {
	item := &domain.EmailDelivery{
		ID: record.ID, EventID: record.EventID, UserID: record.UserID, Recipient: record.Recipient,
		Subject: record.Subject, Body: record.Body, Status: record.Status, Attempts: record.Attempts,
		LastError: record.LastError, CreatedAt: record.CreatedAt, UpdatedAt: record.UpdatedAt,
	}
	if record.SentAt != nil {
		item.SentAt = *record.SentAt
	}
	return item
}

func deviceRecord(item domain.DeviceInstallation) deviceInstallationModel {
	return deviceInstallationModel{
		ID: item.ID, DeviceID: item.DeviceID, UserID: item.UserID, Application: item.Application,
		Platform: item.Platform, RegistrationToken: item.RegistrationToken, LastSeenAt: item.LastSeenAt,
		CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt,
	}
}

func deviceDomain(record deviceInstallationModel) *domain.DeviceInstallation {
	item := &domain.DeviceInstallation{
		ID: record.ID, DeviceID: record.DeviceID, UserID: record.UserID, Application: record.Application,
		Platform: record.Platform, RegistrationToken: record.RegistrationToken, LastSeenAt: record.LastSeenAt,
		CreatedAt: record.CreatedAt, UpdatedAt: record.UpdatedAt,
	}
	if record.DisabledAt != nil {
		item.DisabledAt = *record.DisabledAt
	}
	return item
}

func pushDomain(record pushDeliveryModel) *domain.PushDelivery {
	item := &domain.PushDelivery{
		ID: record.ID, EventID: record.EventID, NotificationID: record.NotificationID, UserID: record.UserID,
		Application: record.Application, Platform: record.Platform, RegistrationToken: record.RegistrationToken,
		NotificationType: record.NotificationType, Title: record.Title, Body: record.Body, Data: record.Data,
		Status: record.Status, Attempts: record.Attempts, LastError: record.LastError,
		ProviderMessageID: record.ProviderMessageID, CreatedAt: record.CreatedAt, UpdatedAt: record.UpdatedAt,
	}
	if record.InstallationID != nil {
		item.InstallationID = *record.InstallationID
	}
	if record.SentAt != nil {
		item.SentAt = *record.SentAt
	}
	return item
}
