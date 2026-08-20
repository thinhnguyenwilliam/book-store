package domain

import (
	"errors"
	"strings"
	"time"
)

var (
	ErrNotFound      = errors.New("user profile not found")
	ErrAlreadyExists = errors.New("user profile already exists")
	ErrInvalidInput  = errors.New("invalid user profile input")
)

type User struct {
	ID          string
	Email       string
	DisplayName string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (u *User) UpdateDisplayName(displayName string, now time.Time) error {
	displayName = strings.TrimSpace(displayName)
	if len(displayName) > 100 {
		return ErrInvalidInput
	}
	u.DisplayName = displayName
	u.UpdatedAt = now.UTC()
	return nil
}
