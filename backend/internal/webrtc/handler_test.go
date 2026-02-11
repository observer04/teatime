package webrtc

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/observer/teatime/internal/pubsub"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestCallHandler creates a CallHandler with real Manager + MemoryPubSub
// but nil DB repos. Tests that touch DB repos will fail/panic — that's intentional
// to show the code isn't easily unit-testable for those paths.
func newTestCallHandler(t *testing.T) (*CallHandler, *Manager, pubsub.PubSub) {
	t.Helper()
	ps := pubsub.NewMemoryPubSub()
	t.Cleanup(func() { _ = ps.Close() })

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	cfg := &Config{STUNURLs: []string{"stun:stun.l.google.com:19302"}}
	mgr := NewManager(cfg, ps, logger)

	handler := NewCallHandler(mgr, nil, nil, ps, logger)
	return handler, mgr, ps
}

// =============================================================================
// HandleJoin Tests
// =============================================================================

func TestCallHandler_HandleJoin_InvalidPayload(t *testing.T) {
	handler, _, _ := newTestCallHandler(t)
	ctx := context.Background()
	sigCtx := &SignalingContext{UserID: uuid.New(), Username: "alice"}

	// Garbage JSON
	config, err := handler.HandleJoin(ctx, sigCtx, json.RawMessage(`{invalid`))
	assert.Nil(t, config)
	require.Error(t, err)

	callErr, ok := err.(*CallError)
	require.True(t, ok, "expected *CallError, got %T", err)
	assert.Equal(t, "invalid_payload", callErr.Code)
}

func TestCallHandler_HandleJoin_InvalidRoomID(t *testing.T) {
	handler, _, _ := newTestCallHandler(t)
	ctx := context.Background()
	sigCtx := &SignalingContext{UserID: uuid.New(), Username: "alice"}

	payload := json.RawMessage(`{"room_id": "not-a-uuid"}`)
	config, err := handler.HandleJoin(ctx, sigCtx, payload)
	assert.Nil(t, config)
	require.Error(t, err)

	callErr, ok := err.(*CallError)
	require.True(t, ok)
	assert.Equal(t, "invalid_room", callErr.Code)
}

func TestCallHandler_HandleJoin_EmptyRoomID(t *testing.T) {
	handler, _, _ := newTestCallHandler(t)
	ctx := context.Background()
	sigCtx := &SignalingContext{UserID: uuid.New(), Username: "alice"}

	payload := json.RawMessage(`{"room_id": ""}`)
	config, err := handler.HandleJoin(ctx, sigCtx, payload)
	assert.Nil(t, config)
	require.Error(t, err)

	callErr, ok := err.(*CallError)
	require.True(t, ok)
	assert.Equal(t, "invalid_room", callErr.Code)
}

func TestCallHandler_HandleJoin_MissingRoomIDField(t *testing.T) {
	handler, _, _ := newTestCallHandler(t)
	ctx := context.Background()
	sigCtx := &SignalingContext{UserID: uuid.New(), Username: "alice"}

	// Valid JSON but no room_id field — room_id will be empty string → invalid UUID
	payload := json.RawMessage(`{}`)
	config, err := handler.HandleJoin(ctx, sigCtx, payload)
	assert.Nil(t, config)
	require.Error(t, err)

	callErr, ok := err.(*CallError)
	require.True(t, ok)
	assert.Equal(t, "invalid_room", callErr.Code)
}

// This test will fail/panic because convRepo is nil — demonstrates that
// HandleJoin's membership check requires a real database repository.
// This is intentional: it surfaces a testability gap in the architecture.
func TestCallHandler_HandleJoin_NilConvRepo_Panics(t *testing.T) {
	handler, _, _ := newTestCallHandler(t)
	ctx := context.Background()
	sigCtx := &SignalingContext{UserID: uuid.New(), Username: "alice"}
	roomID := uuid.New()

	payload, _ := json.Marshal(CallJoinPayload{RoomID: roomID.String()})

	// This should panic because convRepo is nil — the handler calls
	// h.convRepo.IsMember() which dereferences a nil pointer
	assert.Panics(t, func() {
		_, _ = handler.HandleJoin(ctx, sigCtx, payload)
	}, "HandleJoin should panic with nil convRepo — testability gap")
}

// =============================================================================
// HandleLeave Tests
// =============================================================================

func TestCallHandler_HandleLeave_InvalidPayload(t *testing.T) {
	handler, _, _ := newTestCallHandler(t)
	ctx := context.Background()
	sigCtx := &SignalingContext{UserID: uuid.New(), Username: "alice"}

	err := handler.HandleLeave(ctx, sigCtx, json.RawMessage(`{invalid`))
	require.Error(t, err)

	callErr, ok := err.(*CallError)
	require.True(t, ok)
	assert.Equal(t, "invalid_payload", callErr.Code)
}

