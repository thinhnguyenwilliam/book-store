package http

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"
	redis "github.com/redis/go-redis/v9"
	bookstorev1 "github.com/thinhnguyenwilliam/book-store/backend/gen/bookstore/v1"
)

type ChatRealtimeConfig struct {
	RedisAddress    string
	RedisPassword   string
	RedisDatabase   int
	RedisNamespace  string
	RedisChannel    string
	TicketTTL       time.Duration
	PresenceTTL     time.Duration
	PingInterval    time.Duration
	CallTimeout     time.Duration
	MaxMessageBytes int64
	AllowedOrigins  []string
}

type ChatRealtime struct {
	redis          *redis.Client
	chat           bookstorev1.ChatServiceClient
	namespace      string
	channel        string
	ticketTTL      time.Duration
	presenceTTL    time.Duration
	pingInterval   time.Duration
	callTimeout    time.Duration
	maxBytes       int64
	allowedOrigins map[string]struct{}
	clientsMu      sync.RWMutex
	clients        map[string]map[*chatSocketClient]struct{}
	cancel         context.CancelFunc
	pubsub         *redis.PubSub
	workers        sync.WaitGroup
}

type chatSocketClient struct {
	hub       *ChatRealtime
	conn      *websocket.Conn
	principal Principal
	send      chan []byte
}

type chatTicket struct {
	UserID string   `json:"user_id"`
	Email  string   `json:"email"`
	Roles  []string `json:"roles"`
}

type redisChatEvent struct {
	Type          string          `json:"type"`
	AudienceIDs   []string        `json:"audience_ids,omitempty"`
	AdminAudience bool            `json:"admin_audience,omitempty"`
	Data          json.RawMessage `json:"data"`
}

type publicChatEvent struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

type inboundChatEvent struct {
	Type      string          `json:"type"`
	RequestID string          `json:"request_id,omitempty"`
	Data      json.RawMessage `json:"data"`
}

var decrementPresence = redis.NewScript(`
local value = tonumber(redis.call('GET', KEYS[1]) or '0')
if value <= 1 then
  redis.call('DEL', KEYS[1])
  return 0
end
return redis.call('DECR', KEYS[1])
`)

func NewChatRealtime(ctx context.Context, config ChatRealtimeConfig, chat bookstorev1.ChatServiceClient) (*ChatRealtime, error) {
	client := redis.NewClient(&redis.Options{Addr: config.RedisAddress, Password: config.RedisPassword, DB: config.RedisDatabase})
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("connect chat realtime Redis: %w", err)
	}
	origins := make(map[string]struct{}, len(config.AllowedOrigins))
	for _, origin := range config.AllowedOrigins {
		origins[strings.TrimSpace(origin)] = struct{}{}
	}
	return &ChatRealtime{redis: client, chat: chat, namespace: strings.Trim(config.RedisNamespace, ":"), channel: config.RedisChannel, ticketTTL: config.TicketTTL, presenceTTL: config.PresenceTTL, pingInterval: config.PingInterval, callTimeout: config.CallTimeout, maxBytes: config.MaxMessageBytes, allowedOrigins: origins, clients: make(map[string]map[*chatSocketClient]struct{})}, nil
}

func (h *ChatRealtime) Start(parent context.Context) error {
	ctx, cancel := context.WithCancel(parent)
	pubsub := h.redis.Subscribe(ctx, h.key(h.channel))
	if _, err := pubsub.Receive(ctx); err != nil {
		cancel()
		_ = pubsub.Close()
		return fmt.Errorf("subscribe chat realtime channel: %w", err)
	}
	h.cancel = cancel
	h.pubsub = pubsub
	h.workers.Add(1)
	go func() {
		defer h.workers.Done()
		defer func() { _ = pubsub.Close() }()
		for message := range pubsub.Channel() {
			var event redisChatEvent
			if err := json.Unmarshal([]byte(message.Payload), &event); err != nil {
				slog.Warn("discard invalid chat realtime event", "error", err)
				continue
			}
			h.broadcast(event)
		}
	}()
	return nil
}

func (h *ChatRealtime) Close(ctx context.Context) error {
	if h.cancel != nil {
		h.cancel()
	}
	if h.pubsub != nil {
		// Channel() only closes after PubSub.Close. Cancelling the subscription
		// context alone leaves the subscriber worker blocked during shutdown.
		_ = h.pubsub.Close()
	}
	h.clientsMu.RLock()
	sockets := make([]*websocket.Conn, 0)
	for _, userConnections := range h.clients {
		for client := range userConnections {
			sockets = append(sockets, client.conn)
		}
	}
	h.clientsMu.RUnlock()
	for _, socket := range sockets {
		_ = socket.Close()
	}
	done := make(chan struct{})
	go func() { h.workers.Wait(); close(done) }()
	select {
	case <-done:
	case <-ctx.Done():
		return ctx.Err()
	}
	return h.redis.Close()
}

