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

func validEmail(email string) bool {
	address, err := mail.ParseAddress(email)
	return err == nil && address.Address == email
}
