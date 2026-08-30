package domain

import (
	"errors"
	"strings"
	"time"
)

var (
	ErrNotFound           = errors.New("book not found")
	ErrISBNExists         = errors.New("ISBN already exists")
	ErrInvalidInput       = errors.New("invalid book input")
	ErrInsufficientStock  = errors.New("insufficient book stock")
	ErrReservationMissing = errors.New("stock reservation not found")
	ErrReservationState   = errors.New("invalid stock reservation state")
	ErrBookInUse          = errors.New("book has stock reservation history and cannot be deleted")
)

type Book struct {
	ID         string
	Title      string
	Author     string
	ISBN       string
	PriceCents int64
	Stock      int32
	SellerID   string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

func (b *Book) Validate() error {
	b.Title = strings.TrimSpace(b.Title)
	b.Author = strings.TrimSpace(b.Author)
	b.ISBN = strings.TrimSpace(b.ISBN)
	b.SellerID = strings.TrimSpace(b.SellerID)
	if b.Title == "" || b.Author == "" || b.ISBN == "" || b.PriceCents < 0 || b.Stock < 0 {
		return ErrInvalidInput
	}
	return nil
}

type StockReservation struct {
	ID             string
	OrderID        string
	BookID         string
	Quantity       int32
	Status         string
	IdempotencyKey string
	ExpiresAt      time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}
