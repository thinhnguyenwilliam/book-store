package events

import (
	"context"

	"github.com/thinhnguyenwilliam/book-store/backend/internal/messaging/rabbitmq"
	"github.com/thinhnguyenwilliam/book-store/backend/internal/notification/application"
)

type Handler struct{ service *application.Service }

func NewHandler(service *application.Service) *Handler { return &Handler{service: service} }

func (h *Handler) Handle(ctx context.Context, eventType string, payload []byte) error {
	return h.service.HandleEvent(ctx, rabbitmq.EventIDFromContext(ctx), eventType, payload)
}
