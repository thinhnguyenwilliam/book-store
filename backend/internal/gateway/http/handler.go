package http

import (
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	bookstorev1 "github.com/thinhnguyenwilliam/book-store/backend/gen/bookstore/v1"
	gatewaygraphql "github.com/thinhnguyenwilliam/book-store/backend/internal/gateway/graphql"
)

type Handler struct {
	auth             bookstorev1.AuthServiceClient
	users            bookstorev1.UserServiceClient
	books            bookstorev1.BookServiceClient
	orders           bookstorev1.OrderServiceClient
	payments         bookstorev1.PaymentServiceClient
	notifications    bookstorev1.NotificationServiceClient
	comments         bookstorev1.CommentServiceClient
	chat             bookstorev1.ChatServiceClient
	realtime         *ChatRealtime
	refreshCookie    RefreshCookieConfig
	trustedOrigins   map[string]struct{}
	graphQL          http.Handler
	graphQLBodyLimit string
}

type RefreshCookieConfig struct {
	Name     string
	Secure   bool
	SameSite http.SameSite
}

type GraphQLConfig struct {
	BodyLimit            string
	MaxComplexity        int
	MaxDepth             int
	ParserTokenLimit     int
	IntrospectionEnabled bool
}

func NewHandler(
	auth bookstorev1.AuthServiceClient,
	users bookstorev1.UserServiceClient,
	books bookstorev1.BookServiceClient,
	orders bookstorev1.OrderServiceClient,
	payments bookstorev1.PaymentServiceClient,
	notifications bookstorev1.NotificationServiceClient,
	comments bookstorev1.CommentServiceClient,
	chat bookstorev1.ChatServiceClient,
	realtime *ChatRealtime,
	refreshCookie RefreshCookieConfig,
	graphQLConfig GraphQLConfig,
	trustedOrigins []string,
) (*Handler, error) {
	origins := make(map[string]struct{}, len(trustedOrigins))
	for _, origin := range trustedOrigins {
		origins[origin] = struct{}{}
	}
	graphQLServer, err := gatewaygraphql.NewServer(gatewaygraphql.ServerConfig{
		MaxComplexity: graphQLConfig.MaxComplexity, MaxDepth: graphQLConfig.MaxDepth,
		ParserTokenLimit: graphQLConfig.ParserTokenLimit, IntrospectionEnabled: graphQLConfig.IntrospectionEnabled,
	}, books, users, orders, payments, comments)
	if err != nil {
		return nil, err
	}
	return &Handler{
		auth:             auth,
		users:            users,
		books:            books,
		orders:           orders,
		payments:         payments,
		notifications:    notifications,
		comments:         comments,
		chat:             chat,
		realtime:         realtime,
		refreshCookie:    refreshCookie,
		trustedOrigins:   origins,
		graphQL:          graphQLServer,
		graphQLBodyLimit: graphQLConfig.BodyLimit,
	}, nil
}

