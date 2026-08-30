package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/thinhnguyenwilliam/book-store/backend/internal/chat/domain"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const eventMessageCreated = "chat.message.created"

type Repository struct{ db *gorm.DB }

func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }

type conversationModel struct {
	ID                  string `gorm:"type:uuid;primaryKey"`
	CustomerID          string `gorm:"type:uuid;not null"`
	Type                string
	Status              string
	LastMessageSequence int64
	LastMessagePreview  string
	LastMessageAt       *time.Time
	UnreadCount         int64     `gorm:"column:unread_count;->"`
	CreatedAt           time.Time `gorm:"not null"`
	UpdatedAt           time.Time `gorm:"not null"`
}

type memberModel struct {
	ConversationID   string `gorm:"type:uuid;primaryKey"`
	UserID           string `gorm:"type:uuid;primaryKey"`
	MemberRole       string
	LastReadSequence int64
	JoinedAt         time.Time
	LeftAt           *time.Time
}

type messageModel struct {
	ID              string `gorm:"type:uuid;primaryKey"`
	ConversationID  string `gorm:"type:uuid;not null"`
	SenderID        string `gorm:"type:uuid;not null"`
	SenderName      string
	ClientMessageID string `gorm:"type:uuid;not null"`
	SequenceNumber  int64
	Content         string
	MessageType     string
	CreatedAt       time.Time
	EditedAt        *time.Time
	DeletedAt       *time.Time
	AudienceIDs     []string `gorm:"-"`
	AdminAudience   bool     `gorm:"-"`
}

type outboxModel struct {
	ID          string `gorm:"type:uuid;primaryKey"`
	AggregateID string `gorm:"type:uuid;not null"`
	EventType   string
	TraceID     string
	Payload     []byte `gorm:"type:jsonb"`
	AvailableAt time.Time
	CreatedAt   time.Time
}

func (r *Repository) CreateSupportConversation(ctx context.Context, customerID string, now time.Time) (*domain.Conversation, error) {
	var result conversationModel
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		candidate := conversationModel{ID: uuid.NewString(), CustomerID: customerID, Type: domain.ConversationSupport, Status: domain.ConversationOpen, CreatedAt: now, UpdatedAt: now}
		created := tx.Table("chat.conversations").Clauses(clause.OnConflict{DoNothing: true}).Create(&candidate)
		if created.Error != nil {
			return created.Error
		}
		if err := tx.Table("chat.conversations").Where("customer_id = ? AND type = ? AND status = ?", customerID, domain.ConversationSupport, domain.ConversationOpen).First(&result).Error; err != nil {
			return err
		}
		member := memberModel{ConversationID: result.ID, UserID: customerID, MemberRole: domain.MemberCustomer, JoinedAt: now}
		return tx.Table("chat.conversation_members").Clauses(clause.OnConflict{DoNothing: true}).Create(&member).Error
	})
	if err != nil {
		return nil, fmt.Errorf("create support conversation: %w", err)
	}
	return conversationDomain(result), nil
}

func (r *Repository) GetConversation(ctx context.Context, conversationID, userID string, isAdmin bool) (*domain.Conversation, error) {
	record, err := authorizeConversation(r.db.WithContext(ctx), conversationID, userID, isAdmin)
	if err != nil {
		return nil, err
	}
	return conversationDomain(record), nil
}

func (r *Repository) ListConversations(ctx context.Context, userID string, isAdmin bool, limit int32, cursor *domain.ConversationCursor) ([]*domain.Conversation, error) {
	selectSQL := `c.*, GREATEST(c.last_message_sequence - COALESCE(member.last_read_sequence, 0), 0) AS unread_count`
	query := r.db.WithContext(ctx).Table("chat.conversations AS c").Select(selectSQL).
		Joins("LEFT JOIN chat.conversation_members AS member ON member.conversation_id = c.id AND member.user_id = ? AND member.left_at IS NULL", userID)
	if !isAdmin {
		query = query.Where("member.user_id IS NOT NULL")
	}
	if cursor != nil {
		query = query.Where("(c.updated_at, c.id) < (?, ?)", cursor.UpdatedAt, cursor.ID)
	}
	var records []conversationModel
	if err := query.Order("c.updated_at DESC, c.id DESC").Limit(int(limit)).Find(&records).Error; err != nil {
		return nil, fmt.Errorf("list conversations: %w", err)
	}
	items := make([]*domain.Conversation, 0, len(records))
	for _, record := range records {
		items = append(items, conversationDomain(record))
	}
	return items, nil
}

