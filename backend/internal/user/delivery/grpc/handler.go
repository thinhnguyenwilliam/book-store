package grpc

import (
	"context"
	"errors"
	"time"

	bookstorev1 "github.com/thinhnguyenwilliam/book-store/backend/gen/bookstore/v1"
	"github.com/thinhnguyenwilliam/book-store/backend/internal/user/application"
	"github.com/thinhnguyenwilliam/book-store/backend/internal/user/domain"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Handler struct {
	bookstorev1.UnimplementedUserServiceServer
	service *application.Service
}

func NewHandler(service *application.Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) CreateProfile(ctx context.Context, request *bookstorev1.CreateProfileRequest) (*bookstorev1.User, error) {
	user, err := h.service.Create(ctx, request.GetId(), request.GetEmail(), request.GetDisplayName())
	if err != nil {
		return nil, mapError(err)
	}
	return toProto(user), nil
}

func (h *Handler) GetProfile(ctx context.Context, request *bookstorev1.GetProfileRequest) (*bookstorev1.User, error) {
	user, err := h.service.Get(ctx, request.GetId())
	if err != nil {
		return nil, mapError(err)
	}
	return toProto(user), nil
}

func (h *Handler) UpdateProfile(ctx context.Context, request *bookstorev1.UpdateProfileRequest) (*bookstorev1.User, error) {
	user, err := h.service.Update(ctx, request.GetId(), request.GetDisplayName())
	if err != nil {
		return nil, mapError(err)
	}
	return toProto(user), nil
}

func toProto(user *domain.User) *bookstorev1.User {
	return &bookstorev1.User{
		Id:          user.ID,
		Email:       user.Email,
		DisplayName: user.DisplayName,
		CreatedAt:   user.CreatedAt.Format(time.RFC3339),
		UpdatedAt:   user.UpdatedAt.Format(time.RFC3339),
	}
}

func mapError(err error) error {
	switch {
	case errors.Is(err, domain.ErrInvalidInput):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, domain.ErrNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, domain.ErrAlreadyExists):
		return status.Error(codes.AlreadyExists, err.Error())
	default:
		return status.Error(codes.Internal, "internal server error")
	}
}
