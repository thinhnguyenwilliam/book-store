package http

import (
	"net/http"
	"strconv"
	"time"

	"github.com/labstack/echo/v4"
	bookstorev1 "github.com/thinhnguyenwilliam/book-store/backend/gen/bookstore/v1"
)

const requestTimeout = 5 * time.Second

type Handler struct {
	auth           bookstorev1.AuthServiceClient
	users          bookstorev1.UserServiceClient
	books          bookstorev1.BookServiceClient
	refreshCookie  RefreshCookieConfig
	trustedOrigins map[string]struct{}
}

type RefreshCookieConfig struct {
	Name     string
	Secure   bool
	SameSite http.SameSite
}

func NewHandler(
	auth bookstorev1.AuthServiceClient,
	users bookstorev1.UserServiceClient,
	books bookstorev1.BookServiceClient,
	refreshCookie RefreshCookieConfig,
	trustedOrigins []string,
) *Handler {
	origins := make(map[string]struct{}, len(trustedOrigins))
	for _, origin := range trustedOrigins {
		origins[origin] = struct{}{}
	}
	return &Handler{
		auth:           auth,
		users:          users,
		books:          books,
		refreshCookie:  refreshCookie,
		trustedOrigins: origins,
	}
}

func (h *Handler) RegisterRoutes(e *echo.Echo) {
	e.GET("/healthz", h.health)

	api := e.Group("/api/v1")
	api.POST("/auth/register", h.register)
	api.POST("/auth/login", h.login)
	api.POST("/auth/refresh", h.refresh)
	api.POST("/auth/logout", h.logout)
	api.GET("/books", h.listBooks)
	api.GET("/books/:id", h.getBook)

	secured := api.Group("")
	secured.Use(h.Authenticate)
	secured.GET("/users/me", h.getMe)
	secured.PUT("/users/me", h.updateMe)

	admin := secured.Group("/admin")
	admin.Use(RequireRole("admin"))
	admin.POST("/books", h.createBook)
	admin.PUT("/books/:id", h.updateBook)
	admin.DELETE("/books/:id", h.deleteBook)
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
// @Success 201 {object} AuthResponse
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

	ctx, cancel := contextWithTimeout(c)
	defer cancel()
	authResult, err := h.auth.Register(ctx, &bookstorev1.RegisterRequest{
		Email:       request.Email,
		Password:    request.Password,
		DisplayName: request.DisplayName,
	})
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

	ctx, cancel := contextWithTimeout(c)
	defer cancel()
	response, err := h.auth.Login(ctx, &bookstorev1.LoginRequest{
		Email:    request.Email,
		Password: request.Password,
	})
	if err != nil {
		return errorResponse(c, err)
	}
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

	ctx, cancel := contextWithTimeout(c)
	defer cancel()
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
		ctx, cancel := contextWithTimeout(c)
		defer cancel()
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
	ctx, cancel := contextWithTimeout(c)
	defer cancel()
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
	ctx, cancel := contextWithTimeout(c)
	defer cancel()
	user, err := h.users.UpdateProfile(ctx, &bookstorev1.UpdateProfileRequest{
		Id:          principal.UserID,
		DisplayName: request.DisplayName,
	})
	if err != nil {
		return errorResponse(c, err)
	}
	return c.JSON(http.StatusOK, userJSON(user))
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
	ctx, cancel := contextWithTimeout(c)
	defer cancel()
	response, err := h.books.ListBooks(ctx, &bookstorev1.ListBooksRequest{
		Limit:  limit,
		Cursor: c.QueryParam("cursor"),
	})
	if err != nil {
		return errorResponse(c, err)
	}

	books := make([]BookResponse, 0, len(response.GetBooks()))
	for _, book := range response.GetBooks() {
		books = append(books, bookJSON(book))
	}
	return c.JSON(http.StatusOK, BookListResponse{
		Data: books,
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
	ctx, cancel := contextWithTimeout(c)
	defer cancel()
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

	ctx, cancel := contextWithTimeout(c)
	defer cancel()
	book, err := h.books.CreateBook(ctx, &bookstorev1.CreateBookRequest{
		Title:      request.Title,
		Author:     request.Author,
		Isbn:       request.ISBN,
		PriceCents: request.PriceCents,
		Stock:      request.Stock,
	})
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

	ctx, cancel := contextWithTimeout(c)
	defer cancel()
	book, err := h.books.UpdateBook(ctx, &bookstorev1.UpdateBookRequest{
		Id:         c.Param("id"),
		Title:      request.Title,
		Author:     request.Author,
		Isbn:       request.ISBN,
		PriceCents: request.PriceCents,
		Stock:      request.Stock,
	})
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
	ctx, cancel := contextWithTimeout(c)
	defer cancel()
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
