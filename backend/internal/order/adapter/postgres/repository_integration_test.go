//go:build integration

package postgres

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/thinhnguyenwilliam/book-store/backend/internal/order/domain"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestRestoreCartItemsIsIdempotent(t *testing.T) {
	dsn := os.Getenv("BOOKSTORE_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("BOOKSTORE_TEST_DATABASE_URL is not set")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{TranslateError: true})
	if err != nil {
		t.Fatalf("open PostgreSQL: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sql.DB: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	ctx := context.Background()
	userID, bookID, cartItemID := uuid.NewString(), uuid.NewString(), uuid.NewString()
	t.Cleanup(func() {
		_ = db.WithContext(context.Background()).Exec(
			"DELETE FROM orders.cart_items WHERE user_id = ?", userID,
		).Error
	})
	repository := NewRepository(db)
	now := time.Now().UTC()
	items := []domain.Item{{CartItemID: cartItemID, BookID: bookID, Quantity: 2}}
	if err := repository.RestoreCartItems(ctx, userID, items, now); err != nil {
		t.Fatalf("first restore: %v", err)
	}
	items[0].Quantity = 1
	if err := repository.RestoreCartItems(ctx, userID, items, now.Add(time.Second)); err != nil {
		t.Fatalf("idempotent restore: %v", err)
	}
	var quantity int32
	if err := db.WithContext(ctx).Raw(
		"SELECT quantity FROM orders.cart_items WHERE user_id = ? AND book_id = ?", userID, bookID,
	).Scan(&quantity).Error; err != nil {
		t.Fatalf("read restored cart: %v", err)
	}
	if quantity != 2 {
		t.Fatalf("restored quantity = %d, want 2", quantity)
	}
}
