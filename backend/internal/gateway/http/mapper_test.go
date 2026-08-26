package http

import (
	"testing"

	bookstorev1 "github.com/thinhnguyenwilliam/book-store/backend/gen/bookstore/v1"
)

func TestAuthRequestMappers(t *testing.T) {
	register := registerProto(RegisterRequest{
		Email:       "reader@example.com",
		Password:    "password123",
		DisplayName: "Reader",
	})
	if register.GetEmail() != "reader@example.com" || register.GetDisplayName() != "Reader" {
		t.Fatalf("unexpected register protobuf: %+v", register)
	}

	google := googleLoginProto(GoogleLoginRequest{Credential: "credential", State: "state", CreateAccount: true})
	if google.GetCredential() != "credential" || google.GetNonce() != "state" || !google.GetCreateAccount() {
		t.Fatalf("unexpected Google protobuf: %+v", google)
	}

	facebook := facebookLoginProto(FacebookLoginRequest{AccessToken: "token", State: "state", CreateAccount: true})
	if facebook.GetAccessToken() != "token" || !facebook.GetCreateAccount() {
		t.Fatalf("unexpected Facebook protobuf: %+v", facebook)
	}
}

func TestBookRequestMappers(t *testing.T) {
	request := BookRequest{
		Title:      "Clean Architecture",
		Author:     "Robert C. Martin",
		ISBN:       "9780134494166",
		PriceCents: 3999,
		Stock:      10,
	}
	created := createBookProto(request)
	updated := updateBookProto("book-id", request)
	if created.GetIsbn() != request.ISBN || created.GetPriceCents() != request.PriceCents {
		t.Fatalf("unexpected create protobuf: %+v", created)
	}
	if updated.GetId() != "book-id" || updated.GetStock() != request.Stock {
		t.Fatalf("unexpected update protobuf: %+v", updated)
	}
}

func TestResponseMappersDoNotExposeProtobuf(t *testing.T) {
	books := booksJSON([]*bookstorev1.Book{{
		Id: "book-id", Title: "Domain-Driven Design", Isbn: "9780321125217",
	}})
	if len(books) != 1 || books[0].ID != "book-id" || books[0].ISBN != "9780321125217" {
		t.Fatalf("unexpected HTTP books: %+v", books)
	}

	users := usersJSON([]*bookstorev1.User{{Id: "user-id", Email: "reader@example.com"}})
	if len(users) != 1 || users[0].ID != "user-id" || users[0].Email != "reader@example.com" {
		t.Fatalf("unexpected HTTP users: %+v", users)
	}
}