func (h *ChatRealtime) IssueTicket(ctx context.Context, principal Principal) (string, time.Duration, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", 0, fmt.Errorf("generate WebSocket ticket: %w", err)
	}
	token := base64.RawURLEncoding.EncodeToString(bytes)
	payload, err := json.Marshal(chatTicket(principal))
	if err != nil {
		return "", 0, err
	}
	if err := h.redis.Set(ctx, h.ticketKey(token), payload, h.ticketTTL).Err(); err != nil {
		return "", 0, err
	}
	return token, h.ticketTTL, nil
}

func (h *ChatRealtime) Publish(ctx context.Context, eventType string, data any, audience []string, adminAudience bool) error {
	payload, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("encode chat event data: %w", err)
	}
	event, err := json.Marshal(redisChatEvent{Type: eventType, AudienceIDs: audience, AdminAudience: adminAudience, Data: payload})
	if err != nil {
		return fmt.Errorf("encode chat event: %w", err)
	}
	return h.redis.Publish(ctx, h.key(h.channel), event).Err()
}

func (h *ChatRealtime) ServeWebSocket(c echo.Context) error {
	ticket, err := h.consumeTicket(c.Request().Context(), c.QueryParam("ticket"))
	if err != nil {
		return c.JSON(http.StatusUnauthorized, errorBody("invalid or expired WebSocket ticket"))
	}
	upgrader := websocket.Upgrader{HandshakeTimeout: 5 * time.Second, CheckOrigin: h.originAllowed}
	connection, err := upgrader.Upgrade(c.Response(), c.Request(), nil)
	if err != nil {
		return err
	}
	client := &chatSocketClient{hub: h, conn: connection, principal: Principal(ticket), send: make(chan []byte, 64)}
	h.register(client)
	defer h.unregister(client)
	h.workers.Add(1)
	go func() { defer h.workers.Done(); client.writePump() }()
	client.readPump()
	return nil
}

func (h *ChatRealtime) consumeTicket(ctx context.Context, token string) (chatTicket, error) {
	if strings.TrimSpace(token) == "" {
		return chatTicket{}, errors.New("missing ticket")
	}
	payload, err := h.redis.GetDel(ctx, h.ticketKey(token)).Bytes()
	if err != nil {
		return chatTicket{}, err
	}
	var ticket chatTicket
	if err := json.Unmarshal(payload, &ticket); err != nil || ticket.UserID == "" {
		return chatTicket{}, errors.New("invalid ticket")
	}
	return ticket, nil
}

func (h *ChatRealtime) originAllowed(request *http.Request) bool {
	origin := strings.TrimSpace(request.Header.Get("Origin"))
	if origin == "" {
		return true
	}
	_, ok := h.allowedOrigins[origin]
	return ok
}

func (h *ChatRealtime) register(client *chatSocketClient) {
	h.clientsMu.Lock()
	connections := h.clients[client.principal.UserID]
	if connections == nil {
		connections = make(map[*chatSocketClient]struct{})
		h.clients[client.principal.UserID] = connections
	}
	connections[client] = struct{}{}
	h.clientsMu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), h.callTimeout)
	defer cancel()
	count, err := h.redis.Incr(ctx, h.presenceKey(client.principal.UserID)).Result()
	if err == nil {
		_ = h.redis.Expire(ctx, h.presenceKey(client.principal.UserID), h.presenceTTL).Err()
		if count == 1 {
			_ = h.Publish(ctx, "presence.changed", map[string]any{"user_id": client.principal.UserID, "online": true}, []string{client.principal.UserID}, true)
		}
	}
}

func (h *ChatRealtime) unregister(client *chatSocketClient) {
	h.clientsMu.Lock()
	if connections := h.clients[client.principal.UserID]; connections != nil {
		delete(connections, client)
		if len(connections) == 0 {
			delete(h.clients, client.principal.UserID)
		}
	}
	close(client.send)
	h.clientsMu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), h.callTimeout)
	defer cancel()
	count, err := decrementPresence.Run(ctx, h.redis, []string{h.presenceKey(client.principal.UserID)}).Int64()
	if err == nil && count == 0 {
		_ = h.Publish(ctx, "presence.changed", map[string]any{"user_id": client.principal.UserID, "online": false}, []string{client.principal.UserID}, true)
	}
}

func (h *ChatRealtime) broadcast(event redisChatEvent) {
	publicPayload, err := json.Marshal(publicChatEvent{Type: event.Type, Data: event.Data})
	if err != nil {
		return
	}
	audience := make(map[string]struct{}, len(event.AudienceIDs))
	for _, userID := range event.AudienceIDs {
		audience[userID] = struct{}{}
	}
	h.clientsMu.RLock()
	defer h.clientsMu.RUnlock()
	for userID, connections := range h.clients {
		_, direct := audience[userID]
		for client := range connections {
			if !direct && (!event.AdminAudience || !hasRole(client.principal, "admin")) {
				continue
			}
			select {
			case client.send <- publicPayload:
			default:
				// The database is authoritative. A slow client can reconnect and
				// recover missed messages through cursor pagination.
			}
		}
	}
}

