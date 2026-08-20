package security

import "testing"

func TestRefreshTokenManagerGeneratesRandomOpaqueTokensAndStableHashes(t *testing.T) {
	manager := NewRefreshTokenManager()
	first, firstHash, err := manager.Generate()
	if err != nil {
		t.Fatalf("Generate() first error = %v", err)
	}
	second, secondHash, err := manager.Generate()
	if err != nil {
		t.Fatalf("Generate() second error = %v", err)
	}
	if first == second || firstHash == secondHash {
		t.Fatal("refresh tokens must be random")
	}
	if len(firstHash) != 64 || manager.Hash(first) != firstHash {
		t.Fatalf("unexpected SHA-256 hash: %q", firstHash)
	}
	if first == firstHash {
		t.Fatal("raw refresh token must not equal its stored hash")
	}
}
