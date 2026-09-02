package http

// HealthResponse describes the gateway health status.
type HealthResponse struct {
	Status string `json:"status" example:"ok"`
}

// RegisterRequest is the public HTTP payload for account registration.
type RegisterRequest struct {
	Email       string `json:"email" example:"reader@example.com"`
	Password    string `json:"password" example:"password123"`
	DisplayName string `json:"display_name" example:"Reader"`
}

// LoginRequest is the public HTTP payload for authentication.
type LoginRequest struct {
	Email    string `json:"email" example:"reader@example.com"`
	Password string `json:"password" example:"password123"`
}

// ProviderStateRequest starts a short-lived external login transaction.
type ProviderStateRequest struct {
	Provider      string `json:"provider" example:"google"`
	CreateAccount bool   `json:"create_account"`
}

// ProviderStateResponse contains the opaque anti-CSRF state value.
type ProviderStateResponse struct {
	State     string `json:"state"`
	ExpiresIn int64  `json:"expires_in" example:"600"`
}

// GoogleLoginRequest contains the Google Identity Services credential.
type GoogleLoginRequest struct {
	Credential    string `json:"credential"`
	State         string `json:"state"`
	CreateAccount bool   `json:"create_account"`
}

// FacebookLoginRequest contains the Facebook Login user access token.
type FacebookLoginRequest struct {
	AccessToken   string `json:"access_token"`
	State         string `json:"state"`
	CreateAccount bool   `json:"create_account"`
}

// UpdateProfileRequest is the public HTTP payload for updating a profile.
type UpdateProfileRequest struct {
	DisplayName string `json:"display_name" example:"Reader Nguyen"`
}

// BookRequest is the public HTTP payload for creating or updating a book.
type BookRequest struct {
	Title      string `json:"title" example:"Clean Architecture"`
	Author     string `json:"author" example:"Robert C. Martin"`
	ISBN       string `json:"isbn" example:"9780134494166"`
	PriceCents int64  `json:"price_cents" example:"3999"`
	Stock      int32  `json:"stock" example:"10"`
	SellerID   string `json:"seller_id,omitempty" format:"uuid"`
}

// AuthResponse is returned after registration or login.
type AuthResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type" example:"Bearer"`
	UserID      string `json:"user_id" format:"uuid"`
	ExpiresIn   int64  `json:"expires_in" example:"3600"`
}

// UserResponse is the public representation of a user profile.
type UserResponse struct {
	ID          string `json:"id" format:"uuid"`
	Email       string `json:"email" example:"reader@example.com"`
	DisplayName string `json:"display_name" example:"Reader"`
	CreatedAt   string `json:"created_at" format:"date-time"`
	UpdatedAt   string `json:"updated_at" format:"date-time"`
}

// BookResponse is the public representation of a book.
type BookResponse struct {
	ID         string `json:"id" format:"uuid"`
	Title      string `json:"title" example:"Clean Architecture"`
	Author     string `json:"author" example:"Robert C. Martin"`
	ISBN       string `json:"isbn" example:"9780134494166"`
	PriceCents int64  `json:"price_cents" example:"3999"`
	Stock      int32  `json:"stock" example:"10"`
	SellerID   string `json:"seller_id,omitempty" format:"uuid"`
	CreatedAt  string `json:"created_at" format:"date-time"`
	UpdatedAt  string `json:"updated_at" format:"date-time"`
}

// CursorPagination describes the next position in a cursor-paginated response.
type CursorPagination struct {
	NextCursor string `json:"next_cursor,omitempty"`
	HasMore    bool   `json:"has_more" example:"true"`
}

// BookListResponse is returned by the book listing endpoint.
type BookListResponse struct {
	Data       []BookResponse   `json:"data"`
	Pagination CursorPagination `json:"pagination"`
}

type BookSearchHitResponse struct {
	Book       BookResponse      `json:"book"`
	Score      float64           `json:"score"`
	Highlights map[string]string `json:"highlights,omitempty"`
}

