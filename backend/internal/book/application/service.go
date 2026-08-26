package application

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/thinhnguyenwilliam/book-store/backend/internal/book/domain"
	"golang.org/x/sync/singleflight"
)

type Service struct {
	repository     BookRepository
	cache          Cache
	now            func() time.Time
	reservationTTL time.Duration
	cacheTTL       time.Duration
	cacheLockTTL   time.Duration
	cacheFlight    singleflight.Group
}

type BookPage struct {
	Books      []*domain.Book
	NextCursor string
	HasMore    bool
}

func NewService(repository BookRepository) *Service {
	return &Service{repository: repository, now: time.Now, reservationTTL: 15 * time.Minute}
}

func (s *Service) SetReservationTTL(ttl time.Duration) {
	if ttl > 0 {
		s.reservationTTL = ttl
	}
}

func (s *Service) SetCache(cache Cache, ttl, lockTTL time.Duration) {
	if cache == nil || ttl <= 0 || lockTTL <= 0 {
		return
	}
	s.cache = cache
	s.cacheTTL = ttl
	s.cacheLockTTL = lockTTL
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
	s.invalidateCache(ctx)
	return book, nil
}

func (s *Service) Get(ctx context.Context, id string) (*domain.Book, error) {
	if _, err := uuid.Parse(id); err != nil {
		return nil, domain.ErrInvalidInput
	}
	return s.getCached(ctx, id)
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

	return s.listCached(ctx, rawCursor, limit, cursor)
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
	s.invalidateCache(ctx)
	return book, nil
}

func (s *Service) Delete(ctx context.Context, id string) error {
	if _, err := uuid.Parse(id); err != nil {
		return domain.ErrInvalidInput
	}
	if err := s.repository.Delete(ctx, id); err != nil {
		return err
	}
	s.invalidateCache(ctx)
	return nil
}

func (s *Service) ReserveStock(
	ctx context.Context,
	orderID, bookID string,
	quantity int32,
	idempotencyKey string,
) (*domain.StockReservation, error) {
	if _, err := uuid.Parse(orderID); err != nil {
		return nil, domain.ErrInvalidInput
	}
	if _, err := uuid.Parse(bookID); err != nil {
		return nil, domain.ErrInvalidInput
	}
	if quantity < 1 || quantity > 100 || idempotencyKey == "" {
		return nil, domain.ErrInvalidInput
	}
	now := s.now().UTC()
	reservation, err := s.repository.ReserveStock(ctx, &domain.StockReservation{
		ID:             uuid.NewString(),
		OrderID:        orderID,
		BookID:         bookID,
		Quantity:       quantity,
		Status:         "reserved",
		IdempotencyKey: idempotencyKey,
		ExpiresAt:      now.Add(s.reservationTTL),
		CreatedAt:      now,
		UpdatedAt:      now,
	})
	if err != nil {
		return nil, err
	}
	s.invalidateCache(ctx)
	return reservation, nil
}

func (s *Service) CommitStock(ctx context.Context, orderID, bookID string) (*domain.StockReservation, error) {
	if _, err := uuid.Parse(orderID); err != nil {
		return nil, domain.ErrInvalidInput
	}
	if _, err := uuid.Parse(bookID); err != nil {
		return nil, domain.ErrInvalidInput
	}
	return s.repository.CommitStock(ctx, orderID, bookID, s.now().UTC())
}

func (s *Service) ReleaseStock(ctx context.Context, orderID, bookID string) (*domain.StockReservation, error) {
	if _, err := uuid.Parse(orderID); err != nil {
		return nil, domain.ErrInvalidInput
	}
	if _, err := uuid.Parse(bookID); err != nil {
		return nil, domain.ErrInvalidInput
	}
	reservation, err := s.repository.ReleaseStock(ctx, orderID, bookID, s.now().UTC())
	if err != nil {
		return nil, err
	}
	s.invalidateCache(ctx)
	return reservation, nil
}
