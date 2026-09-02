//go:build integration

package postgres

import (
	"context"
	"errors"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/thinhnguyenwilliam/book-store/backend/internal/book/domain"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestReserveStockPreventsOversell(t *testing.T) {
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
	sqlDB.SetMaxOpenConns(25)
	t.Cleanup(func() { _ = sqlDB.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	repository := NewRepository(db)
	now := time.Now().UTC()
	book := &domain.Book{
		ID: uuid.NewString(), Title: "Oversell integration test", Author: "Book Store",
		ISBN: "oversell-" + uuid.NewString(), PriceCents: 10000, Stock: 5,
		CreatedAt: now, UpdatedAt: now,
	}
	if err := repository.Create(ctx, book); err != nil {
		t.Fatalf("create book: %v", err)
	}
	t.Cleanup(func() {
		cleanup := context.Background()
		_ = db.WithContext(cleanup).Exec("DELETE FROM orders.cart_items WHERE book_id = ?", book.ID).Error
		_ = db.WithContext(cleanup).Exec("DELETE FROM catalog.stock_reservations WHERE book_id = ?", book.ID).Error
		_ = db.WithContext(cleanup).Exec("DELETE FROM catalog.outbox_events WHERE aggregate_id = ?", book.ID).Error
		_ = db.WithContext(cleanup).Exec("DELETE FROM catalog.books WHERE id = ?", book.ID).Error
	})

	const contenders = 20
	start := make(chan struct{})
	var successes atomic.Int32
	var insufficient atomic.Int32
	var unexpected atomic.Int32
	var wait sync.WaitGroup
	wait.Add(contenders)
	for range contenders {
		go func() {
			defer wait.Done()
			orderID := uuid.NewString()
			<-start
			_, reserveErr := repository.ReserveStock(ctx, &domain.StockReservation{
				ID: uuid.NewString(), OrderID: orderID, BookID: book.ID, Quantity: 1,
				Status: "reserved", IdempotencyKey: "stock:" + orderID + ":" + book.ID,
				ExpiresAt: now.Add(15 * time.Minute), CreatedAt: now, UpdatedAt: now,
			})
			switch {
			case reserveErr == nil:
				successes.Add(1)
			case errors.Is(reserveErr, domain.ErrInsufficientStock):
				insufficient.Add(1)
			default:
				unexpected.Add(1)
				t.Errorf("unexpected reservation error: %v", reserveErr)
			}
		}()
	}
	close(start)
	wait.Wait()

	if got := successes.Load(); got != 5 {
		t.Fatalf("successful reservations = %d, want 5", got)
	}
	if got := insufficient.Load(); got != contenders-5 {
		t.Fatalf("insufficient-stock results = %d, want %d", got, contenders-5)
	}
	if got := unexpected.Load(); got != 0 {
		t.Fatalf("unexpected errors = %d, want 0", got)
	}
	var remaining int32
	if err := db.WithContext(ctx).Raw("SELECT stock FROM catalog.books WHERE id = ?", book.ID).
		Scan(&remaining).Error; err != nil {
		t.Fatalf("read remaining stock: %v", err)
	}
	if remaining != 0 {
		t.Fatalf("remaining stock = %d, want 0", remaining)
	}
}
