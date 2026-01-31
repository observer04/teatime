package websocket

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Event types for client -> server
const (
	EventTypeAuth        = "auth"
	EventTypeRoomJoin    = "room.join"
	EventTypeRoomLeave   = "room.leave"
	EventTypeMessageSend = "message.send"
	EventTypeTypingStart = "typing.start"
	EventTypeTypingStop  = "typing.stop"
	EventTypeReceiptRead = "receipt.read"
)

// Event types for server -> client
const (
	EventTypeError         = "error"
	EventTypeAuthSuccess   = "auth.success"
	EventTypeMessageNew    = "message.new"
	EventTypeTyping        = "typing"
	EventTypeReceiptUpdate = "receipt.updated"
	EventTypeMemberJoined  = "room.member_joined"
	EventTypeMemberLeft    = "room.member_left"
	EventTypePresence      = "presence"
)

// Message is the base WebSocket message envelope
type Message struct {
	Type      string          `json:"type"`
	Payload   json.RawMessage `json:"payload,omitempty"`
	Timestamp time.Time       `json:"timestamp,omitempty"`
}

// NewMessage creates a message with the current timestamp
func NewMessage(eventType string, payload interface{}) (*Message, error) {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return &Message{
		Type:      eventType,
		Payload:   payloadBytes,
		Timestamp: time.Now(),
	}, nil
}

// ============================================================================
// Client -> Server Payloads
// ============================================================================

// AuthPayload for authenticating the WebSocket connection
type AuthPayload struct {
	Token string `json:"token"` // JWT access token
}

// RoomJoinPayload for joining a conversation room
type RoomJoinPayload struct {
	ConversationID string `json:"conversation_id"`
}

// RoomLeavePayload for leaving a conversation room
type RoomLeavePayload struct {
	ConversationID string `json:"conversation_id"`
}

// MessageSendPayload for sending a message via WebSocket
type MessageSendPayload struct {
	ConversationID string `json:"conversation_id"`
	BodyText       string `json:"body_text"`
	AttachmentID   string `json:"attachment_id,omitempty"`
	TempID         string `json:"temp_id,omitempty"` // Client-side temp ID for optimistic UI
}

// TypingPayload for typing indicators
type TypingPayload struct {
	ConversationID string `json:"conversation_id"`
}

// ReceiptReadPayload for marking messages as read
type ReceiptReadPayload struct {
	MessageID string `json:"message_id"`
}

// ============================================================================
// Server -> Client Payloads
// ============================================================================

// ErrorPayload for error responses
type ErrorPayload struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// AuthSuccessPayload confirms successful authentication
type AuthSuccessPayload struct {
	UserID   uuid.UUID `json:"user_id"`
	Username string    `json:"username"`
}

// MessageNewPayload broadcasts a new message to room members
type MessageNewPayload struct {
	ID             uuid.UUID          `json:"id"`
	ConversationID uuid.UUID          `json:"conversation_id"`
	SenderID       uuid.UUID          `json:"sender_id"`
	SenderUsername string             `json:"sender_username"`
	BodyText       string             `json:"body_text"`
	AttachmentID   *uuid.UUID         `json:"attachment_id,omitempty"`
	Attachment     *AttachmentPayload `json:"attachment,omitempty"`
	CreatedAt      time.Time          `json:"created_at"`
	TempID         string             `json:"temp_id,omitempty"` // Echo back for sender
}

// AttachmentPayload contains attachment details
type AttachmentPayload struct {
	ID        uuid.UUID `json:"id"`
	Filename  string    `json:"filename"`
	MimeType  string    `json:"mime_type"`
	SizeBytes int64     `json:"size_bytes"`
}

// TypingBroadcastPayload broadcasts typing status
type TypingBroadcastPayload struct {
	ConversationID uuid.UUID `json:"conversation_id"`
	UserID         uuid.UUID `json:"user_id"`
	Username       string    `json:"username"`
	IsTyping       bool      `json:"is_typing"`
}

// PresencePayload for online/offline status
type PresencePayload struct {
	UserID   uuid.UUID `json:"user_id"`
	Username string    `json:"username"`
	Online   bool      `json:"online"`
}