func (r *Repository) ListMessages(ctx context.Context, conversationID, userID string, isAdmin bool, limit int32, cursor *int64) ([]*domain.Message, error) {
	if _, err := authorizeConversation(r.db.WithContext(ctx), conversationID, userID, isAdmin); err != nil {
		return nil, err
	}
	query := r.db.WithContext(ctx).Table("chat.messages").Where("conversation_id = ?", conversationID)
	if cursor != nil {
		query = query.Where("sequence_number < ?", *cursor)
	}
	var records []messageModel
	if err := query.Order("sequence_number DESC").Limit(int(limit)).Find(&records).Error; err != nil {
		return nil, fmt.Errorf("list messages: %w", err)
	}
	items := make([]*domain.Message, 0, len(records))
	for _, record := range records {
		items = append(items, messageDomain(record))
	}
	return items, nil
}

func (r *Repository) CreateMessage(ctx context.Context, item *domain.Message, isAdmin bool, traceID string) (*domain.Message, error) {
	var result messageModel
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		conversation, err := lockConversation(tx, item.ConversationID)
		if err != nil {
			return err
		}
		if !isAdmin && conversation.CustomerID != item.SenderID {
			return domain.ErrForbidden
		}
		if conversation.Status != domain.ConversationOpen {
			return domain.ErrNotEditable
		}
		var existing messageModel
		err = tx.Table("chat.messages").Where("sender_id = ? AND client_message_id = ?", item.SenderID, item.ClientMessageID).First(&existing).Error
		if err == nil {
			if existing.ConversationID != item.ConversationID || existing.Content != item.Content {
				return domain.ErrIdempotency
			}
			result = existing
			return loadAudience(tx, conversation, &result)
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if isAdmin {
			member := memberModel{ConversationID: conversation.ID, UserID: item.SenderID, MemberRole: domain.MemberAdmin, JoinedAt: item.CreatedAt}
			if err := tx.Table("chat.conversation_members").Clauses(clause.OnConflict{DoNothing: true}).Create(&member).Error; err != nil {
				return err
			}
		}
		item.SequenceNumber = conversation.LastMessageSequence + 1
		result = messageRecord(item)
		if err := tx.Table("chat.messages").Create(&result).Error; err != nil {
			return err
		}
		if err := tx.Table("chat.conversations").Where("id = ?", conversation.ID).Updates(map[string]any{
			"last_message_sequence": item.SequenceNumber,
			"last_message_preview":  preview(item.Content),
			"last_message_at":       item.CreatedAt,
			"updated_at":            item.CreatedAt,
		}).Error; err != nil {
			return err
		}
		// Sending a message means the sender has seen the conversation through
		// this sequence; otherwise their own messages would inflate unread_count.
		if err := tx.Table("chat.conversation_members").Where("conversation_id = ? AND user_id = ?", conversation.ID, item.SenderID).
			Update("last_read_sequence", gorm.Expr("GREATEST(last_read_sequence, ?)", item.SequenceNumber)).Error; err != nil {
			return err
		}
		var recipientIDs []string
		if err := tx.Table("chat.conversation_members").Where("conversation_id = ? AND user_id <> ? AND left_at IS NULL", conversation.ID, item.SenderID).Pluck("user_id", &recipientIDs).Error; err != nil {
			return err
		}
		if err := createOutboxEvents(tx, item, recipientIDs, traceID); err != nil {
			return err
		}
		return loadAudience(tx, conversation, &result)
	})
	if err != nil {
		return nil, mapRepositoryError("create message", err)
	}
	return messageDomain(result), nil
}

