package application

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
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
	found   *domain.Book
	finds   int
	lists   int
}

func (r *bookRepositoryStub) Create(_ context.Context, book *domain.Book) error {
	r.created = book
	return nil
}

func (r *bookRepositoryStub) FindByID(context.Context, string) (*domain.Book, error) {
	r.finds++
	if r.found == nil {
		return nil, domain.ErrNotFound
	}
	return r.found, nil
}

func (r *bookRepositoryStub) List(_ context.Context, limit int32, cursor *BookCursor) ([]*domain.Book, error) {
	r.limit, r.cursor = limit, cursor
	r.lists++
	return r.books, nil
}

func (r *bookRepositoryStub) Update(context.Context, *domain.Book) error { return nil }
func (r *bookRepositoryStub) Delete(context.Context, string) error       { return nil }
func (r *bookRepositoryStub) ReserveStock(
	context.Context,
	*domain.StockReservation,
) (*domain.StockReservation, error) {
	return nil, domain.ErrReservationMissing
}
func (r *bookRepositoryStub) CommitStock(
	context.Context,
	string,
	string,
	time.Time,
) (*domain.StockReservation, error) {
	return nil, domain.ErrReservationMissing
}
func (r *bookRepositoryStub) ReleaseStock(
	context.Context,
	string,
	string,
	time.Time,
) (*domain.StockReservation, error) {
	return nil, domain.ErrReservationMissing
}

type bookCacheStub struct {
	mu       sync.Mutex
	values   map[string][]byte
	versions map[string]int64
}

func newBookCacheStub() *bookCacheStub {
	return &bookCacheStub{values: make(map[string][]byte), versions: make(map[string]int64)}
}

func (c *bookCacheStub) GetJSON(_ context.Context, key string, destination any) (bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	payload, ok := c.values[key]
	if !ok {
		return false, nil
	}
	return true, json.Unmarshal(payload, destination)
}

func (c *bookCacheStub) SetJSON(_ context.Context, key string, value any, _ time.Duration) error {
	payload, err := json.Marshal(value)
	if err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.values[key] = payload
	return nil
}

func (c *bookCacheStub) Version(_ context.Context, key string) (int64, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.versions[key], nil
}

func (c *bookCacheStub) BumpVersion(_ context.Context, key string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.versions[key]++
	return nil
}

func (c *bookCacheStub) TryLock(context.Context, string, time.Duration) (string, bool, error) {
	return "test-lock", true, nil
}

func (c *bookCacheStub) Unlock(context.Context, string, string) error { return nil }

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

func TestGetUsesCacheAndWriteInvalidatesVersion(t *testing.T) {
	bookID := uuid.NewString()
	repository := &bookRepositoryStub{found: &domain.Book{
		ID: bookID, Title: "Cached book", Author: "Author", ISBN: "cache-1", PriceCents: 100, Stock: 5,
	}}
	cache := newBookCacheStub()
	service := NewService(repository)
	service.SetCache(cache, time.Minute, time.Second)

	for range 2 {
		if _, err := service.Get(context.Background(), bookID); err != nil {
			t.Fatalf("Get() error = %v", err)
		}
	}
	if repository.finds != 1 {
		t.Fatalf("repository FindByID calls = %d, want 1", repository.finds)
	}

	if _, err := service.Create(context.Background(), &domain.Book{
		Title: "New book", Author: "Author", ISBN: "cache-2", PriceCents: 200, Stock: 2,
	}); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if _, err := service.Get(context.Background(), bookID); err != nil {
		t.Fatalf("Get() after invalidation error = %v", err)
	}
	if repository.finds != 2 {
		t.Fatalf("repository FindByID calls after invalidation = %d, want 2", repository.finds)
	}
}

func TestListUsesCache(t *testing.T) {
	repository := &bookRepositoryStub{books: []*domain.Book{{
		ID: uuid.NewString(), Title: "Cached list", CreatedAt: time.Now().UTC(),
	}}}
	service := NewService(repository)
	service.SetCache(newBookCacheStub(), time.Minute, time.Second)

	for range 2 {
		page, err := service.List(context.Background(), "", 20)
		if err != nil || len(page.Books) != 1 {
			t.Fatalf("List() page = %+v, error = %v", page, err)
		}
	}
	if repository.lists != 1 {
		t.Fatalf("repository List calls = %d, want 1", repository.lists)
	}
}
