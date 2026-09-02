package http

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"
	bookstorev1 "github.com/thinhnguyenwilliam/book-store/backend/gen/bookstore/v1"
)

// searchBooks godoc
// @Summary Search books
// @Description Full-text catalog search with typo tolerance, autocomplete-aware relevance, filters and cursor pagination.
// @Tags Books
// @Produce json
// @Param q query string false "Title, author or ISBN" maxlength(200)
// @Param limit query int false "Items per request" default(20) minimum(1) maximum(50)
// @Param cursor query string false "Opaque cursor returned by the previous request"
// @Param min_price_cents query int false "Minimum price in cents" minimum(0)
// @Param max_price_cents query int false "Maximum price in cents" minimum(0)
// @Param in_stock query bool false "Filter by stock availability"
// @Param seller_id query string false "Seller UUID"
// @Param author query string false "Exact author filter"
// @Param sort query string false "Sort mode" Enums(relevance,newest,price_asc,price_desc) default(relevance)
// @Success 200 {object} BookSearchResponse
// @Failure 400 {object} ErrorResponse
// @Failure 503 {object} ErrorResponse
// @Router /api/v1/books/search [get]
func (h *Handler) searchBooks(c echo.Context) error {
	if h.search == nil {
		return c.JSON(http.StatusServiceUnavailable, errorBody("book search is temporarily unavailable"))
	}
	request := &bookstorev1.SearchBooksRequest{
		Query: c.QueryParam("q"), Cursor: c.QueryParam("cursor"), Sort: c.QueryParam("sort"),
		SellerId: c.QueryParam("seller_id"), Author: c.QueryParam("author"),
		Limit: int32Query(c, "limit", 20),
	}
	var err error
	if raw := strings.TrimSpace(c.QueryParam("min_price_cents")); raw != "" {
		request.MinPriceCents, err = strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return c.JSON(http.StatusBadRequest, errorBody("invalid search input"))
		}
	}
	if raw := strings.TrimSpace(c.QueryParam("max_price_cents")); raw != "" {
		request.MaxPriceCents, err = strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return c.JSON(http.StatusBadRequest, errorBody("invalid search input"))
		}
	}
	if raw := strings.TrimSpace(c.QueryParam("in_stock")); raw != "" {
		value, parseErr := strconv.ParseBool(raw)
		if parseErr != nil {
			return c.JSON(http.StatusBadRequest, errorBody("invalid search input"))
		}
		request.InStock = &value
	}
	response, err := h.search.SearchBooks(grpcContext(c), request)
	if err != nil {
		return errorResponse(c, err)
	}
	return c.JSON(http.StatusOK, BookSearchResponse{
		Data:       searchHitsJSON(response.GetHits()),
		Pagination: CursorPagination{NextCursor: response.GetNextCursor(), HasMore: response.GetHasMore()},
		Total:      response.GetTotal(), TookMS: response.GetTookMs(),
	})
}

// suggestBooks godoc
// @Summary Autocomplete books
// @Description Returns typo-tolerant title and author suggestions for an as-you-type query.
// @Tags Books
// @Produce json
// @Param q query string true "At least two characters" minlength(2) maxlength(120)
// @Param limit query int false "Maximum suggestions" default(8) minimum(1) maximum(10)
// @Success 200 {object} BookSuggestionResponse
// @Failure 400 {object} ErrorResponse
// @Failure 503 {object} ErrorResponse
// @Router /api/v1/books/suggest [get]
func (h *Handler) suggestBooks(c echo.Context) error {
	if h.search == nil {
		return c.JSON(http.StatusServiceUnavailable, errorBody("book search is temporarily unavailable"))
	}
	response, err := h.search.SuggestBooks(grpcContext(c), &bookstorev1.SuggestBooksRequest{
		Query: c.QueryParam("q"), Limit: int32Query(c, "limit", 8),
	})
	if err != nil {
		return errorResponse(c, err)
	}
	return c.JSON(http.StatusOK, BookSuggestionResponse{Data: searchHitsJSON(response.GetHits()), TookMS: response.GetTookMs()})
}

func searchHitsJSON(hits []*bookstorev1.SearchBookHit) []BookSearchHitResponse {
	result := make([]BookSearchHitResponse, 0, len(hits))
	for _, hit := range hits {
		result = append(result, BookSearchHitResponse{
			Book: bookJSON(hit.GetBook()), Score: hit.GetScore(), Highlights: hit.GetHighlights(),
		})
	}
	return result
}
