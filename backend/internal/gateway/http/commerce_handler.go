package http

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
	bookstorev1 "github.com/thinhnguyenwilliam/book-store/backend/gen/bookstore/v1"
	customeractivity "github.com/thinhnguyenwilliam/book-store/backend/internal/events/customeractivity"
)

const idempotencyHeader = "Idempotency-Key"

// listCartItems godoc
// @Summary List current cart items
// @Tags Cart
// @Produce json
// @Security BearerAuth
// @Success 200 {object} CartListResponse
// @Failure 401 {object} ErrorResponse
// @Router /api/v1/cart/items [get]
func (h *Handler) listCartItems(c echo.Context) error {
	response, err := h.orders.ListCart(grpcContext(c), &bookstorev1.ListCartRequest{
		UserId: principalFromContext(c).UserID,
	})
	if err != nil {
		return errorResponse(c, err)
	}
	return c.JSON(http.StatusOK, CartListResponse{Data: cartItemsJSON(response.GetItems())})
}

// addCartItem godoc
// @Summary Add a book to the current cart
// @Tags Cart
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body AddCartItemRequest true "Cart item"
// @Success 201 {object} CartItemResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /api/v1/cart/items [post]
func (h *Handler) addCartItem(c echo.Context) error {
	var request AddCartItemRequest
	if err := c.Bind(&request); err != nil {
		return errorResponse(c, err)
	}
	item, err := h.orders.AddToCart(grpcContext(c), &bookstorev1.AddToCartRequest{
		UserId: principalFromContext(c).UserID, BookId: request.BookID, Quantity: request.Quantity,
	})
	if err != nil {
		return errorResponse(c, err)
	}
	h.recordServerActivity(c, customeractivity.EventBookAddedToCart, item.GetBookId(), "", "", item.GetQuantity())
	return c.JSON(http.StatusCreated, cartItemJSON(item))
}

// updateCartItem godoc
// @Summary Replace a cart item quantity
// @Tags Cart
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Cart item ID" format(uuid)
// @Param request body UpdateCartItemRequest true "New quantity"
// @Success 200 {object} CartItemResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /api/v1/cart/items/{id} [put]
func (h *Handler) updateCartItem(c echo.Context) error {
	var request UpdateCartItemRequest
	if err := c.Bind(&request); err != nil {
		return errorResponse(c, err)
	}
	item, err := h.orders.UpdateCartItem(grpcContext(c), &bookstorev1.UpdateCartItemRequest{
		UserId: principalFromContext(c).UserID, ItemId: c.Param("id"), Quantity: request.Quantity,
	})
	if err != nil {
		return errorResponse(c, err)
	}
	return c.JSON(http.StatusOK, cartItemJSON(item))
}

// removeCartItem godoc
// @Summary Remove one cart item
// @Tags Cart
// @Security BearerAuth
// @Param id path string true "Cart item ID" format(uuid)
// @Success 204
// @Failure 404 {object} ErrorResponse
// @Router /api/v1/cart/items/{id} [delete]
func (h *Handler) removeCartItem(c echo.Context) error {
	_, err := h.orders.RemoveFromCart(grpcContext(c), &bookstorev1.RemoveFromCartRequest{
		UserId: principalFromContext(c).UserID, ItemId: c.Param("id"),
	})
	if err != nil {
		return errorResponse(c, err)
	}
	return c.NoContent(http.StatusNoContent)
}

// batchRemoveCartItems godoc
// @Summary Remove multiple cart items
// @Description POST /batch-delete is preferred because some proxies do not reliably forward DELETE request bodies.
// @Tags Cart
// @Accept json
// @Security BearerAuth
// @Param request body []BatchCartItemRequest true "Cart item IDs"
// @Success 204
// @Failure 400 {object} ErrorResponse
// @Router /api/v1/cart/items/batch-delete [post]
func (h *Handler) batchRemoveCartItems(c echo.Context) error {
	var request []BatchCartItemRequest
	if err := c.Bind(&request); err != nil {
		return errorResponse(c, err)
	}
	ids := make([]string, 0, len(request))
	for _, item := range request {
		ids = append(ids, item.ItemID)
	}
	_, err := h.orders.BatchRemoveFromCart(grpcContext(c), &bookstorev1.BatchRemoveFromCartRequest{
		UserId: principalFromContext(c).UserID, ItemIds: ids,
	})
	if err != nil {
		return errorResponse(c, err)
	}
	return c.NoContent(http.StatusNoContent)
}