func (r *Repository) UpdateMessage(ctx context.Context, id, actorID string, _ bool, content string, now time.Time) (*domain.Message, error) {
	var result messageModel
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := findMessage(tx, id, &result); err != nil {
			return err
		}
		if result.SenderID != actorID {
			return domain.ErrForbidden
		}
		if result.DeletedAt != nil {
			return domain.ErrNotEditable
		}
		if err := tx.Table("chat.messages").Where("id = ?", id).Updates(map[string]any{"content": content, "edited_at": now}).Error; err != nil {
			return err
		}
		result.Content, result.EditedAt = content, &now
		if err := updateLastPreview(tx, result.ConversationID, result.SequenceNumber, preview(content), now); err != nil {
			return err
		}
		conversation, err := authorizeConversation(tx, result.ConversationID, actorID, true)
		if err != nil {
			return err
		}
		return loadAudience(tx, conversation, &result)
	})
	if err != nil {
		return nil, mapRepositoryError("update message", err)
	}
	return messageDomain(result), nil
}

func (r *Repository) SoftDeleteMessage(ctx context.Context, id, actorID string, isAdmin bool, now time.Time) (*domain.Message, error) {
	var result messageModel
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := findMessage(tx, id, &result); err != nil {
			return err
		}
		if !isAdmin && result.SenderID != actorID {
			return domain.ErrForbidden
		}
		if result.DeletedAt != nil {
			conversation, err := authorizeConversation(tx, result.ConversationID, actorID, true)
			if err != nil {
				return err
			}
			return loadAudience(tx, conversation, &result)
		}
		if err := tx.Table("chat.messages").Where("id = ?", id).Update("deleted_at", now).Error; err != nil {
			return err
		}
		result.DeletedAt = &now
		if err := updateLastPreview(tx, result.ConversationID, result.SequenceNumber, "Tin nhắn đã được xoá.", now); err != nil {
			return err
		}
		conversation, err := authorizeConversation(tx, result.ConversationID, actorID, true)
		if err != nil {
			return err
		}
		return loadAudience(tx, conversation, &result)
	})
	if err != nil {
		return nil, mapRepositoryError("delete message", err)
	}
	return messageDomain(result), nil
}

func (r *Repository) MarkRead(ctx context.Context, conversationID, userID string, isAdmin bool, sequence int64, now time.Time) (int64, error) {
	var lastRead int64
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		conversation, err := lockConversation(tx, conversationID)
		if err != nil {
			return err
		}
		if !isAdmin && conversation.CustomerID != userID {
			return domain.ErrForbidden
		}
		if sequence == 0 || sequence > conversation.LastMessageSequence {
			sequence = conversation.LastMessageSequence
		}
		role := domain.MemberCustomer
		if isAdmin {
			role = domain.MemberAdmin
		}
		member := memberModel{ConversationID: conversationID, UserID: userID, MemberRole: role, LastReadSequence: sequence, JoinedAt: now}
		if err := tx.Table("chat.conversation_members").Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "conversation_id"}, {Name: "user_id"}},
			DoUpdates: clause.Assignments(map[string]any{"last_read_sequence": gorm.Expr("GREATEST(\"conversation_members\".\"last_read_sequence\", EXCLUDED.\"last_read_sequence\")"), "left_at": nil}),
		}).Create(&member).Error; err != nil {
			return err
		}
		return tx.Table("chat.conversation_members").Select("last_read_sequence").Where("conversation_id = ? AND user_id = ?", conversationID, userID).Scan(&lastRead).Error
	})
	if err != nil {
		return 0, mapRepositoryError("mark conversation read", err)
	}
	return lastRead, nil
}

func (r *Repository) UnreadCount(ctx context.Context, userID string, isAdmin bool) (int64, error) {
	query := r.db.WithContext(ctx).Table("chat.conversations AS c").
		Joins("LEFT JOIN chat.conversation_members AS member ON member.conversation_id = c.id AND member.user_id = ? AND member.left_at IS NULL", userID).
		Select("COALESCE(SUM(GREATEST(c.last_message_sequence - COALESCE(member.last_read_sequence, 0), 0)), 0)")
	if !isAdmin {
		query = query.Where("member.user_id IS NOT NULL")
	}
	var count int64
	if err := query.Scan(&count).Error; err != nil {
		return 0, fmt.Errorf("count unread chat messages: %w", err)
	}
	return count, nil
}

func authorizeConversation(db *gorm.DB, conversationID, userID string, isAdmin bool) (conversationModel, error) {
	var conversation conversationModel
	err := db.Table("chat.conversations").Where("id = ?", conversationID).First(&conversation).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return conversation, domain.ErrConversationGone
	}
	if err != nil {
		return conversation, fmt.Errorf("find conversation: %w", err)
	}
	if isAdmin || conversation.CustomerID == userID {
		return conversation, nil
	}
	return conversation, domain.ErrForbidden
}

