package worker

import (
	"context"
	"testing"

	bookstorev1 "github.com/thinhnguyenwilliam/book-store/backend/gen/bookstore/v1"
	"github.com/thinhnguyenwilliam/book-store/backend/internal/messaging"
	"google.golang.org/grpc"
)

type userClientStub struct {
	bookstorev1.UserServiceClient
	createdID string
	deletedID string
}

func (s *userClientStub) CreateProfile(_ context.Context, request *bookstorev1.CreateProfileRequest, _ ...grpc.CallOption) (*bookstorev1.User, error) {
	s.createdID = request.GetId()
	return &bookstorev1.User{Id: request.GetId()}, nil
}

func (s *userClientStub) DeleteProfile(_ context.Context, request *bookstorev1.DeleteProfileRequest, _ ...grpc.CallOption) (*bookstorev1.DeleteProfileResponse, error) {
	s.deletedID = request.GetId()
	return &bookstorev1.DeleteProfileResponse{}, nil
}

func TestProfileHandlerDispatchesAccountDeleted(t *testing.T) {
	client := &userClientStub{}
	handler := NewProfileHandler(client)

	err := handler.Handle(
		context.Background(),
		messaging.EventAccountDeleted,
		[]byte(`{"user_id":"6b92edb2-f406-43dd-851a-ac7acb13cfc2","deleted_at":"2026-08-20T10:00:00Z"}`),
	)
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if client.deletedID != "6b92edb2-f406-43dd-851a-ac7acb13cfc2" {
		t.Fatalf("deleted ID = %q", client.deletedID)
	}
}

func TestProfileHandlerRejectsUnknownEvent(t *testing.T) {
	handler := NewProfileHandler(&userClientStub{})
	if err := handler.Handle(context.Background(), "unknown.event", []byte(`{}`)); err == nil {
		t.Fatal("Handle() error = nil, want unsupported event error")
	}
}
