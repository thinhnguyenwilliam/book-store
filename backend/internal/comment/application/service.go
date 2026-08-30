package application

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/thinhnguyenwilliam/book-store/backend/internal/comment/domain"
	"golang.org/x/sync/errgroup"
)

type Service struct {
	repository Repository
	books      BookResolver
	authors    AuthorResolver
	now        func() time.Time
}

func NewService(repository Repository, books BookResolver, authors AuthorResolver) *Service {
	return &Service{repository: repository, books: books, authors: authors, now: time.Now}
}

func (s *Service) Create(ctx context.Context, bookID, authorID, parentID, content string) (*domain.Comment, error) {
	if !validID(bookID) || !validID(authorID) || (parentID != "" && !validID(parentID)) {
		return nil, domain.ErrInvalidInput
	}
	content, err := domain.NormalizeContent(content)
	if err != nil {
		return nil, err
	}
	var authorName string
	group, resolveCtx := errgroup.WithContext(ctx)
	group.Go(func() error { return s.books.Exists(resolveCtx, bookID) })
	group.Go(func() error {
		var resolveErr error
		authorName, resolveErr = s.authors.DisplayName(resolveCtx, authorID)
		return resolveErr
	})
	if err := group.Wait(); err != nil {
		return nil, err
	}
	id := uuid.NewString()
	item := &domain.Comment{ID: id, BookID: bookID, AuthorID: authorID, AuthorName: strings.TrimSpace(authorName), ParentID: parentID, RootID: id, Status: domain.StatusPublished}
	if item.AuthorName == "" {
		item.AuthorName = "Độc giả"
	}
	if parentID != "" {
		parent, err := s.repository.FindByID(ctx, parentID)
		if err != nil {
			return nil, err
		}
		if parent.BookID != bookID {
			return nil, domain.ErrParentMismatch
		}
		if parent.Status != domain.StatusPublished {
			return nil, domain.ErrNotEditable
		}
		if parent.Depth >= domain.MaxDepth {
			return nil, domain.ErrMaxDepth
		}
		item.RootID, item.Depth = parent.RootID, parent.Depth+1
	}
	now := s.now().UTC()
	item.Content = content
	item.CreatedAt = now
	item.UpdatedAt = now
	if err := s.repository.Create(ctx, item); err != nil {
		return nil, err
	}
	return item, nil
}

func (s *Service) ListRoots(ctx context.Context, bookID, rawCursor string, limit int32) (domain.Page, error) {
	if !validID(bookID) {
		return domain.Page{}, domain.ErrInvalidInput
	}
	return s.list(rawCursor, limit, func(limit int32, cursor *domain.Cursor) ([]*domain.Comment, error) {
		return s.repository.ListRoots(ctx, bookID, limit, cursor)
	})
}

func (s *Service) ListReplies(ctx context.Context, rootID, rawCursor string, limit int32) (domain.Page, error) {
	if !validID(rootID) {
		return domain.Page{}, domain.ErrInvalidInput
	}
	root, err := s.repository.FindByID(ctx, rootID)
	if err != nil {
		return domain.Page{}, err
	}
	if root.ParentID != "" {
		return domain.Page{}, domain.ErrInvalidInput
	}
	return s.list(rawCursor, limit, func(limit int32, cursor *domain.Cursor) ([]*domain.Comment, error) {
		return s.repository.ListReplies(ctx, rootID, limit, cursor)
	})
}

func (s *Service) list(rawCursor string, limit int32, query func(int32, *domain.Cursor) ([]*domain.Comment, error)) (domain.Page, error) {
	if limit < 1 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	cursor, err := decodeCursor(rawCursor)
	if err != nil {
		return domain.Page{}, err
	}
	items, err := query(limit+1, cursor)
	if err != nil {
		return domain.Page{}, err
	}
	hasMore := len(items) > int(limit)
	if hasMore {
		items = items[:limit]
	}
	page := domain.Page{Comments: items, HasMore: hasMore}
	if hasMore && len(items) > 0 {
		last := items[len(items)-1]
		page.NextCursor, err = encodeCursor(domain.Cursor{CreatedAt: last.CreatedAt, ID: last.ID})
	}
	return page, err
}

func (s *Service) Update(ctx context.Context, id, authorID, content string) (*domain.Comment, error) {
	if !validID(id) || !validID(authorID) {
		return nil, domain.ErrInvalidInput
	}
	content, err := domain.NormalizeContent(content)
	if err != nil {
		return nil, err
	}
	item, err := s.repository.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if item.AuthorID != authorID {
		return nil, domain.ErrForbidden
	}
	if item.Status != domain.StatusPublished {
		return nil, domain.ErrNotEditable
	}
	return s.repository.Update(ctx, id, authorID, content, s.now().UTC())
}

func (s *Service) Delete(ctx context.Context, id, actorID string, isAdmin bool) (*domain.Comment, error) {
	if !validID(id) || !validID(actorID) {
		return nil, domain.ErrInvalidInput
	}
	item, err := s.repository.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if !isAdmin && item.AuthorID != actorID {
		return nil, domain.ErrForbidden
	}
	if item.Status == domain.StatusDeleted {
		return item, nil
	}
	return s.repository.SoftDelete(ctx, id, s.now().UTC())
}

func (s *Service) Moderate(ctx context.Context, id, status string) (*domain.Comment, error) {
	if !validID(id) || (status != domain.StatusPublished && status != domain.StatusHidden) {
		return nil, domain.ErrInvalidInput
	}
	item, err := s.repository.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if item.Status == domain.StatusDeleted {
		return nil, domain.ErrNotEditable
	}
	return s.repository.Moderate(ctx, id, status, s.now().UTC())
}

func validID(value string) bool { _, err := uuid.Parse(value); return err == nil }
