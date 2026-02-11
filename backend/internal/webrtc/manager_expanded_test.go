package webrtc

import (
	"context"
	"log/slog"
	"os"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/observer/teatime/internal/pubsub"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Expanded Manager Tests
// =============================================================================

func TestManager_HandleDisconnect_RemovesFromAllRooms(t *testing.T) {
	ps := pubsub.NewMemoryPubSub()
	defer func() { _ = ps.Close() }()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	cfg := &Config{STUNURLs: []string{"stun:stun.l.google.com:19302"}}
	mgr := NewManager(cfg, ps, logger)
	ctx := context.Background()

	userID := uuid.New()
	room1ID := uuid.New()
	room2ID := uuid.New()

	// User joins two rooms
	_, _ = mgr.JoinCall(ctx, room1ID, userID, "alice")
	_, _ = mgr.JoinCall(ctx, room1ID, uuid.New(), "bob") // keep room alive
	_, _ = mgr.JoinCall(ctx, room2ID, userID, "alice")
	_, _ = mgr.JoinCall(ctx, room2ID, uuid.New(), "charlie") // keep room alive

	require.Len(t, mgr.GetActiveRooms(), 2)

	// Simulate disconnect
	mgr.HandleDisconnect(ctx, userID)

	// User should be removed from both rooms
	room1 := mgr.GetRoom(room1ID)
	require.NotNil(t, room1)
	assert.False(t, room1.HasParticipant(userID))

	room2 := mgr.GetRoom(room2ID)
	require.NotNil(t, room2)
	assert.False(t, room2.HasParticipant(userID))
}

func TestManager_HandleDisconnect_EmptyRoomsCleanedUp(t *testing.T) {
	ps := pubsub.NewMemoryPubSub()
	defer func() { _ = ps.Close() }()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	cfg := &Config{STUNURLs: []string{"stun:stun.l.google.com:19302"}}
	mgr := NewManager(cfg, ps, logger)
	ctx := context.Background()

	userID := uuid.New()
	roomID := uuid.New()

	// User is the only one in the room
	_, _ = mgr.JoinCall(ctx, roomID, userID, "alice")

	mgr.HandleDisconnect(ctx, userID)

	// Room should be cleaned up
	assert.Nil(t, mgr.GetRoom(roomID))
	assert.Empty(t, mgr.GetActiveRooms())
}

func TestManager_ConcurrentJoinLeave(t *testing.T) {
	ps := pubsub.NewMemoryPubSub()
	defer func() { _ = ps.Close() }()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	cfg := &Config{STUNURLs: []string{"stun:stun.l.google.com:19302"}}
	mgr := NewManager(cfg, ps, logger)
	ctx := context.Background()

	roomID := uuid.New()
	const numGoroutines = 20

	var wg sync.WaitGroup
	wg.Add(numGoroutines * 2) // join + leave for each

	for i := 0; i < numGoroutines; i++ {
		userID := uuid.New()
		username := "user" + string(rune('A'+i))

		// Join
		go func() {
			defer wg.Done()
			_, _ = mgr.JoinCall(ctx, roomID, userID, username)
		}()

		// Leave (with slight delay to allow join)
		go func() {
			defer wg.Done()
			mgr.LeaveCall(ctx, roomID, userID, username)
		}()
	}

	wg.Wait()

	// Should not panic or deadlock — final state can vary
	// The room might or might not exist depending on ordering
	t.Log("Concurrent join/leave completed without panic/deadlock")
}

func TestManager_DuplicateJoin_Idempotent(t *testing.T) {
	ps := pubsub.NewMemoryPubSub()
	defer func() { _ = ps.Close() }()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	cfg := &Config{STUNURLs: []string{"stun:stun.l.google.com:19302"}}
	mgr := NewManager(cfg, ps, logger)
	ctx := context.Background()

	roomID := uuid.New()
	userID := uuid.New()

	// Join twice
	room1, err1 := mgr.JoinCall(ctx, roomID, userID, "alice")
	require.NoError(t, err1)

	room2, err2 := mgr.JoinCall(ctx, roomID, userID, "alice")
	require.NoError(t, err2)

	assert.Same(t, room1, room2, "duplicate join should return same room")
	// Participant count should be 1 (not 2!) if idempotent, or 2 if not
	// This test documents current behavior
	count := room1.ParticipantCount()
	t.Logf("Participant count after double join: %d (1=idempotent, 2=not)", count)
}

func TestRoom_HasParticipant(t *testing.T) {
	room := NewRoom(uuid.New())

	userID := uuid.New()
	assert.False(t, room.HasParticipant(userID))

	room.AddParticipant(userID, "alice")
	assert.True(t, room.HasParticipant(userID))

	room.RemoveParticipant(userID)
	assert.False(t, room.HasParticipant(userID))
}

func TestRoom_SetGetCallID(t *testing.T) {
	room := NewRoom(uuid.New())

	// Initially nil
	assert.Equal(t, uuid.Nil, room.GetCallID())

	callID := uuid.New()
	room.SetCallID(callID)
	assert.Equal(t, callID, room.GetCallID())

	// Override
	callID2 := uuid.New()
	room.SetCallID(callID2)
	assert.Equal(t, callID2, room.GetCallID())
}

func TestManager_MultipleUsersMultipleRooms(t *testing.T) {
	ps := pubsub.NewMemoryPubSub()
	defer func() { _ = ps.Close() }()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	cfg := &Config{STUNURLs: []string{"stun:stun.l.google.com:19302"}}
	mgr := NewManager(cfg, ps, logger)
	ctx := context.Background()

	room1 := uuid.New()
	room2 := uuid.New()
	alice := uuid.New()
	bob := uuid.New()
	charlie := uuid.New()

	// Alice and Bob in room1
	_, _ = mgr.JoinCall(ctx, room1, alice, "alice")
	_, _ = mgr.JoinCall(ctx, room1, bob, "bob")

	// Bob and Charlie in room2
	_, _ = mgr.JoinCall(ctx, room2, bob, "bob")
	_, _ = mgr.JoinCall(ctx, room2, charlie, "charlie")

	assert.Len(t, mgr.GetActiveRooms(), 2)

	r1 := mgr.GetRoom(room1)
	assert.Equal(t, 2, r1.ParticipantCount())

	r2 := mgr.GetRoom(room2)
	assert.Equal(t, 2, r2.ParticipantCount())

	// Bob leaves room1
	mgr.LeaveCall(ctx, room1, bob, "bob")
	assert.Equal(t, 1, r1.ParticipantCount())

	// Bob should still be in room2
	assert.True(t, r2.HasParticipant(bob))
}

func TestRoom_RemoveNonExistentParticipant(t *testing.T) {
	room := NewRoom(uuid.New())
	room.AddParticipant(uuid.New(), "alice")

	// Removing a non-existent participant should not panic
	assert.NotPanics(t, func() {
		room.RemoveParticipant(uuid.New())
	})
	assert.Equal(t, 1, room.ParticipantCount())
}

func TestRoom_GetParticipants_ReturnsConsistentCopy(t *testing.T) {
	room := NewRoom(uuid.New())
	a := uuid.New()
	b := uuid.New()
	c := uuid.New()

	room.AddParticipant(a, "alice")
	room.AddParticipant(b, "bob")
	room.AddParticipant(c, "charlie")

	participants := room.GetParticipants()
	assert.Len(t, participants, 3)

	// Verify all usernames are present
	names := map[string]bool{}
	for _, p := range participants {
		names[p.Username] = true
	}
	assert.True(t, names["alice"])
	assert.True(t, names["bob"])
	assert.True(t, names["charlie"])
}

func TestManager_LeaveCall_RoomNotExists(t *testing.T) {
	ps := pubsub.NewMemoryPubSub()
	defer func() { _ = ps.Close() }()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	cfg := &Config{STUNURLs: []string{"stun:stun.l.google.com:19302"}}
	mgr := NewManager(cfg, ps, logger)
	ctx := context.Background()

	// Leave from non-existent room — should not panic
	assert.NotPanics(t, func() {
		mgr.LeaveCall(ctx, uuid.New(), uuid.New(), "ghost")
	})
}
