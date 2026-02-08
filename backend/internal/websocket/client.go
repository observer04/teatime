package websocket

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/observer/teatime/internal/pubsub"
)

const (
	// Time allowed to write a message to the peer
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer
	pongWait = 60 * time.Second

	// Send pings to peer with this period (must be less than pongWait)
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer (64KB for attachment metadata)
	maxMessageSize = 65536
)

// Client represents a connected WebSocket client
type Client struct {
	hub      *Hub
	conn     *websocket.Conn
	send     chan []byte
	userID   uuid.UUID
	username string
	rooms    map[uuid.UUID]bool  // conversation IDs this client is subscribed to
	userSub  pubsub.Subscription // subscription for user-specific events
	mu       sync.RWMutex
	logger   *slog.Logger
	cancel   context.CancelFunc
}

// NewClient creates a new client
func NewClient(hub *Hub, conn *websocket.Conn, logger *slog.Logger) *Client {
	return &Client{
		hub:    hub,
		conn:   conn,
		send:   make(chan []byte, 256),
		rooms:  make(map[uuid.UUID]bool),
		logger: logger,
	}
}

// SetCancelFunc sets the context cancel function for cleanup
func (c *Client) SetCancelFunc(cancel context.CancelFunc) {
	c.cancel = cancel
}

// SetUser sets the authenticated user info
func (c *Client) SetUser(userID uuid.UUID, username string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.userID = userID
	c.username = username
}

// UserID returns the client's user ID
func (c *Client) UserID() uuid.UUID {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.userID
}

// Username returns the client's username
func (c *Client) Username() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.username
}

// IsAuthenticated returns true if the client has authenticated
func (c *Client) IsAuthenticated() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.userID != uuid.Nil
}

// JoinRoom subscribes the client to a room
func (c *Client) JoinRoom(roomID uuid.UUID) {
	c.mu.Lock()
	c.rooms[roomID] = true
	c.mu.Unlock()
}

// LeaveRoom unsubscribes the client from a room
func (c *Client) LeaveRoom(roomID uuid.UUID) {
	c.mu.Lock()
	delete(c.rooms, roomID)
	c.mu.Unlock()
}

// IsInRoom checks if client is subscribed to a room
func (c *Client) IsInRoom(roomID uuid.UUID) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.rooms[roomID]
}

// GetRooms returns all rooms the client is in
func (c *Client) GetRooms() []uuid.UUID {
	c.mu.RLock()
	defer c.mu.RUnlock()
	rooms := make([]uuid.UUID, 0, len(c.rooms))
	for id := range c.rooms {
		rooms = append(rooms, id)
	}
	return rooms
}

// ReadPump pumps messages from the WebSocket connection to the hub
func (c *Client) ReadPump(ctx context.Context) {
	defer func() {
		c.hub.Unregister(c)
		_ = c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMessageSize)
	_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		select {
		case <-ctx.Done():
			return
		default:
			_, message, err := c.conn.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					c.logger.Warn("websocket read error", "error", err, "user_id", c.userID)
				}
				return
			}

			// Parse message
			var msg Message
			if err := json.Unmarshal(message, &msg); err != nil {
				c.sendError("invalid_message", "Failed to parse message")
				continue
			}

			// Handle message
			c.hub.HandleMessage(c, &msg)
		}
	}
}

// WritePump pumps messages from the hub to the WebSocket connection
func (c *Client) WritePump(ctx context.Context) {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		_ = c.conn.Close()
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case message, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// Hub closed the channel
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			_, _ = w.Write(message)

			// Add queued messages to the current websocket message
			n := len(c.send)
			for i := 0; i < n; i++ {
				_, _ = w.Write([]byte{'\n'})
				_, _ = w.Write(<-c.send)
			}

			if err := w.Close(); err != nil {
				return
			}
		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// Send sends a message to the client
func (c *Client) Send(msg *Message) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	select {
	case c.send <- data:
	default:
		// Buffer full, drop message
		c.logger.Warn("client send buffer full, dropping message", "user_id", c.userID)
	}
	return nil
}

// sendError sends an error message to the client
func (c *Client) sendError(code, message string) {
	msg, _ := NewMessage(EventTypeError, ErrorPayload{
		Code:    code,
		Message: message,
	})
	_ = c.Send(msg)
}
