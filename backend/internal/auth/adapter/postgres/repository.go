package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/thinhnguyenwilliam/book-store/backend/internal/auth/application"
	"github.com/thinhnguyenwilliam/book-store/backend/internal/auth/domain"
	"github.com/thinhnguyenwilliam/book-store/backend/internal/messaging"
	"gorm.io/gorm"
)

type Repository struct {
	db *gorm.DB
}

type accountModel struct {
	ID           string         `gorm:"type:uuid;primaryKey"`
	Email        string         `gorm:"not null;uniqueIndex"`
	PasswordHash string         `gorm:"not null"`
	Roles        pq.StringArray `gorm:"type:text[];not null"`
	CreatedAt    time.Time      `gorm:"not null"`
	UpdatedAt    time.Time      `gorm:"not null"`
}

type outboxModel struct {
	ID           string    `gorm:"type:uuid;primaryKey"`
	AggregateID  string    `gorm:"type:uuid;not null"`
	EventType    string    `gorm:"not null"`
	Payload      []byte    `gorm:"type:jsonb;not null"`
	Attempts     int       `gorm:"not null;default:0"`
	AvailableAt  time.Time `gorm:"not null"`
	ProcessingAt *time.Time
	PublishedAt  *time.Time
	LastError    string
	CreatedAt    time.Time `gorm:"not null"`
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Create(
	ctx context.Context,
	account *domain.Account,
	profile application.ProfileRegistration,
) error {
	payload, err := json.Marshal(messaging.AccountRegisteredPayload{
		UserID:      account.ID,
		Email:       account.Email,
		DisplayName: profile.DisplayName,
	})
	if err != nil {
		return fmt.Errorf("marshal account registered event: %w", err)
	}

	err = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		record := accountModel{
			ID:           account.ID,
			Email:        account.Email,
			PasswordHash: account.PasswordHash,
			Roles:        pq.StringArray(account.Roles),
			CreatedAt:    account.CreatedAt,
			UpdatedAt:    account.UpdatedAt,
		}
		if err := tx.Table("auth.accounts").Create(&record).Error; err != nil {
			return err
		}

		event := outboxModel{
			ID:          uuid.NewString(),
			AggregateID: account.ID,
			EventType:   messaging.EventAccountRegistered,
			Payload:     payload,
			AvailableAt: account.CreatedAt,
			CreatedAt:   account.CreatedAt,
		}
		return tx.Table("auth.outbox_events").Create(&event).Error
	})
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return domain.ErrEmailAlreadyExists
	}
	if err != nil {
		return fmt.Errorf("create account with outbox event: %w", err)
	}
	return nil
}

func (r *Repository) FindByEmail(ctx context.Context, email string) (*domain.Account, error) {
	var record accountModel
	err := r.db.WithContext(ctx).
		Table("auth.accounts").
		Where("email = ?", email).
		First(&record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrInvalidCredentials
	}
	if err != nil {
		return nil, fmt.Errorf("find account by email: %w", err)
	}

	return &domain.Account{
		ID:           record.ID,
		Email:        record.Email,
		PasswordHash: record.PasswordHash,
		Roles:        []string(record.Roles),
		CreatedAt:    record.CreatedAt,
		UpdatedAt:    record.UpdatedAt,
	}, nil
}
