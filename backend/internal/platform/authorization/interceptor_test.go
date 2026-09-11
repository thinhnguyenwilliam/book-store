package authorization

import (
	"context"
	"testing"

	pb "github.com/thinhnguyenwilliam/book-store/backend/gen/bookstore/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func TestPrivilegedMethodsFailClosed(t *testing.T) {
	for method, permission := range methodPermissions {
		t.Run(method, func(t *testing.T) {
			for _, allow := range []bool{false, true} {
				guard := Interceptor(func(context.Context, string) (string, []string, error) {
					if allow {
						return "actor", []string{"admin.access", permission}, nil
					}
					return "actor", []string{"admin.access"}, nil
				})
				called := false
				next := func(context.Context, any) (any, error) { called = true; return nil, nil }
				_, err := guard(metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer token")), nil, &grpc.UnaryServerInfo{FullMethod: method}, next)
				if allow {
					if err != nil || !called {
						t.Fatalf("permission rejected: %v", err)
					}
				} else if status.Code(err) != codes.PermissionDenied || called {
					t.Fatal("missing permission accepted")
				}
				called = false
				_, err = guard(context.Background(), nil, &grpc.UnaryServerInfo{FullMethod: method}, next)
				if status.Code(err) != codes.Unauthenticated || called {
					t.Fatal("anonymous RPC accepted")
				}
			}
		})
	}
}
func TestOwnerAndChatIdentity(t *testing.T) {
	guard := Interceptor(func(context.Context, string) (string, []string, error) { return "actor", nil, nil })
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer token"))
	for _, tc := range []struct {
		name, method string
		request      any
		allowed      bool
	}{
		{"own profile", "/bookstore.v1.UserService/UpdateProfile", &pb.UpdateProfileRequest{Id: "actor"}, true},
		{"other profile", "/bookstore.v1.UserService/UpdateProfile", &pb.UpdateProfileRequest{Id: "other"}, false},
		{"own get profile", "/bookstore.v1.UserService/GetProfile", &pb.GetProfileRequest{Id: "actor"}, true},
		{"other get profile", "/bookstore.v1.UserService/GetProfile", &pb.GetProfileRequest{Id: "other"}, false},
		{"own chat", "/bookstore.v1.ChatService/ListConversations", &pb.ListConversationsRequest{UserId: "actor"}, true},
		{"spoofed chat owner", "/bookstore.v1.ChatService/ListConversations", &pb.ListConversationsRequest{UserId: "other"}, false},
		{"spoofed admin", "/bookstore.v1.ChatService/ListConversations", &pb.ListConversationsRequest{UserId: "actor", IsAdmin: true}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			called := false
			_, err := guard(ctx, tc.request, &grpc.UnaryServerInfo{FullMethod: tc.method}, func(context.Context, any) (any, error) { called = true; return nil, nil })
			if called != tc.allowed {
				t.Fatalf("allowed=%v error=%v", called, err)
			}
		})
	}
	called := false
	_, err := guard(context.Background(), &pb.GetProfileRequest{Id: "anyone"}, &grpc.UnaryServerInfo{FullMethod: "/bookstore.v1.UserService/GetProfile"}, func(context.Context, any) (any, error) {
		called = true
		return nil, nil
	})
	if err != nil || !called {
		t.Fatalf("internal GetProfile without bearer must remain available: %v", err)
	}
}