func TestCallHandler_HandleLeave_InvalidRoomID(t *testing.T) {
	handler, _, _ := newTestCallHandler(t)
	ctx := context.Background()
	sigCtx := &SignalingContext{UserID: uuid.New(), Username: "alice"}

	payload := json.RawMessage(`{"room_id": "not-a-uuid"}`)
	err := handler.HandleLeave(ctx, sigCtx, payload)
	require.Error(t, err)

	callErr, ok := err.(*CallError)
	require.True(t, ok)
	assert.Equal(t, "invalid_room", callErr.Code)
}

func TestCallHandler_HandleLeave_NoExistingRoom(t *testing.T) {
	handler, _, _ := newTestCallHandler(t)
	ctx := context.Background()
	sigCtx := &SignalingContext{UserID: uuid.New(), Username: "alice"}
	roomID := uuid.New()

	payload, _ := json.Marshal(CallLeavePayload{RoomID: roomID.String()})

	// Leaving a non-existent room should not error
	err := handler.HandleLeave(ctx, sigCtx, payload)
	assert.NoError(t, err)
}

func TestCallHandler_HandleLeave_UserLeavesRoom(t *testing.T) {
	handler, mgr, _ := newTestCallHandler(t)
	ctx := context.Background()

	roomID := uuid.New()
	userID := uuid.New()

	// Pre-populate room via manager
	_, err := mgr.JoinCall(ctx, roomID, userID, "alice")
	require.NoError(t, err)

	user2ID := uuid.New()
	_, err = mgr.JoinCall(ctx, roomID, user2ID, "bob")
	require.NoError(t, err)

	require.NotNil(t, mgr.GetRoom(roomID))
	require.Equal(t, 2, mgr.GetRoom(roomID).ParticipantCount())

	// User leaves
	sigCtx := &SignalingContext{UserID: userID, Username: "alice"}
	payload, _ := json.Marshal(CallLeavePayload{RoomID: roomID.String()})
	err = handler.HandleLeave(ctx, sigCtx, payload)
	assert.NoError(t, err)

	// Room should still exist with 1 participant
	room := mgr.GetRoom(roomID)
	require.NotNil(t, room)
	assert.Equal(t, 1, room.ParticipantCount())
	assert.False(t, room.HasParticipant(userID))
	assert.True(t, room.HasParticipant(user2ID))
}

func TestCallHandler_HandleLeave_LastUserLeavesDeletesRoom(t *testing.T) {
	handler, mgr, _ := newTestCallHandler(t)
	ctx := context.Background()

	roomID := uuid.New()
	userID := uuid.New()

	// Pre-populate room
	_, err := mgr.JoinCall(ctx, roomID, userID, "alice")
	require.NoError(t, err)

	// Leave
	sigCtx := &SignalingContext{UserID: userID, Username: "alice"}
	payload, _ := json.Marshal(CallLeavePayload{RoomID: roomID.String()})
	err = handler.HandleLeave(ctx, sigCtx, payload)
	assert.NoError(t, err)

	// Room should be deleted
	assert.Nil(t, mgr.GetRoom(roomID))
}

// =============================================================================
// HandleOffer Tests
// =============================================================================

func TestCallHandler_HandleOffer_InvalidPayload(t *testing.T) {
	handler, _, _ := newTestCallHandler(t)
	ctx := context.Background()
	sigCtx := &SignalingContext{UserID: uuid.New(), Username: "alice"}

	err := handler.HandleOffer(ctx, sigCtx, json.RawMessage(`{invalid`))
	require.Error(t, err)

	callErr, ok := err.(*CallError)
	require.True(t, ok)
	assert.Equal(t, "invalid_payload", callErr.Code)
}

func TestCallHandler_HandleOffer_InvalidRoomID(t *testing.T) {
	handler, _, _ := newTestCallHandler(t)
	ctx := context.Background()
	sigCtx := &SignalingContext{UserID: uuid.New(), Username: "alice"}

	payload := json.RawMessage(`{"room_id": "bad", "target_id": "bad", "sdp": "v=0..."}`)
	err := handler.HandleOffer(ctx, sigCtx, payload)
	require.Error(t, err)

	callErr, ok := err.(*CallError)
	require.True(t, ok)
	assert.Equal(t, "invalid_room", callErr.Code)
}

func TestCallHandler_HandleOffer_InvalidTargetID(t *testing.T) {
	handler, _, _ := newTestCallHandler(t)
	ctx := context.Background()
	sigCtx := &SignalingContext{UserID: uuid.New(), Username: "alice"}
	roomID := uuid.New()

	payload, _ := json.Marshal(map[string]string{
		"room_id":   roomID.String(),
		"target_id": "not-a-uuid",
		"sdp":       "v=0...",
	})
	err := handler.HandleOffer(ctx, sigCtx, payload)
	require.Error(t, err)

	callErr, ok := err.(*CallError)
	require.True(t, ok)
	assert.Equal(t, "invalid_target", callErr.Code)
}