type BookSearchResponse struct {
	Data       []BookSearchHitResponse `json:"data"`
	Pagination CursorPagination        `json:"pagination"`
	Total      int64                   `json:"total"`
	TookMS     int64                   `json:"took_ms"`
}

type BookSuggestionResponse struct {
	Data   []BookSearchHitResponse `json:"data"`
	TookMS int64                   `json:"took_ms"`
}

type AddCartItemRequest struct {
	BookID   string `json:"bookId" format:"uuid"`
	Quantity int32  `json:"quantity" minimum:"1" maximum:"100"`
}

type UpdateCartItemRequest struct {
	Quantity int32 `json:"quantity" minimum:"1" maximum:"100"`
}

type BatchCartItemRequest struct {
	ItemID string `json:"itemId" format:"uuid"`
}

type CartItemResponse struct {
	ID        string `json:"id" format:"uuid"`
	BookID    string `json:"book_id" format:"uuid"`
	Quantity  int32  `json:"quantity"`
	CreatedAt string `json:"created_at" format:"date-time"`
	UpdatedAt string `json:"updated_at" format:"date-time"`
}

type CartListResponse struct {
	Data []CartItemResponse `json:"data"`
}

type OrderItemResponse struct {
	ID             string `json:"id" format:"uuid"`
	BookID         string `json:"book_id" format:"uuid"`
	SellerID       string `json:"seller_id"`
	Title          string `json:"title"`
	UnitPriceCents int64  `json:"unit_price_cents"`
	Quantity       int32  `json:"quantity"`
	SubtotalCents  int64  `json:"subtotal_cents"`
}

type OrderResponse struct {
	ID                   string              `json:"id" format:"uuid"`
	Status               string              `json:"status"`
	TotalCents           int64               `json:"total_cents"`
	Currency             string              `json:"currency"`
	Items                []OrderItemResponse `json:"items"`
	PaymentID            string              `json:"payment_id,omitempty" format:"uuid"`
	FailureReason        string              `json:"failure_reason,omitempty"`
	CreatedAt            string              `json:"created_at" format:"date-time"`
	UpdatedAt            string              `json:"updated_at" format:"date-time"`
	ReservationExpiresAt string              `json:"reservation_expires_at" format:"date-time"`
}

type OrderListResponse struct {
	Data       []OrderResponse  `json:"data"`
	Pagination CursorPagination `json:"pagination"`
}

type CreatePaymentRequest struct {
	OrderID  string `json:"orderId" format:"uuid"`
	Provider string `json:"provider,omitempty" enums:"wallet,vnpay"`
	Locale   string `json:"locale,omitempty" enums:"vn,en"`
	BankCode string `json:"bankCode,omitempty"`
}

type PaymentResponse struct {
	ID                    string `json:"id" format:"uuid"`
	OrderID               string `json:"order_id" format:"uuid"`
	Status                string `json:"status"`
	AmountCents           int64  `json:"amount_cents"`
	PlatformFeeCents      int64  `json:"platform_fee_cents"`
	Currency              string `json:"currency"`
	FailureReason         string `json:"failure_reason,omitempty"`
	Provider              string `json:"provider"`
	ProviderTransactionID string `json:"provider_transaction_id,omitempty"`
	CheckoutURL           string `json:"checkout_url,omitempty"`
	ExpiresAt             string `json:"expires_at,omitempty" format:"date-time"`
	PaidAt                string `json:"paid_at,omitempty" format:"date-time"`
	CreatedAt             string `json:"created_at" format:"date-time"`
	UpdatedAt             string `json:"updated_at" format:"date-time"`
}

type VNPayWebhookResponse struct {
	ResponseCode string `json:"RspCode"`
	Message      string `json:"Message"`
}

type UpdateWalletBalanceRequest struct {
	DeltaCents int64  `json:"delta_cents"`
	Reason     string `json:"reason"`
}

