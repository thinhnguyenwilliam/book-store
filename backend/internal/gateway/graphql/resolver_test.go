package graphql

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	bookstorev1 "github.com/thinhnguyenwilliam/book-store/backend/gen/bookstore/v1"
	"google.golang.org/grpc"
)

type bookClientStub struct {
	bookstorev1.BookServiceClient
	get  func(context.Context, *bookstorev1.GetBookRequest) (*bookstorev1.Book, error)
	list func(context.Context, *bookstorev1.ListBooksRequest) (*bookstorev1.ListBooksResponse, error)
}

func (s bookClientStub) GetBook(ctx context.Context, request *bookstorev1.GetBookRequest, _ ...grpc.CallOption) (*bookstorev1.Book, error) {
	return s.get(ctx, request)
}

func (s bookClientStub) ListBooks(ctx context.Context, request *bookstorev1.ListBooksRequest, _ ...grpc.CallOption) (*bookstorev1.ListBooksResponse, error) {
	return s.list(ctx, request)
}

type commentClientStub struct {
	bookstorev1.CommentServiceClient
	list func(context.Context, *bookstorev1.ListBookCommentsRequest) (*bookstorev1.ListCommentsResponse, error)
}

func (s commentClientStub) ListBookComments(ctx context.Context, request *bookstorev1.ListBookCommentsRequest, _ ...grpc.CallOption) (*bookstorev1.ListCommentsResponse, error) {
	return s.list(ctx, request)
}

func TestBookDetailAggregatesBookAndComments(t *testing.T) {
	t.Parallel()
	resolver := &queryResolver{Resolver: &Resolver{
		Books: bookClientStub{get: func(_ context.Context, request *bookstorev1.GetBookRequest) (*bookstorev1.Book, error) {
			return &bookstorev1.Book{Id: request.GetId(), Title: "Clean Architecture", PriceCents: 399900}, nil
		}},
		Comments: commentClientStub{list: func(_ context.Context, request *bookstorev1.ListBookCommentsRequest) (*bookstorev1.ListCommentsResponse, error) {
			return &bookstorev1.ListCommentsResponse{Comments: []*bookstorev1.Comment{{Id: "comment-1", BookId: request.GetBookId(), Content: "Hay"}}, HasMore: true, NextCursor: "next"}, nil
		}},
	}}
	limit := int32(20)
	result, err := resolver.BookDetail(context.Background(), "book-1", &limit, nil)
	if err != nil {
		t.Fatalf("BookDetail() error = %v", err)
	}
	if result.Book.ID != "book-1" || len(result.Comments.Nodes) != 1 || !result.Comments.PageInfo.HasNextPage {
		t.Fatalf("unexpected aggregate: %+v", result)
	}
}

func TestAdminDashboardRequiresAdminRole(t *testing.T) {
	t.Parallel()
	resolver := &queryResolver{Resolver: &Resolver{}}
	_, err := resolver.AdminDashboard(ContextWithPrincipal(context.Background(), Principal{UserID: "user-1", Roles: []string{"customer"}}), nil, nil, nil, nil)
	if err == nil {
		t.Fatal("AdminDashboard() error = nil, want authorization error")
	}
}

func TestCatalogSnapshotCountsWholeLoadedPage(t *testing.T) {
	t.Parallel()
	items := make([]*bookstorev1.Book, 0, 7)
	for index := 0; index < 7; index++ {
		items = append(items, &bookstorev1.Book{Id: string(rune('a' + index)), Stock: int32(index), PriceCents: 100})
	}
	snapshot := catalogSnapshot(&bookstorev1.ListBooksResponse{Books: items})
	if snapshot.LowStockCount != 6 {
		t.Fatalf("LowStockCount = %d, want 6", snapshot.LowStockCount)
	}
	if len(snapshot.LowStockBooks) != 5 {
		t.Fatalf("len(LowStockBooks) = %d, want display cap 5", len(snapshot.LowStockBooks))
	}
}

func TestServerRejectsQueryAboveDepthLimit(t *testing.T) {
	t.Parallel()
	server, err := NewServer(ServerConfig{MaxComplexity: 100, MaxDepth: 2, ParserTokenLimit: 1000}, nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}
	payload, _ := json.Marshal(map[string]any{"query": `query { bookDetail(id: "book-1") { book { id } } }`})
	request := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/graphql", bytes.NewReader(payload))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	server.ServeHTTP(recorder, request)
	if !bytes.Contains(recorder.Body.Bytes(), []byte("QUERY_TOO_DEEP")) {
		t.Fatalf("response = %s, want QUERY_TOO_DEEP", recorder.Body.String())
	}
}
