package domain

import (
	"errors"
	"strings"
	"time"
)

var (
	ErrInvalidInput     = errors.New("invalid chat input")
	ErrConversationGone = errors.New("conversation not found")
	ErrMessageGone      = errors.New("message not found")
	ErrForbidden        = errors.New("chat access forbidden")
	ErrNotEditable      = errors.New("message is not editable")
	ErrIdempotency      = errors.New("message idempotency conflict")
)

const (
	ConversationSupport = "support"
	ConversationOpen    = "open"
	ConversationClosed  = "closed"
	MemberCustomer      = "customer"
	MemberAdmin         = "admin"
	MessageText         = "text"
	MaxMessageLength    = 4000
)

type Conversation struct {
	ID                  string
	CustomerID          string
	Type                string
	Status              string
	LastMessageSequence int64
	LastMessagePreview  string
	LastMessageAt       time.Time
	UnreadCount         int64
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

type Message struct {
	ID              string
	ConversationID  string
	SenderID        string
	SenderName      string
	ClientMessageID string
	SequenceNumber  int64
	Content         string
	MessageType     string
	CreatedAt       time.Time
	EditedAt        time.Time
	DeletedAt       time.Time
	AudienceIDs     []string
	AdminAudience   bool
}

type ConversationCursor struct {
	UpdatedAt time.Time
	ID        string
}

type ConversationPage struct {
	Conversations []*Conversation
	NextCursor    string
	HasMore       bool
}

type MessagePage struct {
	Messages   []*Message
	NextCursor string
	HasMore    bool
}

func NormalizeMessage(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" || len([]rune(value)) > MaxMessageLength {
		return "", ErrInvalidInput
	}
	return value, nil
}
