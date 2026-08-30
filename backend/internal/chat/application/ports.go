package application

import (
	"context"
	"time"

	"github.com/thinhnguyenwilliam/book-store/backend/internal/chat/domain"
)

type Repository interface {
	CreateSupportConversation(context.Context, string, time.Time) (*domain.Conversation, error)
	GetConversation(context.Context, string, string, bool) (*domain.Conversation, error)
	ListConversations(context.Context, string, bool, int32, *domain.ConversationCursor) ([]*domain.Conversation, error)
	ListMessages(context.Context, string, string, bool, int32, *int64) ([]*domain.Message, error)
	CreateMessage(context.Context, *domain.Message, bool, string) (*domain.Message, error)
	UpdateMessage(context.Context, string, string, bool, string, time.Time) (*domain.Message, error)
	SoftDeleteMessage(context.Context, string, string, bool, time.Time) (*domain.Message, error)
	MarkRead(context.Context, string, string, bool, int64, time.Time) (int64, error)
	UnreadCount(context.Context, string, bool) (int64, error)
}

type AuthorResolver interface {
	DisplayName(context.Context, string) (string, error)
}
