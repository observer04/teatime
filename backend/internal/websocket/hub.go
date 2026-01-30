package websocket

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/observer/teatime/internal/auth"
	"github.com/observer/teatime/internal/database"
	"github.com/observer/teatime/internal/domain"
)

// Hub maintains the set of active clients and broadcasts messages
type Hub struct {
	// Registered clients by user ID (one user can have multiple connections)
	clients map[uuid.UUID]map[*Client]bool

	// Room subscriptions: conversation_id -> set of clients
	rooms map[uuid.UUID]map[*Client]bool

	// Channel for registering clients
	register chan *Client

	// Channel for unregistering clients
	unregister chan *Client

	// Mutex for thread-safe access
	mu sync.RWMutex

	// Dependencies
	authService *auth.Service
	convRepo    *database.ConversationRepository
	userRepo    *database.UserRepository
	logger      *slog.Logger
}

// NewHub creates a new Hub
func NewHub(authService *auth.Service, convRepo *database.ConversationRepository, userRepo *database.UserRepository, logger *slog.Logger) *Hub {
	return &Hub{
		clients:     make(map[uuid.UUID]map[*Client]bool),
		rooms:       make(map[uuid.UUID]map[*Client]bool),
		register:    make(chan *Client),
		unregister:  make(chan *Client),
		authService: authService,
		convRepo:    convRepo,
		userRepo:    userRepo,
		logger:      logger,
	}
}

// Run starts the hub's main loop
func (h *Hub) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case client := <-h.register:
			h.handleRegister(client)
		case client := <-h.unregister:
			h.handleUnregister(client)
		}
	}
}

// Register adds a client to the hub
func (h *Hub) Register(client *Client) {
	h.register <- client
}

// Unregister removes a client from the hub
func (h *Hub) Unregister(client *Client) {
	h.unregister <- client
}

func (h *Hub) handleRegister(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Client not authenticated yet, just track it
	h.logger.Debug("client connected", "remote_addr", client.conn.RemoteAddr())
}

func (h *Hub) handleUnregister(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	userID := client.UserID()
	if userID != uuid.Nil {
		// Remove from user's client set
		if clients, ok := h.clients[userID]; ok {
			delete(clients, client)
			if len(clients) == 0 {
				delete(h.clients, userID)
				// User is now offline - could broadcast presence here
			}
		}
	}

	// Remove from all rooms
	for roomID := range client.rooms {
		if room, ok := h.rooms[roomID]; ok {
			delete(room, client)
			if len(room) == 0 {
				delete(h.rooms, roomID)
			}
		}
	}

	close(client.send)
	h.logger.Debug("client disconnected", "user_id", userID)
}

// HandleMessage processes incoming WebSocket messages
func (h *Hub) HandleMessage(client *Client, msg *Message) {
	switch msg.Type {
	case EventTypeAuth:
		h.handleAuth(client, msg.Payload)
	case EventTypeRoomJoin:
		h.handleRoomJoin(client, msg.Payload)
	case EventTypeRoomLeave:
		h.handleRoomLeave(client, msg.Payload)
	case EventTypeMessageSend:
		h.handleMessageSend(client, msg.Payload)
	case EventTypeTypingStart:
		h.handleTyping(client, msg.Payload, true)
	case EventTypeTypingStop:
		h.handleTyping(client, msg.Payload, false)
	default:
		client.sendError("unknown_event", "Unknown event type: "+msg.Type)
	}
}

func (h *Hub) handleAuth(client *Client, payload json.RawMessage) {
	var p AuthPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		client.sendError("invalid_payload", "Invalid auth payload")
		return
	}

	// Validate token
	claims, err := h.authService.ValidateToken(p.Token)
	if err != nil {
		client.sendError("auth_failed", "Invalid or expired token")
		return
	}

	// Set user info on client
	client.SetUser(claims.UserID, claims.Username)

	// Register client to user's connection set
	h.mu.Lock()
	if h.clients[claims.UserID] == nil {
		h.clients[claims.UserID] = make(map[*Client]bool)
	}
	h.clients[claims.UserID][client] = true
	h.mu.Unlock()

	// Send success
	msg, _ := NewMessage(EventTypeAuthSuccess, AuthSuccessPayload{
		UserID:   claims.UserID,
		Username: claims.Username,
	})
	client.Send(msg)

	h.logger.Info("client authenticated", "user_id", claims.UserID, "username", claims.Username)
}

func (h *Hub) handleRoomJoin(client *Client, payload json.RawMessage) {
	if !client.IsAuthenticated() {
		client.sendError("not_authenticated", "Must authenticate first")
		return
	}

	var p RoomJoinPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		client.sendError("invalid_payload", "Invalid room join payload")
		return
	}

	convID, err := uuid.Parse(p.ConversationID)
	if err != nil {
		client.sendError("invalid_conversation", "Invalid conversation ID")
		return
	}

	// Check if user is a member
	ctx := context.Background()
	isMember, err := h.convRepo.IsMember(ctx, convID, client.UserID())
	if err != nil || !isMember {
		client.sendError("not_member", "Not a member of this conversation")
		return
	}

	// Add to room
	client.JoinRoom(convID)

	h.mu.Lock()
	if h.rooms[convID] == nil {
		h.rooms[convID] = make(map[*Client]bool)
	}
	h.rooms[convID][client] = true
	h.mu.Unlock()

	h.logger.Debug("client joined room", "user_id", client.UserID(), "room_id", convID)
}

