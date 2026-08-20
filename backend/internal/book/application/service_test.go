package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/thinhnguyenwilliam/book-store/backend/internal/book/domain"
)

type bookRepositoryStub struct {
	created *domain.Book
	limit   int32
	cursor  *BookCursor
	books   []*domain.Book
}

func (r *bookRepositoryStub) Create(_ context.Context, book *domain.Book) error {
	r.created = book
	return nil
}

func (r *bookRepositoryStub) FindByID(context.Context, string) (*domain.Book, error) {
	return nil, domain.ErrNotFound
}

func (r *bookRepositoryStub) List(_ context.Context, limit int32, cursor *BookCursor) ([]*domain.Book, error) {
	r.limit, r.cursor = limit, cursor
	return r.books, nil
}

func (r *bookRepositoryStub) Update(context.Context, *domain.Book) error { return nil }
func (r *bookRepositoryStub) Delete(context.Context, string) error       { return nil }

func TestCreateBookValidatesAndAssignsID(t *testing.T) {
	repository := &bookRepositoryStub{}
	service := NewService(repository)

	book, err := service.Create(context.Background(), &domain.Book{
		Title:      " Domain-Driven Design ",
		Author:     "Eric Evans",
		ISBN:       "978-0321125217",
		PriceCents: 4500,
		Stock:      10,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if book.ID == "" || repository.created == nil {
		t.Fatalf("book was not assigned an ID or persisted")
	}
	if book.Title != "Domain-Driven Design" {
		t.Fatalf("title = %q, want trimmed title", book.Title)
	}
}

func TestListCapsLimitAndFetchesOneExtra(t *testing.T) {
	repository := &bookRepositoryStub{}
	service := NewService(repository)

	_, err := service.List(context.Background(), "", 1000)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if repository.limit != 101 {
		t.Fatalf("repository limit = %d, want 101", repository.limit)
	}
	if repository.cursor != nil {
		t.Fatalf("repository cursor = %+v, want nil", repository.cursor)
	}
}

func TestListReturnsNextCursorWhenMoreBooksExist(t *testing.T) {
	createdAt := time.Date(2026, time.August, 20, 10, 30, 0, 123000000, time.UTC)
	books := []*domain.Book{
		{ID: uuid.NewString(), CreatedAt: createdAt.Add(2 * time.Minute)},
		{ID: uuid.NewString(), CreatedAt: createdAt.Add(time.Minute)},
		{ID: uuid.NewString(), CreatedAt: createdAt},
	}
	repository := &bookRepositoryStub{books: books}
	service := NewService(repository)

	page, err := service.List(context.Background(), "", 2)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(page.Books) != 2 || !page.HasMore || page.NextCursor == "" {
		t.Fatalf("unexpected page: %+v", page)
	}

	cursor, err := decodeCursor(page.NextCursor)
	if err != nil {
		t.Fatalf("decodeCursor() error = %v", err)
	}
	if cursor.ID != books[1].ID || !cursor.CreatedAt.Equal(books[1].CreatedAt) {
		t.Fatalf("cursor = %+v, want book %+v", cursor, books[1])
	}
}

func TestListPassesDecodedCursorToRepository(t *testing.T) {
	want := BookCursor{
		CreatedAt: time.Date(2026, time.August, 20, 10, 30, 0, 123000000, time.UTC),
		ID:        uuid.NewString(),
	}
	raw, err := encodeCursor(want)
	if err != nil {
		t.Fatalf("encodeCursor() error = %v", err)
	}
	repository := &bookRepositoryStub{}
	service := NewService(repository)

	if _, err := service.List(context.Background(), raw, 20); err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if repository.cursor == nil || repository.cursor.ID != want.ID || !repository.cursor.CreatedAt.Equal(want.CreatedAt) {
		t.Fatalf("repository cursor = %+v, want %+v", repository.cursor, want)
	}
}

func TestListRejectsInvalidCursor(t *testing.T) {
	repository := &bookRepositoryStub{}
	service := NewService(repository)

	_, err := service.List(context.Background(), "not-a-valid-cursor", 20)
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("List() error = %v, want %v", err, domain.ErrInvalidInput)
	}
	if repository.limit != 0 {
		t.Fatalf("repository should not be called for an invalid cursor")
	}
}
