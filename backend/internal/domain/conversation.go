package domain

import (
	"time"

	"github.com/google/uuid"
)

type ConversationType string

const (
	ConversationTypeDM    ConversationType = "dm"
	ConversationTypeGroup ConversationType = "group"
)

type MemberRole string

const (
	MemberRoleMember MemberRole = "member"
	MemberRoleAdmin  MemberRole = "admin"
)

// Conversation represents a chat (DM or group)
type Conversation struct {
	ID         uuid.UUID        `json:"id"`
	Type       ConversationType `json:"type"`
	Title      string           `json:"title,omitempty"` // only for groups
	CreatedBy  *uuid.UUID       `json:"created_by,omitempty"`
	CreatedAt  time.Time        `json:"created_at"`
	UpdatedAt  time.Time        `json:"updated_at"`
	ArchivedAt *time.Time       `json:"archived_at,omitempty"`

	// Populated on fetch
	Members     []ConversationMember `json:"members,omitempty"`
	UnreadCount int                  `json:"unread_count,omitempty"`
	LastMessage *Message             `json:"last_message,omitempty"`
	OtherUser   *PublicUser          `json:"other_user,omitempty"` // For DMs
	MemberCount int                  `json:"member_count,omitempty"`
}

// ConversationMember represents a user's membership in a conversation
type ConversationMember struct {
	ConversationID uuid.UUID  `json:"conversation_id"`
	UserID         uuid.UUID  `json:"user_id"`
	Role           MemberRole `json:"role"`
	JoinedAt       time.Time  `json:"joined_at"`

	// Populated on fetch
	User *PublicUser `json:"user,omitempty"`
}

// Message represents a chat message
type Message struct {
	ID             uuid.UUID  `json:"id"`
	ConversationID uuid.UUID  `json:"conversation_id"`
	SenderID       *uuid.UUID `json:"sender_id,omitempty"` // nil if sender deleted
	BodyText       string     `json:"body_text"`
	AttachmentID   *uuid.UUID `json:"attachment_id,omitempty"` // Link to attachment
	CreatedAt      time.Time  `json:"created_at"`

	// Populated on fetch
	Sender        *PublicUser `json:"sender,omitempty"`
	Attachment    *Attachment `json:"attachment,omitempty"`
	ReceiptStatus string      `json:"receipt_status,omitempty"` // "sent", "delivered", "read"
}

// MessageReceipt tracks delivered/read status per user
type MessageReceipt struct {
	MessageID   uuid.UUID  `json:"message_id"`
	UserID      uuid.UUID  `json:"user_id"`
	DeliveredAt *time.Time `json:"delivered_at,omitempty"`
	ReadAt      *time.Time `json:"read_at,omitempty"`
}

// StarredMessage represents a message starred by a user
type StarredMessage struct {
	UserID    uuid.UUID `json:"user_id"`
	MessageID uuid.UUID `json:"message_id"`
	StarredAt time.Time `json:"starred_at"`
	Message   *Message  `json:"message,omitempty"` // Populated on fetch
}

// MessageSearchResult represents a search result with context
type MessageSearchResult struct {
	Message          *Message  `json:"message"`
	ConversationID   uuid.UUID `json:"conversation_id"`
	ConversationType string    `json:"conversation_type"`
	ConversationName string    `json:"conversation_name"`
	Highlight        string    `json:"highlight,omitempty"` // Highlighted text snippet
	Rank             float64   `json:"rank"`
}

// ReadStatus tracks when a user last read a conversation
type ReadStatus struct {
	ConversationID    uuid.UUID  `json:"conversation_id"`
	UserID            uuid.UUID  `json:"user_id"`
	LastReadAt        time.Time  `json:"last_read_at"`
	LastReadMessageID *uuid.UUID `json:"last_read_message_id,omitempty"`
}