func (c *chatSocketClient) readPump() {
	defer func() { _ = c.conn.Close() }()
	c.conn.SetReadLimit(c.hub.maxBytes)
	_ = c.conn.SetReadDeadline(time.Now().Add(2 * c.hub.presenceTTL))
	c.conn.SetPongHandler(func(string) error {
		_ = c.conn.SetReadDeadline(time.Now().Add(2 * c.hub.presenceTTL))
		ctx, cancel := context.WithTimeout(context.Background(), c.hub.callTimeout)
		defer cancel()
		return c.hub.redis.Expire(ctx, c.hub.presenceKey(c.principal.UserID), c.hub.presenceTTL).Err()
	})
	for {
		var event inboundChatEvent
		if err := c.conn.ReadJSON(&event); err != nil {
			return
		}
		c.hub.handleCommand(c, event)
	}
}

func (c *chatSocketClient) writePump() {
	ticker := time.NewTicker(c.hub.pingInterval)
	defer ticker.Stop()
	for {
		select {
		case payload, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
			if !ok {
				_ = c.conn.WriteMessage(websocket.CloseMessage, nil)
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, payload); err != nil {
				return
			}
		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (h *ChatRealtime) handleCommand(client *chatSocketClient, event inboundChatEvent) {
	ctx, cancel := context.WithTimeout(context.Background(), h.callTimeout)
	defer cancel()
	isAdmin := hasRole(client.principal, "admin")
	switch event.Type {
	case "message.send":
		var request ChatMessageRequest
		if json.Unmarshal(event.Data, &request) != nil {
			h.commandError(client, event.RequestID, "invalid message payload")
			return
		}
		var envelope struct {
			ConversationID string `json:"conversation_id"`
		}
		_ = json.Unmarshal(event.Data, &envelope)
		request.ClientMessageID = messageID(request.ClientMessageID)
		item, err := h.chat.SendMessage(ctx, &bookstorev1.SendMessageRequest{ConversationId: envelope.ConversationID, SenderId: client.principal.UserID, IsAdmin: isAdmin, ClientMessageId: request.ClientMessageID, Content: request.Content})
		if err != nil {
			h.commandError(client, event.RequestID, "could not send message")
			return
		}
		if err := h.Publish(ctx, "message.created", chatMessageJSON(item), item.GetAudienceIds(), item.GetAdminAudience()); err != nil {
			h.commandError(client, event.RequestID, "message saved; realtime delivery is delayed")
		}
	case "conversation.read":
		var request struct {
			ConversationID string `json:"conversation_id"`
			SequenceNumber int64  `json:"sequence_number"`
		}
		if json.Unmarshal(event.Data, &request) != nil {
			h.commandError(client, event.RequestID, "invalid read payload")
			return
		}
		conversation, err := h.chat.GetConversation(ctx, &bookstorev1.GetConversationRequest{ConversationId: request.ConversationID, UserId: client.principal.UserID, IsAdmin: isAdmin})
		if err != nil {
			h.commandError(client, event.RequestID, "conversation is not accessible")
			return
		}
		response, err := h.chat.MarkConversationRead(ctx, &bookstorev1.MarkConversationReadRequest{ConversationId: request.ConversationID, UserId: client.principal.UserID, IsAdmin: isAdmin, SequenceNumber: request.SequenceNumber})
		if err != nil {
			h.commandError(client, event.RequestID, "could not mark conversation read")
			return
		}
		_ = h.Publish(ctx, "conversation.read", map[string]any{"conversation_id": request.ConversationID, "user_id": client.principal.UserID, "sequence_number": response.GetLastReadSequence()}, []string{conversation.GetCustomerId()}, true)
	case "typing.changed":
		var request struct {
			ConversationID string `json:"conversation_id"`
			Active         bool   `json:"active"`
		}
		if json.Unmarshal(event.Data, &request) != nil {
			return
		}
		conversation, err := h.chat.GetConversation(ctx, &bookstorev1.GetConversationRequest{ConversationId: request.ConversationID, UserId: client.principal.UserID, IsAdmin: isAdmin})
		if err != nil {
			return
		}
		_ = h.Publish(ctx, "typing.changed", map[string]any{"conversation_id": request.ConversationID, "user_id": client.principal.UserID, "active": request.Active}, []string{conversation.GetCustomerId()}, true)
	case "ping":
		payload, _ := json.Marshal(publicChatEvent{Type: "pong", Data: json.RawMessage(`{}`)})
		select {
		case client.send <- payload:
		default:
		}
	default:
		h.commandError(client, event.RequestID, "unsupported chat event")
	}
}

func (h *ChatRealtime) commandError(client *chatSocketClient, requestID, message string) {
	data, _ := json.Marshal(map[string]string{"request_id": requestID, "message": message})
	payload, _ := json.Marshal(publicChatEvent{Type: "error", Data: data})
	select {
	case client.send <- payload:
	default:
	}
}

func (h *ChatRealtime) key(value string) string {
	value = strings.TrimLeft(value, ":")
	if h.namespace == "" {
		return value
	}
	return h.namespace + ":" + value
}

func (h *ChatRealtime) ticketKey(token string) string    { return h.key("chat:ws-ticket:" + token) }
func (h *ChatRealtime) presenceKey(userID string) string { return h.key("chat:presence:" + userID) }