func TestCallHandler_HandleOffer_NoActiveCall(t *testing.T) {
	handler, _, _ := newTestCallHandler(t)
	ctx := context.Background()
	sigCtx := &SignalingContext{UserID: uuid.New(), Username: "alice"}
	roomID := uuid.New()
	targetID := uuid.New()

	payload, _ := json.Marshal(CallOfferPayload{
		RoomID:   roomID.String(),
		TargetID: targetID.String(),
		SDP:      "v=0...",
	})
	err := handler.HandleOffer(ctx, sigCtx, payload)
	require.Error(t, err)

	callErr, ok := err.(*CallError)
	require.True(t, ok)
	assert.Equal(t, "no_call", callErr.Code)
}

func TestCallHandler_HandleOffer_TargetNotInRoom(t *testing.T) {
	handler, mgr, _ := newTestCallHandler(t)
	ctx := context.Background()

	roomID := uuid.New()
	aliceID := uuid.New()
	bobID := uuid.New()

	// Only Alice is in the room
	_, err := mgr.JoinCall(ctx, roomID, aliceID, "alice")
	require.NoError(t, err)

	sigCtx := &SignalingContext{UserID: aliceID, Username: "alice"}
	payload, _ := json.Marshal(CallOfferPayload{
		RoomID:   roomID.String(),
		TargetID: bobID.String(), // Bob is NOT in the room
		SDP:      "v=0...",
	})
	err = handler.HandleOffer(ctx, sigCtx, payload)
	require.Error(t, err)

	callErr, ok := err.(*CallError)
	require.True(t, ok)
	assert.Equal(t, "target_not_found", callErr.Code)
}

func TestCallHandler_HandleOffer_ValidRelay(t *testing.T) {
	handler, mgr, ps := newTestCallHandler(t)
	ctx := context.Background()

	roomID := uuid.New()
	aliceID := uuid.New()
	bobID := uuid.New()

	// Both users in room
	_, err := mgr.JoinCall(ctx, roomID, aliceID, "alice")
	require.NoError(t, err)
	_, err = mgr.JoinCall(ctx, roomID, bobID, "bob")
	require.NoError(t, err)

	// Subscribe to Bob's user topic to verify relay
	received := make(chan *pubsub.Message, 1)
	sub, err := ps.Subscribe(ctx, pubsub.Topics.User(bobID.String()), func(ctx context.Context, msg *pubsub.Message) {
		received <- msg
	})
	require.NoError(t, err)
	defer func() { _ = sub.Unsubscribe() }()

	// Alice sends offer to Bob
	sigCtx := &SignalingContext{UserID: aliceID, Username: "alice"}
	testSDP := "v=0\r\no=- 12345 2 IN IP4 127.0.0.1\r\n..."
	payload, _ := json.Marshal(CallOfferPayload{
		RoomID:   roomID.String(),
		TargetID: bobID.String(),
		SDP:      testSDP,
	})
	err = handler.HandleOffer(ctx, sigCtx, payload)
	require.NoError(t, err)

	// Verify Bob received the offer
	select {
	case msg := <-received:
		assert.Equal(t, EventTypeCallOffer, msg.Type)
		var relayPayload map[string]interface{}
		err := json.Unmarshal(msg.Payload, &relayPayload)
		require.NoError(t, err)
		assert.Equal(t, roomID.String(), relayPayload["room_id"])
		assert.Equal(t, aliceID.String(), relayPayload["from_id"])
		assert.Equal(t, testSDP, relayPayload["sdp"])
		assert.Equal(t, "alice", relayPayload["from_name"])
	case <-time.After(200 * time.Millisecond):
		t.Fatal("Bob did not receive the relayed offer")
	}
}

// =============================================================================
// HandleAnswer Tests
// =============================================================================

func TestCallHandler_HandleAnswer_InvalidPayload(t *testing.T) {
	handler, _, _ := newTestCallHandler(t)
	ctx := context.Background()
	sigCtx := &SignalingContext{UserID: uuid.New(), Username: "bob"}

	err := handler.HandleAnswer(ctx, sigCtx, json.RawMessage(`{garbage}`))
	require.Error(t, err)

	callErr, ok := err.(*CallError)
	require.True(t, ok)
	assert.Equal(t, "invalid_payload", callErr.Code)
}

func TestCallHandler_HandleAnswer_NoActiveCall(t *testing.T) {
	handler, _, _ := newTestCallHandler(t)
	ctx := context.Background()
	sigCtx := &SignalingContext{UserID: uuid.New(), Username: "bob"}
	roomID := uuid.New()
	targetID := uuid.New()

	payload, _ := json.Marshal(CallAnswerPayload{
		RoomID:   roomID.String(),
		TargetID: targetID.String(),
		SDP:      "answer-sdp",
	})
	err := handler.HandleAnswer(ctx, sigCtx, payload)
	require.Error(t, err)

	callErr, ok := err.(*CallError)
	require.True(t, ok)
	assert.Equal(t, "no_call", callErr.Code)
}

