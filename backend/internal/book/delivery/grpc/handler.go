package grpc

import (
	"context"
	"errors"
	"time"

	bookstorev1 "github.com/thinhnguyenwilliam/book-store/backend/gen/bookstore/v1"
	"github.com/thinhnguyenwilliam/book-store/backend/internal/book/application"
	"github.com/thinhnguyenwilliam/book-store/backend/internal/book/domain"
	grpcerror "github.com/thinhnguyenwilliam/book-store/backend/internal/platform/grpcerror"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Handler struct {
	bookstorev1.UnimplementedBookServiceServer
	service *application.Service
}

func NewHandler(service *application.Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) CreateBook(ctx context.Context, request *bookstorev1.CreateBookRequest) (*bookstorev1.Book, error) {
	book, err := h.service.Create(ctx, &domain.Book{
		Title:      request.GetTitle(),
		Author:     request.GetAuthor(),
		ISBN:       request.GetIsbn(),
		PriceCents: request.GetPriceCents(),
		Stock:      request.GetStock(),
		SellerID:   request.GetSellerId(),
	})
	if err != nil {
		return nil, mapError(err)
	}
	return toProto(book), nil
}

func (h *Handler) GetBook(ctx context.Context, request *bookstorev1.GetBookRequest) (*bookstorev1.Book, error) {
	book, err := h.service.Get(ctx, request.GetId())
	if err != nil {
		return nil, mapError(err)
	}
	return toProto(book), nil
}

func (h *Handler) ListBooks(ctx context.Context, request *bookstorev1.ListBooksRequest) (*bookstorev1.ListBooksResponse, error) {
	page, err := h.service.List(ctx, request.GetCursor(), request.GetLimit())
	if err != nil {
		return nil, mapError(err)
	}

	result := make([]*bookstorev1.Book, 0, len(page.Books))
	for _, book := range page.Books {
		result = append(result, toProto(book))
	}
	return &bookstorev1.ListBooksResponse{
		Books:      result,
		NextCursor: page.NextCursor,
		HasMore:    page.HasMore,
	}, nil
}

func (h *Handler) UpdateBook(ctx context.Context, request *bookstorev1.UpdateBookRequest) (*bookstorev1.Book, error) {
	book, err := h.service.Update(ctx, &domain.Book{
		ID:         request.GetId(),
		Title:      request.GetTitle(),
		Author:     request.GetAuthor(),
		ISBN:       request.GetIsbn(),
		PriceCents: request.GetPriceCents(),
		Stock:      request.GetStock(),
		SellerID:   request.GetSellerId(),
	})
	if err != nil {
		return nil, mapError(err)
	}
	return toProto(book), nil
}

func (h *Handler) ReserveStock(
	ctx context.Context,
	request *bookstorev1.ReserveStockRequest,
) (*bookstorev1.StockReservation, error) {
	reservation, err := h.service.ReserveStock(
		ctx,
		request.GetOrderId(),
		request.GetBookId(),
		request.GetQuantity(),
		request.GetIdempotencyKey(),
	)
	if err != nil {
		return nil, mapError(err)
	}
	return reservationProto(reservation), nil
}

func (h *Handler) CommitStock(
	ctx context.Context,
	request *bookstorev1.CommitStockRequest,
) (*bookstorev1.StockReservation, error) {
	reservation, err := h.service.CommitStock(ctx, request.GetOrderId(), request.GetBookId())
	if err != nil {
		return nil, mapError(err)
	}
	return reservationProto(reservation), nil
}

func (h *Handler) ReleaseStock(
	ctx context.Context,
	request *bookstorev1.ReleaseStockRequest,
) (*bookstorev1.StockReservation, error) {
	reservation, err := h.service.ReleaseStock(ctx, request.GetOrderId(), request.GetBookId())
	if err != nil {
		return nil, mapError(err)
	}
	return reservationProto(reservation), nil
}

func (h *Handler) DeleteBook(ctx context.Context, request *bookstorev1.DeleteBookRequest) (*bookstorev1.DeleteBookResponse, error) {
	if err := h.service.Delete(ctx, request.GetId()); err != nil {
		return nil, mapError(err)
	}
	return &bookstorev1.DeleteBookResponse{}, nil
}

func toProto(book *domain.Book) *bookstorev1.Book {
	return &bookstorev1.Book{
		Id:         book.ID,
		Title:      book.Title,
		Author:     book.Author,
		Isbn:       book.ISBN,
		PriceCents: book.PriceCents,
		Stock:      book.Stock,
		SellerId:   book.SellerID,
		CreatedAt:  book.CreatedAt.Format(time.RFC3339),
		UpdatedAt:  book.UpdatedAt.Format(time.RFC3339),
	}
}

func reservationProto(reservation *domain.StockReservation) *bookstorev1.StockReservation {
	return &bookstorev1.StockReservation{
		Id:        reservation.ID,
		OrderId:   reservation.OrderID,
		BookId:    reservation.BookID,
		Quantity:  reservation.Quantity,
		Status:    reservation.Status,
		ExpiresAt: reservation.ExpiresAt.Format(time.RFC3339),
	}
}

func mapError(err error) error {
	if mapped := grpcerror.FromContext(err); mapped != nil {
		return mapped
	}
	switch {
	case errors.Is(err, domain.ErrInvalidInput):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, domain.ErrNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, domain.ErrISBNExists):
		return status.Error(codes.AlreadyExists, err.Error())
	case errors.Is(err, domain.ErrInsufficientStock):
		return status.Error(codes.FailedPrecondition, err.Error())
	case errors.Is(err, domain.ErrReservationMissing):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, domain.ErrReservationState):
		return status.Error(codes.FailedPrecondition, err.Error())
	default:
		return status.Error(codes.Internal, "internal server error")
	}
}
