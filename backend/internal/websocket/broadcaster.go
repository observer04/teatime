package websocket

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/observer/teatime/internal/pubsub"
)

// RoomBroadcaster provides a way for API handlers to broadcast events to room members.
// This interface decouples the API layer from the WebSocket implementation.
type RoomBroadcaster interface {
	// BroadcastMemberJoined notifies room members that a new member was added
	BroadcastMemberJoined(ctx context.Context, convID, userID uuid.UUID, username, role string, addedBy uuid.UUID) error

	// BroadcastMemberLeft notifies room members that a member left or was removed
	BroadcastMemberLeft(ctx context.Context, convID, userID uuid.UUID, username string, removedBy uuid.UUID) error

	// BroadcastRoomUpdated notifies room members that the conversation was updated
	BroadcastRoomUpdated(ctx context.Context, convID uuid.UUID, title string, updatedBy uuid.UUID) error

	// BroadcastMessageDeleted notifies room members that a message was deleted
	BroadcastMessageDeleted(ctx context.Context, messageID, convID, deletedBy uuid.UUID) error
}

// PubSubBroadcaster implements RoomBroadcaster using the PubSub system
type PubSubBroadcaster struct {
	ps pubsub.PubSub
}

// NewPubSubBroadcaster creates a new broadcaster that uses the PubSub system
func NewPubSubBroadcaster(ps pubsub.PubSub) *PubSubBroadcaster {
	return &PubSubBroadcaster{ps: ps}
}

func (b *PubSubBroadcaster) BroadcastMemberJoined(ctx context.Context, convID, userID uuid.UUID, username, role string, addedBy uuid.UUID) error {
	payload := MemberJoinedPayload{
		ConversationID: convID,
		UserID:         userID,
		Username:       username,
		Role:           role,
		AddedBy:        addedBy,
	}
	return b.broadcast(ctx, convID, EventTypeMemberJoined, payload)
}

func (b *PubSubBroadcaster) BroadcastMemberLeft(ctx context.Context, convID, userID uuid.UUID, username string, removedBy uuid.UUID) error {
	payload := MemberLeftPayload{
		ConversationID: convID,
		UserID:         userID,
		Username:       username,
		RemovedBy:      removedBy,
	}
	return b.broadcast(ctx, convID, EventTypeMemberLeft, payload)
}

func (b *PubSubBroadcaster) BroadcastRoomUpdated(ctx context.Context, convID uuid.UUID, title string, updatedBy uuid.UUID) error {
	payload := RoomUpdatedPayload{
		ConversationID: convID,
		Title:          title,
		UpdatedBy:      updatedBy,
	}
	return b.broadcast(ctx, convID, EventTypeRoomUpdated, payload)
}

func (b *PubSubBroadcaster) BroadcastMessageDeleted(ctx context.Context, messageID, convID, deletedBy uuid.UUID) error {
	payload := MessageDeletedPayload{
		MessageID:      messageID,
		ConversationID: convID,
		DeletedBy:      deletedBy,
	}
	return b.broadcast(ctx, convID, EventTypeMessageDeleted, payload)
}

func (b *PubSubBroadcaster) broadcast(ctx context.Context, convID uuid.UUID, eventType string, payload interface{}) error {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	msg := &pubsub.Message{
		Topic:   pubsub.Topics.Room(convID.String()),
		Type:    eventType,
		Payload: payloadBytes,
	}

	return b.ps.Publish(ctx, msg.Topic, msg)
}
