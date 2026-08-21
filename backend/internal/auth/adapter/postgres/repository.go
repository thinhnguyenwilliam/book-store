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
	apptrace "github.com/thinhnguyenwilliam/book-store/backend/internal/platform/trace"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
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

type identityModel struct {
	Provider  string    `gorm:"primaryKey;type:varchar(32)"`
	Subject   string    `gorm:"primaryKey;type:varchar(255)"`
	AccountID string    `gorm:"type:uuid;not null;index"`
	Email     string    `gorm:"not null"`
	CreatedAt time.Time `gorm:"not null"`
}

type outboxModel struct {
	ID           string    `gorm:"type:uuid;primaryKey"`
	AggregateID  string    `gorm:"type:uuid;not null"`
	EventType    string    `gorm:"not null"`
	TraceID      string    `gorm:"type:varchar(32);not null"`
	Payload      []byte    `gorm:"type:jsonb;not null"`
	Attempts     int       `gorm:"not null;default:0"`
	AvailableAt  time.Time `gorm:"not null"`
	ProcessingAt *time.Time
	PublishedAt  *time.Time
	LastError    string
	CreatedAt    time.Time `gorm:"not null"`
}

type refreshSessionModel struct {
	ID           string    `gorm:"type:uuid;primaryKey"`
	AccountID    string    `gorm:"type:uuid;not null;index"`
	FamilyID     string    `gorm:"type:uuid;not null;index"`
	TokenHash    string    `gorm:"type:char(64);not null;uniqueIndex"`
	ExpiresAt    time.Time `gorm:"not null"`
	RevokedAt    *time.Time
	ReplacedByID *string `gorm:"type:uuid"`
	LastUsedAt   *time.Time
	CreatedAt    time.Time `gorm:"not null"`
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Create(
	ctx context.Context,
	account *domain.Account,
	profile application.ProfileRegistration,
	session *domain.RefreshSession,
	identity *domain.Identity,
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
		if identity != nil {
			if err := tx.Table("auth.account_identities").Create(identityRecord(identity)).Error; err != nil {
				return err
			}
		}

		traceID := apptrace.IDFromContext(ctx)
		if traceID == "" {
			traceID, err = apptrace.NewID()
			if err != nil {
				return fmt.Errorf("generate outbox trace ID: %w", err)
			}
		}
		event := outboxModel{
			ID:          uuid.NewString(),
			AggregateID: account.ID,
			EventType:   messaging.EventAccountRegistered,
			TraceID:     traceID,
			Payload:     payload,
			AvailableAt: account.CreatedAt,
			CreatedAt:   account.CreatedAt,
		}
		if err := tx.Table("auth.outbox_events").Create(&event).Error; err != nil {
			return err
		}

		return tx.Table("auth.refresh_sessions").Create(refreshSessionRecord(session)).Error
	})
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return domain.ErrEmailAlreadyExists
	}
	if err != nil {
		return fmt.Errorf("create account with outbox event: %w", err)
	}
	return nil
}

func (r *Repository) FindByIdentity(ctx context.Context, provider, subject string) (*domain.Account, error) {
	var record accountModel
	err := r.db.WithContext(ctx).
		Table("auth.accounts AS accounts").
		Select("accounts.*").
		Joins("JOIN auth.account_identities AS identities ON identities.account_id = accounts.id").
		Where("identities.provider = ? AND identities.subject = ?", provider, subject).
		First(&record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find account by external identity: %w", err)
	}
	return accountFromRecord(record), nil
}

func (r *Repository) LinkIdentity(
	ctx context.Context,
	identity *domain.Identity,
	session *domain.RefreshSession,
) error {
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Table("auth.account_identities").Create(identityRecord(identity)).Error; err != nil {
			return err
		}
		return tx.Table("auth.refresh_sessions").Create(refreshSessionRecord(session)).Error
	})
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return domain.ErrIdentityConflict
	}
	if err != nil {
		return fmt.Errorf("link external identity: %w", err)
	}
	return nil
}

func (r *Repository) CreateRefreshSession(ctx context.Context, session *domain.RefreshSession) error {
	if err := r.db.WithContext(ctx).
		Table("auth.refresh_sessions").
		Create(refreshSessionRecord(session)).Error; err != nil {
		return fmt.Errorf("create refresh session: %w", err)
	}
	return nil
}

