package application

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"
	catalogevent "github.com/thinhnguyenwilliam/book-store/backend/internal/events/catalog"
	"github.com/thinhnguyenwilliam/book-store/backend/internal/search/domain"
)

const reindexPageSize int32 = 100

type Service struct{ index Index }

type Page struct {
	Hits       []domain.Hit
	NextCursor string
	HasMore    bool
	Total      int64
	TookMS     int64
}

type cursorPayload struct {
	Version     int    `json:"v"`
	Fingerprint string `json:"f"`
	Sort        []any  `json:"s"`
}

func NewService(index Index) *Service { return &Service{index: index} }

func (s *Service) Ensure(ctx context.Context) (bool, error) { return s.index.Ensure(ctx) }

func (s *Service) ApplyCatalogEvent(ctx context.Context, event catalogevent.Event) error {
	if event.EventType == catalogevent.EventBookDeleted {
		return s.index.Delete(ctx, event.BookID, event.Version)
	}
	if event.Book == nil {
		return fmt.Errorf("catalog upsert event has no book snapshot")
	}
	return s.index.Upsert(ctx, documentFromEvent(*event.Book), event.Version)
}

func (s *Service) Search(ctx context.Context, request domain.Request, rawCursor string) (Page, error) {
	request.Query = strings.TrimSpace(request.Query)
	request.Filters.Author = strings.TrimSpace(request.Filters.Author)
	request.Filters.SellerID = strings.TrimSpace(request.Filters.SellerID)
	if request.Limit < 1 {
		request.Limit = 20
	}
	if request.Limit > 50 || len(request.Query) > 200 || len(request.Filters.Author) > 120 ||
		request.Filters.MinPriceCents < 0 || request.Filters.MaxPriceCents < 0 ||
		(request.Filters.MaxPriceCents > 0 && request.Filters.MaxPriceCents < request.Filters.MinPriceCents) {
		return Page{}, domain.ErrInvalidInput
	}
	if request.Filters.SellerID != "" {
		if _, err := uuid.Parse(request.Filters.SellerID); err != nil {
			return Page{}, domain.ErrInvalidInput
		}
	}
	if request.Sort == "" {
		request.Sort = "relevance"
	}
	switch request.Sort {
	case "relevance", "newest", "price_asc", "price_desc":
	default:
		return Page{}, domain.ErrInvalidInput
	}
	fingerprint := requestFingerprint(request)
	if rawCursor != "" {
		cursor, err := decodeCursor(rawCursor)
		if err != nil || cursor.Version != 1 || cursor.Fingerprint != fingerprint || len(cursor.Sort) == 0 {
			return Page{}, domain.ErrInvalidInput
		}
		request.SearchAfter = cursor.Sort
	}
	request.Limit++
	result, err := s.index.Search(ctx, request)
	if err != nil {
		return Page{}, err
	}
	hasMore := len(result.Hits) == request.Limit
	if hasMore {
		result.Hits = result.Hits[:len(result.Hits)-1]
	}
	nextCursor := ""
	if hasMore && len(result.Hits) > 0 {
		nextCursor, err = encodeCursor(cursorPayload{
			Version: 1, Fingerprint: fingerprint, Sort: result.Hits[len(result.Hits)-1].Sort,
		})
		if err != nil {
			return Page{}, fmt.Errorf("encode search cursor: %w", err)
		}
	}
	return Page{Hits: result.Hits, NextCursor: nextCursor, HasMore: hasMore, Total: result.Total, TookMS: result.TookMS}, nil
}

func (s *Service) Suggest(ctx context.Context, query string, limit int) (domain.Result, error) {
	query = strings.TrimSpace(query)
	if len([]rune(query)) < 2 || len(query) > 120 {
		return domain.Result{}, domain.ErrInvalidInput
	}
	if limit < 1 {
		limit = 8
	}
	if limit > 10 {
		return domain.Result{}, domain.ErrInvalidInput
	}
	return s.index.Suggest(ctx, query, limit)
}

func (s *Service) Reindex(ctx context.Context, catalog CatalogReader) (int, error) {
	cursor, indexed := "", 0
	for {
		books, next, hasMore, err := catalog.List(ctx, cursor, reindexPageSize)
		if err != nil {
			return indexed, fmt.Errorf("list catalog for reindex: %w", err)
		}
		if len(books) > 0 {
			if err := s.index.BulkUpsert(ctx, books); err != nil {
				return indexed, fmt.Errorf("bulk reindex catalog: %w", err)
			}
			indexed += len(books)
		}
		if !hasMore {
			return indexed, nil
		}
		if next == "" || next == cursor {
			return indexed, fmt.Errorf("catalog returned invalid reindex cursor")
		}
		cursor = next
	}
}

func documentFromEvent(book catalogevent.Book) domain.BookDocument {
	return domain.BookDocument{
		ID: book.ID, Title: book.Title, Author: book.Author, ISBN: book.ISBN,
		PriceCents: book.PriceCents, Stock: book.Stock, SellerID: book.SellerID,
		CreatedAt: book.CreatedAt, UpdatedAt: book.UpdatedAt,
	}
}

func requestFingerprint(request domain.Request) string {
	request.SearchAfter = nil
	payload, _ := json.Marshal(request)
	sum := sha256.Sum256(payload)
	return base64.RawURLEncoding.EncodeToString(sum[:12])
}

func encodeCursor(cursor cursorPayload) (string, error) {
	payload, err := json.Marshal(cursor)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(payload), nil
}

func decodeCursor(raw string) (cursorPayload, error) {
	payload, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return cursorPayload{}, err
	}
	var cursor cursorPayload
	if err := json.Unmarshal(payload, &cursor); err != nil {
		return cursorPayload{}, err
	}
	return cursor, nil
}
