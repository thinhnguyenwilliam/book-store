package domain

import (
	"encoding/json"
	"errors"
	"time"
)

var (
	ErrInvalidInput            = errors.New("invalid notification input")
	ErrNotificationNotFound    = errors.New("notification not found")
	ErrPushRegistrationInvalid = errors.New("push registration is invalid")
)

const (
	EmailPending = "pending"
	EmailSending = "sending"
	EmailSent    = "sent"
	EmailFailed  = "failed"
	EmailSkipped = "skipped"
	PushPending  = "pending"
	PushSending  = "sending"
	PushSent     = "sent"
	PushFailed   = "failed"
	PushSkipped  = "skipped"
)

type Notification struct {
	ID        string
	UserID    string
	EventID   string
	Type      string
	Title     string
	Body      string
	Data      json.RawMessage
	ReadAt    time.Time
	CreatedAt time.Time
}

type EmailDelivery struct {
	ID        string
	EventID   string
	UserID    string
	Recipient string
	Subject   string
	Body      string
	Status    string
	Attempts  int
	LastError string
	SentAt    time.Time
	CreatedAt time.Time
	UpdatedAt time.Time
}

type PushMessage struct {
	Title string
	Body  string
	Data  json.RawMessage
}

type PushDelivery struct {
	ID                string
	EventID           string
	NotificationID    string
	UserID            string
	InstallationID    string
	RegistrationToken string
	Application       string
	Platform          string
	NotificationType  string
	Title             string
	Body              string
	Data              json.RawMessage
	Status            string
	Attempts          int
	LastError         string
	ProviderMessageID string
	SentAt            time.Time
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type DeviceInstallation struct {
	ID                string
	DeviceID          string
	UserID            string
	Platform          string
	RegistrationToken string
	Application       string
	LastSeenAt        time.Time
	DisabledAt        time.Time
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type Event struct {
	ID           string
	Type         string
	Payload      json.RawMessage
	Notification Notification
	Email        *EmailDelivery
	Push         *PushMessage
	ReceivedAt   time.Time
}

type Recipient struct {
	UserID      string
	Email       string
	DisplayName string
}

type Cursor struct {
	CreatedAt time.Time
	ID        string
}

type Page struct {
	Notifications []*Notification
	NextCursor    string
	HasMore       bool
}
