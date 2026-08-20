package application

import (
	"context"
	"testing"

	"github.com/thinhnguyenwilliam/book-store/backend/internal/book/domain"
)

type bookRepositoryStub struct {
	created *domain.Book
	limit   int32
	offset  int32
}

func (r *bookRepositoryStub) Create(_ context.Context, book *domain.Book) error {
	r.created = book
	return nil
}

func (r *bookRepositoryStub) FindByID(context.Context, string) (*domain.Book, error) {
	return nil, domain.ErrNotFound
}

func (r *bookRepositoryStub) List(_ context.Context, limit, offset int32) ([]*domain.Book, int64, error) {
	r.limit, r.offset = limit, offset
	return nil, 0, nil
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

func TestListCapsPageSize(t *testing.T) {
	repository := &bookRepositoryStub{}
	service := NewService(repository)

	_, _, err := service.List(context.Background(), 2, 1000)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if repository.limit != 100 || repository.offset != 100 {
		t.Fatalf("limit/offset = %d/%d, want 100/100", repository.limit, repository.offset)
	}
}
