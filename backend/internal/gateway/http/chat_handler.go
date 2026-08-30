package http

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	bookstorev1 "github.com/thinhnguyenwilliam/book-store/backend/gen/bookstore/v1"
)

type ChatMessageRequest struct {
	ClientMessageID string `json:"client_message_id"`
	Content         string `json:"content"`
}

type ChatMessageUpdateRequest struct {
	Content string `json:"content"`
}

type ChatReadRequest struct {
	SequenceNumber int64 `json:"sequence_number"`
}

type ConversationResponse struct {
	ID                  string `json:"id"`
	CustomerID          string `json:"customer_id"`
	Type                string `json:"type"`
	Status              string `json:"status"`
	LastMessageSequence int64  `json:"last_message_sequence"`
	LastMessagePreview  string `json:"last_message_preview"`
	LastMessageAt       string `json:"last_message_at,omitempty"`
	UnreadCount         int64  `json:"unread_count"`
	CreatedAt           string `json:"created_at"`
	UpdatedAt           string `json:"updated_at"`
}

type ChatMessageResponse struct {
	ID              string `json:"id"`
	ConversationID  string `json:"conversation_id"`
	SenderID        string `json:"sender_id"`
	SenderName      string `json:"sender_name"`
	ClientMessageID string `json:"client_message_id"`
	SequenceNumber  int64  `json:"sequence_number"`
	Content         string `json:"content"`
	MessageType     string `json:"message_type"`
	CreatedAt       string `json:"created_at"`
	EditedAt        string `json:"edited_at,omitempty"`
	DeletedAt       string `json:"deleted_at,omitempty"`
}

type ConversationListResponse struct {
	Data       []ConversationResponse `json:"data"`
	Pagination CursorPagination       `json:"pagination"`
}

type ChatMessageListResponse struct {
	Data       []ChatMessageResponse `json:"data"`
	Pagination CursorPagination      `json:"pagination"`
}

// createSupportConversation godoc
// @Summary Open or return the current support conversation
// @Tags Chat
// @Security BearerAuth
// @Produce json
// @Success 201 {object} ConversationResponse
// @Router /api/v1/chat/conversations/support [post]
func (h *Handler) createSupportConversation(c echo.Context) error {
	principal := principalFromContext(c)
	item, err := h.chat.CreateSupportConversation(grpcContext(c), &bookstorev1.CreateSupportConversationRequest{CustomerId: principal.UserID})
	if err != nil {
		return errorResponse(c, err)
	}
	h.publishChatEvent(c, "conversation.updated", conversationJSON(item), []string{principal.UserID}, true)
	return c.JSON(http.StatusCreated, conversationJSON(item))
}

// listChatConversations godoc
// @Summary List accessible chat conversations
// @Tags Chat
// @Security BearerAuth
// @Produce json
// @Param limit query int false "Page size"
// @Param cursor query string false "Opaque cursor"
// @Success 200 {object} ConversationListResponse
// @Router /api/v1/chat/conversations [get]
func (h *Handler) listChatConversations(c echo.Context) error {
	principal := principalFromContext(c)
	response, err := h.chat.ListConversations(grpcContext(c), &bookstorev1.ListConversationsRequest{UserId: principal.UserID, IsAdmin: hasRole(principal, "admin"), Limit: int32Query(c, "limit", 20), Cursor: c.QueryParam("cursor")})
	if err != nil {
		return errorResponse(c, err)
	}
	items := make([]ConversationResponse, 0, len(response.GetConversations()))
	for _, item := range response.GetConversations() {
		items = append(items, conversationJSON(item))
	}
	return c.JSON(http.StatusOK, ConversationListResponse{Data: items, Pagination: CursorPagination{NextCursor: response.GetNextCursor(), HasMore: response.GetHasMore()}})
}

// listChatMessages godoc
// @Summary List conversation messages using cursor pagination
// @Tags Chat
// @Security BearerAuth
// @Produce json
// @Param id path string true "Conversation ID"
// @Param limit query int false "Page size"
// @Param cursor query string false "Opaque cursor"
// @Success 200 {object} ChatMessageListResponse
// @Router /api/v1/chat/conversations/{id}/messages [get]
func (h *Handler) listChatMessages(c echo.Context) error {
	principal := principalFromContext(c)
	response, err := h.chat.ListMessages(grpcContext(c), &bookstorev1.ListMessagesRequest{ConversationId: c.Param("id"), UserId: principal.UserID, IsAdmin: hasRole(principal, "admin"), Limit: int32Query(c, "limit", 30), Cursor: c.QueryParam("cursor")})
	if err != nil {
		return errorResponse(c, err)
	}
	items := make([]ChatMessageResponse, 0, len(response.GetMessages()))
	for _, item := range response.GetMessages() {
		items = append(items, chatMessageJSON(item))
	}
	return c.JSON(http.StatusOK, ChatMessageListResponse{Data: items, Pagination: CursorPagination{NextCursor: response.GetNextCursor(), HasMore: response.GetHasMore()}})
}

