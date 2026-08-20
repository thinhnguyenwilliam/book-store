package application

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/thinhnguyenwilliam/book-store/backend/internal/book/domain"
)

type Service struct {
	repository BookRepository
	now        func() time.Time
}

type BookPage struct {
	Books      []*domain.Book
	NextCursor string
	HasMore    bool
}

func NewService(repository BookRepository) *Service {
	return &Service{repository: repository, now: time.Now}
}

func (s *Service) Create(ctx context.Context, book *domain.Book) (*domain.Book, error) {
	if err := book.Validate(); err != nil {
		return nil, err
	}
	now := s.now().UTC()
	book.ID = uuid.NewString()
	book.CreatedAt = now
	book.UpdatedAt = now
	if err := s.repository.Create(ctx, book); err != nil {
		return nil, err
	}
	return book, nil
}

func (s *Service) Get(ctx context.Context, id string) (*domain.Book, error) {
	if _, err := uuid.Parse(id); err != nil {
		return nil, domain.ErrInvalidInput
	}
	return s.repository.FindByID(ctx, id)
}

func (s *Service) List(ctx context.Context, rawCursor string, limit int32) (BookPage, error) {
	if limit < 1 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	cursor, err := decodeCursor(rawCursor)
	if err != nil {
		return BookPage{}, err
	}

	books, err := s.repository.List(ctx, limit+1, cursor)
	if err != nil {
		return BookPage{}, err
	}
	hasMore := len(books) > int(limit)
	if hasMore {
		books = books[:limit]
	}

	page := BookPage{Books: books, HasMore: hasMore}
	if !hasMore || len(books) == 0 {
		return page, nil
	}
	lastBook := books[len(books)-1]
	page.NextCursor, err = encodeCursor(BookCursor{CreatedAt: lastBook.CreatedAt, ID: lastBook.ID})
	if err != nil {
		return BookPage{}, err
	}
	return page, nil
}

func (s *Service) Update(ctx context.Context, book *domain.Book) (*domain.Book, error) {
	if _, err := uuid.Parse(book.ID); err != nil {
		return nil, domain.ErrInvalidInput
	}
	if err := book.Validate(); err != nil {
		return nil, err
	}
	book.UpdatedAt = s.now().UTC()
	if err := s.repository.Update(ctx, book); err != nil {
		return nil, err
	}
	return book, nil
}

func (s *Service) Delete(ctx context.Context, id string) error {
	if _, err := uuid.Parse(id); err != nil {
		return domain.ErrInvalidInput
	}
	return s.repository.Delete(ctx, id)
}
