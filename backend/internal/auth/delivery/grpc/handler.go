package grpc

import (
	"context"
	"errors"

	bookstorev1 "github.com/thinhnguyenwilliam/book-store/backend/gen/bookstore/v1"
	"github.com/thinhnguyenwilliam/book-store/backend/internal/auth/application"
	"github.com/thinhnguyenwilliam/book-store/backend/internal/auth/domain"
	grpcerror "github.com/thinhnguyenwilliam/book-store/backend/internal/platform/grpcerror"
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

func (h *Handler) LoginWithGoogle(
	ctx context.Context,
	request *bookstorev1.GoogleLoginRequest,
) (*bookstorev1.AuthResponse, error) {
	result, err := h.service.LoginWithGoogle(ctx, request.GetCredential(), request.GetNonce(), request.GetCreateAccount())
	if err != nil {
		return nil, mapError(err)
	}
	return authResponse(result), nil
}

func (h *Handler) LoginWithFacebook(
	ctx context.Context,
	request *bookstorev1.FacebookLoginRequest,
) (*bookstorev1.AuthResponse, error) {
	result, err := h.service.LoginWithFacebook(ctx, request.GetAccessToken(), request.GetCreateAccount())
	if err != nil {
		return nil, mapError(err)
	}
	return authResponse(result), nil
}

func (h *Handler) Refresh(ctx context.Context, request *bookstorev1.RefreshRequest) (*bookstorev1.AuthResponse, error) {
	result, err := h.service.Refresh(ctx, request.GetRefreshToken())
	if err != nil {
		return nil, mapError(err)
	}
	return authResponse(result), nil
}

func (h *Handler) Logout(ctx context.Context, request *bookstorev1.LogoutRequest) (*bookstorev1.LogoutResponse, error) {
	if err := h.service.Logout(ctx, request.GetRefreshToken()); err != nil {
		return nil, mapError(err)
	}
	return &bookstorev1.LogoutResponse{}, nil
}

func (h *Handler) DeleteAccount(ctx context.Context, request *bookstorev1.DeleteAccountRequest) (*bookstorev1.DeleteAccountResponse, error) {
	if err := h.service.DeleteAccount(ctx, request.GetId()); err != nil {
		return nil, mapError(err)
	}
	return &bookstorev1.DeleteAccountResponse{}, nil
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
		AccessToken:      result.AccessToken,
		UserId:           result.UserID,
		ExpiresIn:        result.ExpiresIn,
		RefreshToken:     result.RefreshToken,
		RefreshExpiresIn: result.RefreshExpiresIn,
	}
}

func mapError(err error) error {
	if mapped := grpcerror.FromContext(err); mapped != nil {
		return mapped
	}
	switch {
	case errors.Is(err, domain.ErrInvalidInput):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, domain.ErrEmailAlreadyExists):
		return status.Error(codes.AlreadyExists, err.Error())
	case errors.Is(err, domain.ErrIdentityConflict):
		return status.Error(codes.AlreadyExists, err.Error())
	case errors.Is(err, domain.ErrIdentityUnavailable):
		return status.Error(codes.FailedPrecondition, err.Error())
	case errors.Is(err, domain.ErrIdentityProvider):
		return status.Error(codes.Unavailable, err.Error())
	case errors.Is(err, domain.ErrNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, domain.ErrInvalidCredentials),
		errors.Is(err, domain.ErrInvalidIdentity),
		errors.Is(err, domain.ErrInvalidToken),
		errors.Is(err, domain.ErrInvalidRefreshToken),
		errors.Is(err, domain.ErrRefreshTokenReused):
		return status.Error(codes.Unauthenticated, err.Error())
	default:
		return status.Error(codes.Internal, "internal server error")
	}
}
