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

// GoogleLoginRequest contains the Google Identity Services credential.
type GoogleLoginRequest struct {
	Credential    string `json:"credential"`
	CreateAccount bool   `json:"create_account"`
}

// FacebookLoginRequest contains the Facebook Login user access token.
type FacebookLoginRequest struct {
	AccessToken   string `json:"access_token"`
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
	Message string `json:"message" example:"invalid request"`
}

// ErrorResponse is the common HTTP error envelope.
type ErrorResponse struct {
	Error ErrorDetail `json:"error"`
}