func TestCallHandler_HandleAnswer_TargetNotInRoom(t *testing.T) {
	handler, mgr, _ := newTestCallHandler(t)
	ctx := context.Background()

	roomID := uuid.New()
	bobID := uuid.New()
	charlieID := uuid.New() // Not in room

	_, _ = mgr.JoinCall(ctx, roomID, bobID, "bob")

	sigCtx := &SignalingContext{UserID: bobID, Username: "bob"}
	payload, _ := json.Marshal(CallAnswerPayload{
		RoomID:   roomID.String(),
		TargetID: charlieID.String(),
		SDP:      "answer-sdp",
	})
	err := handler.HandleAnswer(ctx, sigCtx, payload)
	require.Error(t, err)

	callErr, ok := err.(*CallError)
	require.True(t, ok)
	assert.Equal(t, "target_not_found", callErr.Code)
}

func TestCallHandler_HandleAnswer_ValidRelay(t *testing.T) {
	handler, mgr, ps := newTestCallHandler(t)
	ctx := context.Background()

	roomID := uuid.New()
	aliceID := uuid.New()
	bobID := uuid.New()

	_, _ = mgr.JoinCall(ctx, roomID, aliceID, "alice")
	_, _ = mgr.JoinCall(ctx, roomID, bobID, "bob")

	// Subscribe to Alice's topic
	received := make(chan *pubsub.Message, 1)
	sub, _ := ps.Subscribe(ctx, pubsub.Topics.User(aliceID.String()), func(ctx context.Context, msg *pubsub.Message) {
		received <- msg
	})
	defer func() { _ = sub.Unsubscribe() }()

	// Bob sends answer to Alice
	sigCtx := &SignalingContext{UserID: bobID, Username: "bob"}
	answerSDP := "v=0\r\nSDP ANSWER\r\n"
	payload, _ := json.Marshal(CallAnswerPayload{
		RoomID:   roomID.String(),
		TargetID: aliceID.String(),
		SDP:      answerSDP,
	})
	err := handler.HandleAnswer(ctx, sigCtx, payload)
	require.NoError(t, err)

	select {
	case msg := <-received:
		assert.Equal(t, EventTypeCallAnswer, msg.Type)
		var relayPayload map[string]interface{}
		_ = json.Unmarshal(msg.Payload, &relayPayload)
		assert.Equal(t, answerSDP, relayPayload["sdp"])
		assert.Equal(t, bobID.String(), relayPayload["from_id"])
	case <-time.After(200 * time.Millisecond):
		t.Fatal("Alice did not receive the relayed answer")
	}
}

// =============================================================================
// HandleICECandidate Tests
// =============================================================================

func TestCallHandler_HandleICECandidate_InvalidPayload(t *testing.T) {
	handler, _, _ := newTestCallHandler(t)
	ctx := context.Background()
	sigCtx := &SignalingContext{UserID: uuid.New(), Username: "alice"}

	err := handler.HandleICECandidate(ctx, sigCtx, json.RawMessage(`not json`))
	require.Error(t, err)
	callErr, ok := err.(*CallError)
	require.True(t, ok)
	assert.Equal(t, "invalid_payload", callErr.Code)
}

func TestCallHandler_HandleICECandidate_NoActiveCall(t *testing.T) {
	handler, _, _ := newTestCallHandler(t)
	ctx := context.Background()
	sigCtx := &SignalingContext{UserID: uuid.New(), Username: "alice"}
	roomID := uuid.New()
	targetID := uuid.New()

	payload, _ := json.Marshal(CallICECandidatePayload{
		RoomID:    roomID.String(),
		TargetID:  targetID.String(),
		Candidate: map[string]string{"candidate": "candidate:1 1 UDP 2122194687 192.168.1.1 12345 typ host"},
	})
	err := handler.HandleICECandidate(ctx, sigCtx, payload)
	require.Error(t, err)

	callErr, ok := err.(*CallError)
	require.True(t, ok)
	assert.Equal(t, "no_call", callErr.Code)
}

func TestCallHandler_HandleICECandidate_TargetNotInRoom(t *testing.T) {
	handler, mgr, _ := newTestCallHandler(t)
	ctx := context.Background()

	roomID := uuid.New()
	aliceID := uuid.New()
	phantomID := uuid.New()

	_, _ = mgr.JoinCall(ctx, roomID, aliceID, "alice")

	sigCtx := &SignalingContext{UserID: aliceID, Username: "alice"}
	payload, _ := json.Marshal(CallICECandidatePayload{
		RoomID:    roomID.String(),
		TargetID:  phantomID.String(),
		Candidate: "candidate:...",
	})
	err := handler.HandleICECandidate(ctx, sigCtx, payload)
	require.Error(t, err)

	callErr, ok := err.(*CallError)
	require.True(t, ok)
	assert.Equal(t, "target_not_found", callErr.Code)
}

