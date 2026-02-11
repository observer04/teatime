package websocket

import (
	"log/slog"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Client Identity Tests
// =============================================================================

func TestClient_SetUser(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	client := &Client{
		send:   make(chan []byte, 256),
		rooms:  make(map[uuid.UUID]bool),
		logger: logger,
	}

	userID := uuid.New()
	client.SetUser(userID, "alice")

	assert.Equal(t, userID, client.UserID())
	assert.Equal(t, "alice", client.Username())
}

func TestClient_IsAuthenticated_FalseByDefault(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	client := &Client{
		send:   make(chan []byte, 256),
		rooms:  make(map[uuid.UUID]bool),
		logger: logger,
	}

	assert.False(t, client.IsAuthenticated())
}

func TestClient_IsAuthenticated_TrueAfterSetUser(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	client := &Client{
		send:   make(chan []byte, 256),
		rooms:  make(map[uuid.UUID]bool),
		logger: logger,
	}

	client.SetUser(uuid.New(), "bob")
	assert.True(t, client.IsAuthenticated())
}

func TestClient_IsAuthenticated_FalseForNilUUID(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	client := &Client{
		send:   make(chan []byte, 256),
		rooms:  make(map[uuid.UUID]bool),
		logger: logger,
	}

	client.SetUser(uuid.Nil, "ghost")
	assert.False(t, client.IsAuthenticated())
}

// =============================================================================
// Room Subscription Tests
// =============================================================================

func TestClient_JoinLeaveRoom(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	client := &Client{
		send:   make(chan []byte, 256),
		rooms:  make(map[uuid.UUID]bool),
		logger: logger,
	}

	roomID := uuid.New()

	assert.False(t, client.IsInRoom(roomID))

	client.JoinRoom(roomID)
	assert.True(t, client.IsInRoom(roomID))

	client.LeaveRoom(roomID)
	assert.False(t, client.IsInRoom(roomID))
}

func TestClient_GetRooms(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	client := &Client{
		send:   make(chan []byte, 256),
		rooms:  make(map[uuid.UUID]bool),
		logger: logger,
	}

	r1 := uuid.New()
	r2 := uuid.New()
	r3 := uuid.New()

	client.JoinRoom(r1)
	client.JoinRoom(r2)
	client.JoinRoom(r3)

	rooms := client.GetRooms()
	assert.Len(t, rooms, 3)

	roomSet := map[uuid.UUID]bool{}
	for _, r := range rooms {
		roomSet[r] = true
	}
	assert.True(t, roomSet[r1])
	assert.True(t, roomSet[r2])
	assert.True(t, roomSet[r3])
}

func TestClient_GetRooms_Empty(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	client := &Client{
		send:   make(chan []byte, 256),
		rooms:  make(map[uuid.UUID]bool),
		logger: logger,
	}

	rooms := client.GetRooms()
	assert.Empty(t, rooms)
}

func TestClient_JoinRoom_Idempotent(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	client := &Client{
		send:   make(chan []byte, 256),
		rooms:  make(map[uuid.UUID]bool),
		logger: logger,
	}

	roomID := uuid.New()
	client.JoinRoom(roomID)
	client.JoinRoom(roomID) // join again

	rooms := client.GetRooms()
	assert.Len(t, rooms, 1)
}

func TestClient_LeaveRoom_NotJoined(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	client := &Client{
		send:   make(chan []byte, 256),
		rooms:  make(map[uuid.UUID]bool),
		logger: logger,
	}

	// Leaving a room we never joined should not panic
	assert.NotPanics(t, func() {
		client.LeaveRoom(uuid.New())
	})
}

// =============================================================================
// Send Tests
// =============================================================================

func TestClient_Send_Normal(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	client := &Client{
		send:   make(chan []byte, 256),
		rooms:  make(map[uuid.UUID]bool),
		logger: logger,
	}

	msg, _ := NewMessage("test.event", map[string]string{"key": "value"})
	err := client.Send(msg)
	require.NoError(t, err)

	// Verify message was queued
	select {
	case data := <-client.send:
		assert.NotEmpty(t, data)
	default:
		t.Fatal("message was not queued to send channel")
	}
}

func TestClient_Send_BufferFull(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	client := &Client{
		send:   make(chan []byte, 1), // Very small buffer
		rooms:  make(map[uuid.UUID]bool),
		logger: logger,
	}

	msg1, _ := NewMessage("test.1", nil)
	msg2, _ := NewMessage("test.2", nil)

	// First message should succeed
	err1 := client.Send(msg1)
	assert.NoError(t, err1)

	// Second message fills buffer, third should be dropped silently
	err2 := client.Send(msg2)
	assert.NoError(t, err2) // Send returns nil even when buffer is full — drops silently
}

func TestClient_SendError(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	client := &Client{
		send:   make(chan []byte, 256),
		rooms:  make(map[uuid.UUID]bool),
		logger: logger,
	}

	client.sendError("test_code", "test message")

	// Verify error message was queued
	select {
	case data := <-client.send:
		assert.Contains(t, string(data), "error")
		assert.Contains(t, string(data), "test_code")
		assert.Contains(t, string(data), "test message")
	default:
		t.Fatal("error message was not queued")
	}
}
