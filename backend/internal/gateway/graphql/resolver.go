package graphql

import (
	"context"
	"errors"
	"sort"
	"strings"

	bookstorev1 "github.com/thinhnguyenwilliam/book-store/backend/gen/bookstore/v1"
	"github.com/thinhnguyenwilliam/book-store/backend/internal/gateway/graphql/model"
	"github.com/vektah/gqlparser/v2/gqlerror"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Principal struct {
	UserID string
	Email  string
	Roles  []string
}

type principalContextKey struct{}

func ContextWithPrincipal(ctx context.Context, principal Principal) context.Context {
	return context.WithValue(ctx, principalContextKey{}, principal)
}

func principalFromContext(ctx context.Context) Principal {
	principal, _ := ctx.Value(principalContextKey{}).(Principal)
	return principal
}

func requireAuthenticated(ctx context.Context) (Principal, error) {
	principal := principalFromContext(ctx)
	if strings.TrimSpace(principal.UserID) == "" {
		return Principal{}, graphError("UNAUTHENTICATED", "authentication required")
	}
	return principal, nil
}

func requireAdmin(ctx context.Context) error {
	principal, err := requireAuthenticated(ctx)
	if err != nil {
		return err
	}
	for _, role := range principal.Roles {
		if role == "admin" {
			return nil
		}
	}
	return graphError("FORBIDDEN", "admin role required")
}

func graphError(code, message string) error {
	return &gqlerror.Error{Message: message, Extensions: map[string]any{"code": code}}
}

func grpcGraphError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.Canceled) {
		return graphError("CANCELED", "request canceled")
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return graphError("DEADLINE_EXCEEDED", "upstream service timed out")
	}
	item, ok := status.FromError(err)
	if !ok {
		return graphError("INTERNAL_SERVER_ERROR", "internal server error")
	}
	switch item.Code() {
	case codes.InvalidArgument, codes.OutOfRange:
		return graphError("BAD_USER_INPUT", item.Message())
	case codes.Unauthenticated:
		return graphError("UNAUTHENTICATED", item.Message())
	case codes.PermissionDenied:
		return graphError("FORBIDDEN", item.Message())
	case codes.NotFound:
		return graphError("NOT_FOUND", item.Message())
	case codes.AlreadyExists, codes.Aborted:
		return graphError("CONFLICT", item.Message())
	case codes.FailedPrecondition:
		return graphError("FAILED_PRECONDITION", item.Message())
	case codes.ResourceExhausted:
		return graphError("RESOURCE_EXHAUSTED", item.Message())
	case codes.Canceled:
		return graphError("CANCELED", "request canceled")
	case codes.DeadlineExceeded:
		return graphError("DEADLINE_EXCEEDED", "upstream service timed out")
	case codes.Unavailable:
		return graphError("SERVICE_UNAVAILABLE", "upstream service unavailable")
	default:
		return graphError("INTERNAL_SERVER_ERROR", "internal server error")
	}
}

type Resolver struct {
	Books    bookstorev1.BookServiceClient
	Users    bookstorev1.UserServiceClient
	Orders   bookstorev1.OrderServiceClient
	Payments bookstorev1.PaymentServiceClient
	Comments bookstorev1.CommentServiceClient
}

