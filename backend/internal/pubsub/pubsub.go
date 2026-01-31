// Package pubsub provides an interface-driven pub/sub system for realtime messaging.
// The MVP uses an in-memory implementation, but the interface allows for Redis/NATS backends.
package pubsub

import (
	"context"
	"encoding/json"
)

// Message represents a pub/sub message with typed payload
type Message struct {
	Topic   string          `json:"topic"`
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

// Handler is a callback for processing messages
type Handler func(ctx context.Context, msg *Message)

// Subscription represents an active subscription that can be closed
type Subscription interface {
	// Unsubscribe removes the subscription
	Unsubscribe() error
}

// PubSub defines the interface for publish/subscribe operations.
// All implementations must be safe for concurrent use.
type PubSub interface {
	// Publish sends a message to all subscribers of the given topic.
	// Returns error if the message could not be published.
	Publish(ctx context.Context, topic string, msg *Message) error

	// Subscribe registers a handler for messages on the given topic.
	// The handler is called for each message published to the topic.
	// Returns a Subscription that can be used to unsubscribe.
	Subscribe(ctx context.Context, topic string, handler Handler) (Subscription, error)

	// Close shuts down the pub/sub system and releases resources.
	Close() error
}

// TopicBuilder helps construct consistent topic names
type TopicBuilder struct{}

// Room returns the topic for a conversation/room
func (t TopicBuilder) Room(roomID string) string {
	return "room:" + roomID
}

// User returns the topic for user-specific events
func (t TopicBuilder) User(userID string) string {
	return "user:" + userID
}

// Presence returns the topic for presence updates
func (t TopicBuilder) Presence() string {
	return "presence"
}

// Call returns the topic for a call room
func (t TopicBuilder) Call(roomID string) string {
	return "call:" + roomID
}

// Topics is a helper for building topic names
var Topics = TopicBuilder{}
