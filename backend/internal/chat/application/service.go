package application

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/thinhnguyenwilliam/book-store/backend/internal/chat/domain"
	apptrace "github.com/thinhnguyenwilliam/book-store/backend/internal/platform/trace"
)

type Service struct {
	repository Repository
	authors    AuthorResolver
	now        func() time.Time
}

func NewService(repository Repository, authors AuthorResolver) *Service {
	return &Service{repository: repository, authors: authors, now: time.Now}
}

func (s *Service) CreateSupportConversation(ctx context.Context, customerID string) (*domain.Conversation, error) {
	if !validID(customerID) {
		return nil, domain.ErrInvalidInput
	}
	return s.repository.CreateSupportConversation(ctx, customerID, s.now().UTC())
}

func (s *Service) GetConversation(ctx context.Context, conversationID, userID string, isAdmin bool) (*domain.Conversation, error) {
	if !validID(conversationID) || !validID(userID) {
		return nil, domain.ErrInvalidInput
	}
	return s.repository.GetConversation(ctx, conversationID, userID, isAdmin)
}

func (s *Service) ListConversations(ctx context.Context, userID string, isAdmin bool, rawCursor string, limit int32) (domain.ConversationPage, error) {
	if !validID(userID) {
		return domain.ConversationPage{}, domain.ErrInvalidInput
	}
	limit = pageLimit(limit, 20)
	cursor, err := decodeConversationCursor(rawCursor)
	if err != nil {
		return domain.ConversationPage{}, err
	}
	items, err := s.repository.ListConversations(ctx, userID, isAdmin, limit+1, cursor)
	if err != nil {
		return domain.ConversationPage{}, err
	}
	hasMore := len(items) > int(limit)
	if hasMore {
		items = items[:limit]
	}
	page := domain.ConversationPage{Conversations: items, HasMore: hasMore}
	if hasMore {
		last := items[len(items)-1]
		page.NextCursor, err = encodeConversationCursor(domain.ConversationCursor{UpdatedAt: last.UpdatedAt, ID: last.ID})
	}
	return page, err
}

func (s *Service) ListMessages(ctx context.Context, conversationID, userID string, isAdmin bool, rawCursor string, limit int32) (domain.MessagePage, error) {
	if !validID(conversationID) || !validID(userID) {
		return domain.MessagePage{}, domain.ErrInvalidInput
	}
	limit = pageLimit(limit, 30)
	cursor, err := decodeMessageCursor(rawCursor)
	if err != nil {
		return domain.MessagePage{}, err
	}
	items, err := s.repository.ListMessages(ctx, conversationID, userID, isAdmin, limit+1, cursor)
	if err != nil {
		return domain.MessagePage{}, err
	}
	hasMore := len(items) > int(limit)
	if hasMore {
		items = items[:limit]
	}
	page := domain.MessagePage{Messages: items, HasMore: hasMore}
	if hasMore {
		page.NextCursor, err = encodeMessageCursor(items[len(items)-1].SequenceNumber)
	}
	for left, right := 0, len(items)-1; left < right; left, right = left+1, right-1 {
		items[left], items[right] = items[right], items[left]
	}
	return page, err
}

func (s *Service) SendMessage(ctx context.Context, conversationID, senderID string, isAdmin bool, clientMessageID, content string) (*domain.Message, error) {
	if !validID(conversationID) || !validID(senderID) || !validID(clientMessageID) {
		return nil, domain.ErrInvalidInput
	}
	content, err := domain.NormalizeMessage(content)
	if err != nil {
		return nil, err
	}
	senderName, err := s.authors.DisplayName(ctx, senderID)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(senderName) == "" {
		senderName = "Người dùng"
	}
	now := s.now().UTC()
	item := &domain.Message{ID: uuid.NewString(), ConversationID: conversationID, SenderID: senderID, SenderName: strings.TrimSpace(senderName), ClientMessageID: clientMessageID, Content: content, MessageType: domain.MessageText, CreatedAt: now}
	return s.repository.CreateMessage(ctx, item, isAdmin, apptrace.IDFromContext(ctx))
}

func (s *Service) UpdateMessage(ctx context.Context, id, actorID string, isAdmin bool, content string) (*domain.Message, error) {
	if !validID(id) || !validID(actorID) {
		return nil, domain.ErrInvalidInput
	}
	content, err := domain.NormalizeMessage(content)
	if err != nil {
		return nil, err
	}
	return s.repository.UpdateMessage(ctx, id, actorID, isAdmin, content, s.now().UTC())
}

func (s *Service) DeleteMessage(ctx context.Context, id, actorID string, isAdmin bool) (*domain.Message, error) {
	if !validID(id) || !validID(actorID) {
		return nil, domain.ErrInvalidInput
	}
	return s.repository.SoftDeleteMessage(ctx, id, actorID, isAdmin, s.now().UTC())
}

func (s *Service) MarkRead(ctx context.Context, conversationID, userID string, isAdmin bool, sequence int64) (int64, error) {
	if !validID(conversationID) || !validID(userID) || sequence < 0 {
		return 0, domain.ErrInvalidInput
	}
	return s.repository.MarkRead(ctx, conversationID, userID, isAdmin, sequence, s.now().UTC())
}

func (s *Service) UnreadCount(ctx context.Context, userID string, isAdmin bool) (int64, error) {
	if !validID(userID) {
		return 0, domain.ErrInvalidInput
	}
	return s.repository.UnreadCount(ctx, userID, isAdmin)
}

func pageLimit(value, fallback int32) int32 {
	if value < 1 {
		return fallback
	}
	if value > 100 {
		return 100
	}
	return value
}

func validID(value string) bool { _, err := uuid.Parse(value); return err == nil }
