package security

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
)

const refreshTokenBytes = 32

type RefreshTokenManager struct{}

func NewRefreshTokenManager() *RefreshTokenManager {
	return &RefreshTokenManager{}
}

func (*RefreshTokenManager) Generate() (string, string, error) {
	buffer := make([]byte, refreshTokenBytes)
	if _, err := rand.Read(buffer); err != nil {
		return "", "", err
	}
	rawToken := base64.RawURLEncoding.EncodeToString(buffer)
	return rawToken, hashRefreshToken(rawToken), nil
}

func (*RefreshTokenManager) Hash(rawToken string) string {
	return hashRefreshToken(rawToken)
}

func hashRefreshToken(rawToken string) string {
	digest := sha256.Sum256([]byte(rawToken))
	return hex.EncodeToString(digest[:])
}
