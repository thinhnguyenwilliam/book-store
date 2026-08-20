package domain

import (
	"errors"
	"strings"
	"time"
)

var (
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrEmailAlreadyExists = errors.New("email already exists")
	ErrInvalidInput       = errors.New("invalid account input")
	ErrInvalidToken       = errors.New("invalid access token")
)

type Account struct {
	ID           string
	Email        string
	PasswordHash string
	Roles        []string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}