func (h *Handler) RegisterRoutes(e *echo.Echo) {
	e.GET("/healthz", h.health)
	e.POST("/graphql", h.graphQLRequest, middleware.BodyLimit(h.graphQLBodyLimit))

	api := e.Group("/api/v1")
	api.POST("/auth/register", h.register)
	api.POST("/auth/login", h.login)
	api.POST("/auth/provider-state", h.providerState)
	api.POST("/auth/google", h.googleLogin)
	api.POST("/auth/facebook", h.facebookLogin)
	api.POST("/auth/refresh", h.refresh)
	api.POST("/auth/logout", h.logout)
	api.GET("/books", h.listBooks)
	api.GET("/books/:id", h.getBook)
	api.GET("/books/:id/comments", h.listBookComments)
	api.GET("/comments/:id/replies", h.listCommentReplies)
	api.GET("/payments/webhooks/vnpay", h.vnpayWebhook)
	api.GET("/chat/ws", h.chatWebSocket)

	secured := api.Group("")
	secured.Use(h.Authenticate)
	secured.GET("/users/me", h.getMe)
	secured.PUT("/users/me", h.updateMe)
	secured.GET("/cart/items", h.listCartItems)
	secured.POST("/cart/items", h.addCartItem)
	secured.PUT("/cart/items/:id", h.updateCartItem)
	secured.DELETE("/cart/items/:id", h.removeCartItem)
	secured.DELETE("/cart/items", h.batchRemoveCartItems)
	secured.POST("/cart/items/batch-delete", h.batchRemoveCartItems)
	secured.POST("/orders", h.createOrder)
	secured.GET("/orders", h.listOrders)
	secured.GET("/orders/:id", h.getOrder)
	secured.PUT("/orders/:id/cancel", h.cancelOrder)
	secured.POST("/payments", h.createPayment)
	secured.GET("/payments/:id", h.getPayment)
	secured.GET("/payments/order/:order_id", h.getPaymentByOrder)
	secured.POST("/wallets/me", h.createWallet)
	secured.GET("/wallets/me", h.getWallet)
	secured.GET("/notifications", h.listNotifications)
	secured.GET("/notifications/unread-count", h.unreadNotificationCount)
	secured.PUT("/notifications/:id/read", h.markNotificationRead)
	secured.PUT("/notifications/read-all", h.markAllNotificationsRead)
	secured.POST("/notifications/devices", h.registerPushDevice)
	secured.DELETE("/notifications/devices/:device_id", h.unregisterPushDevice)
	secured.POST("/books/:id/comments", h.createComment)
	secured.PUT("/comments/:id", h.updateComment)
	secured.DELETE("/comments/:id", h.deleteComment)
	secured.POST("/chat/conversations/support", h.createSupportConversation)
	secured.GET("/chat/conversations", h.listChatConversations)
	secured.GET("/chat/conversations/:id/messages", h.listChatMessages)
	secured.POST("/chat/conversations/:id/messages", h.sendChatMessage)
	secured.PUT("/chat/conversations/:id/read", h.markChatRead)
	secured.GET("/chat/unread-count", h.unreadChatCount)
	secured.PUT("/chat/messages/:id", h.updateChatMessage)
	secured.DELETE("/chat/messages/:id", h.deleteChatMessage)
	secured.POST("/chat/ws-ticket", h.issueChatWebSocketTicket)

	admin := secured.Group("/admin")
	admin.Use(RequireRole("admin"))
	admin.GET("/customers", h.listCustomers)
	admin.GET("/customers/:id", h.getCustomer)
	admin.PUT("/customers/:id", h.updateCustomer)
	admin.DELETE("/customers/:id", h.deleteCustomer)
	admin.POST("/books", h.createBook)
	admin.PUT("/books/:id", h.updateBook)
	admin.DELETE("/books/:id", h.deleteBook)
	admin.PUT("/wallets/:owner_id/balance", h.updateWalletBalance)
	admin.PUT("/comments/:id/status", h.moderateComment)
}

// health godoc
// @Summary Check gateway health
// @Description Returns the current health status of the HTTP gateway.
// @Tags System
// @Produce json
// @Success 200 {object} HealthResponse
// @Router /healthz [get]
func (h *Handler) health(c echo.Context) error {
	return c.JSON(http.StatusOK, HealthResponse{Status: "ok"})
}

// register godoc
// @Summary Register an account
// @Description Creates an auth account and schedules profile creation through the transactional outbox.
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body RegisterRequest true "Registration payload"
// @Failure 400 {object} ErrorResponse
// @Failure 409 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/auth/register [post]
func (h *Handler) register(c echo.Context) error {
	if !h.isTrustedOrigin(c) {
		return c.JSON(http.StatusForbidden, errorBody("untrusted request origin"))
	}
	request := RegisterRequest{}
	if err := c.Bind(&request); err != nil {
		return errorResponse(c, err)
	}

	ctx := grpcContext(c)
	authResult, err := h.auth.Register(ctx, registerProto(request))
	if err != nil {
		return errorResponse(c, err)
	}

	h.setRefreshCookie(c, authResult.GetRefreshToken(), authResult.GetRefreshExpiresIn())
	c.Response().Header().Set(echo.HeaderCacheControl, "no-store")
	return c.JSON(http.StatusCreated, authJSON(authResult))
}

// login godoc
// @Summary Log in
// @Description Authenticates an account and returns a bearer access token.
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body LoginRequest true "Login payload"
// @Success 200 {object} AuthResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/auth/login [post]
func (h *Handler) login(c echo.Context) error {
	if !h.isTrustedOrigin(c) {
		return c.JSON(http.StatusForbidden, errorBody("untrusted request origin"))
	}
	request := LoginRequest{}
	if err := c.Bind(&request); err != nil {
		return errorResponse(c, err)
	}

	ctx := grpcContext(c)
	response, err := h.auth.Login(ctx, loginProto(request))
	if err != nil {
		return errorResponse(c, err)
	}
	h.setRefreshCookie(c, response.GetRefreshToken(), response.GetRefreshExpiresIn())
	c.Response().Header().Set(echo.HeaderCacheControl, "no-store")
	return c.JSON(http.StatusOK, authJSON(response))
}

