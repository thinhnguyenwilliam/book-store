package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/thinhnguyenwilliam/book-store/backend/internal/comment/domain"
)

type repositoryStub struct {
	items   map[string]*domain.Comment
	created *domain.Comment
}

func (r *repositoryStub) Create(_ context.Context, item *domain.Comment) error {
	r.created = item
	r.items[item.ID] = item
	return nil
}
func (r *repositoryStub) FindByID(_ context.Context, id string) (*domain.Comment, error) {
	item := r.items[id]
	if item == nil {
		return nil, domain.ErrNotFound
	}
	return item, nil
}
func (r *repositoryStub) ListRoots(context.Context, string, int32, *domain.Cursor) ([]*domain.Comment, error) {
	return nil, nil
}
func (r *repositoryStub) ListReplies(context.Context, string, int32, *domain.Cursor) ([]*domain.Comment, error) {
	return nil, nil
}
func (r *repositoryStub) Update(_ context.Context, id, _ string, content string, now time.Time) (*domain.Comment, error) {
	item := r.items[id]
	item.Content, item.UpdatedAt = content, now
	return item, nil
}
func (r *repositoryStub) SoftDelete(_ context.Context, id string, now time.Time) (*domain.Comment, error) {
	item := r.items[id]
	item.Status, item.DeletedAt = domain.StatusDeleted, now
	return item, nil
}
func (r *repositoryStub) Moderate(_ context.Context, id, status string, now time.Time) (*domain.Comment, error) {
	item := r.items[id]
	item.Status, item.UpdatedAt = status, now
	return item, nil
}

type bookResolverStub struct{ err error }

func (r bookResolverStub) Exists(context.Context, string) error { return r.err }

type authorResolverStub struct{ name string }

func (r authorResolverStub) DisplayName(context.Context, string) (string, error) { return r.name, nil }

func TestCreateReplyUsesRootAndDepth(t *testing.T) {
	bookID, authorID, rootID, parentID := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	repository := &repositoryStub{items: map[string]*domain.Comment{parentID: {ID: parentID, BookID: bookID, RootID: rootID, Depth: 1, Status: domain.StatusPublished}}}
	service := NewService(repository, bookResolverStub{}, authorResolverStub{name: "Thịnh"})
	item, err := service.Create(context.Background(), bookID, authorID, parentID, "  Cảm ơn bạn  ")
	if err != nil {
		t.Fatal(err)
	}
	if item.RootID != rootID || item.Depth != 2 || item.Content != "Cảm ơn bạn" {
		t.Fatalf("unexpected reply: %+v", item)
	}
}

func TestCreateReplyRejectsDepthAboveLimit(t *testing.T) {
	bookID, authorID, parentID := uuid.NewString(), uuid.NewString(), uuid.NewString()
	repository := &repositoryStub{items: map[string]*domain.Comment{parentID: {ID: parentID, BookID: bookID, RootID: uuid.NewString(), Depth: domain.MaxDepth, Status: domain.StatusPublished}}}
	service := NewService(repository, bookResolverStub{}, authorResolverStub{name: "Reader"})
	_, err := service.Create(context.Background(), bookID, authorID, parentID, "reply")
	if !errors.Is(err, domain.ErrMaxDepth) {
		t.Fatalf("Create() error = %v", err)
	}
}

func TestUpdateRejectsAnotherAuthor(t *testing.T) {
	id := uuid.NewString()
	repository := &repositoryStub{items: map[string]*domain.Comment{id: {ID: id, AuthorID: uuid.NewString(), Status: domain.StatusPublished}}}
	service := NewService(repository, bookResolverStub{}, authorResolverStub{})
	_, err := service.Update(context.Background(), id, uuid.NewString(), "updated")
	if !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("Update() error = %v", err)
	}
}

func TestDeletePreservesNodeAsTombstone(t *testing.T) {
	authorID, id := uuid.NewString(), uuid.NewString()
	repository := &repositoryStub{items: map[string]*domain.Comment{id: {ID: id, AuthorID: authorID, Status: domain.StatusPublished}}}
	service := NewService(repository, bookResolverStub{}, authorResolverStub{})
	item, err := service.Delete(context.Background(), id, authorID, false)
	if err != nil {
		t.Fatal(err)
	}
	if item.Status != domain.StatusDeleted {
		t.Fatalf("status = %q", item.Status)
	}
}
