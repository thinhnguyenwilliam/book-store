package grpcclient

import (
	"context"
	"fmt"
	"time"

	bookstorev1 "github.com/thinhnguyenwilliam/book-store/backend/gen/bookstore/v1"
	"github.com/thinhnguyenwilliam/book-store/backend/internal/search/domain"
)

type Catalog struct{ client bookstorev1.BookServiceClient }

func NewCatalog(client bookstorev1.BookServiceClient) *Catalog { return &Catalog{client: client} }

func (c *Catalog) List(ctx context.Context, cursor string, limit int32) ([]domain.BookDocument, string, bool, error) {
	response, err := c.client.ListBooks(ctx, &bookstorev1.ListBooksRequest{Cursor: cursor, Limit: limit})
	if err != nil {
		return nil, "", false, err
	}
	books := make([]domain.BookDocument, 0, len(response.GetBooks()))
	for _, book := range response.GetBooks() {
		createdAt, err := time.Parse(time.RFC3339, book.GetCreatedAt())
		if err != nil {
			return nil, "", false, fmt.Errorf("parse book created_at: %w", err)
		}
		updatedAt, err := time.Parse(time.RFC3339, book.GetUpdatedAt())
		if err != nil {
			return nil, "", false, fmt.Errorf("parse book updated_at: %w", err)
		}
		books = append(books, domain.BookDocument{
			ID: book.GetId(), Title: book.GetTitle(), Author: book.GetAuthor(), ISBN: book.GetIsbn(),
			PriceCents: book.GetPriceCents(), Stock: book.GetStock(), SellerID: book.GetSellerId(),
			CreatedAt: createdAt, UpdatedAt: updatedAt,
		})
	}
	return books, response.GetNextCursor(), response.GetHasMore(), nil
}
