package grpc

import (
	"context"
	"errors"
	"time"

	bookstorev1 "github.com/thinhnguyenwilliam/book-store/backend/gen/bookstore/v1"
	grpcerror "github.com/thinhnguyenwilliam/book-store/backend/internal/platform/grpcerror"
	searchapp "github.com/thinhnguyenwilliam/book-store/backend/internal/search/application"
	"github.com/thinhnguyenwilliam/book-store/backend/internal/search/domain"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Handler struct {
	bookstorev1.UnimplementedSearchServiceServer
	service *searchapp.Service
}

func NewHandler(service *searchapp.Service) *Handler { return &Handler{service: service} }

func (h *Handler) SearchBooks(ctx context.Context, request *bookstorev1.SearchBooksRequest) (*bookstorev1.SearchBooksResponse, error) {
	filters := domain.Filters{
		MinPriceCents: request.GetMinPriceCents(), MaxPriceCents: request.GetMaxPriceCents(),
		SellerID: request.GetSellerId(), Author: request.GetAuthor(),
	}
	if request.InStock != nil {
		value := request.GetInStock()
		filters.InStock = &value
	}
	page, err := h.service.Search(ctx, domain.Request{
		Query: request.GetQuery(), Limit: int(request.GetLimit()), Filters: filters, Sort: request.GetSort(),
	}, request.GetCursor())
	if err != nil {
		return nil, mapError(err)
	}
	hits := make([]*bookstorev1.SearchBookHit, 0, len(page.Hits))
	for _, hit := range page.Hits {
		hits = append(hits, hitProto(hit))
	}
	return &bookstorev1.SearchBooksResponse{
		Hits: hits, NextCursor: page.NextCursor, HasMore: page.HasMore,
		Total: page.Total, TookMs: page.TookMS,
	}, nil
}

func (h *Handler) SuggestBooks(ctx context.Context, request *bookstorev1.SuggestBooksRequest) (*bookstorev1.SuggestBooksResponse, error) {
	result, err := h.service.Suggest(ctx, request.GetQuery(), int(request.GetLimit()))
	if err != nil {
		return nil, mapError(err)
	}
	hits := make([]*bookstorev1.SearchBookHit, 0, len(result.Hits))
	for _, hit := range result.Hits {
		hits = append(hits, hitProto(hit))
	}
	return &bookstorev1.SuggestBooksResponse{Hits: hits, TookMs: result.TookMS}, nil
}

func hitProto(hit domain.Hit) *bookstorev1.SearchBookHit {
	book := hit.Book
	return &bookstorev1.SearchBookHit{
		Book: &bookstorev1.Book{
			Id: book.ID, Title: book.Title, Author: book.Author, Isbn: book.ISBN,
			PriceCents: book.PriceCents, Stock: book.Stock, SellerId: book.SellerID,
			CreatedAt: book.CreatedAt.Format(time.RFC3339), UpdatedAt: book.UpdatedAt.Format(time.RFC3339),
		},
		Score: hit.Score, Highlights: hit.Highlights,
	}
}

func mapError(err error) error {
	if mapped := grpcerror.FromContext(err); mapped != nil {
		return mapped
	}
	switch {
	case errors.Is(err, domain.ErrInvalidInput):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, domain.ErrUnavailable):
		return status.Error(codes.Unavailable, err.Error())
	default:
		return status.Error(codes.Internal, "internal server error")
	}
}
