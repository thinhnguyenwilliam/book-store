package domain

import (
	"errors"
	"strings"
	"time"
)

var (
	ErrNotFound     = errors.New("book not found")
	ErrISBNExists   = errors.New("ISBN already exists")
	ErrInvalidInput = errors.New("invalid book input")
)

type Book struct {
	ID         string
	Title      string
	Author     string
	ISBN       string
	PriceCents int64
	Stock      int32
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

func (b *Book) Validate() error {
	b.Title = strings.TrimSpace(b.Title)
	b.Author = strings.TrimSpace(b.Author)
	b.ISBN = strings.TrimSpace(b.ISBN)
	if b.Title == "" || b.Author == "" || b.ISBN == "" || b.PriceCents < 0 || b.Stock < 0 {
		return ErrInvalidInput
	}
	return nil
}
