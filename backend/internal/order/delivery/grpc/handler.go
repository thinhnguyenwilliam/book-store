package grpc

import (
	"context"
	"errors"
	"time"

	bookstorev1 "github.com/thinhnguyenwilliam/book-store/backend/gen/bookstore/v1"
	"github.com/thinhnguyenwilliam/book-store/backend/internal/order/application"
	"github.com/thinhnguyenwilliam/book-store/backend/internal/order/domain"
	grpcerror "github.com/thinhnguyenwilliam/book-store/backend/internal/platform/grpcerror"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Handler struct {
	bookstorev1.UnimplementedOrderServiceServer
	service *application.Service
}

func NewHandler(service *application.Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) AddToCart(
	ctx context.Context,
	request *bookstorev1.AddToCartRequest,
) (*bookstorev1.CartItem, error) {
	item, err := h.service.AddToCart(ctx, request.GetUserId(), request.GetBookId(), request.GetQuantity())
	if err != nil {
		return nil, mapError(err)
	}
	return cartItemProto(item), nil
}

func (h *Handler) UpdateCartItem(
	ctx context.Context,
	request *bookstorev1.UpdateCartItemRequest,
) (*bookstorev1.CartItem, error) {
	item, err := h.service.UpdateCartItem(
		ctx, request.GetUserId(), request.GetItemId(), request.GetQuantity(),
	)
	if err != nil {
		return nil, mapError(err)
	}
	return cartItemProto(item), nil
}

func (h *Handler) RemoveFromCart(
	ctx context.Context,
	request *bookstorev1.RemoveFromCartRequest,
) (*bookstorev1.RemoveFromCartResponse, error) {
	if err := h.service.RemoveFromCart(ctx, request.GetUserId(), []string{request.GetItemId()}); err != nil {
		return nil, mapError(err)
	}
	return &bookstorev1.RemoveFromCartResponse{}, nil
}

func (h *Handler) BatchRemoveFromCart(
	ctx context.Context,
	request *bookstorev1.BatchRemoveFromCartRequest,
) (*bookstorev1.RemoveFromCartResponse, error) {
	if err := h.service.RemoveFromCart(ctx, request.GetUserId(), request.GetItemIds()); err != nil {
		return nil, mapError(err)
	}
	return &bookstorev1.RemoveFromCartResponse{}, nil
}

func (h *Handler) ListCart(
	ctx context.Context,
	request *bookstorev1.ListCartRequest,
) (*bookstorev1.ListCartResponse, error) {
	items, err := h.service.ListCart(ctx, request.GetUserId())
	if err != nil {
		return nil, mapError(err)
	}
	result := make([]*bookstorev1.CartItem, 0, len(items))
	for _, item := range items {
		result = append(result, cartItemProto(item))
	}
	return &bookstorev1.ListCartResponse{Items: result}, nil
}

func (h *Handler) CreateOrder(
	ctx context.Context,
	request *bookstorev1.CreateOrderRequest,
) (*bookstorev1.Order, error) {
	order, err := h.service.CreateOrder(ctx, request.GetUserId(), request.GetIdempotencyKey())
	if err != nil {
		return nil, mapError(err)
	}
	return orderProto(order), nil
}

func (h *Handler) GetOrder(
	ctx context.Context,
	request *bookstorev1.GetOrderRequest,
) (*bookstorev1.Order, error) {
	order, err := h.service.GetOrder(ctx, request.GetUserId(), request.GetId())
	if err != nil {
		return nil, mapError(err)
	}
	return orderProto(order), nil
}

func (h *Handler) ListOrders(
	ctx context.Context,
	request *bookstorev1.ListOrdersRequest,
) (*bookstorev1.ListOrdersResponse, error) {
	page, err := h.service.ListOrders(ctx, request.GetUserId(), request.GetCursor(), request.GetLimit())
	if err != nil {
		return nil, mapError(err)
	}
	orders := make([]*bookstorev1.Order, 0, len(page.Orders))
	for _, order := range page.Orders {
		orders = append(orders, orderProto(order))
	}
	return &bookstorev1.ListOrdersResponse{
		Orders: orders, NextCursor: page.NextCursor, HasMore: page.HasMore,
	}, nil
}

func (h *Handler) CancelOrder(
	ctx context.Context,
	request *bookstorev1.CancelOrderRequest,
) (*bookstorev1.Order, error) {
	order, err := h.service.CancelOrder(ctx, request.GetUserId(), request.GetId())
	if err != nil {
		return nil, mapError(err)
	}
	return orderProto(order), nil
}

func (h *Handler) PayOrder(
	ctx context.Context,
	request *bookstorev1.PayOrderRequest,
) (*bookstorev1.Payment, error) {
	payment, err := h.service.PayOrder(
		ctx, request.GetUserId(), request.GetOrderId(), request.GetIdempotencyKey(),
		domain.PaymentOptions{
			Provider: request.GetProvider(), ClientIP: request.GetClientIp(),
			Locale: request.GetLocale(), BankCode: request.GetBankCode(),
		},
	)
	if err != nil {
		return nil, mapError(err)
	}
	return paymentProto(payment), nil
}

func cartItemProto(item *domain.CartItem) *bookstorev1.CartItem {
	return &bookstorev1.CartItem{
		Id: item.ID, UserId: item.UserID, BookId: item.BookID, Quantity: item.Quantity,
		CreatedAt: item.CreatedAt.Format(time.RFC3339), UpdatedAt: item.UpdatedAt.Format(time.RFC3339),
	}
}

func orderProto(order *domain.Order) *bookstorev1.Order {
	items := make([]*bookstorev1.OrderItem, 0, len(order.Items))
	for _, item := range order.Items {
		items = append(items, &bookstorev1.OrderItem{
			Id: item.ID, BookId: item.BookID, SellerId: item.SellerID, Title: item.Title,
			UnitPriceCents: item.UnitPriceCents, Quantity: item.Quantity, SubtotalCents: item.SubtotalCents,
		})
	}
	return &bookstorev1.Order{
		Id: order.ID, UserId: order.UserID, Status: order.Status, TotalCents: order.TotalCents,
		Currency: order.Currency, Items: items, PaymentId: order.PaymentID,
		FailureReason: order.FailureReason, CreatedAt: order.CreatedAt.Format(time.RFC3339),
		UpdatedAt:            order.UpdatedAt.Format(time.RFC3339),
		ReservationExpiresAt: order.ReservationExpiresAt.Format(time.RFC3339),
	}
}

func paymentProto(payment *domain.Payment) *bookstorev1.Payment {
	return &bookstorev1.Payment{
		Id: payment.ID, OrderId: payment.OrderID, BuyerId: payment.BuyerID,
		Status: payment.Status, AmountCents: payment.AmountCents,
		PlatformFeeCents: payment.PlatformFeeCents, Currency: payment.Currency,
		FailureReason: payment.FailureReason, CreatedAt: payment.CreatedAt.Format(time.RFC3339),
		UpdatedAt: payment.UpdatedAt.Format(time.RFC3339),
		Provider:  payment.Provider, ProviderTransactionId: payment.ProviderTransactionID,
		CheckoutUrl: payment.CheckoutURL, ExpiresAt: formatOptionalTime(payment.ExpiresAt),
		PaidAt: formatOptionalTime(payment.PaidAt),
	}
}

func formatOptionalTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.Format(time.RFC3339)
}

func mapError(err error) error {
	if mapped := grpcerror.FromContext(err); mapped != nil {
		return mapped
	}
	switch {
	case errors.Is(err, domain.ErrInvalidInput), errors.Is(err, domain.ErrCartEmpty):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, domain.ErrCartItemNotFound), errors.Is(err, domain.ErrOrderNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, domain.ErrOrderState), errors.Is(err, domain.ErrPaymentDeclined):
		return status.Error(codes.FailedPrecondition, err.Error())
	case errors.Is(err, domain.ErrIdempotencyConflict):
		return status.Error(codes.AlreadyExists, err.Error())
	default:
		if nested, ok := status.FromError(err); ok && nested.Code() != codes.Unknown {
			return status.Error(nested.Code(), nested.Message())
		}
		return status.Error(codes.Internal, "internal server error")
	}
}
