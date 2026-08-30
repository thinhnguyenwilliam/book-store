package application

import (
	"encoding/base64"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/thinhnguyenwilliam/book-store/backend/internal/comment/domain"
)

type cursorPayload struct {
	Version   int    `json:"v"`
	CreatedAt string `json:"created_at"`
	ID        string `json:"id"`
}

func encodeCursor(cursor domain.Cursor) (string, error) {
	payload, err := json.Marshal(cursorPayload{Version: 1, CreatedAt: cursor.CreatedAt.UTC().Format(time.RFC3339Nano), ID: cursor.ID})
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(payload), nil
}

func decodeCursor(raw string) (*domain.Cursor, error) {
	if raw == "" {
		return nil, nil
	}
	payload, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return nil, domain.ErrInvalidInput
	}
	var item cursorPayload
	if err := json.Unmarshal(payload, &item); err != nil || item.Version != 1 {
		return nil, domain.ErrInvalidInput
	}
	createdAt, err := time.Parse(time.RFC3339Nano, item.CreatedAt)
	if err != nil {
		return nil, domain.ErrInvalidInput
	}
	if _, err := uuid.Parse(item.ID); err != nil {
		return nil, domain.ErrInvalidInput
	}
	return &domain.Cursor{CreatedAt: createdAt, ID: item.ID}, nil
}
