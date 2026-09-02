package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/thinhnguyenwilliam/book-store/backend/internal/search/domain"
)

func TestSearchBuildsAndValidatesCursor(t *testing.T) {
	index := &indexStub{searchResult: domain.Result{Hits: []domain.Hit{
		{Book: domain.BookDocument{ID: uuid.NewString()}, Sort: []any{5.2, "2026-09-01T00:00:00Z", "a"}},
		{Book: domain.BookDocument{ID: uuid.NewString()}, Sort: []any{4.1, "2026-08-01T00:00:00Z", "b"}},
		{Book: domain.BookDocument{ID: uuid.NewString()}, Sort: []any{3.0, "2026-07-01T00:00:00Z", "c"}},
	}, Total: 9}}
	service := NewService(index)
	page, err := service.Search(context.Background(), domain.Request{Query: "clean architecture", Limit: 2}, "")
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if !page.HasMore || len(page.Hits) != 2 || page.NextCursor == "" || index.searchRequest.Limit != 3 {
		t.Fatalf("unexpected first page: %+v request=%+v", page, index.searchRequest)
	}

	index.searchResult.Hits = nil
	if _, err := service.Search(context.Background(), domain.Request{Query: "different", Limit: 2}, page.NextCursor); !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("Search() cursor mismatch error = %v", err)
	}
	if _, err := service.Search(context.Background(), domain.Request{Query: "clean architecture", Limit: 2}, page.NextCursor); err != nil {
		t.Fatalf("Search() next cursor error = %v", err)
	}
	if len(index.searchRequest.SearchAfter) != 3 {
		t.Fatalf("SearchAfter = %#v", index.searchRequest.SearchAfter)
	}
}

func TestSearchRejectsInvalidFilters(t *testing.T) {
	service := NewService(&indexStub{})
	falseValue := false
	for _, request := range []domain.Request{
		{Limit: 51},
		{Filters: domain.Filters{MinPriceCents: 200, MaxPriceCents: 100}},
		{Filters: domain.Filters{SellerID: "not-a-uuid"}},
		{Sort: "popular"},
		{Filters: domain.Filters{InStock: &falseValue}, Limit: 20, Query: string(make([]byte, 201))},
	} {
		if _, err := service.Search(context.Background(), request, ""); !errors.Is(err, domain.ErrInvalidInput) {
			t.Fatalf("Search(%+v) error = %v", request, err)
		}
	}
}

func TestReindexAllPages(t *testing.T) {
	index := &indexStub{}
	service := NewService(index)
	catalog := &catalogStub{pages: [][]domain.BookDocument{
		{{ID: uuid.NewString(), UpdatedAt: time.Now()}},
		{{ID: uuid.NewString(), UpdatedAt: time.Now()}},
	}}
	count, err := service.Reindex(context.Background(), catalog)
	if err != nil {
		t.Fatalf("Reindex() error = %v", err)
	}
	if count != 2 || index.bulkCount != 2 {
		t.Fatalf("count = %d, bulkCount = %d", count, index.bulkCount)
	}
}

type indexStub struct {
	searchRequest domain.Request
	searchResult  domain.Result
	bulkCount     int
}

func (s *indexStub) Ensure(context.Context) (bool, error)                     { return false, nil }
func (s *indexStub) Upsert(context.Context, domain.BookDocument, int64) error { return nil }
func (s *indexStub) Delete(context.Context, string, int64) error              { return nil }
func (s *indexStub) BulkUpsert(_ context.Context, books []domain.BookDocument) error {
	s.bulkCount += len(books)
	return nil
}
func (s *indexStub) Search(_ context.Context, request domain.Request) (domain.Result, error) {
	s.searchRequest = request
	return s.searchResult, nil
}
func (s *indexStub) Suggest(context.Context, string, int) (domain.Result, error) {
	return domain.Result{}, nil
}

type catalogStub struct {
	pages [][]domain.BookDocument
	call  int
}

func (s *catalogStub) List(context.Context, string, int32) ([]domain.BookDocument, string, bool, error) {
	page := s.pages[s.call]
	s.call++
	return page, string(rune('a' + s.call)), s.call < len(s.pages), nil
}