func TestCallHandler_HandleICECandidate_ValidRelay(t *testing.T) {
	handler, mgr, ps := newTestCallHandler(t)
	ctx := context.Background()

	roomID := uuid.New()
	aliceID := uuid.New()
	bobID := uuid.New()

	_, _ = mgr.JoinCall(ctx, roomID, aliceID, "alice")
	_, _ = mgr.JoinCall(ctx, roomID, bobID, "bob")

	received := make(chan *pubsub.Message, 1)
	sub, _ := ps.Subscribe(ctx, pubsub.Topics.User(bobID.String()), func(ctx context.Context, msg *pubsub.Message) {
		received <- msg
	})
	defer func() { _ = sub.Unsubscribe() }()

	candidateData := map[string]string{
		"candidate":     "candidate:1 1 UDP 2122194687 192.168.1.1 12345 typ host",
		"sdpMid":        "0",
		"sdpMLineIndex": "0",
	}

	sigCtx := &SignalingContext{UserID: aliceID, Username: "alice"}
	payload, _ := json.Marshal(CallICECandidatePayload{
		RoomID:    roomID.String(),
		TargetID:  bobID.String(),
		Candidate: candidateData,
	})
	err := handler.HandleICECandidate(ctx, sigCtx, payload)
	require.NoError(t, err)

	select {
	case msg := <-received:
		assert.Equal(t, EventTypeCallICECandidate, msg.Type)
		var relayPayload map[string]interface{}
		_ = json.Unmarshal(msg.Payload, &relayPayload)
		assert.Equal(t, aliceID.String(), relayPayload["from_id"])
		assert.NotNil(t, relayPayload["candidate"])
	case <-time.After(200 * time.Millisecond):
		t.Fatal("Bob did not receive the relayed ICE candidate")
	}
}

// =============================================================================
// HandleReady Tests
// =============================================================================

func TestCallHandler_HandleReady_InvalidPayload(t *testing.T) {
	handler, _, _ := newTestCallHandler(t)
	ctx := context.Background()
	sigCtx := &SignalingContext{UserID: uuid.New(), Username: "alice"}

	err := handler.HandleReady(ctx, sigCtx, json.RawMessage(`not json`))
	require.Error(t, err)
	callErr, ok := err.(*CallError)
	require.True(t, ok)
	assert.Equal(t, "invalid_payload", callErr.Code)
}

func TestCallHandler_HandleReady_InvalidRoomID(t *testing.T) {
	handler, _, _ := newTestCallHandler(t)
	ctx := context.Background()
	sigCtx := &SignalingContext{UserID: uuid.New(), Username: "alice"}

	payload := json.RawMessage(`{"room_id": "invalid-uuid"}`)
	err := handler.HandleReady(ctx, sigCtx, payload)
	require.Error(t, err)

	callErr, ok := err.(*CallError)
	require.True(t, ok)
	assert.Equal(t, "invalid_room", callErr.Code)
}

func TestCallHandler_HandleReady_NoActiveRoom(t *testing.T) {
	handler, _, _ := newTestCallHandler(t)
	ctx := context.Background()
	sigCtx := &SignalingContext{UserID: uuid.New(), Username: "alice"}
	roomID := uuid.New()

	payload, _ := json.Marshal(map[string]string{"room_id": roomID.String()})
	err := handler.HandleReady(ctx, sigCtx, payload)
	require.Error(t, err)

	callErr, ok := err.(*CallError)
	require.True(t, ok)
	assert.Equal(t, "no_call", callErr.Code)
}

func TestCallHandler_HandleReady_ValidBroadcast(t *testing.T) {
	handler, mgr, ps := newTestCallHandler(t)
	ctx := context.Background()

	roomID := uuid.New()
	aliceID := uuid.New()
	bobID := uuid.New()

	_, _ = mgr.JoinCall(ctx, roomID, aliceID, "alice")
	_, _ = mgr.JoinCall(ctx, roomID, bobID, "bob")

	// Subscribe to room topic to verify broadcast
	received := make(chan *pubsub.Message, 1)
	sub, _ := ps.Subscribe(ctx, pubsub.Topics.Room(roomID.String()), func(ctx context.Context, msg *pubsub.Message) {
		received <- msg
	})
	defer func() { _ = sub.Unsubscribe() }()

	sigCtx := &SignalingContext{UserID: bobID, Username: "bob"}
	payload, _ := json.Marshal(map[string]string{"room_id": roomID.String()})
	err := handler.HandleReady(ctx, sigCtx, payload)
	require.NoError(t, err)

	select {
	case msg := <-received:
		assert.Equal(t, EventTypeCallReady, msg.Type)
		var relayPayload map[string]string
		_ = json.Unmarshal(msg.Payload, &relayPayload)
		assert.Equal(t, roomID.String(), relayPayload["room_id"])
		assert.Equal(t, bobID.String(), relayPayload["from_id"])
	case <-time.After(200 * time.Millisecond):
		t.Fatal("Room topic did not receive call.ready broadcast")
	}
}

