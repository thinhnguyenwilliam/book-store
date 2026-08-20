package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/thinhnguyenwilliam/book-store/backend/internal/user/application"
	"github.com/thinhnguyenwilliam/book-store/backend/internal/user/domain"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Repository struct {
	db *gorm.DB
}

type userModel struct {
	ID          string    `gorm:"type:uuid;primaryKey"`
	Email       string    `gorm:"not null;uniqueIndex"`
	DisplayName string    `gorm:"not null"`
	CreatedAt   time.Time `gorm:"not null"`
	UpdatedAt   time.Time `gorm:"not null"`
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Create(ctx context.Context, user *domain.User) error {
	record := toModel(user)
	result := r.db.WithContext(ctx).
		Table("users.user_profiles").
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "id"}},
			DoNothing: true,
		}).
		Create(&record)
	if errors.Is(result.Error, gorm.ErrDuplicatedKey) {
		return domain.ErrAlreadyExists
	}
	if result.Error != nil {
		return fmt.Errorf("insert user profile: %w", result.Error)
	}

	// RabbitMQ provides at-least-once delivery. Receiving the same user ID again
	// is therefore a successful no-op, making this consumer idempotent.
	if result.RowsAffected == 0 {
		existing, err := r.FindByID(ctx, user.ID)
		if err != nil {
			return err
		}
		if existing.Email != user.Email {
			return domain.ErrAlreadyExists
		}
	}
	return nil
}

func (r *Repository) FindByID(ctx context.Context, id string) (*domain.User, error) {
	var record userModel
	err := r.db.WithContext(ctx).
		Table("users.user_profiles").
		Where("id = ?", id).
		First(&record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find user profile: %w", err)
	}
	return toDomain(record), nil
}

func (r *Repository) List(ctx context.Context, limit int32, cursor *application.UserCursor) ([]*domain.User, error) {
	db := r.db.WithContext(ctx).
		Table("users.user_profiles").
		Order("created_at DESC").
		Order("id DESC").
		Limit(int(limit))
	if cursor != nil {
		db = db.Where("(created_at, id) < (?, ?)", cursor.CreatedAt, cursor.ID)
	}

	var records []userModel
	if err := db.Find(&records).Error; err != nil {
		return nil, fmt.Errorf("list user profiles: %w", err)
	}
	users := make([]*domain.User, 0, len(records))
	for _, record := range records {
		users = append(users, toDomain(record))
	}
	return users, nil
}

func (r *Repository) Update(ctx context.Context, user *domain.User) error {
	result := r.db.WithContext(ctx).
		Table("users.user_profiles").
		Where("id = ?", user.ID).
		Updates(map[string]any{
			"display_name": user.DisplayName,
			"updated_at":   user.UpdatedAt,
		})
	if result.Error != nil {
		return fmt.Errorf("update user profile: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *Repository) Delete(ctx context.Context, id string) error {
	if err := r.db.WithContext(ctx).
		Table("users.user_profiles").
		Where("id = ?", id).
		Delete(&userModel{}).Error; err != nil {
		return fmt.Errorf("delete user profile: %w", err)
	}
	// Deletion events can be delivered more than once. A profile that no longer
	// exists is already in the desired state, so this operation is idempotent.
	return nil
}

func toModel(user *domain.User) userModel {
	return userModel{
		ID:          user.ID,
		Email:       user.Email,
		DisplayName: user.DisplayName,
		CreatedAt:   user.CreatedAt,
		UpdatedAt:   user.UpdatedAt,
	}
}

func toDomain(record userModel) *domain.User {
	return &domain.User{
		ID:          record.ID,
		Email:       record.Email,
		DisplayName: record.DisplayName,
		CreatedAt:   record.CreatedAt,
		UpdatedAt:   record.UpdatedAt,
	}
}