// googleLogin godoc
// @Summary Sign in with Google
// @Description Verifies a Google Identity Services ID token. Storefront clients may request account creation; admin clients should only sign in existing accounts.
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body GoogleLoginRequest true "Google credential"
// @Success 200 {object} AuthResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 409 {object} ErrorResponse
// @Failure 412 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/auth/google [post]
func (h *Handler) googleLogin(c echo.Context) error {
	if !h.isTrustedOrigin(c) {
		return c.JSON(http.StatusForbidden, errorBody("untrusted request origin"))
	}
	request := GoogleLoginRequest{}
	if err := c.Bind(&request); err != nil {
		return errorResponse(c, err)
	}
	if !h.validProviderState(c, providerGoogle, request.CreateAccount, request.State) {
		h.clearProviderStateCookie(c, providerGoogle)
		return providerError(c, http.StatusForbidden, providerGoogle, "invalid_oauth_state", "external login state is invalid or expired", false)
	}

	ctx := grpcContext(c)
	response, err := h.auth.LoginWithGoogle(ctx, googleLoginProto(request))
	if err != nil {
		return providerErrorResponse(c, providerGoogle, err)
	}
	h.clearProviderStateCookie(c, providerGoogle)
	h.setRefreshCookie(c, response.GetRefreshToken(), response.GetRefreshExpiresIn())
	c.Response().Header().Set(echo.HeaderCacheControl, "no-store")
	return c.JSON(http.StatusOK, authJSON(response))
}

// facebookLogin godoc
// @Summary Sign in with Facebook
// @Description Validates a Facebook user access token against the configured Meta app. Storefront clients may request account creation; admin clients should only sign in existing accounts.
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body FacebookLoginRequest true "Facebook user access token"
// @Success 200 {object} AuthResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 409 {object} ErrorResponse
// @Failure 412 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Failure 503 {object} ErrorResponse
// @Router /api/v1/auth/facebook [post]
func (h *Handler) facebookLogin(c echo.Context) error {
	if !h.isTrustedOrigin(c) {
		return c.JSON(http.StatusForbidden, errorBody("untrusted request origin"))
	}
	request := FacebookLoginRequest{}
	if err := c.Bind(&request); err != nil {
		return errorResponse(c, err)
	}
	if !h.validProviderState(c, providerFacebook, request.CreateAccount, request.State) {
		h.clearProviderStateCookie(c, providerFacebook)
		return providerError(c, http.StatusForbidden, providerFacebook, "invalid_oauth_state", "external login state is invalid or expired", false)
	}

	ctx := grpcContext(c)
	response, err := h.auth.LoginWithFacebook(ctx, facebookLoginProto(request))
	if err != nil {
		return providerErrorResponse(c, providerFacebook, err)
	}
	h.clearProviderStateCookie(c, providerFacebook)
	h.setRefreshCookie(c, response.GetRefreshToken(), response.GetRefreshExpiresIn())
	c.Response().Header().Set(echo.HeaderCacheControl, "no-store")
	return c.JSON(http.StatusOK, authJSON(response))
}

// refresh godoc
// @Summary Refresh an access token
// @Description Rotates the HttpOnly refresh cookie and returns a new short-lived access token.
// @Tags Auth
// @Produce json
// @Success 200 {object} AuthResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/auth/refresh [post]
func (h *Handler) refresh(c echo.Context) error {
	if !h.isTrustedOrigin(c) {
		return c.JSON(http.StatusForbidden, errorBody("untrusted request origin"))
	}
	cookie, err := c.Cookie(h.refreshCookie.Name)
	if err != nil || cookie.Value == "" {
		h.clearRefreshCookie(c)
		return c.JSON(http.StatusUnauthorized, errorBody("invalid refresh token"))
	}

	ctx := grpcContext(c)
	response, err := h.auth.Refresh(ctx, &bookstorev1.RefreshRequest{RefreshToken: cookie.Value})
	if err != nil {
		h.clearRefreshCookie(c)
		return errorResponse(c, err)
	}

	h.setRefreshCookie(c, response.GetRefreshToken(), response.GetRefreshExpiresIn())
	c.Response().Header().Set(echo.HeaderCacheControl, "no-store")
	return c.JSON(http.StatusOK, authJSON(response))
}

