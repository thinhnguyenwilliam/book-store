package application

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/thinhnguyenwilliam/book-store/backend/internal/user/domain"
)

const userCursorVersion = 1

type userCursorPayload struct {
	Version   int    `json:"v"`
	CreatedAt string `json:"created_at"`
	ID        string `json:"id"`
}

func encodeCursor(cursor UserCursor) (string, error) {
	payload, err := json.Marshal(userCursorPayload{
		Version:   userCursorVersion,
		CreatedAt: cursor.CreatedAt.UTC().Format(time.RFC3339Nano),
		ID:        cursor.ID,
	})
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(payload), nil
}

func decodeCursor(raw string) (*UserCursor, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}

	payloadJSON, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return nil, domain.ErrInvalidInput
	}
	payload := userCursorPayload{}
	if err := json.Unmarshal(payloadJSON, &payload); err != nil || payload.Version != userCursorVersion {
		return nil, domain.ErrInvalidInput
	}
	createdAt, err := time.Parse(time.RFC3339Nano, payload.CreatedAt)
	if err != nil {
		return nil, domain.ErrInvalidInput
	}
	if _, err := uuid.Parse(payload.ID); err != nil {
		return nil, domain.ErrInvalidInput
	}
	return &UserCursor{CreatedAt: createdAt.UTC(), ID: payload.ID}, nil
}
