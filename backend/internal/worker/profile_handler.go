package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	bookstorev1 "github.com/thinhnguyenwilliam/book-store/backend/gen/bookstore/v1"
	"github.com/thinhnguyenwilliam/book-store/backend/internal/messaging"
)

type ProfileHandler struct {
	users bookstorev1.UserServiceClient
}

func NewProfileHandler(users bookstorev1.UserServiceClient) *ProfileHandler {
	return &ProfileHandler{users: users}
}

func (h *ProfileHandler) Handle(ctx context.Context, body []byte) error {
	var payload messaging.AccountRegisteredPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return fmt.Errorf("decode account registered event: %w", err)
	}
	if payload.UserID == "" || payload.Email == "" {
		return fmt.Errorf("account registered event has missing required fields")
	}

	requestCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	_, err := h.users.CreateProfile(requestCtx, &bookstorev1.CreateProfileRequest{
		Id:          payload.UserID,
		Email:       payload.Email,
		DisplayName: payload.DisplayName,
	})
	if err != nil {
		return fmt.Errorf("create user profile through gRPC: %w", err)
	}
	return nil
}
