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
	ID        uuid.UUID        `json:"id"`
	Type      ConversationType `json:"type"`
	Title     string           `json:"title,omitempty"` // only for groups
	CreatedBy *uuid.UUID       `json:"created_by,omitempty"`
	CreatedAt time.Time        `json:"created_at"`
	UpdatedAt time.Time        `json:"updated_at"`

	// Populated on fetch
	Members []ConversationMember `json:"members,omitempty"`
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
	CreatedAt      time.Time  `json:"created_at"`

	// Populated on fetch
	Sender *PublicUser `json:"sender,omitempty"`
}

// MessageReceipt tracks delivered/read status per user
type MessageReceipt struct {
	MessageID   uuid.UUID  `json:"message_id"`
	UserID      uuid.UUID  `json:"user_id"`
	DeliveredAt *time.Time `json:"delivered_at,omitempty"`
	ReadAt      *time.Time `json:"read_at,omitempty"`
}