// sendChatMessage godoc
// @Summary Send an idempotent text message
// @Tags Chat
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path string true "Conversation ID"
// @Param request body ChatMessageRequest true "Message"
// @Success 201 {object} ChatMessageResponse
// @Router /api/v1/chat/conversations/{id}/messages [post]
func (h *Handler) sendChatMessage(c echo.Context) error {
	var request ChatMessageRequest
	if err := c.Bind(&request); err != nil {
		return errorResponse(c, err)
	}
	if strings.TrimSpace(request.ClientMessageID) == "" {
		request.ClientMessageID = c.Request().Header.Get("Idempotency-Key")
	}
	principal := principalFromContext(c)
	item, err := h.chat.SendMessage(grpcContext(c), &bookstorev1.SendMessageRequest{ConversationId: c.Param("id"), SenderId: principal.UserID, IsAdmin: hasRole(principal, "admin"), ClientMessageId: request.ClientMessageID, Content: request.Content})
	if err != nil {
		return errorResponse(c, err)
	}
	h.publishMessage(c, "message.created", item)
	return c.JSON(http.StatusCreated, chatMessageJSON(item))
}

// updateChatMessage godoc
// @Summary Update an own chat message
// @Tags Chat
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path string true "Message ID"
// @Param request body ChatMessageUpdateRequest true "New content"
// @Success 200 {object} ChatMessageResponse
// @Router /api/v1/chat/messages/{id} [put]
func (h *Handler) updateChatMessage(c echo.Context) error {
	var request ChatMessageUpdateRequest
	if err := c.Bind(&request); err != nil {
		return errorResponse(c, err)
	}
	principal := principalFromContext(c)
	item, err := h.chat.UpdateMessage(grpcContext(c), &bookstorev1.UpdateMessageRequest{Id: c.Param("id"), ActorId: principal.UserID, IsAdmin: hasRole(principal, "admin"), Content: request.Content})
	if err != nil {
		return errorResponse(c, err)
	}
	h.publishMessage(c, "message.updated", item)
	return c.JSON(http.StatusOK, chatMessageJSON(item))
}

// deleteChatMessage godoc
// @Summary Soft-delete an own message; admins may remove abusive messages
// @Tags Chat
// @Security BearerAuth
// @Produce json
// @Param id path string true "Message ID"
// @Success 200 {object} ChatMessageResponse
// @Router /api/v1/chat/messages/{id} [delete]
func (h *Handler) deleteChatMessage(c echo.Context) error {
	principal := principalFromContext(c)
	item, err := h.chat.DeleteMessage(grpcContext(c), &bookstorev1.DeleteMessageRequest{Id: c.Param("id"), ActorId: principal.UserID, IsAdmin: hasRole(principal, "admin")})
	if err != nil {
		return errorResponse(c, err)
	}
	h.publishMessage(c, "message.deleted", item)
	return c.JSON(http.StatusOK, chatMessageJSON(item))
}

// markChatRead godoc
// @Summary Advance the current user's read cursor
// @Tags Chat
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path string true "Conversation ID"
// @Param request body ChatReadRequest true "Last read sequence; zero means latest"
// @Success 200 {object} map[string]int64
// @Router /api/v1/chat/conversations/{id}/read [put]
func (h *Handler) markChatRead(c echo.Context) error {
	var request ChatReadRequest
	if err := c.Bind(&request); err != nil {
		return errorResponse(c, err)
	}
	principal := principalFromContext(c)
	response, err := h.chat.MarkConversationRead(grpcContext(c), &bookstorev1.MarkConversationReadRequest{ConversationId: c.Param("id"), UserId: principal.UserID, IsAdmin: hasRole(principal, "admin"), SequenceNumber: request.SequenceNumber})
	if err != nil {
		return errorResponse(c, err)
	}
	conversation, err := h.chat.GetConversation(grpcContext(c), &bookstorev1.GetConversationRequest{ConversationId: c.Param("id"), UserId: principal.UserID, IsAdmin: hasRole(principal, "admin")})
	if err == nil {
		h.publishChatEvent(c, "conversation.read", map[string]any{"conversation_id": c.Param("id"), "user_id": principal.UserID, "sequence_number": response.GetLastReadSequence()}, []string{conversation.GetCustomerId()}, true)
	}
	return c.JSON(http.StatusOK, map[string]any{"last_read_sequence": response.GetLastReadSequence()})
}

