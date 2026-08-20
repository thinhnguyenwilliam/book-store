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

func (s *Service) List(ctx context.Context, page, pageSize int32) ([]*domain.Book, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return s.repository.List(ctx, pageSize, (page-1)*pageSize)
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
