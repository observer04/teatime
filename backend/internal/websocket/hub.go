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
	"github.com/observer/teatime/internal/pubsub"
	"github.com/observer/teatime/internal/webrtc"
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
	authService    *auth.Service
	convRepo       *database.ConversationRepository
	userRepo       *database.UserRepository
	attachmentRepo *database.AttachmentRepository
	pubsub         pubsub.PubSub
	callHandler    *webrtc.CallHandler
	sfuHandler     *webrtc.SFUHandler
	logger         *slog.Logger

	// PubSub subscriptions for room-level events
	roomSubs map[uuid.UUID]pubsub.Subscription
}

// NewHub creates a new Hub
func NewHub(authService *auth.Service, convRepo *database.ConversationRepository, userRepo *database.UserRepository, attachmentRepo *database.AttachmentRepository, ps pubsub.PubSub, logger *slog.Logger) *Hub {
	return &Hub{
		clients:        make(map[uuid.UUID]map[*Client]bool),
		rooms:          make(map[uuid.UUID]map[*Client]bool),
		register:       make(chan *Client),
		unregister:     make(chan *Client),
		authService:    authService,
		convRepo:       convRepo,
		userRepo:       userRepo,
		attachmentRepo: attachmentRepo,
		pubsub:         ps,
		roomSubs:       make(map[uuid.UUID]pubsub.Subscription),
		logger:         logger,
	}
}

// SetCallHandler sets the WebRTC call handler for processing call events
func (h *Hub) SetCallHandler(ch *webrtc.CallHandler) {
	h.callHandler = ch
}