// createOrder godoc
// @Summary Create an order from the current cart and reserve stock
// @Tags Orders
// @Produce json
// @Security BearerAuth
// @Param Idempotency-Key header string true "Unique retry key"
// @Success 201 {object} OrderResponse
// @Failure 400 {object} ErrorResponse
// @Failure 412 {object} ErrorResponse
// @Router /api/v1/orders [post]
func (h *Handler) createOrder(c echo.Context) error {
	key, ok := requireIdempotencyKey(c)
	if !ok {
		return nil
	}
	order, err := h.orders.CreateOrder(grpcContext(c), &bookstorev1.CreateOrderRequest{
		UserId: principalFromContext(c).UserID, IdempotencyKey: key,
	})
	if err != nil {
		return errorResponse(c, err)
	}
	h.recordServerActivity(c, customeractivity.EventCheckoutStarted, "", order.GetId(), "", 0)
	return c.JSON(http.StatusCreated, orderJSON(order))
}

// getOrder godoc
// @Summary Get one current-user order
// @Tags Orders
// @Produce json
// @Security BearerAuth
// @Param id path string true "Order ID" format(uuid)
// @Success 200 {object} OrderResponse
// @Failure 404 {object} ErrorResponse
// @Router /api/v1/orders/{id} [get]
func (h *Handler) getOrder(c echo.Context) error {
	order, err := h.orders.GetOrder(grpcContext(c), &bookstorev1.GetOrderRequest{
		UserId: principalFromContext(c).UserID, Id: c.Param("id"),
	})
	if err != nil {
		return errorResponse(c, err)
	}
	return c.JSON(http.StatusOK, orderJSON(order))
}

// listOrders godoc
// @Summary List current-user orders
// @Tags Orders
// @Produce json
// @Security BearerAuth
// @Param limit query int false "Items per request" default(20) minimum(1) maximum(100)
// @Param cursor query string false "Opaque cursor"
// @Success 200 {object} OrderListResponse
// @Router /api/v1/orders [get]
func (h *Handler) listOrders(c echo.Context) error {
	response, err := h.orders.ListOrders(grpcContext(c), &bookstorev1.ListOrdersRequest{
		UserId: principalFromContext(c).UserID, Limit: int32Query(c, "limit", 20),
		Cursor: c.QueryParam("cursor"),
	})
	if err != nil {
		return errorResponse(c, err)
	}
	return c.JSON(http.StatusOK, OrderListResponse{
		Data:       ordersJSON(response.GetOrders()),
		Pagination: CursorPagination{NextCursor: response.GetNextCursor(), HasMore: response.GetHasMore()},
	})
}

// cancelOrder godoc
// @Summary Cancel an unpaid order and release reserved stock
// @Tags Orders
// @Produce json
// @Security BearerAuth
// @Param id path string true "Order ID" format(uuid)
// @Success 200 {object} OrderResponse
// @Failure 412 {object} ErrorResponse
// @Router /api/v1/orders/{id}/cancel [put]
func (h *Handler) cancelOrder(c echo.Context) error {
	order, err := h.orders.CancelOrder(grpcContext(c), &bookstorev1.CancelOrderRequest{
		UserId: principalFromContext(c).UserID, Id: c.Param("id"),
	})
	if err != nil {
		return errorResponse(c, err)
	}
	return c.JSON(http.StatusOK, orderJSON(order))
}

// createPayment godoc
// @Summary Pay a stock-reserved order
// @Tags Payments
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param Idempotency-Key header string true "Unique retry key"
// @Param request body CreatePaymentRequest true "Order to pay"
// @Success 201 {object} PaymentResponse
// @Failure 400 {object} ErrorResponse
// @Failure 412 {object} ErrorResponse
// @Router /api/v1/payments [post]
func (h *Handler) createPayment(c echo.Context) error {
	key, ok := requireIdempotencyKey(c)
	if !ok {
		return nil
	}
	var request CreatePaymentRequest
	if err := c.Bind(&request); err != nil {
		return errorResponse(c, err)
	}
	payment, err := h.orders.PayOrder(grpcContext(c), &bookstorev1.PayOrderRequest{
		UserId: principalFromContext(c).UserID, OrderId: request.OrderID, IdempotencyKey: key,
		Provider: request.Provider, ClientIp: c.RealIP(), Locale: request.Locale, BankCode: request.BankCode,
	})
	if err != nil {
		return errorResponse(c, err)
	}
	return c.JSON(http.StatusCreated, paymentJSON(payment))
}

// vnpayWebhook godoc
// @Summary Receive signed VNPAY IPN notifications
// @Description Public server-to-server endpoint. Signature, merchant reference and amount are verified before state changes.
// @Tags Payment Webhooks
// @Produce json
// @Success 200 {object} VNPayWebhookResponse
// @Router /api/v1/payments/webhooks/vnpay [get]
func (h *Handler) vnpayWebhook(c echo.Context) error {
	parameters := make(map[string]string, len(c.QueryParams()))
	for key, values := range c.QueryParams() {
		if len(values) > 0 {
			parameters[key] = values[0]
		}
	}
	response, err := h.payments.ProcessWebhook(grpcContext(c), &bookstorev1.ProcessPaymentWebhookRequest{
		Provider: "vnpay", Parameters: parameters,
	})
	if err != nil {
		slog.ErrorContext(c.Request().Context(), "process VNPAY webhook", "error", err)
		return c.JSON(http.StatusOK, VNPayWebhookResponse{
			ResponseCode: "99", Message: "Processing error",
		})
	}
	return c.JSON(http.StatusOK, VNPayWebhookResponse{
		ResponseCode: response.GetResponseCode(), Message: response.GetMessage(),
	})
}

