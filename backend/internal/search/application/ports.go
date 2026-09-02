package application

import (
	"context"

	catalogevent "github.com/thinhnguyenwilliam/book-store/backend/internal/events/catalog"
	"github.com/thinhnguyenwilliam/book-store/backend/internal/search/domain"
)

type Index interface {
	Ensure(ctx context.Context) (created bool, err error)
	Upsert(ctx context.Context, book domain.BookDocument, version int64) error
	Delete(ctx context.Context, bookID string, version int64) error
	BulkUpsert(ctx context.Context, books []domain.BookDocument) error
	Search(ctx context.Context, request domain.Request) (domain.Result, error)
	Suggest(ctx context.Context, query string, limit int) (domain.Result, error)
}

type CatalogReader interface {
	List(ctx context.Context, cursor string, limit int32) (books []domain.BookDocument, nextCursor string, hasMore bool, err error)
}

type CatalogEvent = catalogevent.Event
