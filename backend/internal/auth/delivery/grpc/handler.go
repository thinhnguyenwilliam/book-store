package grpc

import (
	"context"
	"errors"

	bookstorev1 "github.com/thinhnguyenwilliam/book-store/backend/gen/bookstore/v1"
	"github.com/thinhnguyenwilliam/book-store/backend/internal/auth/application"
	"github.com/thinhnguyenwilliam/book-store/backend/internal/auth/domain"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Handler struct {
	bookstorev1.UnimplementedAuthServiceServer
	service *application.Service
}

func NewHandler(service *application.Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) Register(ctx context.Context, request *bookstorev1.RegisterRequest) (*bookstorev1.AuthResponse, error) {
	result, err := h.service.Register(ctx, request.GetEmail(), request.GetPassword(), request.GetDisplayName())
	if err != nil {
		return nil, mapError(err)
	}
	return authResponse(result), nil
}

func (h *Handler) Login(ctx context.Context, request *bookstorev1.LoginRequest) (*bookstorev1.AuthResponse, error) {
	result, err := h.service.Login(ctx, request.GetEmail(), request.GetPassword())
	if err != nil {
		return nil, mapError(err)
	}
	return authResponse(result), nil
}

func (h *Handler) VerifyToken(ctx context.Context, request *bookstorev1.VerifyTokenRequest) (*bookstorev1.VerifyTokenResponse, error) {
	claims, err := h.service.VerifyToken(ctx, request.GetAccessToken())
	if err != nil {
		return nil, mapError(err)
	}
	return &bookstorev1.VerifyTokenResponse{
		UserId: claims.UserID,
		Email:  claims.Email,
		Roles:  claims.Roles,
	}, nil
}

func authResponse(result application.AuthResult) *bookstorev1.AuthResponse {
	return &bookstorev1.AuthResponse{
		AccessToken: result.AccessToken,
		UserId:      result.UserID,
		ExpiresIn:   result.ExpiresIn,
	}
}

func mapError(err error) error {
	switch {
	case errors.Is(err, domain.ErrInvalidInput):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, domain.ErrEmailAlreadyExists):
		return status.Error(codes.AlreadyExists, err.Error())
	case errors.Is(err, domain.ErrInvalidCredentials), errors.Is(err, domain.ErrInvalidToken):
		return status.Error(codes.Unauthenticated, err.Error())
	default:
		return status.Error(codes.Internal, "internal server error")
	}
}