func lockConversation(tx *gorm.DB, id string) (conversationModel, error) {
	var conversation conversationModel
	err := tx.Table("chat.conversations").Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", id).First(&conversation).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return conversation, domain.ErrConversationGone
	}
	return conversation, err
}

func findMessage(tx *gorm.DB, id string, destination *messageModel) error {
	err := tx.Table("chat.messages").Where("id = ?", id).First(destination).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.ErrMessageGone
	}
	return err
}

func createOutboxEvents(tx *gorm.DB, item *domain.Message, recipients []string, traceID string) error {
	for _, recipientID := range recipients {
		payload, err := json.Marshal(map[string]any{
			"message_id": item.ID, "conversation_id": item.ConversationID,
			"sender_id": item.SenderID, "sender_name": item.SenderName,
			"recipient_id": recipientID, "preview": preview(item.Content),
			"occurred_at": item.CreatedAt.Format(time.RFC3339Nano),
		})
		if err != nil {
			return err
		}
		event := outboxModel{ID: uuid.NewString(), AggregateID: item.ID, EventType: eventMessageCreated, TraceID: traceID, Payload: payload, AvailableAt: item.CreatedAt, CreatedAt: item.CreatedAt}
		if err := tx.Table("chat.outbox_events").Create(&event).Error; err != nil {
			return err
		}
	}
	return nil
}

func loadAudience(tx *gorm.DB, conversation conversationModel, message *messageModel) error {
	var ids []string
	if err := tx.Table("chat.conversation_members").Where("conversation_id = ? AND left_at IS NULL", conversation.ID).Pluck("user_id", &ids).Error; err != nil {
		return err
	}
	message.AudienceIDs = ids
	message.AdminAudience = true
	return nil
}

func updateLastPreview(tx *gorm.DB, conversationID string, sequence int64, value string, now time.Time) error {
	return tx.Table("chat.conversations").Where("id = ? AND last_message_sequence = ?", conversationID, sequence).
		Updates(map[string]any{"last_message_preview": value, "updated_at": now}).Error
}

func preview(value string) string {
	value = strings.Join(strings.Fields(value), " ")
	runes := []rune(value)
	if len(runes) > 300 {
		return string(runes[:299]) + "…"
	}
	return value
}

func mapRepositoryError(operation string, err error) error {
	if errors.Is(err, domain.ErrConversationGone) || errors.Is(err, domain.ErrMessageGone) || errors.Is(err, domain.ErrForbidden) || errors.Is(err, domain.ErrNotEditable) || errors.Is(err, domain.ErrIdempotency) {
		return err
	}
	return fmt.Errorf("%s: %w", operation, err)
}

func conversationDomain(record conversationModel) *domain.Conversation {
	item := &domain.Conversation{ID: record.ID, CustomerID: record.CustomerID, Type: record.Type, Status: record.Status, LastMessageSequence: record.LastMessageSequence, LastMessagePreview: record.LastMessagePreview, UnreadCount: record.UnreadCount, CreatedAt: record.CreatedAt, UpdatedAt: record.UpdatedAt}
	if record.LastMessageAt != nil {
		item.LastMessageAt = *record.LastMessageAt
	}
	return item
}

func messageRecord(item *domain.Message) messageModel {
	return messageModel{ID: item.ID, ConversationID: item.ConversationID, SenderID: item.SenderID, SenderName: item.SenderName, ClientMessageID: item.ClientMessageID, SequenceNumber: item.SequenceNumber, Content: item.Content, MessageType: item.MessageType, CreatedAt: item.CreatedAt}
}

func messageDomain(record messageModel) *domain.Message {
	item := &domain.Message{ID: record.ID, ConversationID: record.ConversationID, SenderID: record.SenderID, SenderName: record.SenderName, ClientMessageID: record.ClientMessageID, SequenceNumber: record.SequenceNumber, Content: record.Content, MessageType: record.MessageType, CreatedAt: record.CreatedAt, AudienceIDs: record.AudienceIDs, AdminAudience: record.AdminAudience}
	if record.EditedAt != nil {
		item.EditedAt = *record.EditedAt
	}
	if record.DeletedAt != nil {
		item.DeletedAt = *record.DeletedAt
	}
	return item
}