func pageLimit(value *int32, fallback int32) (int32, error) {
	limit := fallback
	if value != nil {
		limit = *value
	}
	if limit < 1 || limit > 100 {
		return 0, graphError("BAD_USER_INPUT", "page size must be between 1 and 100")
	}
	return limit, nil
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func optionalString(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func bookModel(item *bookstorev1.Book) *model.Book {
	return &model.Book{
		ID: item.GetId(), Title: item.GetTitle(), Author: item.GetAuthor(), Isbn: item.GetIsbn(),
		PriceCents: item.GetPriceCents(), Stock: item.GetStock(), SellerID: item.GetSellerId(),
		CreatedAt: item.GetCreatedAt(), UpdatedAt: item.GetUpdatedAt(),
	}
}

func commentModel(item *bookstorev1.Comment) *model.Comment {
	return &model.Comment{
		ID: item.GetId(), BookID: item.GetBookId(), AuthorID: item.GetAuthorId(),
		AuthorName: item.GetAuthorName(), ParentID: optionalString(item.GetParentId()), RootID: item.GetRootId(),
		Depth: item.GetDepth(), Content: item.GetContent(), Status: item.GetStatus(),
		ReplyCount: item.GetReplyCount(), CreatedAt: item.GetCreatedAt(), UpdatedAt: item.GetUpdatedAt(),
	}
}

func customerModel(item *bookstorev1.User) *model.Customer {
	return &model.Customer{
		ID: item.GetId(), Email: item.GetEmail(), DisplayName: item.GetDisplayName(),
		CreatedAt: item.GetCreatedAt(), UpdatedAt: item.GetUpdatedAt(),
	}
}

func orderModel(item *bookstorev1.Order) *model.Order {
	items := make([]*model.OrderItem, 0, len(item.GetItems()))
	for _, orderItem := range item.GetItems() {
		items = append(items, &model.OrderItem{
			ID: orderItem.GetId(), BookID: orderItem.GetBookId(), SellerID: orderItem.GetSellerId(),
			Title: orderItem.GetTitle(), UnitPriceCents: orderItem.GetUnitPriceCents(),
			Quantity: orderItem.GetQuantity(), SubtotalCents: orderItem.GetSubtotalCents(),
		})
	}
	return &model.Order{
		ID: item.GetId(), UserID: item.GetUserId(), Status: item.GetStatus(), TotalCents: item.GetTotalCents(),
		Currency: item.GetCurrency(), Items: items, PaymentID: optionalString(item.GetPaymentId()),
		FailureReason: optionalString(item.GetFailureReason()), ReservationExpiresAt: optionalString(item.GetReservationExpiresAt()),
		CreatedAt: item.GetCreatedAt(), UpdatedAt: item.GetUpdatedAt(),
	}
}

func paymentModel(item *bookstorev1.Payment) *model.Payment {
	if item == nil {
		return nil
	}
	return &model.Payment{
		ID: item.GetId(), OrderID: item.GetOrderId(), BuyerID: item.GetBuyerId(), Status: item.GetStatus(),
		AmountCents: item.GetAmountCents(), PlatformFeeCents: item.GetPlatformFeeCents(), Currency: item.GetCurrency(),
		FailureReason: optionalString(item.GetFailureReason()), Provider: item.GetProvider(),
		ProviderTransactionID: optionalString(item.GetProviderTransactionId()), CheckoutURL: optionalString(item.GetCheckoutUrl()),
		ExpiresAt: optionalString(item.GetExpiresAt()), PaidAt: optionalString(item.GetPaidAt()),
		CreatedAt: item.GetCreatedAt(), UpdatedAt: item.GetUpdatedAt(),
	}
}

func catalogSnapshot(response *bookstorev1.ListBooksResponse) *model.CatalogSnapshot {
	books := make([]*model.Book, 0, len(response.GetBooks()))
	lowStock := make([]*model.Book, 0)
	var inventoryUnits, inventoryValue int64
	for _, item := range response.GetBooks() {
		book := bookModel(item)
		books = append(books, book)
		inventoryUnits += int64(book.Stock)
		inventoryValue += int64(book.Stock) * book.PriceCents
		if book.Stock <= 5 {
			lowStock = append(lowStock, book)
		}
	}
	sort.Slice(lowStock, func(i, j int) bool { return lowStock[i].Stock < lowStock[j].Stock })
	recent := books
	if len(recent) > 5 {
		recent = recent[:5]
	}
	if len(lowStock) > 5 {
		lowStock = lowStock[:5]
	}
	lowStockCount := int32(0)
	for _, book := range books {
		if book.Stock <= 5 {
			lowStockCount++
		}
	}
	return &model.CatalogSnapshot{
		LoadedCount: int32(len(books)), InventoryUnits: inventoryUnits, LowStockCount: lowStockCount,
		InventoryValueCents: inventoryValue, RecentBooks: recent, LowStockBooks: lowStock,
		PageInfo: &model.PageInfo{HasNextPage: response.GetHasMore(), EndCursor: optionalString(response.GetNextCursor())},
	}
}
