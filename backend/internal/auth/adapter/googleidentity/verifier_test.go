package googleidentity

import "testing"

func TestNonceMatches(t *testing.T) {
	tests := []struct {
		name     string
		claims   map[string]any
		expected string
		want     bool
	}{
		{name: "exact", claims: map[string]any{"nonce": "state-123"}, expected: "state-123", want: true},
		{name: "mismatch", claims: map[string]any{"nonce": "another-state"}, expected: "state-123", want: false},
		{name: "missing", claims: map[string]any{}, expected: "state-123", want: false},
		{name: "empty expected", claims: map[string]any{"nonce": "state-123"}, expected: "", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := nonceMatches(tt.claims, tt.expected); got != tt.want {
				t.Fatalf("nonceMatches() = %t, want %t", got, tt.want)
			}
		})
	}
}
