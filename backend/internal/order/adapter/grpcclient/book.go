package grpcclient

import (
	"context"

	bookstorev1 "github.com/thinhnguyenwilliam/book-store/backend/gen/bookstore/v1"
	"github.com/thinhnguyenwilliam/book-store/backend/internal/order/domain"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type BookClient struct {
	client bookstorev1.BookServiceClient
}

func NewBookClient(client bookstorev1.BookServiceClient) *BookClient {
	return &BookClient{client: client}
}

func (c *BookClient) GetBook(ctx context.Context, bookID string) (domain.BookSnapshot, error) {
	book, err := c.client.GetBook(ctx, &bookstorev1.GetBookRequest{Id: bookID})
	if err != nil {
		return domain.BookSnapshot{}, err
	}
	return domain.BookSnapshot{
		ID: book.GetId(), SellerID: book.GetSellerId(), Title: book.GetTitle(), PriceCents: book.GetPriceCents(),
	}, nil
}

func (c *BookClient) ReserveStock(
	ctx context.Context,
	orderID, bookID string,
	quantity int32,
	idempotencyKey string,
) error {
	_, err := c.client.ReserveStock(ctx, &bookstorev1.ReserveStockRequest{
		OrderId: orderID, BookId: bookID, Quantity: quantity, IdempotencyKey: idempotencyKey,
	})
	return err
}

func (c *BookClient) CommitStock(ctx context.Context, orderID, bookID string) error {
	_, err := c.client.CommitStock(ctx, &bookstorev1.CommitStockRequest{OrderId: orderID, BookId: bookID})
	return err
}

func (c *BookClient) ReleaseStock(ctx context.Context, orderID, bookID string) error {
	_, err := c.client.ReleaseStock(ctx, &bookstorev1.ReleaseStockRequest{OrderId: orderID, BookId: bookID})
	// Compensation is at-least-once. A missing reservation means there is
	// nothing left to release and is therefore an idempotent success.
	if status.Code(err) == codes.NotFound {
		return nil
	}
	return err
}
