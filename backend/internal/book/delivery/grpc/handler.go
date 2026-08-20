package grpc

import (
	"context"
	"errors"
	"time"

	bookstorev1 "github.com/thinhnguyenwilliam/book-store/backend/gen/bookstore/v1"
	"github.com/thinhnguyenwilliam/book-store/backend/internal/book/application"
	"github.com/thinhnguyenwilliam/book-store/backend/internal/book/domain"
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
	books, total, err := h.service.List(ctx, request.GetPage(), request.GetPageSize())
	if err != nil {
		return nil, mapError(err)
	}

	result := make([]*bookstorev1.Book, 0, len(books))
	for _, book := range books {
		result = append(result, toProto(book))
	}
	return &bookstorev1.ListBooksResponse{Books: result, Total: total}, nil
}

func (h *Handler) UpdateBook(ctx context.Context, request *bookstorev1.UpdateBookRequest) (*bookstorev1.Book, error) {
	book, err := h.service.Update(ctx, &domain.Book{
		ID:         request.GetId(),
		Title:      request.GetTitle(),
		Author:     request.GetAuthor(),
		ISBN:       request.GetIsbn(),
		PriceCents: request.GetPriceCents(),
		Stock:      request.GetStock(),
	})
	if err != nil {
		return nil, mapError(err)
	}
	return toProto(book), nil
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
		CreatedAt:  book.CreatedAt.Format(time.RFC3339),
		UpdatedAt:  book.UpdatedAt.Format(time.RFC3339),
	}
}

func mapError(err error) error {
	switch {
	case errors.Is(err, domain.ErrInvalidInput):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, domain.ErrNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, domain.ErrISBNExists):
		return status.Error(codes.AlreadyExists, err.Error())
	default:
		return status.Error(codes.Internal, "internal server error")
	}
}