// getPayment godoc
// @Summary Get one current-user payment
// @Tags Payments
// @Produce json
// @Security BearerAuth
// @Param id path string true "Payment ID" format(uuid)
// @Success 200 {object} PaymentResponse
// @Failure 404 {object} ErrorResponse
// @Router /api/v1/payments/{id} [get]
func (h *Handler) getPayment(c echo.Context) error {
	payment, err := h.payments.GetPayment(grpcContext(c), &bookstorev1.GetPaymentRequest{
		Id: c.Param("id"), BuyerId: principalFromContext(c).UserID,
	})
	if err != nil {
		return errorResponse(c, err)
	}
	return c.JSON(http.StatusOK, paymentJSON(payment))
}

// getPaymentByOrder godoc
// @Summary Get the current-user payment for an order
// @Tags Payments
// @Produce json
// @Security BearerAuth
// @Param order_id path string true "Order ID" format(uuid)
// @Success 200 {object} PaymentResponse
// @Failure 404 {object} ErrorResponse
// @Router /api/v1/payments/order/{order_id} [get]
func (h *Handler) getPaymentByOrder(c echo.Context) error {
	payment, err := h.payments.GetPaymentByOrder(grpcContext(c), &bookstorev1.GetPaymentByOrderRequest{
		OrderId: c.Param("order_id"), BuyerId: principalFromContext(c).UserID,
	})
	if err != nil {
		return errorResponse(c, err)
	}
	return c.JSON(http.StatusOK, paymentJSON(payment))
}

// createWallet godoc
// @Summary Create the current-user wallet
// @Tags Wallets
// @Produce json
// @Security BearerAuth
// @Success 201 {object} WalletResponse
// @Router /api/v1/wallets/me [post]
func (h *Handler) createWallet(c echo.Context) error {
	wallet, err := h.payments.CreateWallet(grpcContext(c), &bookstorev1.CreateWalletRequest{
		OwnerId: principalFromContext(c).UserID,
	})
	if err != nil {
		return errorResponse(c, err)
	}
	return c.JSON(http.StatusCreated, walletJSON(wallet))
}

// getWallet godoc
// @Summary Get the current-user wallet balance
// @Tags Wallets
// @Produce json
// @Security BearerAuth
// @Success 200 {object} WalletResponse
// @Failure 404 {object} ErrorResponse
// @Router /api/v1/wallets/me [get]
func (h *Handler) getWallet(c echo.Context) error {
	wallet, err := h.payments.GetBalance(grpcContext(c), &bookstorev1.GetBalanceRequest{
		OwnerId: principalFromContext(c).UserID,
	})
	if err != nil {
		return errorResponse(c, err)
	}
	return c.JSON(http.StatusOK, walletJSON(wallet))
}

// updateWalletBalance godoc
// @Summary Adjust a wallet balance through double-entry ledger records
// @Description Administrative funding/testing endpoint. It never directly overwrites the stored balance.
// @Tags Admin Wallets
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param owner_id path string true "Wallet owner ID"
// @Param Idempotency-Key header string true "Unique retry key"
// @Param request body UpdateWalletBalanceRequest true "Signed balance delta"
// @Success 200 {object} WalletResponse
// @Failure 400 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Router /api/v1/admin/wallets/{owner_id}/balance [put]
func (h *Handler) updateWalletBalance(c echo.Context) error {
	key, ok := requireIdempotencyKey(c)
	if !ok {
		return nil
	}
	var request UpdateWalletBalanceRequest
	if err := c.Bind(&request); err != nil {
		return errorResponse(c, err)
	}
	wallet, err := h.payments.UpdateBalance(grpcContext(c), &bookstorev1.UpdateBalanceRequest{
		OwnerId: c.Param("owner_id"), DeltaCents: request.DeltaCents,
		IdempotencyKey: key, Reason: request.Reason,
	})
	if err != nil {
		return errorResponse(c, err)
	}
	return c.JSON(http.StatusOK, walletJSON(wallet))
}

func requireIdempotencyKey(c echo.Context) (string, bool) {
	key := strings.TrimSpace(c.Request().Header.Get(idempotencyHeader))
	if key != "" && len(key) <= 200 {
		return key, true
	}
	_ = c.JSON(http.StatusBadRequest, errorBody("Idempotency-Key header is required and must not exceed 200 characters"))
	return "", false
}