// =============================================================================
// HandleMuteUpdate Tests
// =============================================================================

func TestCallHandler_HandleMuteUpdate_InvalidPayload(t *testing.T) {
	handler, _, _ := newTestCallHandler(t)
	ctx := context.Background()
	sigCtx := &SignalingContext{UserID: uuid.New(), Username: "alice"}

	err := handler.HandleMuteUpdate(ctx, sigCtx, json.RawMessage(`{bad`))
	require.Error(t, err)

	callErr, ok := err.(*CallError)
	require.True(t, ok)
	assert.Equal(t, "invalid_payload", callErr.Code)
}

func TestCallHandler_HandleMuteUpdate_NoActiveRoom(t *testing.T) {
	handler, _, _ := newTestCallHandler(t)
	ctx := context.Background()
	sigCtx := &SignalingContext{UserID: uuid.New(), Username: "alice"}
	roomID := uuid.New()

	payload, _ := json.Marshal(map[string]interface{}{
		"room_id": roomID.String(),
		"kind":    "audio",
		"muted":   true,
	})
	err := handler.HandleMuteUpdate(ctx, sigCtx, payload)
	require.Error(t, err)

	callErr, ok := err.(*CallError)
	require.True(t, ok)
	assert.Equal(t, "no_call", callErr.Code)
}

func TestCallHandler_HandleMuteUpdate_RelaysToOtherParticipants(t *testing.T) {
	handler, mgr, ps := newTestCallHandler(t)
	ctx := context.Background()

	roomID := uuid.New()
	aliceID := uuid.New()
	bobID := uuid.New()

	_, _ = mgr.JoinCall(ctx, roomID, aliceID, "alice")
	_, _ = mgr.JoinCall(ctx, roomID, bobID, "bob")

	// Bob subscribes to receive mute updates
	received := make(chan *pubsub.Message, 1)
	sub, _ := ps.Subscribe(ctx, pubsub.Topics.User(bobID.String()), func(ctx context.Context, msg *pubsub.Message) {
		received <- msg
	})
	defer func() { _ = sub.Unsubscribe() }()

	// Alice mutes audio
	sigCtx := &SignalingContext{UserID: aliceID, Username: "alice"}
	payload, _ := json.Marshal(map[string]interface{}{
		"room_id": roomID.String(),
		"kind":    "audio",
		"muted":   true,
	})
	err := handler.HandleMuteUpdate(ctx, sigCtx, payload)
	require.NoError(t, err)

	select {
	case msg := <-received:
		assert.Equal(t, EventTypeCallMuteUpdate, msg.Type)
		var mutePayload map[string]interface{}
		_ = json.Unmarshal(msg.Payload, &mutePayload)
		assert.Equal(t, aliceID.String(), mutePayload["user_id"])
		assert.Equal(t, "audio", mutePayload["kind"])
		assert.Equal(t, true, mutePayload["muted"])
	case <-time.After(200 * time.Millisecond):
		t.Fatal("Bob did not receive mute update")
	}
}

func TestCallHandler_HandleMuteUpdate_DoesNotRelayBackToSender(t *testing.T) {
	handler, mgr, ps := newTestCallHandler(t)
	ctx := context.Background()

	roomID := uuid.New()
	aliceID := uuid.New()

	_, _ = mgr.JoinCall(ctx, roomID, aliceID, "alice")

	// Alice subscribes to her own topic
	selfReceived := make(chan *pubsub.Message, 1)
	sub, _ := ps.Subscribe(ctx, pubsub.Topics.User(aliceID.String()), func(ctx context.Context, msg *pubsub.Message) {
		selfReceived <- msg
	})
	defer func() { _ = sub.Unsubscribe() }()

	// Alice toggles mute — alone in room, no one should receive
	sigCtx := &SignalingContext{UserID: aliceID, Username: "alice"}
	payload, _ := json.Marshal(map[string]interface{}{
		"room_id": roomID.String(),
		"kind":    "video",
		"muted":   true,
	})
	err := handler.HandleMuteUpdate(ctx, sigCtx, payload)
	require.NoError(t, err)

	select {
	case <-selfReceived:
		t.Fatal("Mute update should NOT be relayed back to the sender")
	case <-time.After(200 * time.Millisecond):
		// Correct — sender should not receive their own mute update
	}
}

// =============================================================================
// HandleDeclined Tests
// =============================================================================

