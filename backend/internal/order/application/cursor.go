package application

import (
	"encoding/base64"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/thinhnguyenwilliam/book-store/backend/internal/order/domain"
)

func encodeCursor(cursor domain.Cursor) (string, error) {
	payload, err := json.Marshal(cursor)
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
	var cursor domain.Cursor
	if err := json.Unmarshal(payload, &cursor); err != nil {
		return nil, domain.ErrInvalidInput
	}
	if cursor.CreatedAt.IsZero() {
		return nil, domain.ErrInvalidInput
	}
	if _, err := uuid.Parse(cursor.ID); err != nil {
		return nil, domain.ErrInvalidInput
	}
	return &cursor, nil
}