// unreadChatCount godoc
// @Summary Count unread chat messages
// @Tags Chat
// @Security BearerAuth
// @Produce json
// @Success 200 {object} map[string]int64
// @Router /api/v1/chat/unread-count [get]
func (h *Handler) unreadChatCount(c echo.Context) error {
	principal := principalFromContext(c)
	response, err := h.chat.GetUnreadChatCount(grpcContext(c), &bookstorev1.GetUnreadChatCountRequest{UserId: principal.UserID, IsAdmin: hasRole(principal, "admin")})
	if err != nil {
		return errorResponse(c, err)
	}
	return c.JSON(http.StatusOK, map[string]int64{"count": response.GetCount()})
}

// issueChatWebSocketTicket godoc
// @Summary Issue a one-time short-lived WebSocket ticket
// @Tags Chat
// @Security BearerAuth
// @Produce json
// @Success 201 {object} map[string]interface{}
// @Router /api/v1/chat/ws-ticket [post]
func (h *Handler) issueChatWebSocketTicket(c echo.Context) error {
	principal := principalFromContext(c)
	ticket, expiresIn, err := h.realtime.IssueTicket(c.Request().Context(), principal)
	if err != nil {
		slog.ErrorContext(c.Request().Context(), "issue WebSocket ticket", "error", err)
		return c.JSON(http.StatusServiceUnavailable, errorBody("chat realtime is temporarily unavailable"))
	}
	return c.JSON(http.StatusCreated, map[string]any{"ticket": ticket, "expires_in": int64(expiresIn.Seconds())})
}

// chatWebSocket godoc
// @Summary Upgrade to the chat WebSocket using a one-time ticket
// @Tags Chat
// @Param ticket query string true "One-time WebSocket ticket"
// @Success 101 {string} string "Switching Protocols"
// @Router /api/v1/chat/ws [get]
func (h *Handler) chatWebSocket(c echo.Context) error { return h.realtime.ServeWebSocket(c) }

func (h *Handler) publishMessage(c echo.Context, eventType string, item *bookstorev1.Message) {
	h.publishChatEvent(c, eventType, chatMessageJSON(item), item.GetAudienceIds(), item.GetAdminAudience())
}

func (h *Handler) publishChatEvent(c echo.Context, eventType string, data any, audience []string, adminAudience bool) {
	if h.realtime == nil {
		return
	}
	if err := h.realtime.Publish(c.Request().Context(), eventType, data, audience, adminAudience); err != nil {
		slog.WarnContext(c.Request().Context(), "publish chat realtime event failed", "event_type", eventType, "error", err)
	}
}

func conversationJSON(item *bookstorev1.Conversation) ConversationResponse {
	return ConversationResponse{ID: item.GetId(), CustomerID: item.GetCustomerId(), Type: item.GetType(), Status: item.GetStatus(), LastMessageSequence: item.GetLastMessageSequence(), LastMessagePreview: item.GetLastMessagePreview(), LastMessageAt: item.GetLastMessageAt(), UnreadCount: item.GetUnreadCount(), CreatedAt: item.GetCreatedAt(), UpdatedAt: item.GetUpdatedAt()}
}

func chatMessageJSON(item *bookstorev1.Message) ChatMessageResponse {
	return ChatMessageResponse{ID: item.GetId(), ConversationID: item.GetConversationId(), SenderID: item.GetSenderId(), SenderName: item.GetSenderName(), ClientMessageID: item.GetClientMessageId(), SequenceNumber: item.GetSequenceNumber(), Content: item.GetContent(), MessageType: item.GetMessageType(), CreatedAt: item.GetCreatedAt(), EditedAt: item.GetEditedAt(), DeletedAt: item.GetDeletedAt()}
}

func messageID(value string) string {
	if _, err := uuid.Parse(value); err == nil {
		return value
	}
	return uuid.NewString()
}