func TestCallHandler_HandleDeclined_InvalidPayload(t *testing.T) {
	handler, _, _ := newTestCallHandler(t)
	ctx := context.Background()
	sigCtx := &SignalingContext{UserID: uuid.New(), Username: "bob"}

	err := handler.HandleDeclined(ctx, sigCtx, json.RawMessage(`{bad`))
	require.Error(t, err)

	callErr, ok := err.(*CallError)
	require.True(t, ok)
	assert.Equal(t, "invalid_payload", callErr.Code)
}

func TestCallHandler_HandleDeclined_InvalidCallID(t *testing.T) {
	handler, _, _ := newTestCallHandler(t)
	ctx := context.Background()
	sigCtx := &SignalingContext{UserID: uuid.New(), Username: "bob"}

	payload := json.RawMessage(`{"call_id": "not-uuid", "conversation_id": "abc"}`)
	err := handler.HandleDeclined(ctx, sigCtx, payload)
	require.Error(t, err)

	callErr, ok := err.(*CallError)
	require.True(t, ok)
	assert.Equal(t, "invalid_call_id", callErr.Code)
}

// HandleDeclined with nil callRepo — the handler checks h.callRepo != nil before
// UpdateCallStatus but then unconditionally calls h.callRepo.GetCallLog which panics.
// This is a real bug worth surfacing.
func TestCallHandler_HandleDeclined_NilCallRepo_Panics(t *testing.T) {
	handler, _, _ := newTestCallHandler(t)
	ctx := context.Background()
	sigCtx := &SignalingContext{UserID: uuid.New(), Username: "bob"}
	callID := uuid.New()

	payload, _ := json.Marshal(map[string]string{
		"call_id":         callID.String(),
		"conversation_id": uuid.New().String(),
	})

	// This will panic because after the nil check for UpdateCallStatus,
	// handler.callRepo.GetCallLog is called unconditionally
	assert.Panics(t, func() {
		_ = handler.HandleDeclined(ctx, sigCtx, payload)
	}, "HandleDeclined panics when callRepo is nil — unconditional GetCallLog dereference")
}

// =============================================================================
// IsUserInRoom Tests
// =============================================================================

func TestCallHandler_IsUserInRoom_UserPresent(t *testing.T) {
	handler, mgr, _ := newTestCallHandler(t)
	ctx := context.Background()

	roomID := uuid.New()
	userID := uuid.New()
	_, _ = mgr.JoinCall(ctx, roomID, userID, "alice")

	assert.True(t, handler.IsUserInRoom(roomID, userID))
}

func TestCallHandler_IsUserInRoom_UserNotPresent(t *testing.T) {
	handler, mgr, _ := newTestCallHandler(t)
	ctx := context.Background()

	roomID := uuid.New()
	_, _ = mgr.JoinCall(ctx, roomID, uuid.New(), "alice")

	assert.False(t, handler.IsUserInRoom(roomID, uuid.New()))
}

func TestCallHandler_IsUserInRoom_RoomDoesNotExist(t *testing.T) {
	handler, _, _ := newTestCallHandler(t)
	assert.False(t, handler.IsUserInRoom(uuid.New(), uuid.New()))
}

// =============================================================================
// HandleDisconnect Tests
// =============================================================================

func TestCallHandler_HandleDisconnect_WithNilManager(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	handler := &CallHandler{manager: nil, logger: logger}

	// Should not panic
	assert.NotPanics(t, func() {
		handler.HandleDisconnect(context.Background(), uuid.New(), "alice")
	})
}

func TestCallHandler_HandleDisconnect_CleansUpFromRooms(t *testing.T) {
	handler, mgr, _ := newTestCallHandler(t)
	ctx := context.Background()

	roomID := uuid.New()
	aliceID := uuid.New()
	bobID := uuid.New()

	_, _ = mgr.JoinCall(ctx, roomID, aliceID, "alice")
	_, _ = mgr.JoinCall(ctx, roomID, bobID, "bob")

	// Alice disconnects
	handler.HandleDisconnect(ctx, aliceID, "alice")

	// Alice should be removed from the room
	room := mgr.GetRoom(roomID)
	require.NotNil(t, room, "room should still exist with Bob in it")
	assert.False(t, room.HasParticipant(aliceID))
	assert.True(t, room.HasParticipant(bobID))
}

// =============================================================================
// CallError Tests
// =============================================================================

func TestCallError_Error(t *testing.T) {
	err := &CallError{Code: "test_code", Message: "test message"}
	assert.Equal(t, "test message", err.Error())
}

func TestCallError_ImplementsError(t *testing.T) {
	var err error = &CallError{Code: "test", Message: "msg"}
	assert.Error(t, err)
}

// =============================================================================
// Edge Cases & Security Tests
// =============================================================================

