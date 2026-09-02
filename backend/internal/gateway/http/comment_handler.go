package http

import (
	"net/http"

	"github.com/labstack/echo/v4"
	bookstorev1 "github.com/thinhnguyenwilliam/book-store/backend/gen/bookstore/v1"
	customeractivity "github.com/thinhnguyenwilliam/book-store/backend/internal/events/customeractivity"
)

type CommentRequest struct {
	Content  string `json:"content"`
	ParentID string `json:"parent_id,omitempty"`
}
type UpdateCommentRequest struct {
	Content string `json:"content"`
}
type ModerateCommentRequest struct {
	Status string `json:"status"`
}
type CommentResponse struct {
	ID         string `json:"id"`
	BookID     string `json:"book_id"`
	AuthorID   string `json:"author_id"`
	AuthorName string `json:"author_name"`
	ParentID   string `json:"parent_id,omitempty"`
	RootID     string `json:"root_id"`
	Depth      int32  `json:"depth"`
	Content    string `json:"content"`
	Status     string `json:"status"`
	ReplyCount int64  `json:"reply_count"`
	CreatedAt  string `json:"created_at"`
	UpdatedAt  string `json:"updated_at"`
}
type CommentListResponse struct {
	Data       []CommentResponse `json:"data"`
	Pagination CursorPagination  `json:"pagination"`
}

// listBookComments godoc
// @Summary List root comments of a book
// @Tags Comments
// @Produce json
// @Param id path string true "Book ID"
// @Param limit query int false "Page size"
// @Param cursor query string false "Opaque cursor"
// @Success 200 {object} CommentListResponse
// @Router /api/v1/books/{id}/comments [get]
func (h *Handler) listBookComments(c echo.Context) error {
	response, err := h.comments.ListBookComments(grpcContext(c), &bookstorev1.ListBookCommentsRequest{BookId: c.Param("id"), Limit: int32Query(c, "limit", 20), Cursor: c.QueryParam("cursor")})
	if err != nil {
		return errorResponse(c, err)
	}
	return c.JSON(http.StatusOK, commentListJSON(response))
}

// listCommentReplies godoc
// @Summary List replies in a comment thread
// @Tags Comments
// @Produce json
// @Param id path string true "Root comment ID"
// @Param limit query int false "Page size"
// @Param cursor query string false "Opaque cursor"
// @Success 200 {object} CommentListResponse
// @Router /api/v1/comments/{id}/replies [get]
func (h *Handler) listCommentReplies(c echo.Context) error {
	response, err := h.comments.ListCommentReplies(grpcContext(c), &bookstorev1.ListCommentRepliesRequest{RootId: c.Param("id"), Limit: int32Query(c, "limit", 50), Cursor: c.QueryParam("cursor")})
	if err != nil {
		return errorResponse(c, err)
	}
	return c.JSON(http.StatusOK, commentListJSON(response))
}

// createComment godoc
// @Summary Comment on a book or reply to a comment
// @Tags Comments
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path string true "Book ID"
// @Param request body CommentRequest true "Comment content and optional parent_id"
// @Success 201 {object} CommentResponse
// @Router /api/v1/books/{id}/comments [post]
func (h *Handler) createComment(c echo.Context) error {
	var request CommentRequest
	if err := c.Bind(&request); err != nil {
		return errorResponse(c, err)
	}
	item, err := h.comments.CreateComment(grpcContext(c), &bookstorev1.CreateCommentRequest{BookId: c.Param("id"), AuthorId: principalFromContext(c).UserID, ParentId: request.ParentID, Content: request.Content})
	if err != nil {
		return errorResponse(c, err)
	}
	h.recordServerActivity(c, customeractivity.EventCommentCreated, item.GetBookId(), "", item.GetId(), 0)
	return c.JSON(http.StatusCreated, commentJSON(item))
}

// updateComment godoc
// @Summary Update own comment
// @Tags Comments
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path string true "Comment ID"
// @Param request body UpdateCommentRequest true "New content"
// @Success 200 {object} CommentResponse
// @Router /api/v1/comments/{id} [put]
func (h *Handler) updateComment(c echo.Context) error {
	var request UpdateCommentRequest
	if err := c.Bind(&request); err != nil {
		return errorResponse(c, err)
	}
	item, err := h.comments.UpdateComment(grpcContext(c), &bookstorev1.UpdateCommentRequest{Id: c.Param("id"), AuthorId: principalFromContext(c).UserID, Content: request.Content})
	if err != nil {
		return errorResponse(c, err)
	}
	return c.JSON(http.StatusOK, commentJSON(item))
}

// deleteComment godoc
// @Summary Soft-delete own comment
// @Tags Comments
// @Security BearerAuth
// @Produce json
// @Param id path string true "Comment ID"
// @Success 200 {object} CommentResponse
// @Router /api/v1/comments/{id} [delete]
func (h *Handler) deleteComment(c echo.Context) error {
	principal := principalFromContext(c)
	item, err := h.comments.DeleteComment(grpcContext(c), &bookstorev1.DeleteCommentRequest{Id: c.Param("id"), ActorId: principal.UserID, IsAdmin: hasRole(principal, "admin")})
	if err != nil {
		return errorResponse(c, err)
	}
	return c.JSON(http.StatusOK, commentJSON(item))
}

// moderateComment godoc
// @Summary Hide or publish a comment
// @Tags Admin,Comments
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path string true "Comment ID"
// @Param request body ModerateCommentRequest true "published or hidden"
// @Success 200 {object} CommentResponse
// @Router /api/v1/admin/comments/{id}/status [put]
func (h *Handler) moderateComment(c echo.Context) error {
	var request ModerateCommentRequest
	if err := c.Bind(&request); err != nil {
		return errorResponse(c, err)
	}
	item, err := h.comments.ModerateComment(grpcContext(c), &bookstorev1.ModerateCommentRequest{Id: c.Param("id"), Status: request.Status})
	if err != nil {
		return errorResponse(c, err)
	}
	return c.JSON(http.StatusOK, commentJSON(item))
}

func commentListJSON(response *bookstorev1.ListCommentsResponse) CommentListResponse {
	items := make([]CommentResponse, 0, len(response.GetComments()))
	for _, item := range response.GetComments() {
		items = append(items, commentJSON(item))
	}
	return CommentListResponse{Data: items, Pagination: CursorPagination{NextCursor: response.GetNextCursor(), HasMore: response.GetHasMore()}}
}
func commentJSON(item *bookstorev1.Comment) CommentResponse {
	return CommentResponse{ID: item.GetId(), BookID: item.GetBookId(), AuthorID: item.GetAuthorId(), AuthorName: item.GetAuthorName(), ParentID: item.GetParentId(), RootID: item.GetRootId(), Depth: item.GetDepth(), Content: item.GetContent(), Status: item.GetStatus(), ReplyCount: item.GetReplyCount(), CreatedAt: item.GetCreatedAt(), UpdatedAt: item.GetUpdatedAt()}
}
func hasRole(principal Principal, role string) bool {
	for _, value := range principal.Roles {
		if value == role {
			return true
		}
	}
	return false
}