func (r *Repository) RotateRefreshSession(
	ctx context.Context,
	tokenHash string,
	replacement *domain.RefreshSession,
	now time.Time,
) (*domain.Account, error) {
	var account *domain.Account
	var sessionError error
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var current refreshSessionModel
		err := tx.Table("auth.refresh_sessions").
			Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("token_hash = ?", tokenHash).
			First(&current).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.ErrInvalidRefreshToken
		}
		if err != nil {
			return err
		}

		if current.RevokedAt != nil {
			if err := tx.Table("auth.refresh_sessions").
				Where("family_id = ? AND revoked_at IS NULL", current.FamilyID).
				Update("revoked_at", now).Error; err != nil {
				return err
			}
			sessionError = domain.ErrRefreshTokenReused
			return nil
		}
		if !current.ExpiresAt.After(now) {
			if err := tx.Table("auth.refresh_sessions").
				Where("id = ?", current.ID).
				Updates(map[string]any{"revoked_at": now, "last_used_at": now}).Error; err != nil {
				return err
			}
			sessionError = domain.ErrInvalidRefreshToken
			return nil
		}

		replacement.AccountID = current.AccountID
		replacement.FamilyID = current.FamilyID
		if err := tx.Table("auth.refresh_sessions").Create(refreshSessionRecord(replacement)).Error; err != nil {
			return err
		}
		if err := tx.Table("auth.refresh_sessions").
			Where("id = ? AND revoked_at IS NULL", current.ID).
			Updates(map[string]any{
				"revoked_at":     now,
				"last_used_at":   now,
				"replaced_by_id": replacement.ID,
			}).Error; err != nil {
			return err
		}

		var record accountModel
		if err := tx.Table("auth.accounts").Where("id = ?", current.AccountID).First(&record).Error; err != nil {
			return err
		}
		account = accountFromRecord(record)
		return nil
	})
	if errors.Is(err, domain.ErrInvalidRefreshToken) {
		return nil, err
	}
	if err != nil {
		return nil, fmt.Errorf("rotate refresh session: %w", err)
	}
	if sessionError != nil {
		return nil, sessionError
	}
	return account, nil
}

func (r *Repository) RevokeRefreshSession(ctx context.Context, tokenHash string, now time.Time) error {
	if tokenHash == "" {
		return nil
	}
	if err := r.db.WithContext(ctx).
		Table("auth.refresh_sessions").
		Where("token_hash = ? AND revoked_at IS NULL", tokenHash).
		Updates(map[string]any{"revoked_at": now, "last_used_at": now}).Error; err != nil {
		return fmt.Errorf("revoke refresh session: %w", err)
	}
	return nil
}

func (r *Repository) Delete(ctx context.Context, id string, deletedAt time.Time) error {
	payload, err := json.Marshal(messaging.AccountDeletedPayload{
		UserID:    id,
		DeletedAt: deletedAt.UTC().Format(time.RFC3339Nano),
	})
	if err != nil {
		return fmt.Errorf("marshal account deleted event: %w", err)
	}

	err = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Table("auth.accounts").Where("id = ?", id).Delete(&accountModel{})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return domain.ErrNotFound
		}

		traceID := apptrace.IDFromContext(ctx)
		if traceID == "" {
			traceID, err = apptrace.NewID()
			if err != nil {
				return fmt.Errorf("generate outbox trace ID: %w", err)
			}
		}
		event := outboxModel{
			ID:          uuid.NewString(),
			AggregateID: id,
			EventType:   messaging.EventAccountDeleted,
			TraceID:     traceID,
			Payload:     payload,
			AvailableAt: deletedAt,
			CreatedAt:   deletedAt,
		}
		return tx.Table("auth.outbox_events").Create(&event).Error
	})
	if errors.Is(err, domain.ErrNotFound) {
		return err
	}
	if err != nil {
		return fmt.Errorf("delete account with outbox event: %w", err)
	}
	return nil
}

func (r *Repository) FindByID(ctx context.Context, id string) (*domain.Account, error) {
	var record accountModel
	err := r.db.WithContext(ctx).
		Table("auth.accounts").
		Where("id = ?", id).
		First(&record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find account by ID: %w", err)
	}
	return accountFromRecord(record), nil
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

	return accountFromRecord(record), nil
}

func accountFromRecord(record accountModel) *domain.Account {
	return &domain.Account{
		ID:           record.ID,
		Email:        record.Email,
		PasswordHash: record.PasswordHash,
		Roles:        []string(record.Roles),
		CreatedAt:    record.CreatedAt,
		UpdatedAt:    record.UpdatedAt,
	}
}

func refreshSessionRecord(session *domain.RefreshSession) refreshSessionModel {
	return refreshSessionModel{
		ID:           session.ID,
		AccountID:    session.AccountID,
		FamilyID:     session.FamilyID,
		TokenHash:    session.TokenHash,
		ExpiresAt:    session.ExpiresAt,
		RevokedAt:    session.RevokedAt,
		ReplacedByID: session.ReplacedByID,
		LastUsedAt:   session.LastUsedAt,
		CreatedAt:    session.CreatedAt,
	}
}

func identityRecord(identity *domain.Identity) identityModel {
	return identityModel{
		Provider:  identity.Provider,
		Subject:   identity.Subject,
		AccountID: identity.AccountID,
		Email:     identity.Email,
		CreatedAt: identity.CreatedAt,
	}
}
