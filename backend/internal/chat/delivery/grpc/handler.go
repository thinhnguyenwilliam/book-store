package grpc

import (
	"context"
	"errors"
	"time"

	bookstorev1 "github.com/thinhnguyenwilliam/book-store/backend/gen/bookstore/v1"
	"github.com/thinhnguyenwilliam/book-store/backend/internal/chat/application"
	"github.com/thinhnguyenwilliam/book-store/backend/internal/chat/domain"
	grpcerror "github.com/thinhnguyenwilliam/book-store/backend/internal/platform/grpcerror"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Handler struct {
	bookstorev1.UnimplementedChatServiceServer
	service *application.Service
}

func NewHandler(service *application.Service) *Handler { return &Handler{service: service} }

func (h *Handler) CreateSupportConversation(ctx context.Context, req *bookstorev1.CreateSupportConversationRequest) (*bookstorev1.Conversation, error) {
	item, err := h.service.CreateSupportConversation(ctx, req.GetCustomerId())
	if err != nil {
		return nil, mapError(err)
	}
	return conversationProto(item), nil
}

func (h *Handler) GetConversation(ctx context.Context, req *bookstorev1.GetConversationRequest) (*bookstorev1.Conversation, error) {
	item, err := h.service.GetConversation(ctx, req.GetConversationId(), req.GetUserId(), req.GetIsAdmin())
	if err != nil {
		return nil, mapError(err)
	}
	return conversationProto(item), nil
}

func (h *Handler) ListConversations(ctx context.Context, req *bookstorev1.ListConversationsRequest) (*bookstorev1.ListConversationsResponse, error) {
	page, err := h.service.ListConversations(ctx, req.GetUserId(), req.GetIsAdmin(), req.GetCursor(), req.GetLimit())
	if err != nil {
		return nil, mapError(err)
	}
	items := make([]*bookstorev1.Conversation, 0, len(page.Conversations))
	for _, item := range page.Conversations {
		items = append(items, conversationProto(item))
	}
	return &bookstorev1.ListConversationsResponse{Conversations: items, NextCursor: page.NextCursor, HasMore: page.HasMore}, nil
}

func (h *Handler) ListMessages(ctx context.Context, req *bookstorev1.ListMessagesRequest) (*bookstorev1.ListMessagesResponse, error) {
	page, err := h.service.ListMessages(ctx, req.GetConversationId(), req.GetUserId(), req.GetIsAdmin(), req.GetCursor(), req.GetLimit())
	if err != nil {
		return nil, mapError(err)
	}
	items := make([]*bookstorev1.Message, 0, len(page.Messages))
	for _, item := range page.Messages {
		items = append(items, messageProto(item))
	}
	return &bookstorev1.ListMessagesResponse{Messages: items, NextCursor: page.NextCursor, HasMore: page.HasMore}, nil
}

func (h *Handler) SendMessage(ctx context.Context, req *bookstorev1.SendMessageRequest) (*bookstorev1.Message, error) {
	item, err := h.service.SendMessage(ctx, req.GetConversationId(), req.GetSenderId(), req.GetIsAdmin(), req.GetClientMessageId(), req.GetContent())
	if err != nil {
		return nil, mapError(err)
	}
	return messageProto(item), nil
}

func (h *Handler) UpdateMessage(ctx context.Context, req *bookstorev1.UpdateMessageRequest) (*bookstorev1.Message, error) {
	item, err := h.service.UpdateMessage(ctx, req.GetId(), req.GetActorId(), req.GetIsAdmin(), req.GetContent())
	if err != nil {
		return nil, mapError(err)
	}
	return messageProto(item), nil
}

func (h *Handler) DeleteMessage(ctx context.Context, req *bookstorev1.DeleteMessageRequest) (*bookstorev1.Message, error) {
	item, err := h.service.DeleteMessage(ctx, req.GetId(), req.GetActorId(), req.GetIsAdmin())
	if err != nil {
		return nil, mapError(err)
	}
	return messageProto(item), nil
}

func (h *Handler) MarkConversationRead(ctx context.Context, req *bookstorev1.MarkConversationReadRequest) (*bookstorev1.MarkConversationReadResponse, error) {
	sequence, err := h.service.MarkRead(ctx, req.GetConversationId(), req.GetUserId(), req.GetIsAdmin(), req.GetSequenceNumber())
	if err != nil {
		return nil, mapError(err)
	}
	return &bookstorev1.MarkConversationReadResponse{LastReadSequence: sequence}, nil
}

func (h *Handler) GetUnreadChatCount(ctx context.Context, req *bookstorev1.GetUnreadChatCountRequest) (*bookstorev1.GetUnreadChatCountResponse, error) {
	count, err := h.service.UnreadCount(ctx, req.GetUserId(), req.GetIsAdmin())
	if err != nil {
		return nil, mapError(err)
	}
	return &bookstorev1.GetUnreadChatCountResponse{Count: count}, nil
}

func conversationProto(item *domain.Conversation) *bookstorev1.Conversation {
	return &bookstorev1.Conversation{Id: item.ID, CustomerId: item.CustomerID, Type: item.Type, Status: item.Status, LastMessageSequence: item.LastMessageSequence, LastMessagePreview: item.LastMessagePreview, LastMessageAt: formatTime(item.LastMessageAt), UnreadCount: item.UnreadCount, CreatedAt: formatTime(item.CreatedAt), UpdatedAt: formatTime(item.UpdatedAt)}
}

func messageProto(item *domain.Message) *bookstorev1.Message {
	content := item.Content
	if !item.DeletedAt.IsZero() {
		content = "Tin nhắn đã được xoá."
	}
	return &bookstorev1.Message{Id: item.ID, ConversationId: item.ConversationID, SenderId: item.SenderID, SenderName: item.SenderName, ClientMessageId: item.ClientMessageID, SequenceNumber: item.SequenceNumber, Content: content, MessageType: item.MessageType, CreatedAt: formatTime(item.CreatedAt), EditedAt: formatTime(item.EditedAt), DeletedAt: formatTime(item.DeletedAt), AudienceIds: item.AudienceIDs, AdminAudience: item.AdminAudience}
}

func formatTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.Format(time.RFC3339Nano)
}

func mapError(err error) error {
	if mapped := grpcerror.FromContext(err); mapped != nil {
		return mapped
	}
	switch {
	case errors.Is(err, domain.ErrInvalidInput):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, domain.ErrConversationGone), errors.Is(err, domain.ErrMessageGone):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, domain.ErrForbidden):
		return status.Error(codes.PermissionDenied, err.Error())
	case errors.Is(err, domain.ErrNotEditable), errors.Is(err, domain.ErrIdempotency):
		return status.Error(codes.FailedPrecondition, err.Error())
	default:
		return status.Error(codes.Internal, "internal server error")
	}
}
