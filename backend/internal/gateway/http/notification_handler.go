package http

import (
	"encoding/json"
	"net/http"

	"github.com/labstack/echo/v4"
	bookstorev1 "github.com/thinhnguyenwilliam/book-store/backend/gen/bookstore/v1"
)

type NotificationResponse struct {
	ID        string         `json:"id"`
	Type      string         `json:"type"`
	Title     string         `json:"title"`
	Body      string         `json:"body"`
	Data      map[string]any `json:"data"`
	ReadAt    string         `json:"read_at,omitempty"`
	CreatedAt string         `json:"created_at"`
}

type NotificationListResponse struct {
	Data       []NotificationResponse `json:"data"`
	Pagination CursorPagination       `json:"pagination"`
}

type UnreadCountResponse struct {
	Count int64 `json:"count"`
}
type MarkAllReadResponse struct {
	Updated int64 `json:"updated"`
}

type PushDeviceRequest struct {
	DeviceID          string `json:"device_id" validate:"required"`
	RegistrationToken string `json:"registration_token" validate:"required"`
	Application       string `json:"application" validate:"required"`
	Platform          string `json:"platform" validate:"required"`
}

type PushDeviceResponse struct {
	ID          string `json:"id"`
	DeviceID    string `json:"device_id"`
	Application string `json:"application"`
	Platform    string `json:"platform"`
	LastSeenAt  string `json:"last_seen_at"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

// listNotifications godoc
// @Summary List current user's notifications
// @Tags Notifications
// @Security BearerAuth
// @Produce json
// @Param limit query int false "Page size (1-100)"
// @Param cursor query string false "Opaque cursor"
// @Success 200 {object} NotificationListResponse
// @Router /api/v1/notifications [get]
func (h *Handler) listNotifications(c echo.Context) error {
	response, err := h.notifications.ListNotifications(grpcContext(c), &bookstorev1.ListNotificationsRequest{UserId: principalFromContext(c).UserID, Limit: int32Query(c, "limit", 20), Cursor: c.QueryParam("cursor")})
	if err != nil {
		return errorResponse(c, err)
	}
	items := make([]NotificationResponse, 0, len(response.GetNotifications()))
	for _, item := range response.GetNotifications() {
		items = append(items, notificationJSON(item))
	}
	return c.JSON(http.StatusOK, NotificationListResponse{Data: items, Pagination: CursorPagination{NextCursor: response.GetNextCursor(), HasMore: response.GetHasMore()}})
}

// unreadNotificationCount godoc
// @Summary Get unread notification count
// @Tags Notifications
// @Security BearerAuth
// @Produce json
// @Success 200 {object} UnreadCountResponse
// @Router /api/v1/notifications/unread-count [get]
func (h *Handler) unreadNotificationCount(c echo.Context) error {
	response, err := h.notifications.GetUnreadNotificationCount(grpcContext(c), &bookstorev1.GetUnreadNotificationCountRequest{UserId: principalFromContext(c).UserID})
	if err != nil {
		return errorResponse(c, err)
	}
	return c.JSON(http.StatusOK, UnreadCountResponse{Count: response.GetCount()})
}

// markNotificationRead godoc
// @Summary Mark a notification as read
// @Tags Notifications
// @Security BearerAuth
// @Produce json
// @Param id path string true "Notification ID"
// @Success 200 {object} NotificationResponse
// @Router /api/v1/notifications/{id}/read [put]
func (h *Handler) markNotificationRead(c echo.Context) error {
	response, err := h.notifications.MarkNotificationRead(grpcContext(c), &bookstorev1.MarkNotificationReadRequest{UserId: principalFromContext(c).UserID, Id: c.Param("id")})
	if err != nil {
		return errorResponse(c, err)
	}
	return c.JSON(http.StatusOK, notificationJSON(response))
}

// markAllNotificationsRead godoc
// @Summary Mark all notifications as read
// @Tags Notifications
// @Security BearerAuth
// @Produce json
// @Success 200 {object} MarkAllReadResponse
// @Router /api/v1/notifications/read-all [put]
func (h *Handler) markAllNotificationsRead(c echo.Context) error {
	response, err := h.notifications.MarkAllNotificationsRead(grpcContext(c), &bookstorev1.MarkAllNotificationsReadRequest{UserId: principalFromContext(c).UserID})
	if err != nil {
		return errorResponse(c, err)
	}
	return c.JSON(http.StatusOK, MarkAllReadResponse{Updated: response.GetUpdated()})
}

// registerPushDevice godoc
// @Summary Register or refresh an FCM device installation
// @Tags Notifications
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param payload body PushDeviceRequest true "Push device"
// @Success 201 {object} PushDeviceResponse
// @Router /api/v1/notifications/devices [post]
func (h *Handler) registerPushDevice(c echo.Context) error {
	var request PushDeviceRequest
	if err := c.Bind(&request); err != nil {
		return errorResponse(c, err)
	}
	response, err := h.notifications.RegisterPushDevice(grpcContext(c), &bookstorev1.RegisterPushDeviceRequest{
		UserId: principalFromContext(c).UserID, DeviceId: request.DeviceID,
		RegistrationToken: request.RegistrationToken, Application: request.Application, Platform: request.Platform,
	})
	if err != nil {
		return errorResponse(c, err)
	}
	return c.JSON(http.StatusCreated, pushDeviceJSON(response))
}

// unregisterPushDevice godoc
// @Summary Unregister the current user's push device
// @Tags Notifications
// @Security BearerAuth
// @Param device_id path string true "Stable client device ID"
// @Success 204
// @Router /api/v1/notifications/devices/{device_id} [delete]
func (h *Handler) unregisterPushDevice(c echo.Context) error {
	_, err := h.notifications.UnregisterPushDevice(grpcContext(c), &bookstorev1.UnregisterPushDeviceRequest{
		UserId: principalFromContext(c).UserID, DeviceId: c.Param("device_id"),
	})
	if err != nil {
		return errorResponse(c, err)
	}
	return c.NoContent(http.StatusNoContent)
}

func notificationJSON(item *bookstorev1.Notification) NotificationResponse {
	data := map[string]any{}
	_ = json.Unmarshal([]byte(item.GetDataJson()), &data)
	return NotificationResponse{ID: item.GetId(), Type: item.GetType(), Title: item.GetTitle(), Body: item.GetBody(), Data: data, ReadAt: item.GetReadAt(), CreatedAt: item.GetCreatedAt()}
}

func pushDeviceJSON(item *bookstorev1.PushDevice) PushDeviceResponse {
	return PushDeviceResponse{
		ID: item.GetId(), DeviceID: item.GetDeviceId(), Application: item.GetApplication(), Platform: item.GetPlatform(),
		LastSeenAt: item.GetLastSeenAt(), CreatedAt: item.GetCreatedAt(), UpdatedAt: item.GetUpdatedAt(),
	}
}
