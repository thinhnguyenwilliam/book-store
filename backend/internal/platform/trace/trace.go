package trace

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"strings"
)

const (
	Header      = "X-Trace-ID"
	MetadataKey = "x-trace-id"
	IDLength    = 32
)

type contextKey struct{}

func NewID() (string, error) {
	bytes := make([]byte, IDLength/2)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

func Normalize(id string) string {
	id = strings.ToLower(strings.TrimSpace(id))
	if len(id) != IDLength {
		return ""
	}
	if _, err := hex.DecodeString(id); err != nil {
		return ""
	}
	return id
}

func ContextWithID(ctx context.Context, id string) context.Context {
	if normalized := Normalize(id); normalized != "" {
		return context.WithValue(ctx, contextKey{}, normalized)
	}
	return ctx
}

func IDFromContext(ctx context.Context) string {
	id, _ := ctx.Value(contextKey{}).(string)
	return id
}
