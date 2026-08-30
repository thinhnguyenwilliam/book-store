package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/thinhnguyenwilliam/book-store/backend/internal/chat/domain"
)

type repositoryStub struct {
	conversation *domain.Conversation
	messages     []*domain.Message
	created      *domain.Message
}

func (r *repositoryStub) CreateSupportConversation(context.Context, string, time.Time) (*domain.Conversation, error) {
	return r.conversation, nil
}
func (r *repositoryStub) GetConversation(context.Context, string, string, bool) (*domain.Conversation, error) {
	return r.conversation, nil
}
func (r *repositoryStub) ListConversations(context.Context, string, bool, int32, *domain.ConversationCursor) ([]*domain.Conversation, error) {
	return []*domain.Conversation{r.conversation}, nil
}
func (r *repositoryStub) ListMessages(context.Context, string, string, bool, int32, *int64) ([]*domain.Message, error) {
	return r.messages, nil
}
func (r *repositoryStub) CreateMessage(_ context.Context, message *domain.Message, _ bool, _ string) (*domain.Message, error) {
	r.created = message
	return message, nil
}
func (r *repositoryStub) UpdateMessage(context.Context, string, string, bool, string, time.Time) (*domain.Message, error) {
	return nil, nil
}
func (r *repositoryStub) SoftDeleteMessage(context.Context, string, string, bool, time.Time) (*domain.Message, error) {
	return nil, nil
}
func (r *repositoryStub) MarkRead(context.Context, string, string, bool, int64, time.Time) (int64, error) {
	return 0, nil
}
func (r *repositoryStub) UnreadCount(context.Context, string, bool) (int64, error) { return 0, nil }

type authorStub struct{ name string }

func (a authorStub) DisplayName(context.Context, string) (string, error) { return a.name, nil }

func TestSendMessageNormalizesAndCarriesIdempotencyKey(t *testing.T) {
	repository := &repositoryStub{}
	service := NewService(repository, authorStub{name: " Thịnh "})
	conversationID, senderID, clientID := uuid.NewString(), uuid.NewString(), uuid.NewString()
	message, err := service.SendMessage(context.Background(), conversationID, senderID, false, clientID, "  Xin chào  ")
	if err != nil {
		t.Fatal(err)
	}
	if message.Content != "Xin chào" || message.ClientMessageID != clientID || message.SenderName != "Thịnh" {
		t.Fatalf("unexpected message: %+v", message)
	}
	if repository.created == nil || repository.created.MessageType != domain.MessageText {
		t.Fatalf("message was not passed to repository: %+v", repository.created)
	}
}

func TestListMessagesReturnsChronologicalPage(t *testing.T) {
	conversationID, userID := uuid.NewString(), uuid.NewString()
	repository := &repositoryStub{messages: []*domain.Message{{SequenceNumber: 3}, {SequenceNumber: 2}, {SequenceNumber: 1}}}
	service := NewService(repository, authorStub{})
	page, err := service.ListMessages(context.Background(), conversationID, userID, false, "", 2)
	if err != nil {
		t.Fatal(err)
	}
	if !page.HasMore || len(page.Messages) != 2 || page.Messages[0].SequenceNumber != 2 || page.Messages[1].SequenceNumber != 3 || page.NextCursor == "" {
		t.Fatalf("unexpected page: %+v", page)
	}
}

func TestSendMessageRejectsInvalidAndOversizedInput(t *testing.T) {
	service := NewService(&repositoryStub{}, authorStub{})
	_, err := service.SendMessage(context.Background(), uuid.NewString(), uuid.NewString(), false, "not-a-uuid", "hello")
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("invalid client id error = %v", err)
	}
	oversized := make([]rune, domain.MaxMessageLength+1)
	for index := range oversized {
		oversized[index] = 'a'
	}
	_, err = service.SendMessage(context.Background(), uuid.NewString(), uuid.NewString(), false, uuid.NewString(), string(oversized))
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("oversized content error = %v", err)
	}
}