func (h *Hub) handleRoomLeave(client *Client, payload json.RawMessage) {
	var p RoomLeavePayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return
	}

	convID, err := uuid.Parse(p.ConversationID)
	if err != nil {
		return
	}

	client.LeaveRoom(convID)

	h.mu.Lock()
	if room, ok := h.rooms[convID]; ok {
		delete(room, client)
		if len(room) == 0 {
			delete(h.rooms, convID)
		}
	}
	h.mu.Unlock()
}

func (h *Hub) handleMessageSend(client *Client, payload json.RawMessage) {
	if !client.IsAuthenticated() {
		client.sendError("not_authenticated", "Must authenticate first")
		return
	}

	var p MessageSendPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		client.sendError("invalid_payload", "Invalid message payload")
		return
	}

	convID, err := uuid.Parse(p.ConversationID)
	if err != nil {
		client.sendError("invalid_conversation", "Invalid conversation ID")
		return
	}

	if p.BodyText == "" {
		client.sendError("empty_message", "Message cannot be empty")
		return
	}

	if len(p.BodyText) > 10000 {
		client.sendError("message_too_long", "Message exceeds 10000 characters")
		return
	}

	// Check membership
	ctx := context.Background()
	isMember, err := h.convRepo.IsMember(ctx, convID, client.UserID())
	if err != nil || !isMember {
		client.sendError("not_member", "Not a member of this conversation")
		return
	}

	// Create message
	userID := client.UserID()
	msg := &domain.Message{
		ID:             uuid.New(),
		ConversationID: convID,
		SenderID:       &userID,
		BodyText:       p.BodyText,
		CreatedAt:      time.Now(),
	}

	// Save to database
	if err := h.convRepo.CreateMessage(ctx, msg); err != nil {
		h.logger.Error("failed to save message", "error", err)
		client.sendError("save_failed", "Failed to save message")
		return
	}

	// Broadcast to room
	broadcastPayload := MessageNewPayload{
		ID:             msg.ID,
		ConversationID: convID,
		SenderID:       userID,
		SenderUsername: client.Username(),
		BodyText:       msg.BodyText,
		CreatedAt:      msg.CreatedAt,
		TempID:         p.TempID,
	}

	h.BroadcastToRoom(convID, EventTypeMessageNew, broadcastPayload)
}

func (h *Hub) handleTyping(client *Client, payload json.RawMessage, isTyping bool) {
	if !client.IsAuthenticated() {
		return
	}

	var p TypingPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return
	}

	convID, err := uuid.Parse(p.ConversationID)
	if err != nil {
		return
	}

	// Broadcast typing indicator to other room members
	broadcastPayload := TypingBroadcastPayload{
		ConversationID: convID,
		UserID:         client.UserID(),
		Username:       client.Username(),
		IsTyping:       isTyping,
	}

	h.BroadcastToRoomExcept(convID, client, EventTypeTyping, broadcastPayload)
}

// BroadcastToRoom sends a message to all clients in a room
func (h *Hub) BroadcastToRoom(roomID uuid.UUID, eventType string, payload interface{}) {
	h.mu.RLock()
	room, ok := h.rooms[roomID]
	if !ok {
		h.mu.RUnlock()
		return
	}

	// Copy clients to avoid holding lock during send
	clients := make([]*Client, 0, len(room))
	for client := range room {
		clients = append(clients, client)
	}
	h.mu.RUnlock()

	msg, err := NewMessage(eventType, payload)
	if err != nil {
		h.logger.Error("failed to create broadcast message", "error", err)
		return
	}

	for _, client := range clients {
		client.Send(msg)
	}
}

// BroadcastToRoomExcept sends to all room members except one
func (h *Hub) BroadcastToRoomExcept(roomID uuid.UUID, except *Client, eventType string, payload interface{}) {
	h.mu.RLock()
	room, ok := h.rooms[roomID]
	if !ok {
		h.mu.RUnlock()
		return
	}

	clients := make([]*Client, 0, len(room))
	for client := range room {
		if client != except {
			clients = append(clients, client)
		}
	}
	h.mu.RUnlock()

	msg, err := NewMessage(eventType, payload)
	if err != nil {
		return
	}

	for _, client := range clients {
		client.Send(msg)
	}
}

// BroadcastToUser sends to all connections of a specific user
func (h *Hub) BroadcastToUser(userID uuid.UUID, eventType string, payload interface{}) {
	h.mu.RLock()
	userClients, ok := h.clients[userID]
	if !ok {
		h.mu.RUnlock()
		return
	}

	clients := make([]*Client, 0, len(userClients))
	for client := range userClients {
		clients = append(clients, client)
	}
	h.mu.RUnlock()

	msg, err := NewMessage(eventType, payload)
	if err != nil {
		return
	}

	for _, client := range clients {
		client.Send(msg)
	}
}

// GetOnlineUserIDs returns IDs of all online users
func (h *Hub) GetOnlineUserIDs() []uuid.UUID {
	h.mu.RLock()
	defer h.mu.RUnlock()

	ids := make([]uuid.UUID, 0, len(h.clients))
	for id := range h.clients {
		ids = append(ids, id)
	}
	return ids
}

// IsUserOnline checks if a user has any active connections
func (h *Hub) IsUserOnline(userID uuid.UUID) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	clients, ok := h.clients[userID]
	return ok && len(clients) > 0
}
