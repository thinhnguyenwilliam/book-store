package postgres

import (
	"context"
	"time"

	"github.com/thinhnguyenwilliam/book-store/backend/internal/auth/application"
	"github.com/thinhnguyenwilliam/book-store/backend/internal/auth/domain"
	"gorm.io/gorm"
)

// Persistence model stays separate from application and protobuf models.
type oauthTransactionModel struct {
	StateHash     string
	Provider      string
	RedirectURI   string
	Verifier      string
	CreateAccount bool
	ExpiresAt     time.Time
}

func (r *Repository) SaveOAuthTransaction(ctx context.Context, value application.OAuthTransaction) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("DELETE FROM auth.oauth_transactions WHERE expires_at <= now()").Error; err != nil {
			return err
		}
		return tx.Table("auth.oauth_transactions").Create(&oauthTransactionModel{
			StateHash: value.StateHash, Provider: value.Provider, RedirectURI: value.RedirectURI,
			Verifier: value.Verifier, CreateAccount: value.CreateAccount, ExpiresAt: value.ExpiresAt,
		}).Error
	})
}

func (r *Repository) ConsumeOAuthTransaction(ctx context.Context, hash, provider, redirectURI string, create bool, now time.Time) (application.OAuthTransaction, error) {
	var row oauthTransactionModel
	result := r.db.WithContext(ctx).Raw(
		"DELETE FROM auth.oauth_transactions WHERE state_hash = ? AND provider = ? AND redirect_uri = ? AND create_account = ? AND expires_at > ? RETURNING *",
		hash, provider, redirectURI, create, now,
	).Scan(&row)
	if result.Error != nil {
		return application.OAuthTransaction{}, result.Error
	}
	if result.RowsAffected != 1 {
		return application.OAuthTransaction{}, domain.ErrOAuthState
	}
	return application.OAuthTransaction{StateHash: row.StateHash, Provider: row.Provider,
		RedirectURI: row.RedirectURI, Verifier: row.Verifier, CreateAccount: row.CreateAccount, ExpiresAt: row.ExpiresAt}, nil
}
