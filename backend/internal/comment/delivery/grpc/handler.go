package grpc

import (
	"context"
	"errors"
	"time"

	bookstorev1 "github.com/thinhnguyenwilliam/book-store/backend/gen/bookstore/v1"
	"github.com/thinhnguyenwilliam/book-store/backend/internal/comment/application"
	"github.com/thinhnguyenwilliam/book-store/backend/internal/comment/domain"
	grpcerror "github.com/thinhnguyenwilliam/book-store/backend/internal/platform/grpcerror"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Handler struct {
	bookstorev1.UnimplementedCommentServiceServer
	service *application.Service
}

func NewHandler(service *application.Service) *Handler { return &Handler{service: service} }

func (h *Handler) CreateComment(ctx context.Context, req *bookstorev1.CreateCommentRequest) (*bookstorev1.Comment, error) {
	item, err := h.service.Create(ctx, req.GetBookId(), req.GetAuthorId(), req.GetParentId(), req.GetContent())
	if err != nil {
		return nil, mapError(err)
	}
	return toProto(item), nil
}
func (h *Handler) ListBookComments(ctx context.Context, req *bookstorev1.ListBookCommentsRequest) (*bookstorev1.ListCommentsResponse, error) {
	page, err := h.service.ListRoots(ctx, req.GetBookId(), req.GetCursor(), req.GetLimit())
	if err != nil {
		return nil, mapError(err)
	}
	return pageProto(page), nil
}
func (h *Handler) ListCommentReplies(ctx context.Context, req *bookstorev1.ListCommentRepliesRequest) (*bookstorev1.ListCommentsResponse, error) {
	page, err := h.service.ListReplies(ctx, req.GetRootId(), req.GetCursor(), req.GetLimit())
	if err != nil {
		return nil, mapError(err)
	}
	return pageProto(page), nil
}
func (h *Handler) UpdateComment(ctx context.Context, req *bookstorev1.UpdateCommentRequest) (*bookstorev1.Comment, error) {
	item, err := h.service.Update(ctx, req.GetId(), req.GetAuthorId(), req.GetContent())
	if err != nil {
		return nil, mapError(err)
	}
	return toProto(item), nil
}
func (h *Handler) DeleteComment(ctx context.Context, req *bookstorev1.DeleteCommentRequest) (*bookstorev1.Comment, error) {
	item, err := h.service.Delete(ctx, req.GetId(), req.GetActorId(), req.GetIsAdmin())
	if err != nil {
		return nil, mapError(err)
	}
	return toProto(item), nil
}
func (h *Handler) ModerateComment(ctx context.Context, req *bookstorev1.ModerateCommentRequest) (*bookstorev1.Comment, error) {
	item, err := h.service.Moderate(ctx, req.GetId(), req.GetStatus())
	if err != nil {
		return nil, mapError(err)
	}
	return toProto(item), nil
}

func pageProto(page domain.Page) *bookstorev1.ListCommentsResponse {
	items := make([]*bookstorev1.Comment, 0, len(page.Comments))
	for _, item := range page.Comments {
		items = append(items, toProto(item))
	}
	return &bookstorev1.ListCommentsResponse{Comments: items, NextCursor: page.NextCursor, HasMore: page.HasMore}
}
func toProto(item *domain.Comment) *bookstorev1.Comment {
	content, authorName := item.Content, item.AuthorName
	switch item.Status {
	case domain.StatusDeleted:
		content, authorName = "Bình luận đã được xoá.", ""
	case domain.StatusHidden:
		content, authorName = "Bình luận đã được ẩn.", ""
	}
	return &bookstorev1.Comment{Id: item.ID, BookId: item.BookID, AuthorId: item.AuthorID, AuthorName: authorName, ParentId: item.ParentID, RootId: item.RootID, Depth: item.Depth, Content: content, Status: item.Status, ReplyCount: item.ReplyCount, CreatedAt: item.CreatedAt.Format(time.RFC3339), UpdatedAt: item.UpdatedAt.Format(time.RFC3339)}
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
	case errors.Is(err, domain.ErrForbidden):
		return status.Error(codes.PermissionDenied, err.Error())
	case errors.Is(err, domain.ErrParentMismatch), errors.Is(err, domain.ErrMaxDepth), errors.Is(err, domain.ErrNotEditable):
		return status.Error(codes.FailedPrecondition, err.Error())
	default:
		return status.Error(codes.Internal, "internal server error")
	}
}
