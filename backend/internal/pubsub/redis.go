package pubsub

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"

	"github.com/redis/go-redis/v9"
)

// RedisPubSub implements PubSub using Redis pub/sub for horizontal scaling.
// Messages published on one instance are received by subscribers on all instances.
type RedisPubSub struct {
	client        *redis.Client
	mu            sync.RWMutex
	subscriptions map[uint64]*redisSubscription
	nextID        atomic.Uint64
	closed        bool
	logger        *slog.Logger
}

// redisSubscription manages a single subscription to a Redis channel
type redisSubscription struct {
	ps      *RedisPubSub
	id      uint64
	topic   string
	pubsub  *redis.PubSub
	cancel  context.CancelFunc
	handler Handler
}

func (s *redisSubscription) Unsubscribe() error {
	s.cancel()
	if s.pubsub != nil {
		s.pubsub.Close()
	}
	s.ps.removeSub(s.id)
	return nil
}

// NewRedisPubSub creates a new Redis-backed pub/sub instance.
// url should be in the format: redis://host:port or redis://:password@host:port
func NewRedisPubSub(url string) (*RedisPubSub, error) {
	opts, err := redis.ParseURL(url)
	if err != nil {
		return nil, fmt.Errorf("invalid redis URL: %w", err)
	}

	client := redis.NewClient(opts)

	// Test connection
	ctx := context.Background()
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to redis: %w", err)
	}

	logger := slog.Default().With("component", "pubsub", "backend", "redis")
	logger.Info("connected to Redis", "addr", opts.Addr)

	return &RedisPubSub{
		client:        client,
		subscriptions: make(map[uint64]*redisSubscription),
		logger:        logger,
	}, nil
}

// Publish sends a message to all subscribers of the topic across all instances.
func (ps *RedisPubSub) Publish(ctx context.Context, topic string, msg *Message) error {
	ps.mu.RLock()
	if ps.closed {
		ps.mu.RUnlock()
		return ErrClosed
	}
	ps.mu.RUnlock()

	// Serialize message to JSON
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	// Publish to Redis channel
	result := ps.client.Publish(ctx, topic, data)
	if err := result.Err(); err != nil {
		return fmt.Errorf("failed to publish to redis: %w", err)
	}

	subscribers := result.Val()
	if subscribers == 0 {
		ps.logger.Warn("no subscribers for topic", "topic", topic, "msg_type", msg.Type)
	} else {
		ps.logger.Debug("published to topic", "topic", topic, "msg_type", msg.Type, "subscribers", subscribers)
	}

	return nil
}

// Subscribe registers a handler for messages on the given topic.
// The subscription spans all Redis instances - messages from any publisher will be received.
func (ps *RedisPubSub) Subscribe(ctx context.Context, topic string, handler Handler) (Subscription, error) {
	ps.mu.Lock()
	if ps.closed {
		ps.mu.Unlock()
		return nil, ErrClosed
	}

	// Create Redis pubsub subscription
	redisPubSub := ps.client.Subscribe(ctx, topic)

	// Wait for subscription to be ready
	_, err := redisPubSub.Receive(ctx)
	if err != nil {
		ps.mu.Unlock()
		redisPubSub.Close()
		return nil, fmt.Errorf("failed to subscribe to redis channel: %w", err)
	}

	// Create subscription context
	subCtx, cancel := context.WithCancel(context.Background())

	id := ps.nextID.Add(1)
	sub := &redisSubscription{
		ps:      ps,
		id:      id,
		topic:   topic,
		pubsub:  redisPubSub,
		cancel:  cancel,
		handler: handler,
	}

	ps.subscriptions[id] = sub
	ps.mu.Unlock()

	// Start goroutine to receive messages
	go ps.receiveMessages(subCtx, sub)

	ps.logger.Debug("subscribed to topic", "topic", topic, "sub_id", id)

	return sub, nil
}

// receiveMessages listens for messages on the Redis channel and dispatches to handler
func (ps *RedisPubSub) receiveMessages(ctx context.Context, sub *redisSubscription) {
	ch := sub.pubsub.Channel()

	for {
		select {
		case <-ctx.Done():
			return
		case redisMsg, ok := <-ch:
			if !ok {
				return
			}

			// Deserialize message
			var msg Message
			if err := json.Unmarshal([]byte(redisMsg.Payload), &msg); err != nil {
				ps.logger.Error("failed to unmarshal message", "error", err, "topic", sub.topic)
				continue
			}

			// Call handler in goroutine to avoid blocking
			go sub.handler(ctx, &msg)
		}
	}
}

func (ps *RedisPubSub) removeSub(id uint64) {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	delete(ps.subscriptions, id)
}

// Close shuts down the pub/sub and all subscriptions
func (ps *RedisPubSub) Close() error {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	if ps.closed {
		return nil
	}

	ps.closed = true

	// Close all subscriptions
	for _, sub := range ps.subscriptions {
		sub.cancel()
		if sub.pubsub != nil {
			sub.pubsub.Close()
		}
	}
	ps.subscriptions = make(map[uint64]*redisSubscription)

	// Close Redis client
	if err := ps.client.Close(); err != nil {
		return fmt.Errorf("failed to close redis client: %w", err)
	}

	ps.logger.Info("Redis pubsub closed")
	return nil
}

// SubscriberCount returns the number of local subscribers for a topic.
// Note: This only counts subscribers on this instance, not across the cluster.
func (ps *RedisPubSub) SubscriberCount(topic string) int {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	count := 0
	for _, sub := range ps.subscriptions {
		if sub.topic == topic {
			count++
		}
	}
	return count
}
