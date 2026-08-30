package application

import (
	"encoding/base64"
	"encoding/json"
	"time"

	"github.com/thinhnguyenwilliam/book-store/backend/internal/chat/domain"
)

type cursorPayload struct {
	Version  int    `json:"v"`
	Kind     string `json:"k"`
	Time     string `json:"t,omitempty"`
	ID       string `json:"id,omitempty"`
	Sequence int64  `json:"s,omitempty"`
}

func encodeConversationCursor(cursor domain.ConversationCursor) (string, error) {
	return encodeCursor(cursorPayload{Version: 1, Kind: "conversation", Time: cursor.UpdatedAt.Format(time.RFC3339Nano), ID: cursor.ID})
}

func decodeConversationCursor(raw string) (*domain.ConversationCursor, error) {
	if raw == "" {
		return nil, nil
	}
	payload, err := decodeCursor(raw, "conversation")
	if err != nil {
		return nil, err
	}
	parsed, err := time.Parse(time.RFC3339Nano, payload.Time)
	if err != nil || payload.ID == "" {
		return nil, domain.ErrInvalidInput
	}
	return &domain.ConversationCursor{UpdatedAt: parsed, ID: payload.ID}, nil
}

func encodeMessageCursor(sequence int64) (string, error) {
	return encodeCursor(cursorPayload{Version: 1, Kind: "message", Sequence: sequence})
}

func decodeMessageCursor(raw string) (*int64, error) {
	if raw == "" {
		return nil, nil
	}
	payload, err := decodeCursor(raw, "message")
	if err != nil || payload.Sequence < 1 {
		return nil, domain.ErrInvalidInput
	}
	return &payload.Sequence, nil
}

func encodeCursor(payload cursorPayload) (string, error) {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(encoded), nil
}

func decodeCursor(raw, kind string) (cursorPayload, error) {
	decoded, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return cursorPayload{}, domain.ErrInvalidInput
	}
	var payload cursorPayload
	if err := json.Unmarshal(decoded, &payload); err != nil || payload.Version != 1 || payload.Kind != kind {
		return cursorPayload{}, domain.ErrInvalidInput
	}
	return payload, nil
}
