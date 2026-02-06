package pubsub

import (
	"context"
	"encoding/json"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestMemoryPubSub_PublishSubscribe(t *testing.T) {
	ps := NewMemoryPubSub()
	defer ps.Close()

	topic := "test-topic"
	received := make(chan *Message, 1)

	// Subscribe
	sub, err := ps.Subscribe(context.Background(), topic, func(ctx context.Context, msg *Message) {
		received <- msg
	})
	if err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}
	defer sub.Unsubscribe()

	// Publish
	payload, _ := json.Marshal(map[string]string{"test": "data"})
	msg := &Message{
		Topic:   topic,
		Type:    "test.event",
		Payload: payload,
	}

	err = ps.Publish(context.Background(), topic, msg)
	if err != nil {
		t.Fatalf("Publish failed: %v", err)
	}

	// Wait for message
	select {
	case got := <-received:
		if got.Type != msg.Type {
			t.Errorf("got type %q, want %q", got.Type, msg.Type)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for message")
	}
}

func TestMemoryPubSub_MultipleSubscribers(t *testing.T) {
	ps := NewMemoryPubSub()
	defer ps.Close()

	topic := "multi-sub"
	var count atomic.Int32
	var wg sync.WaitGroup

	// Create 3 subscribers
	for i := 0; i < 3; i++ {
		wg.Add(1)
		sub, err := ps.Subscribe(context.Background(), topic, func(ctx context.Context, msg *Message) {
			count.Add(1)
			wg.Done()
		})
		if err != nil {
			t.Fatalf("Subscribe %d failed: %v", i, err)
		}
		defer sub.Unsubscribe()
	}

	// Publish one message
	msg := &Message{Topic: topic, Type: "test"}
	ps.Publish(context.Background(), topic, msg)

	// Wait with timeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		if count.Load() != 3 {
			t.Errorf("got %d deliveries, want 3", count.Load())
		}
	case <-time.After(time.Second):
		t.Fatalf("timeout: only got %d deliveries", count.Load())
	}
}

func TestMemoryPubSub_Unsubscribe(t *testing.T) {
	ps := NewMemoryPubSub()
	defer ps.Close()

	topic := "unsub-test"
	received := make(chan struct{}, 10)

	sub, _ := ps.Subscribe(context.Background(), topic, func(ctx context.Context, msg *Message) {
		received <- struct{}{}
	})

	// First publish should deliver
	ps.Publish(context.Background(), topic, &Message{Topic: topic, Type: "test"})
	select {
	case <-received:
		// ok
	case <-time.After(time.Second):
		t.Fatal("first message not received")
	}

	// Unsubscribe
	sub.Unsubscribe()

	// Give goroutines time to complete
	time.Sleep(50 * time.Millisecond)

	// Second publish should not deliver
	ps.Publish(context.Background(), topic, &Message{Topic: topic, Type: "test"})

	select {
	case <-received:
		t.Error("received message after unsubscribe")
	case <-time.After(100 * time.Millisecond):
		// ok - no message received
	}
}

func TestMemoryPubSub_Close(t *testing.T) {
	ps := NewMemoryPubSub()

	topic := "close-test"
	ps.Subscribe(context.Background(), topic, func(ctx context.Context, msg *Message) {})

	if ps.TopicCount() != 1 {
		t.Errorf("expected 1 topic, got %d", ps.TopicCount())
	}

	ps.Close()

	if ps.TopicCount() != 0 {
		t.Errorf("expected 0 topics after close, got %d", ps.TopicCount())
	}

	// Operations should fail after close
	err := ps.Publish(context.Background(), topic, &Message{})
	if err != ErrClosed {
		t.Errorf("expected ErrClosed, got %v", err)
	}

	_, err = ps.Subscribe(context.Background(), topic, func(ctx context.Context, msg *Message) {})
	if err != ErrClosed {
		t.Errorf("expected ErrClosed, got %v", err)
	}
}

func TestMemoryPubSub_NoSubscribers(t *testing.T) {
	ps := NewMemoryPubSub()
	defer ps.Close()

	// Publishing to topic with no subscribers should not error
	err := ps.Publish(context.Background(), "empty-topic", &Message{Type: "test"})
	if err != nil {
		t.Errorf("publish to empty topic failed: %v", err)
	}
}

func TestTopicBuilder(t *testing.T) {
	tests := []struct {
		name   string
		method func() string
		want   string
	}{
		{"Room", func() string { return Topics.Room("123") }, "room:123"},
		{"User", func() string { return Topics.User("456") }, "user:456"},
		{"Call", func() string { return Topics.Call("789") }, "call:789"},
		{"Presence", Topics.Presence, "presence"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.method()
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}