// SetSFUHandler sets the SFU handler for group calls
func (h *Hub) SetSFUHandler(sh *webrtc.SFUHandler) {
	h.sfuHandler = sh
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
	// Cleanup user subscription
	client.mu.Lock()
	if client.userSub != nil {
		client.userSub.Unsubscribe()
		client.userSub = nil
	}
	client.mu.Unlock()

	h.mu.Lock()

	userID := client.UserID()
	username := client.Username()
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

	// Track rooms for call cleanup
	roomsForCallCleanup := make([]uuid.UUID, 0, len(client.rooms))
	for roomID := range client.rooms {
		roomsForCallCleanup = append(roomsForCallCleanup, roomID)
	}

	// Remove from all rooms and track which rooms need unsubscribe
	roomsToCheck := make([]uuid.UUID, 0, len(client.rooms))
	for roomID := range client.rooms {
		if room, ok := h.rooms[roomID]; ok {
			delete(room, client)
			if len(room) == 0 {
				delete(h.rooms, roomID)
				roomsToCheck = append(roomsToCheck, roomID)
			}
		}
	}

	h.mu.Unlock()

	// Clean up call participation for this user (they might be in active calls)
	if userID != uuid.Nil && h.callHandler != nil {
		for _, roomID := range roomsForCallCleanup {
			// Attempt to leave any active calls in rooms the user was in
			ctx := context.Background()
			h.callHandler.HandleLeave(ctx, &webrtc.SignalingContext{
				UserID:   userID,
				Username: username,
			}, json.RawMessage(`{"room_id":"`+roomID.String()+`"}`))
		}
	}

	// Unsubscribe from empty rooms (outside lock)
	for _, roomID := range roomsToCheck {
		h.unsubscribeFromRoom(roomID)
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
	case EventTypeReceiptRead:
		h.handleReceiptRead(client, msg.Payload)
	// WebRTC call events
	case webrtc.EventTypeCallJoin:
		h.handleCallJoin(client, msg.Payload)
	case webrtc.EventTypeCallLeave:
		h.handleCallLeave(client, msg.Payload)
	case webrtc.EventTypeCallOffer:
		h.handleCallOffer(client, msg.Payload)
	case webrtc.EventTypeCallAnswer:
		h.handleCallAnswer(client, msg.Payload)
	case webrtc.EventTypeCallICECandidate:
		h.handleCallICECandidate(client, msg.Payload)
	case webrtc.EventTypeCallDeclined:
		h.handleCallDeclined(client, msg.Payload)
	// SFU group call events
	case webrtc.EventTypeSFUAnswer:
		h.handleSFUAnswer(client, msg.Payload)
	case webrtc.EventTypeSFUCandidate:
		h.handleSFUCandidate(client, msg.Payload)
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

	// Subscribe user to their personal event channel
	h.subscribeUserToEvents(client, claims.UserID)
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
	userID := client.UserID()
	isMember, err := h.convRepo.IsMember(ctx, convID, userID)
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

	// Ensure we're subscribed to room events via PubSub
	h.subscribeToRoom(convID)

	// Mark all undelivered messages in this conversation as delivered
	deliveredMsgIDs, err := h.convRepo.MarkConversationMessagesDelivered(ctx, convID, userID)
	if err != nil {
		h.logger.Error("failed to mark messages as delivered", "error", err)
	} else if len(deliveredMsgIDs) > 0 {
		// Broadcast batch receipt update to the room
		broadcastPayload := ReceiptBatchUpdatePayload{
			ConversationID: convID,
			MessageIDs:     deliveredMsgIDs,
			UserID:         userID,
			Status:         "delivered",
			Timestamp:      time.Now(),
		}
		h.BroadcastToRoom(convID, EventTypeReceiptUpdate, broadcastPayload)
	}

	h.logger.Debug("client joined room", "user_id", userID, "room_id", convID)
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

	if p.BodyText == "" && p.AttachmentID == "" {
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

	// Add attachment if provided
	if p.AttachmentID != "" {
		attachmentUUID, err := uuid.Parse(p.AttachmentID)
		if err != nil {
			client.sendError("invalid_attachment", "Invalid attachment ID")
			return
		}
		msg.AttachmentID = &attachmentUUID
	}

	// Save to database
	if err := h.convRepo.CreateMessage(ctx, msg); err != nil {
		h.logger.Error("failed to save message", "error", err)
		client.sendError("save_failed", "Failed to save message")
		return
	}

	// Fetch attachment details if present
	var attachmentPayload *AttachmentPayload
	if msg.AttachmentID != nil {
		attachment, err := h.attachmentRepo.GetAttachmentByID(ctx, msg.AttachmentID.String())
		if err == nil {
			attachmentID, _ := uuid.Parse(attachment.ID)
			attachmentPayload = &AttachmentPayload{
				ID:        attachmentID,
				Filename:  attachment.Filename,
				MimeType:  attachment.MimeType,
				SizeBytes: attachment.SizeBytes,
			}
		}
	}

	// Broadcast to room
	broadcastPayload := MessageNewPayload{
		ID:             msg.ID,
		ConversationID: convID,
		SenderID:       userID,
		SenderUsername: client.Username(),
		BodyText:       msg.BodyText,
		AttachmentID:   msg.AttachmentID,
		Attachment:     attachmentPayload,
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

func (h *Hub) handleReceiptRead(client *Client, payload json.RawMessage) {
	if !client.IsAuthenticated() {
		return
	}

	var p ReceiptReadPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		h.logger.Error("failed to parse receipt read payload", "error", err)
		return
	}

	messageID, err := uuid.Parse(p.MessageID)
	if err != nil {
		h.logger.Error("invalid message_id in receipt read", "error", err)
		return
	}

	ctx := context.Background()
	userID := client.UserID()

	// Get the message to find its conversation
	msg, err := h.convRepo.GetMessageByID(ctx, messageID)
	if err != nil {
		h.logger.Error("failed to get message for receipt", "error", err)
		return
	}

	// Don't mark own messages as read
	if msg.SenderID != nil && *msg.SenderID == userID {
		return
	}

	// Check if user is a member of the conversation
	isMember, err := h.convRepo.IsMember(ctx, msg.ConversationID, userID)
	if err != nil || !isMember {
		return
	}

	// Mark the message as read
	if err := h.convRepo.MarkMessageRead(ctx, messageID, userID); err != nil {
		h.logger.Error("failed to mark message as read", "error", err)
		return
	}

	// Broadcast receipt update to the room
	broadcastPayload := ReceiptUpdatePayload{
		MessageID:      messageID,
		ConversationID: msg.ConversationID,
		UserID:         userID,
		Status:         "read",
		Timestamp:      time.Now(),
	}

	h.BroadcastToRoom(msg.ConversationID, EventTypeReceiptUpdate, broadcastPayload)
}

// ============================================================================
// WebRTC Call Handlers
// ============================================================================

func (h *Hub) handleCallJoin(client *Client, payload json.RawMessage) {
	if !client.IsAuthenticated() {
		client.sendError("not_authenticated", "Must authenticate first")
		return
	}

	if h.callHandler == nil {
		client.sendError("calls_disabled", "Video calls are not enabled")
		return
	}

	sigCtx := &webrtc.SignalingContext{
		UserID:   client.UserID(),
		Username: client.Username(),
	}

	config, err := h.callHandler.HandleJoin(context.Background(), sigCtx, payload)
	if err != nil {
		if callErr, ok := err.(*webrtc.CallError); ok {
			client.sendError(callErr.Code, callErr.Message)
		} else {
			client.sendError("call_error", err.Error())
		}
		return
	}

	// Send config back to client
	msg, _ := NewMessage(webrtc.EventTypeCallConfig, config)
	client.Send(msg)
}

func (h *Hub) handleCallLeave(client *Client, payload json.RawMessage) {
	if !client.IsAuthenticated() || h.callHandler == nil {
		return
	}

	sigCtx := &webrtc.SignalingContext{
		UserID:   client.UserID(),
		Username: client.Username(),
	}

	h.callHandler.HandleLeave(context.Background(), sigCtx, payload)
}

func (h *Hub) handleCallOffer(client *Client, payload json.RawMessage) {
	if !client.IsAuthenticated() {
		client.sendError("not_authenticated", "Must authenticate first")
		return
	}

	if h.callHandler == nil {
		client.sendError("calls_disabled", "Video calls are not enabled")
		return
	}

	sigCtx := &webrtc.SignalingContext{
		UserID:   client.UserID(),
		Username: client.Username(),
	}

	if err := h.callHandler.HandleOffer(context.Background(), sigCtx, payload); err != nil {
		if callErr, ok := err.(*webrtc.CallError); ok {
			client.sendError(callErr.Code, callErr.Message)
		}
	}
}

func (h *Hub) handleCallAnswer(client *Client, payload json.RawMessage) {
	if !client.IsAuthenticated() {
		client.sendError("not_authenticated", "Must authenticate first")
		return
	}

	if h.callHandler == nil {
		client.sendError("calls_disabled", "Video calls are not enabled")
		return
	}

	sigCtx := &webrtc.SignalingContext{
		UserID:   client.UserID(),
		Username: client.Username(),
	}

	if err := h.callHandler.HandleAnswer(context.Background(), sigCtx, payload); err != nil {
		if callErr, ok := err.(*webrtc.CallError); ok {
			client.sendError(callErr.Code, callErr.Message)
		}
	}
}

func (h *Hub) handleCallICECandidate(client *Client, payload json.RawMessage) {
	if !client.IsAuthenticated() || h.callHandler == nil {
		return
	}

	sigCtx := &webrtc.SignalingContext{
		UserID:   client.UserID(),
		Username: client.Username(),
	}

	h.callHandler.HandleICECandidate(context.Background(), sigCtx, payload)
}

func (h *Hub) handleCallDeclined(client *Client, payload json.RawMessage) {
	if !client.IsAuthenticated() || h.callHandler == nil {
		return
	}

	sigCtx := &webrtc.SignalingContext{
		UserID:   client.UserID(),
		Username: client.Username(),
	}

	h.callHandler.HandleDeclined(context.Background(), sigCtx, payload)
}

func (h *Hub) handleSFUAnswer(client *Client, payload json.RawMessage) {
	if !client.IsAuthenticated() {
		client.sendError("not_authenticated", "Must authenticate first")
		return
	}

	if h.sfuHandler == nil {
		client.sendError("sfu_disabled", "SFU group calls are not enabled")
		return
	}

	sigCtx := &webrtc.SignalingContext{
		UserID:   client.UserID(),
		Username: client.Username(),
	}

	if err := h.sfuHandler.HandleSFUAnswer(context.Background(), sigCtx, payload); err != nil {
		if callErr, ok := err.(*webrtc.CallError); ok {
			client.sendError(callErr.Code, callErr.Message)
		}
	}
}

func (h *Hub) handleSFUCandidate(client *Client, payload json.RawMessage) {
	if !client.IsAuthenticated() || h.sfuHandler == nil {
		return
	}

	sigCtx := &webrtc.SignalingContext{
		UserID:   client.UserID(),
		Username: client.Username(),
	}

	h.sfuHandler.HandleSFUCandidate(context.Background(), sigCtx, payload)
}

// BroadcastToRoom sends a message to all clients in a room via PubSub
func (h *Hub) BroadcastToRoom(roomID uuid.UUID, eventType string, payload interface{}) {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		h.logger.Error("failed to marshal broadcast payload", "error", err)
		return
	}

	msg := &pubsub.Message{
		Topic:   pubsub.Topics.Room(roomID.String()),
		Type:    eventType,
		Payload: payloadBytes,
	}

	if err := h.pubsub.Publish(context.Background(), msg.Topic, msg); err != nil {
		h.logger.Error("failed to publish to room", "room_id", roomID, "error", err)
	}
}

// BroadcastToRoomExcept sends to all room members except one (for typing indicators etc)
func (h *Hub) BroadcastToRoomExcept(roomID uuid.UUID, except *Client, eventType string, payload interface{}) {
	// For local broadcasts with exceptions, we use direct delivery
	// since PubSub doesn't support exception-based filtering
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
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		h.logger.Error("failed to marshal user broadcast payload", "error", err)
		return
	}

	msg := &pubsub.Message{
		Topic:   pubsub.Topics.User(userID.String()),
		Type:    eventType,
		Payload: payloadBytes,
	}

	if err := h.pubsub.Publish(context.Background(), msg.Topic, msg); err != nil {
		h.logger.Error("failed to publish to user", "user_id", userID, "error", err)
	}
}

// subscribeToRoom creates a PubSub subscription for a room if one doesn't exist
func (h *Hub) subscribeToRoom(roomID uuid.UUID) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if _, exists := h.roomSubs[roomID]; exists {
		return // Already subscribed
	}

	topic := pubsub.Topics.Room(roomID.String())
	sub, err := h.pubsub.Subscribe(context.Background(), topic, func(ctx context.Context, msg *pubsub.Message) {
		h.deliverToRoom(roomID, msg)
	})
	if err != nil {
		h.logger.Error("failed to subscribe to room", "room_id", roomID, "error", err)
		return
	}

	h.roomSubs[roomID] = sub
}

