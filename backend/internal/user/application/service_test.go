package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/thinhnguyenwilliam/book-store/backend/internal/user/domain"
)

type userRepositoryStub struct {
	limit     int32
	cursor    *UserCursor
	users     []*domain.User
	deletedID string
}

func (r *userRepositoryStub) Create(context.Context, *domain.User) error { return nil }
func (r *userRepositoryStub) FindByID(context.Context, string) (*domain.User, error) {
	return nil, domain.ErrNotFound
}
func (r *userRepositoryStub) List(_ context.Context, limit int32, cursor *UserCursor) ([]*domain.User, error) {
	r.limit, r.cursor = limit, cursor
	return r.users, nil
}
func (r *userRepositoryStub) Update(context.Context, *domain.User) error { return nil }
func (r *userRepositoryStub) Delete(_ context.Context, id string) error {
	r.deletedID = id
	return nil
}

func TestListCapsLimitAndFetchesOneExtra(t *testing.T) {
	repository := &userRepositoryStub{}
	service := NewService(repository)

	if _, err := service.List(context.Background(), "", 1000); err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if repository.limit != 101 {
		t.Fatalf("repository limit = %d, want 101", repository.limit)
	}
	if repository.cursor != nil {
		t.Fatalf("repository cursor = %+v, want nil", repository.cursor)
	}
}

func TestListReturnsNextCursorWhenMoreUsersExist(t *testing.T) {
	createdAt := time.Date(2026, time.August, 20, 10, 30, 0, 123000000, time.UTC)
	users := []*domain.User{
		{ID: uuid.NewString(), CreatedAt: createdAt.Add(2 * time.Minute)},
		{ID: uuid.NewString(), CreatedAt: createdAt.Add(time.Minute)},
		{ID: uuid.NewString(), CreatedAt: createdAt},
	}
	service := NewService(&userRepositoryStub{users: users})

	page, err := service.List(context.Background(), "", 2)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(page.Users) != 2 || !page.HasMore || page.NextCursor == "" {
		t.Fatalf("unexpected page: %+v", page)
	}
	cursor, err := decodeCursor(page.NextCursor)
	if err != nil {
		t.Fatalf("decodeCursor() error = %v", err)
	}
	if cursor.ID != users[1].ID || !cursor.CreatedAt.Equal(users[1].CreatedAt) {
		t.Fatalf("cursor = %+v, want user %+v", cursor, users[1])
	}
}

func TestListPassesDecodedCursorToRepository(t *testing.T) {
	want := UserCursor{
		CreatedAt: time.Date(2026, time.August, 20, 10, 30, 0, 123000000, time.UTC),
		ID:        uuid.NewString(),
	}
	raw, err := encodeCursor(want)
	if err != nil {
		t.Fatalf("encodeCursor() error = %v", err)
	}
	repository := &userRepositoryStub{}
	service := NewService(repository)

	if _, err := service.List(context.Background(), raw, 20); err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if repository.cursor == nil || repository.cursor.ID != want.ID || !repository.cursor.CreatedAt.Equal(want.CreatedAt) {
		t.Fatalf("repository cursor = %+v, want %+v", repository.cursor, want)
	}
}

func TestListRejectsInvalidCursor(t *testing.T) {
	repository := &userRepositoryStub{}
	service := NewService(repository)

	_, err := service.List(context.Background(), "not-a-valid-cursor", 20)
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("List() error = %v, want %v", err, domain.ErrInvalidInput)
	}
	if repository.limit != 0 {
		t.Fatalf("repository should not be called for an invalid cursor")
	}
}

func TestDeleteValidatesIDAndCallsRepository(t *testing.T) {
	repository := &userRepositoryStub{}
	service := NewService(repository)
	id := uuid.NewString()

	if err := service.Delete(context.Background(), id); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if repository.deletedID != id {
		t.Fatalf("deleted ID = %q, want %q", repository.deletedID, id)
	}
}
