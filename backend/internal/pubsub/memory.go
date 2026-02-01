package pubsub

import (
	"context"
	"log/slog"
	"sync"
)

// memorySubscription is a subscription to a topic
type memorySubscription struct {
	ps      *MemoryPubSub
	topic   string
	handler Handler
	id      uint64
}

func (s *memorySubscription) Unsubscribe() error {
	s.ps.unsubscribe(s.topic, s.id)
	return nil
}

// MemoryPubSub implements PubSub using an in-memory map.
// Suitable for single-instance deployments.
type MemoryPubSub struct {
	mu          sync.RWMutex
	subscribers map[string]map[uint64]*memorySubscription
	nextID      uint64
	closed      bool
	logger      *slog.Logger
}

// NewMemoryPubSub creates a new in-memory pub/sub instance
func NewMemoryPubSub() *MemoryPubSub {
	return &MemoryPubSub{
		subscribers: make(map[string]map[uint64]*memorySubscription),
		logger:      slog.Default().With("component", "pubsub"),
	}
}

// Publish sends a message to all subscribers of the topic
func (ps *MemoryPubSub) Publish(ctx context.Context, topic string, msg *Message) error {
	ps.mu.RLock()
	if ps.closed {
		ps.mu.RUnlock()
		return ErrClosed
	}

	subs, ok := ps.subscribers[topic]
	if !ok || len(subs) == 0 {
		ps.mu.RUnlock()
		ps.logger.Warn("no subscribers for topic", "topic", topic, "msg_type", msg.Type, "all_topics", ps.listTopics())
		return nil
	}

	ps.logger.Info("publishing to topic", "topic", topic, "msg_type", msg.Type, "subscriber_count", len(subs))

	// Copy handlers to avoid holding lock during callback
	handlers := make([]Handler, 0, len(subs))
	for _, sub := range subs {
		handlers = append(handlers, sub.handler)
	}
	ps.mu.RUnlock()

	// Call handlers outside lock
	for _, h := range handlers {
		// Call in goroutine to avoid blocking publisher
		go h(ctx, msg)
	}

	return nil
}

// listTopics returns all topics with subscribers (for debugging)
func (ps *MemoryPubSub) listTopics() []string {
	topics := make([]string, 0, len(ps.subscribers))
	for t := range ps.subscribers {
		topics = append(topics, t)
	}
	return topics
}

// Subscribe registers a handler for the given topic
func (ps *MemoryPubSub) Subscribe(ctx context.Context, topic string, handler Handler) (Subscription, error) {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	if ps.closed {
		return nil, ErrClosed
	}

	ps.nextID++
	id := ps.nextID

	sub := &memorySubscription{
		ps:      ps,
		topic:   topic,
		handler: handler,
		id:      id,
	}

	if ps.subscribers[topic] == nil {
		ps.subscribers[topic] = make(map[uint64]*memorySubscription)
	}
	ps.subscribers[topic][id] = sub

	return sub, nil
}

func (ps *MemoryPubSub) unsubscribe(topic string, id uint64) {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	if subs, ok := ps.subscribers[topic]; ok {
		delete(subs, id)
		if len(subs) == 0 {
			delete(ps.subscribers, topic)
		}
	}
}

// Close shuts down the pub/sub and prevents new operations
func (ps *MemoryPubSub) Close() error {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	ps.closed = true
	ps.subscribers = make(map[string]map[uint64]*memorySubscription)
	return nil
}

// SubscriberCount returns the number of subscribers for a topic (useful for testing)
func (ps *MemoryPubSub) SubscriberCount(topic string) int {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	return len(ps.subscribers[topic])
}

// TopicCount returns the number of active topics (useful for testing)
func (ps *MemoryPubSub) TopicCount() int {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	return len(ps.subscribers)
}
