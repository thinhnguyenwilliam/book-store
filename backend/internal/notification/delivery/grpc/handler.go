package grpc

import (
	"context"
	"errors"
	"time"

	bookstorev1 "github.com/thinhnguyenwilliam/book-store/backend/gen/bookstore/v1"
	"github.com/thinhnguyenwilliam/book-store/backend/internal/notification/application"
	"github.com/thinhnguyenwilliam/book-store/backend/internal/notification/domain"
	grpcerror "github.com/thinhnguyenwilliam/book-store/backend/internal/platform/grpcerror"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Handler struct {
	bookstorev1.UnimplementedNotificationServiceServer
	service *application.Service
}

func NewHandler(service *application.Service) *Handler { return &Handler{service: service} }

func (h *Handler) ListNotifications(ctx context.Context, req *bookstorev1.ListNotificationsRequest) (*bookstorev1.ListNotificationsResponse, error) {
	page, err := h.service.List(ctx, req.GetUserId(), req.GetCursor(), req.GetLimit())
	if err != nil {
		return nil, mapError(err)
	}
	items := make([]*bookstorev1.Notification, 0, len(page.Notifications))
	for _, item := range page.Notifications {
		items = append(items, toProto(item))
	}
	return &bookstorev1.ListNotificationsResponse{Notifications: items, NextCursor: page.NextCursor, HasMore: page.HasMore}, nil
}

func (h *Handler) GetUnreadNotificationCount(ctx context.Context, req *bookstorev1.GetUnreadNotificationCountRequest) (*bookstorev1.GetUnreadNotificationCountResponse, error) {
	count, err := h.service.UnreadCount(ctx, req.GetUserId())
	if err != nil {
		return nil, mapError(err)
	}
	return &bookstorev1.GetUnreadNotificationCountResponse{Count: count}, nil
}

func (h *Handler) MarkNotificationRead(ctx context.Context, req *bookstorev1.MarkNotificationReadRequest) (*bookstorev1.Notification, error) {
	item, err := h.service.MarkRead(ctx, req.GetUserId(), req.GetId())
	if err != nil {
		return nil, mapError(err)
	}
	return toProto(item), nil
}

func (h *Handler) MarkAllNotificationsRead(ctx context.Context, req *bookstorev1.MarkAllNotificationsReadRequest) (*bookstorev1.MarkAllNotificationsReadResponse, error) {
	updated, err := h.service.MarkAllRead(ctx, req.GetUserId())
	if err != nil {
		return nil, mapError(err)
	}
	return &bookstorev1.MarkAllNotificationsReadResponse{Updated: updated}, nil
}

func (h *Handler) RegisterPushDevice(ctx context.Context, req *bookstorev1.RegisterPushDeviceRequest) (*bookstorev1.PushDevice, error) {
	item, err := h.service.RegisterDevice(ctx, req.GetUserId(), req.GetDeviceId(), req.GetApplication(), req.GetPlatform(), req.GetRegistrationToken())
	if err != nil {
		return nil, mapError(err)
	}
	return deviceToProto(item), nil
}

func (h *Handler) UnregisterPushDevice(ctx context.Context, req *bookstorev1.UnregisterPushDeviceRequest) (*bookstorev1.UnregisterPushDeviceResponse, error) {
	removed, err := h.service.UnregisterDevice(ctx, req.GetUserId(), req.GetDeviceId())
	if err != nil {
		return nil, mapError(err)
	}
	return &bookstorev1.UnregisterPushDeviceResponse{Removed: removed}, nil
}

func toProto(item *domain.Notification) *bookstorev1.Notification {
	readAt := ""
	if !item.ReadAt.IsZero() {
		readAt = item.ReadAt.Format(time.RFC3339)
	}
	return &bookstorev1.Notification{Id: item.ID, UserId: item.UserID, Type: item.Type, Title: item.Title, Body: item.Body, DataJson: string(item.Data), ReadAt: readAt, CreatedAt: item.CreatedAt.Format(time.RFC3339)}
}

func deviceToProto(item *domain.DeviceInstallation) *bookstorev1.PushDevice {
	return &bookstorev1.PushDevice{
		Id: item.ID, DeviceId: item.DeviceID, UserId: item.UserID, Application: item.Application,
		Platform: item.Platform, LastSeenAt: item.LastSeenAt.Format(time.RFC3339),
		CreatedAt: item.CreatedAt.Format(time.RFC3339), UpdatedAt: item.UpdatedAt.Format(time.RFC3339),
	}
}

func mapError(err error) error {
	if mapped := grpcerror.FromContext(err); mapped != nil {
		return mapped
	}
	switch {
	case errors.Is(err, domain.ErrInvalidInput):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, domain.ErrNotificationNotFound):
		return status.Error(codes.NotFound, err.Error())
	default:
		return status.Error(codes.Internal, "internal server error")
	}
}