// logout godoc
// @Summary Log out
// @Description Revokes the current refresh session and clears its HttpOnly cookie.
// @Tags Auth
// @Success 204
// @Failure 403 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/auth/logout [post]
func (h *Handler) logout(c echo.Context) error {
	if !h.isTrustedOrigin(c) {
		return c.JSON(http.StatusForbidden, errorBody("untrusted request origin"))
	}
	cookie, err := c.Cookie(h.refreshCookie.Name)
	if err == nil && cookie.Value != "" {
		ctx := grpcContext(c)
		if _, err := h.auth.Logout(ctx, &bookstorev1.LogoutRequest{RefreshToken: cookie.Value}); err != nil {
			h.clearRefreshCookie(c)
			return errorResponse(c, err)
		}
	}
	h.clearRefreshCookie(c)
	return c.NoContent(http.StatusNoContent)
}

// getMe godoc
// @Summary Get current profile
// @Description Returns the profile associated with the bearer token.
// @Tags Users
// @Produce json
// @Security BearerAuth
// @Success 200 {object} UserResponse
// @Failure 401 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/users/me [get]
func (h *Handler) getMe(c echo.Context) error {
	principal := principalFromContext(c)
	ctx := grpcContext(c)
	user, err := h.users.GetProfile(ctx, &bookstorev1.GetProfileRequest{Id: principal.UserID})
	if err != nil {
		return errorResponse(c, err)
	}
	return c.JSON(http.StatusOK, userJSON(user))
}

// updateMe godoc
// @Summary Update current profile
// @Description Updates the display name of the profile associated with the bearer token.
// @Tags Users
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body UpdateProfileRequest true "Profile payload"
// @Success 200 {object} UserResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/users/me [put]
func (h *Handler) updateMe(c echo.Context) error {
	request := UpdateProfileRequest{}
	if err := c.Bind(&request); err != nil {
		return errorResponse(c, err)
	}

	principal := principalFromContext(c)
	ctx := grpcContext(c)
	user, err := h.users.UpdateProfile(ctx, updateProfileProto(principal.UserID, request))
	if err != nil {
		return errorResponse(c, err)
	}
	return c.JSON(http.StatusOK, userJSON(user))
}

// listCustomers godoc
// @Summary List customers
// @Description Returns user profiles using cursor pagination. The bearer token must contain the admin role.
// @Tags Admin Customers
// @Produce json
// @Security BearerAuth
// @Param limit query int false "Items per request" default(20) minimum(1) maximum(100)
// @Param cursor query string false "Opaque cursor returned by the previous request"
// @Success 200 {object} CustomerListResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/admin/customers [get]
func (h *Handler) listCustomers(c echo.Context) error {
	ctx := grpcContext(c)
	response, err := h.users.ListProfiles(ctx, &bookstorev1.ListProfilesRequest{
		Limit:  int32Query(c, "limit", 20),
		Cursor: c.QueryParam("cursor"),
	})
	if err != nil {
		return errorResponse(c, err)
	}

	return c.JSON(http.StatusOK, CustomerListResponse{
		Data: usersJSON(response.GetUsers()),
		Pagination: CursorPagination{
			NextCursor: response.GetNextCursor(),
			HasMore:    response.GetHasMore(),
		},
	})
}

// getCustomer godoc
// @Summary Get a customer
// @Description Returns one user profile by ID. The bearer token must contain the admin role.
// @Tags Admin Customers
// @Produce json
// @Security BearerAuth
// @Param id path string true "Customer ID" format(uuid)
// @Success 200 {object} UserResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/admin/customers/{id} [get]
func (h *Handler) getCustomer(c echo.Context) error {
	ctx := grpcContext(c)
	user, err := h.users.GetProfile(ctx, &bookstorev1.GetProfileRequest{Id: c.Param("id")})
	if err != nil {
		return errorResponse(c, err)
	}
	return c.JSON(http.StatusOK, userJSON(user))
}

// updateCustomer godoc
// @Summary Update a customer
// @Description Updates a customer's display name. The bearer token must contain the admin role.
// @Tags Admin Customers
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Customer ID" format(uuid)
// @Param request body UpdateProfileRequest true "Customer profile payload"
// @Success 200 {object} UserResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/admin/customers/{id} [put]
func (h *Handler) updateCustomer(c echo.Context) error {
	request := UpdateProfileRequest{}
	if err := c.Bind(&request); err != nil {
		return errorResponse(c, err)
	}

	ctx := grpcContext(c)
	user, err := h.users.UpdateProfile(ctx, updateProfileProto(c.Param("id"), request))
	if err != nil {
		return errorResponse(c, err)
	}
	return c.JSON(http.StatusOK, userJSON(user))
}