type WalletResponse struct {
	ID           string `json:"id" format:"uuid"`
	OwnerID      string `json:"owner_id"`
	BalanceCents int64  `json:"balance_cents"`
	Currency     string `json:"currency"`
	CreatedAt    string `json:"created_at" format:"date-time"`
	UpdatedAt    string `json:"updated_at" format:"date-time"`
}

type DailyOrderMetricResponse struct {
	Date      string `json:"date" example:"2026-09-01"`
	Created   int64  `json:"created"`
	Confirmed int64  `json:"confirmed"`
	Cancelled int64  `json:"cancelled"`
}

type OrderAnalyticsResponse struct {
	From                       string                     `json:"from" format:"date-time"`
	To                         string                     `json:"to" format:"date-time"`
	TotalOrders                int64                      `json:"total_orders"`
	ConfirmedOrders            int64                      `json:"confirmed_orders"`
	CancelledOrders            int64                      `json:"cancelled_orders"`
	PaymentAttempts            int64                      `json:"payment_attempts"`
	PaymentSucceeded           int64                      `json:"payment_succeeded"`
	PaymentFailed              int64                      `json:"payment_failed"`
	StockReservationFailed     int64                      `json:"stock_reservation_failed"`
	PaymentSuccessRate         float64                    `json:"payment_success_rate"`
	AverageConfirmationSeconds float64                    `json:"average_confirmation_seconds"`
	Daily                      []DailyOrderMetricResponse `json:"daily"`
	LastEventAt                string                     `json:"last_event_at,omitempty" format:"date-time"`
}

type EventCountResponse struct {
	EventType string `json:"event_type"`
	Count     int64  `json:"count"`
}

type BookActivityMetricResponse struct {
	BookID   string  `json:"book_id" format:"uuid"`
	Views    int64   `json:"views"`
	CartAdds int64   `json:"cart_adds"`
	Comments int64   `json:"comments"`
	Score    float64 `json:"score"`
}

type CustomerActivityAnalyticsResponse struct {
	From                string                       `json:"from" format:"date-time"`
	To                  string                       `json:"to" format:"date-time"`
	TotalEvents         int64                        `json:"total_events"`
	UniqueActors        int64                        `json:"unique_actors"`
	AbandonedCarts      int64                        `json:"abandoned_carts"`
	ViewToCartRate      float64                      `json:"view_to_cart_rate"`
	CartToCheckoutRate  float64                      `json:"cart_to_checkout_rate"`
	CheckoutToOrderRate float64                      `json:"checkout_to_order_rate"`
	EventCounts         []EventCountResponse         `json:"event_counts"`
	TopBooks            []BookActivityMetricResponse `json:"top_books"`
	LastEventAt         string                       `json:"last_event_at,omitempty" format:"date-time"`
}

type BookActivityListResponse struct {
	Data []BookActivityMetricResponse `json:"data"`
}

type RelatedBookResponse struct {
	BookID       string  `json:"book_id" format:"uuid"`
	SharedActors int64   `json:"shared_actors"`
	Score        float64 `json:"score"`
}

type RelatedBookListResponse struct {
	Data []RelatedBookResponse `json:"data"`
}

// CustomerListResponse is returned by the admin customer listing endpoint.
type CustomerListResponse struct {
	Data       []UserResponse   `json:"data"`
	Pagination CursorPagination `json:"pagination"`
}

// DeletionAcceptedResponse confirms that asynchronous cleanup was queued.
type DeletionAcceptedResponse struct {
	Status string `json:"status" example:"accepted"`
}

// ErrorDetail contains a safe error message for API clients.
type ErrorDetail struct {
	Message   string `json:"message" example:"invalid request"`
	Code      string `json:"code,omitempty" example:"invalid_provider_credential"`
	Provider  string `json:"provider,omitempty" example:"google"`
	Retryable bool   `json:"retryable,omitempty"`
}

// ErrorResponse is the common HTTP error envelope.
type ErrorResponse struct {
	Error ErrorDetail `json:"error"`
}
