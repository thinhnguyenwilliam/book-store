// Package authorization guards privileged gRPC entry points independently of HTTP.
package authorization

import (
	"context"
	"slices"
	"strings"
	"time"

	pb "github.com/thinhnguyenwilliam/book-store/backend/gen/bookstore/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type Verify func(context.Context, string) (string, []string, error)

func Remote(address string) (grpc.UnaryServerInterceptor, func(), error) {
	conn, err := grpc.NewClient(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, nil, err
	}
	client := pb.NewAuthServiceClient(conn)
	check := func(ctx context.Context, token string) (string, []string, error) {
		ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()
		result, err := client.VerifyToken(ctx, &pb.VerifyTokenRequest{AccessToken: token})
		if err != nil {
			return "", nil, err
		}
		return result.GetUserId(), result.GetPermissions(), nil
	}
	return Interceptor(check), func() { _ = conn.Close() }, nil
}

var methodPermissions = map[string]string{
	"/bookstore.v1.BookService/CreateBook":                        "books.create",
	"/bookstore.v1.BookService/UpdateBook":                        "books.update",
	"/bookstore.v1.BookService/DeleteBook":                        "books.delete",
	"/bookstore.v1.UserService/ListProfiles":                      "customers.read",
	"/bookstore.v1.AuthService/DeleteAccount":                     "customers.delete",
	"/bookstore.v1.PaymentService/UpdateBalance":                  "wallets.adjust",
	"/bookstore.v1.CommentService/ModerateComment":                "comments.moderate",
	"/bookstore.v1.AnalyticsService/GetOrderAnalytics":            "analytics.read",
	"/bookstore.v1.AnalyticsService/GetCustomerActivityAnalytics": "analytics.read",
}

func Interceptor(verify Verify) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, next grpc.UnaryHandler) (any, error) {
		permission := methodPermissions[info.FullMethod]
		var owner string
		var actor string
		requiresIdentity := false
		if value, ok := req.(*pb.UpdateProfileRequest); ok {
			owner = value.GetId()
			permission = "customers.update"
		}
		if value, ok := req.(*pb.GetProfileRequest); ok {
			md, _ := metadata.FromIncomingContext(ctx)
			if len(md.Get("authorization")) == 0 {
				// Notification/comment/chat resolve display names over the private network.
				return next(ctx, req)
			}
			owner = value.GetId()
			permission = "customers.read"
		}
		if strings.HasPrefix(info.FullMethod, "/bookstore.v1.ChatService/") {
			requiresIdentity = true
			actor = requestActor(req)
			if value, ok := req.(interface{ GetIsAdmin() bool }); ok && value.GetIsAdmin() {
				permission = "chat.read"
				if strings.Contains(info.FullMethod, "SendMessage") || strings.Contains(info.FullMethod, "UpdateMessage") || strings.Contains(info.FullMethod, "DeleteMessage") {
					permission = "chat.reply"
				}
			}
		}
		if value, ok := req.(*pb.DeleteCommentRequest); ok && value.GetIsAdmin() {
			permission = "comments.moderate"
		}
		switch value := req.(type) {
		case *pb.CreateCommentRequest:
			requiresIdentity = true
			actor = value.GetAuthorId()
		case *pb.UpdateCommentRequest:
			requiresIdentity = true
			actor = value.GetAuthorId()
		case *pb.DeleteCommentRequest:
			requiresIdentity = true
			actor = value.GetActorId()
		}
		if permission == "" && !requiresIdentity {
			return next(ctx, req)
		}
		md, _ := metadata.FromIncomingContext(ctx)
		values := md.Get("authorization")
		if len(values) != 1 {
			return nil, status.Error(codes.Unauthenticated, "bearer token required")
		}
		parts := strings.Fields(values[0])
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			return nil, status.Error(codes.Unauthenticated, "bearer token required")
		}
		id, permissions, err := verify(ctx, parts[1])
		if err != nil {
			return nil, err
		}
		if id == "" || (requiresIdentity && actor != id) || (permission != "" && owner != id && (!slices.Contains(permissions, permission) || !slices.Contains(permissions, "admin.access"))) {
			return nil, status.Error(codes.PermissionDenied, "insufficient permissions")
		}
		if value, ok := req.(*pb.DeleteAccountRequest); ok && value.GetId() == id {
			return nil, status.Error(codes.PermissionDenied, "cannot delete own account through administration")
		}
		return next(ctx, req)
	}
}

func requestActor(req any) string {
	switch value := req.(type) {
	case interface{ GetUserId() string }:
		return value.GetUserId()
	case interface{ GetSenderId() string }:
		return value.GetSenderId()
	case interface{ GetActorId() string }:
		return value.GetActorId()
	case interface{ GetCustomerId() string }:
		return value.GetCustomerId()
	default:
		return ""
	}
}