// deleteCustomer godoc
// @Summary Delete a customer account
// @Description Deletes the auth account transactionally and queues asynchronous profile deletion through the outbox. The bearer token must contain the admin role.
// @Tags Admin Customers
// @Produce json
// @Security BearerAuth
// @Param id path string true "Customer ID" format(uuid)
// @Success 202 {object} DeletionAcceptedResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 409 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/admin/customers/{id} [delete]
func (h *Handler) deleteCustomer(c echo.Context) error {
	principal := principalFromContext(c)
	if principal.UserID == c.Param("id") {
		return c.JSON(http.StatusConflict, errorBody("you cannot delete your own admin account"))
	}

	ctx := grpcContext(c)
	if _, err := h.auth.DeleteAccount(ctx, &bookstorev1.DeleteAccountRequest{Id: c.Param("id")}); err != nil {
		return errorResponse(c, err)
	}
	return c.JSON(http.StatusAccepted, DeletionAcceptedResponse{Status: "accepted"})
}

// listBooks godoc
// @Summary List books
// @Description Returns a paginated list of books.
// @Tags Books
// @Produce json
// @Param limit query int false "Items per request" default(20) minimum(1) maximum(100)
// @Param cursor query string false "Opaque cursor returned by the previous request"
// @Success 200 {object} BookListResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/books [get]
func (h *Handler) listBooks(c echo.Context) error {
	limit := int32Query(c, "limit", 20)
	ctx := grpcContext(c)
	response, err := h.books.ListBooks(ctx, &bookstorev1.ListBooksRequest{
		Limit:  limit,
		Cursor: c.QueryParam("cursor"),
	})
	if err != nil {
		return errorResponse(c, err)
	}

	return c.JSON(http.StatusOK, BookListResponse{
		Data: booksJSON(response.GetBooks()),
		Pagination: CursorPagination{
			NextCursor: response.GetNextCursor(),
			HasMore:    response.GetHasMore(),
		},
	})
}

// getBook godoc
// @Summary Get a book
// @Description Returns one book by ID.
// @Tags Books
// @Produce json
// @Param id path string true "Book ID" format(uuid)
// @Success 200 {object} BookResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/books/{id} [get]
func (h *Handler) getBook(c echo.Context) error {
	ctx := grpcContext(c)
	book, err := h.books.GetBook(ctx, &bookstorev1.GetBookRequest{Id: c.Param("id")})
	if err != nil {
		return errorResponse(c, err)
	}
	return c.JSON(http.StatusOK, bookJSON(book))
}

// createBook godoc
// @Summary Create a book
// @Description Creates a book. The bearer token must contain the admin role.
// @Tags Admin Books
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body BookRequest true "Book payload"
// @Success 201 {object} BookResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 409 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/admin/books [post]
func (h *Handler) createBook(c echo.Context) error {
	request := BookRequest{}
	if err := c.Bind(&request); err != nil {
		return errorResponse(c, err)
	}

	ctx := grpcContext(c)
	book, err := h.books.CreateBook(ctx, createBookProto(request))
	if err != nil {
		return errorResponse(c, err)
	}
	return c.JSON(http.StatusCreated, bookJSON(book))
}

// updateBook godoc
// @Summary Update a book
// @Description Updates a book. The bearer token must contain the admin role.
// @Tags Admin Books
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Book ID" format(uuid)
// @Param request body BookRequest true "Book payload"
// @Success 200 {object} BookResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/admin/books/{id} [put]
func (h *Handler) updateBook(c echo.Context) error {
	request := BookRequest{}
	if err := c.Bind(&request); err != nil {
		return errorResponse(c, err)
	}

	ctx := grpcContext(c)
	book, err := h.books.UpdateBook(ctx, updateBookProto(c.Param("id"), request))
	if err != nil {
		return errorResponse(c, err)
	}
	return c.JSON(http.StatusOK, bookJSON(book))
}

// deleteBook godoc
// @Summary Delete a book
// @Description Deletes a book. The bearer token must contain the admin role.
// @Tags Admin Books
// @Security BearerAuth
// @Param id path string true "Book ID" format(uuid)
// @Success 204
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/admin/books/{id} [delete]
func (h *Handler) deleteBook(c echo.Context) error {
	ctx := grpcContext(c)
	if _, err := h.books.DeleteBook(ctx, &bookstorev1.DeleteBookRequest{Id: c.Param("id")}); err != nil {
		return errorResponse(c, err)
	}
	return c.NoContent(http.StatusNoContent)
}

func int32Query(c echo.Context, key string, fallback int32) int32 {
	value, err := strconv.ParseInt(c.QueryParam(key), 10, 32)
	if err != nil {
		return fallback
	}
	return int32(value)
}