func TestCallHandler_HandleOffer_SelfTarget(t *testing.T) {
	// Sending an offer to yourself should return target_not_found
	// because the room doesn't have you as a separate target.
	// Actually, the room DOES have you as a participant, so this
	// succeeds and relays to yourself — which is a potential bug.
	handler, mgr, ps := newTestCallHandler(t)
	ctx := context.Background()

	roomID := uuid.New()
	aliceID := uuid.New()

	_, _ = mgr.JoinCall(ctx, roomID, aliceID, "alice")

	selfReceived := make(chan *pubsub.Message, 1)
	sub, _ := ps.Subscribe(ctx, pubsub.Topics.User(aliceID.String()), func(ctx context.Context, msg *pubsub.Message) {
		selfReceived <- msg
	})
	defer func() { _ = sub.Unsubscribe() }()

	sigCtx := &SignalingContext{UserID: aliceID, Username: "alice"}
	payload, _ := json.Marshal(CallOfferPayload{
		RoomID:   roomID.String(),
		TargetID: aliceID.String(), // Targeting self!
		SDP:      "v=0...",
	})

	err := handler.HandleOffer(ctx, sigCtx, payload)
	// NOTE: This test documents that self-targeting is allowed.
	// A production system should probably reject it.
	// If this passes, it means the code has no self-targeting guard.
	assert.NoError(t, err, "self-targeting offer succeeds — possible security gap")

	select {
	case msg := <-selfReceived:
		assert.Equal(t, EventTypeCallOffer, msg.Type, "self-targeted offer was relayed")
	case <-time.After(200 * time.Millisecond):
		// If we get here, self-targeting was blocked (good!)
	}
}

func TestCallHandler_HandleOffer_MultipleOffersToSameTarget(t *testing.T) {
	handler, mgr, ps := newTestCallHandler(t)
	ctx := context.Background()

	roomID := uuid.New()
	aliceID := uuid.New()
	bobID := uuid.New()

	_, _ = mgr.JoinCall(ctx, roomID, aliceID, "alice")
	_, _ = mgr.JoinCall(ctx, roomID, bobID, "bob")

	received := make(chan *pubsub.Message, 10)
	sub, _ := ps.Subscribe(ctx, pubsub.Topics.User(bobID.String()), func(ctx context.Context, msg *pubsub.Message) {
		received <- msg
	})
	defer func() { _ = sub.Unsubscribe() }()

	// Send 3 offers in quick succession (simulates renegotiation without answer)
	sigCtx := &SignalingContext{UserID: aliceID, Username: "alice"}
	for i := 0; i < 3; i++ {
		payload, _ := json.Marshal(CallOfferPayload{
			RoomID:   roomID.String(),
			TargetID: bobID.String(),
			SDP:      "offer-sdp-" + string(rune('A'+i)),
		})
		err := handler.HandleOffer(ctx, sigCtx, payload)
		assert.NoError(t, err, "offer %d should succeed", i)
	}

	// All 3 should be relayed (no rate limiting or dedup)
	// Wait for async pubsub delivery
	time.Sleep(200 * time.Millisecond)
	assert.Equal(t, 3, len(received), "all offers should be relayed without dedup")
}

func TestCallHandler_ThreeParticipantsP2P_MuteRelaysToBothOthers(t *testing.T) {
	handler, mgr, ps := newTestCallHandler(t)
	ctx := context.Background()

	roomID := uuid.New()
	aliceID := uuid.New()
	bobID := uuid.New()
	charlieID := uuid.New()

	_, _ = mgr.JoinCall(ctx, roomID, aliceID, "alice")
	_, _ = mgr.JoinCall(ctx, roomID, bobID, "bob")
	_, _ = mgr.JoinCall(ctx, roomID, charlieID, "charlie")

	bobReceived := make(chan *pubsub.Message, 1)
	charlieReceived := make(chan *pubsub.Message, 1)

	subBob, _ := ps.Subscribe(ctx, pubsub.Topics.User(bobID.String()), func(ctx context.Context, msg *pubsub.Message) {
		bobReceived <- msg
	})
	defer func() { _ = subBob.Unsubscribe() }()

	subCharlie, _ := ps.Subscribe(ctx, pubsub.Topics.User(charlieID.String()), func(ctx context.Context, msg *pubsub.Message) {
		charlieReceived <- msg
	})
	defer func() { _ = subCharlie.Unsubscribe() }()

	// Alice mutes
	sigCtx := &SignalingContext{UserID: aliceID, Username: "alice"}
	payload, _ := json.Marshal(map[string]interface{}{
		"room_id": roomID.String(),
		"kind":    "audio",
		"muted":   true,
	})
	err := handler.HandleMuteUpdate(ctx, sigCtx, payload)
	require.NoError(t, err)

	// Both Bob and Charlie should receive it
	select {
	case <-bobReceived:
		// ok
	case <-time.After(200 * time.Millisecond):
		t.Error("Bob did not receive mute update")
	}

	select {
	case <-charlieReceived:
		// ok
	case <-time.After(200 * time.Millisecond):
		t.Error("Charlie did not receive mute update")
	}
}