// unsubscribeFromRoom removes PubSub subscription when no local clients remain
func (h *Hub) unsubscribeFromRoom(roomID uuid.UUID) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Only unsubscribe if no local clients remain in the room
	if room, ok := h.rooms[roomID]; ok && len(room) > 0 {
		return
	}

	if sub, ok := h.roomSubs[roomID]; ok {
		sub.Unsubscribe()
		delete(h.roomSubs, roomID)
	}
}

// deliverToRoom delivers a PubSub message to all local clients in a room
func (h *Hub) deliverToRoom(roomID uuid.UUID, psMsg *pubsub.Message) {
	h.mu.RLock()
	room, ok := h.rooms[roomID]
	if !ok {
		h.mu.RUnlock()
		return
	}

	clients := make([]*Client, 0, len(room))
	for client := range room {
		clients = append(clients, client)
	}
	h.mu.RUnlock()

	msg := &Message{
		Type:      psMsg.Type,
		Payload:   psMsg.Payload,
		Timestamp: time.Now(),
	}

	for _, client := range clients {
		client.Send(msg)
	}
}

// subscribeUserToEvents creates PubSub subscription for user-specific events
func (h *Hub) subscribeUserToEvents(client *Client, userID uuid.UUID) {
	topic := pubsub.Topics.User(userID.String())
	h.logger.Info("subscribing user to events", "user_id", userID, "topic", topic)

	sub, err := h.pubsub.Subscribe(context.Background(), topic, func(ctx context.Context, msg *pubsub.Message) {
		h.logger.Info("received pubsub message for user", "user_id", userID, "type", msg.Type, "topic", msg.Topic)
		wsMsg := &Message{
			Type:      msg.Type,
			Payload:   msg.Payload,
			Timestamp: time.Now(),
		}
		client.Send(wsMsg)
		h.logger.Info("sent message to client", "user_id", userID, "type", msg.Type)
	})
	if err != nil {
		h.logger.Error("failed to subscribe user to events", "user_id", userID, "error", err)
		return
	}

	h.logger.Info("successfully subscribed user to events", "user_id", userID, "topic", topic)

	// Store subscription on client for cleanup (we'll add this tracking)
	client.mu.Lock()
	client.userSub = sub
	client.mu.Unlock()
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
