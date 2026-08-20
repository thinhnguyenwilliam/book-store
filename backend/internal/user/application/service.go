package application

import (
	"context"
	"net/mail"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/thinhnguyenwilliam/book-store/backend/internal/user/domain"
)

type Service struct {
	repository UserRepository
	now        func() time.Time
}

type UserPage struct {
	Users      []*domain.User
	NextCursor string
	HasMore    bool
}

func NewService(repository UserRepository) *Service {
	return &Service{repository: repository, now: time.Now}
}

func (s *Service) Create(ctx context.Context, id, email, displayName string) (*domain.User, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if _, err := uuid.Parse(id); err != nil || !validEmail(email) {
		return nil, domain.ErrInvalidInput
	}

	now := s.now().UTC()
	user := &domain.User{
		ID:        id,
		Email:     email,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := user.UpdateDisplayName(displayName, now); err != nil {
		return nil, err
	}
	if err := s.repository.Create(ctx, user); err != nil {
		return nil, err
	}
	return user, nil
}

func (s *Service) Get(ctx context.Context, id string) (*domain.User, error) {
	if _, err := uuid.Parse(id); err != nil {
		return nil, domain.ErrInvalidInput
	}
	return s.repository.FindByID(ctx, id)
}

func (s *Service) List(ctx context.Context, rawCursor string, limit int32) (UserPage, error) {
	if limit < 1 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	cursor, err := decodeCursor(rawCursor)
	if err != nil {
		return UserPage{}, err
	}

	users, err := s.repository.List(ctx, limit+1, cursor)
	if err != nil {
		return UserPage{}, err
	}
	hasMore := len(users) > int(limit)
	if hasMore {
		users = users[:limit]
	}

	page := UserPage{Users: users, HasMore: hasMore}
	if !hasMore || len(users) == 0 {
		return page, nil
	}
	lastUser := users[len(users)-1]
	page.NextCursor, err = encodeCursor(UserCursor{CreatedAt: lastUser.CreatedAt, ID: lastUser.ID})
	if err != nil {
		return UserPage{}, err
	}
	return page, nil
}

func (s *Service) Update(ctx context.Context, id, displayName string) (*domain.User, error) {
	user, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := user.UpdateDisplayName(displayName, s.now()); err != nil {
		return nil, err
	}
	if err := s.repository.Update(ctx, user); err != nil {
		return nil, err
	}
	return user, nil
}

func (s *Service) Delete(ctx context.Context, id string) error {
	if _, err := uuid.Parse(id); err != nil {
		return domain.ErrInvalidInput
	}
	return s.repository.Delete(ctx, id)
}

func validEmail(email string) bool {
	address, err := mail.ParseAddress(email)
	return err == nil && address.Address == email
}
