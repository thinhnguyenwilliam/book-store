package trace

import (
	"context"
	"testing"
)

func TestTraceIDRoundTrip(t *testing.T) {
	id, err := NewID()
	if err != nil {
		t.Fatalf("NewID() error = %v", err)
	}
	if len(id) != IDLength || Normalize(id) != id {
		t.Fatalf("NewID() = %q, want %d lowercase hex characters", id, IDLength)
	}
	if got := IDFromContext(ContextWithID(context.Background(), id)); got != id {
		t.Fatalf("IDFromContext() = %q, want %q", got, id)
	}
}

func TestNormalizeRejectsInvalidTraceID(t *testing.T) {
	for _, id := range []string{"", "short", "zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz"} {
		if got := Normalize(id); got != "" {
			t.Fatalf("Normalize(%q) = %q, want empty", id, got)
		}
	}
}
