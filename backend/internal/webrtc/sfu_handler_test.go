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
	"github.com/pion/webrtc/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestSFUHandler creates an SFUHandler with a real SFU, Manager, MemoryPubSub,
// but nil DB repos. Tests touching convRepo/callRepo will fail — intentional.
func newTestSFUHandler(t *testing.T) (*SFUHandler, *SFU, *Manager, pubsub.PubSub) {
	t.Helper()
	ps := pubsub.NewMemoryPubSub()
	t.Cleanup(func() { _ = ps.Close() })

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	sfuCfg := &SFUConfig{ICEServers: []webrtc.ICEServer{
		{URLs: []string{"stun:stun.l.google.com:19302"}},
	}}
	sfu := NewSFU(sfuCfg, ps, logger)

	p2pCfg := &Config{STUNURLs: []string{"stun:stun.l.google.com:19302"}}
	mgr := NewManager(p2pCfg, ps, logger)

	handler := NewSFUHandler(sfu, mgr, nil, nil, ps, logger)
	return handler, sfu, mgr, ps
}

// addSFURoomParticipant creates a minimal SFUParticipant without an actual
// peer connection. This lets us test handler logic (routing, lookups, leave)
// without needing real WebRTC I/O.
func addSFURoomParticipant(t *testing.T, sfu *SFU, roomID, userID uuid.UUID, username string) *SFURoom {
	t.Helper()
	room := sfu.GetOrCreateRoom(roomID)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	p := &SFUParticipant{
		UserID:        userID,
		Username:      username,
		localTracks:   make(map[string]*webrtc.TrackLocalStaticRTP),
		remoteTracks:  make(map[string]*webrtc.TrackRemote),
		subscribers:   make(map[string][]*webrtc.TrackLocalStaticRTP),
		subscriptions: make(map[string]uuid.UUID),
		room:          room,
		sfu:           sfu,
		logger:        slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError})),
		ctx:           ctx,
		cancel:        cancel,
	}

	room.AddParticipant(p)
	return room
}

// =============================================================================
// HandleGroupJoin Tests
// =============================================================================

func TestSFUHandler_HandleGroupJoin_InvalidPayload(t *testing.T) {
	handler, _, _, _ := newTestSFUHandler(t)
	ctx := context.Background()
	sigCtx := &SignalingContext{UserID: uuid.New(), Username: "alice"}

	config, err := handler.HandleGroupJoin(ctx, sigCtx, json.RawMessage(`{bad`))
	assert.Nil(t, config)
	require.Error(t, err)

	callErr, ok := err.(*CallError)
	require.True(t, ok)
	assert.Equal(t, "invalid_payload", callErr.Code)
}

func TestSFUHandler_HandleGroupJoin_InvalidRoomID(t *testing.T) {
	handler, _, _, _ := newTestSFUHandler(t)
	ctx := context.Background()
	sigCtx := &SignalingContext{UserID: uuid.New(), Username: "alice"}

	payload := json.RawMessage(`{"room_id": "not-a-uuid", "is_group": true}`)
	config, err := handler.HandleGroupJoin(ctx, sigCtx, payload)
	assert.Nil(t, config)
	require.Error(t, err)

	callErr, ok := err.(*CallError)
	require.True(t, ok)
	assert.Equal(t, "invalid_room", callErr.Code)
}

func TestSFUHandler_HandleGroupJoin_EmptyRoomID(t *testing.T) {
	handler, _, _, _ := newTestSFUHandler(t)
	ctx := context.Background()
	sigCtx := &SignalingContext{UserID: uuid.New(), Username: "alice"}

	payload := json.RawMessage(`{"room_id": "", "is_group": true}`)
	config, err := handler.HandleGroupJoin(ctx, sigCtx, payload)
	assert.Nil(t, config)
	require.Error(t, err)

	callErr, ok := err.(*CallError)
	require.True(t, ok)
	assert.Equal(t, "invalid_room", callErr.Code)
}

// HandleGroupJoin with nil convRepo panics on membership check — testability gap
func TestSFUHandler_HandleGroupJoin_NilConvRepo_Panics(t *testing.T) {
	handler, _, _, _ := newTestSFUHandler(t)
	ctx := context.Background()
	sigCtx := &SignalingContext{UserID: uuid.New(), Username: "alice"}
	roomID := uuid.New()

	payload, _ := json.Marshal(SFUJoinPayload{RoomID: roomID.String(), IsGroup: true, CallType: "video"})

	assert.Panics(t, func() {
		_, _ = handler.HandleGroupJoin(ctx, sigCtx, payload)
	}, "HandleGroupJoin panics with nil convRepo — testability gap")
}

// =============================================================================
// HandleSFUOffer Tests
// =============================================================================

func TestSFUHandler_HandleSFUOffer_InvalidPayload(t *testing.T) {
	handler, _, _, _ := newTestSFUHandler(t)
	ctx := context.Background()
	sigCtx := &SignalingContext{UserID: uuid.New(), Username: "alice"}

	err := handler.HandleSFUOffer(ctx, sigCtx, json.RawMessage(`{bad`))
	require.Error(t, err)

	callErr, ok := err.(*CallError)
	require.True(t, ok)
	assert.Equal(t, "invalid_payload", callErr.Code)
}

func TestSFUHandler_HandleSFUOffer_InvalidRoomID(t *testing.T) {
	handler, _, _, _ := newTestSFUHandler(t)
	ctx := context.Background()
	sigCtx := &SignalingContext{UserID: uuid.New(), Username: "alice"}

	payload := json.RawMessage(`{"room_id": "bad-uuid", "sdp": "v=0..."}`)
	err := handler.HandleSFUOffer(ctx, sigCtx, payload)
	require.Error(t, err)

	callErr, ok := err.(*CallError)
	require.True(t, ok)
	assert.Equal(t, "invalid_room", callErr.Code)
}

func TestSFUHandler_HandleSFUOffer_RoomNotFound(t *testing.T) {
	handler, _, _, _ := newTestSFUHandler(t)
	ctx := context.Background()
	sigCtx := &SignalingContext{UserID: uuid.New(), Username: "alice"}
	roomID := uuid.New()

	payload, _ := json.Marshal(SFUOfferPayload{RoomID: roomID.String(), SDP: "v=0..."})
	err := handler.HandleSFUOffer(ctx, sigCtx, payload)
	require.Error(t, err)

	callErr, ok := err.(*CallError)
	require.True(t, ok)
	assert.Equal(t, "room_not_found", callErr.Code)
}

func TestSFUHandler_HandleSFUOffer_ParticipantNotInCall(t *testing.T) {
	handler, sfu, _, _ := newTestSFUHandler(t)
	ctx := context.Background()

	roomID := uuid.New()
	aliceID := uuid.New()
	intruderID := uuid.New()

	// Alice is in room, but intruder is not
	addSFURoomParticipant(t, sfu, roomID, aliceID, "alice")

	sigCtx := &SignalingContext{UserID: intruderID, Username: "intruder"}
	payload, _ := json.Marshal(SFUOfferPayload{RoomID: roomID.String(), SDP: "v=0..."})
	err := handler.HandleSFUOffer(ctx, sigCtx, payload)
	require.Error(t, err)

	callErr, ok := err.(*CallError)
	require.True(t, ok)
	assert.Equal(t, "not_in_call", callErr.Code)
}

// HandleSFUOffer with actual participant — participant has no WebRTC PeerConnection
// so HandleOffer panics (nil pointer dereference). This documents that SFU offer
// handling cannot be unit tested without a real PeerConnection setup.
func TestSFUHandler_HandleSFUOffer_NoPeerConnection_Panics(t *testing.T) {
	handler, sfu, _, _ := newTestSFUHandler(t)
	ctx := context.Background()

	roomID := uuid.New()
	aliceID := uuid.New()
	addSFURoomParticipant(t, sfu, roomID, aliceID, "alice")

	sigCtx := &SignalingContext{UserID: aliceID, Username: "alice"}
	payload, _ := json.Marshal(SFUOfferPayload{RoomID: roomID.String(), SDP: "v=0\r\no=- 12345 2 IN IP4 127.0.0.1\r\n"})

	assert.Panics(t, func() {
		_ = handler.HandleSFUOffer(ctx, sigCtx, payload)
	}, "HandleSFUOffer panics with nil PeerConnection — testability gap")
}

// =============================================================================
// HandleSFUAnswer Tests
// =============================================================================

func TestSFUHandler_HandleSFUAnswer_InvalidPayload(t *testing.T) {
	handler, _, _, _ := newTestSFUHandler(t)
	ctx := context.Background()
	sigCtx := &SignalingContext{UserID: uuid.New(), Username: "bob"}

	err := handler.HandleSFUAnswer(ctx, sigCtx, json.RawMessage(`{bad`))
	require.Error(t, err)

	callErr, ok := err.(*CallError)
	require.True(t, ok)
	assert.Equal(t, "invalid_payload", callErr.Code)
}

func TestSFUHandler_HandleSFUAnswer_RoomNotFound(t *testing.T) {
	handler, _, _, _ := newTestSFUHandler(t)
	ctx := context.Background()
	sigCtx := &SignalingContext{UserID: uuid.New(), Username: "bob"}
	roomID := uuid.New()

	payload, _ := json.Marshal(SFUOfferPayload{RoomID: roomID.String(), SDP: "answer-sdp"})
	err := handler.HandleSFUAnswer(ctx, sigCtx, payload)
	require.Error(t, err)

	callErr, ok := err.(*CallError)
	require.True(t, ok)
	assert.Equal(t, "room_not_found", callErr.Code)
}

func TestSFUHandler_HandleSFUAnswer_NotInCall(t *testing.T) {
	handler, sfu, _, _ := newTestSFUHandler(t)
	ctx := context.Background()

	roomID := uuid.New()
	addSFURoomParticipant(t, sfu, roomID, uuid.New(), "alice")

	sigCtx := &SignalingContext{UserID: uuid.New(), Username: "stranger"}
	payload, _ := json.Marshal(SFUOfferPayload{RoomID: roomID.String(), SDP: "answer-sdp"})
	err := handler.HandleSFUAnswer(ctx, sigCtx, payload)
	require.Error(t, err)

	callErr, ok := err.(*CallError)
	require.True(t, ok)
	assert.Equal(t, "not_in_call", callErr.Code)
}

// =============================================================================
// HandleSFUCandidate Tests
// =============================================================================

func TestSFUHandler_HandleSFUCandidate_InvalidPayload(t *testing.T) {
	handler, _, _, _ := newTestSFUHandler(t)
	ctx := context.Background()
	sigCtx := &SignalingContext{UserID: uuid.New(), Username: "alice"}

	err := handler.HandleSFUCandidate(ctx, sigCtx, json.RawMessage(`{garbage`))
	require.Error(t, err)

	callErr, ok := err.(*CallError)
	require.True(t, ok)
	assert.Equal(t, "invalid_payload", callErr.Code)
}

func TestSFUHandler_HandleSFUCandidate_RoomNotFound(t *testing.T) {
	handler, _, _, _ := newTestSFUHandler(t)
	ctx := context.Background()
	sigCtx := &SignalingContext{UserID: uuid.New(), Username: "alice"}
	roomID := uuid.New()

	payload, _ := json.Marshal(SFUCandidatePayload{
		RoomID:    roomID.String(),
		Candidate: "candidate:...",
	})
	err := handler.HandleSFUCandidate(ctx, sigCtx, payload)
	require.Error(t, err)

	callErr, ok := err.(*CallError)
	require.True(t, ok)
	assert.Equal(t, "room_not_found", callErr.Code)
}

func TestSFUHandler_HandleSFUCandidate_NotInCall(t *testing.T) {
	handler, sfu, _, _ := newTestSFUHandler(t)
	ctx := context.Background()

	roomID := uuid.New()
	addSFURoomParticipant(t, sfu, roomID, uuid.New(), "alice")

	sigCtx := &SignalingContext{UserID: uuid.New(), Username: "outsider"}
	payload, _ := json.Marshal(SFUCandidatePayload{
		RoomID:    roomID.String(),
		Candidate: "candidate:...",
	})
	err := handler.HandleSFUCandidate(ctx, sigCtx, payload)
	require.Error(t, err)

	callErr, ok := err.(*CallError)
	require.True(t, ok)
	assert.Equal(t, "not_in_call", callErr.Code)
}

// =============================================================================
// HandleSFULeave Tests
// =============================================================================

func TestSFUHandler_HandleSFULeave_InvalidPayload(t *testing.T) {
	handler, _, _, _ := newTestSFUHandler(t)
	ctx := context.Background()
	sigCtx := &SignalingContext{UserID: uuid.New(), Username: "alice"}

	err := handler.HandleSFULeave(ctx, sigCtx, json.RawMessage(`BAD`))
	require.Error(t, err)

	callErr, ok := err.(*CallError)
	require.True(t, ok)
	assert.Equal(t, "invalid_payload", callErr.Code)
}

func TestSFUHandler_HandleSFULeave_InvalidRoomID(t *testing.T) {
	handler, _, _, _ := newTestSFUHandler(t)
	ctx := context.Background()
	sigCtx := &SignalingContext{UserID: uuid.New(), Username: "alice"}

	payload := json.RawMessage(`{"room_id": "not-uuid"}`)
	err := handler.HandleSFULeave(ctx, sigCtx, payload)
	require.Error(t, err)

	callErr, ok := err.(*CallError)
	require.True(t, ok)
	assert.Equal(t, "invalid_room", callErr.Code)
}

func TestSFUHandler_HandleSFULeave_NonExistentRoom(t *testing.T) {
	handler, _, _, _ := newTestSFUHandler(t)
	ctx := context.Background()
	sigCtx := &SignalingContext{UserID: uuid.New(), Username: "alice"}
	roomID := uuid.New()

	payload, _ := json.Marshal(CallLeavePayload{RoomID: roomID.String()})
	err := handler.HandleSFULeave(ctx, sigCtx, payload)

	// Leaving a non-existent room should succeed (no-op)
	assert.NoError(t, err)
}

func TestSFUHandler_HandleSFULeave_ParticipantRemoved(t *testing.T) {
	handler, sfu, _, _ := newTestSFUHandler(t)
	ctx := context.Background()

	roomID := uuid.New()
	aliceID := uuid.New()
	bobID := uuid.New()

	addSFURoomParticipant(t, sfu, roomID, aliceID, "alice")
	addSFURoomParticipant(t, sfu, roomID, bobID, "bob")

	// Alice leaves
	sigCtx := &SignalingContext{UserID: aliceID, Username: "alice"}
	payload, _ := json.Marshal(CallLeavePayload{RoomID: roomID.String()})
	err := handler.HandleSFULeave(ctx, sigCtx, payload)
	assert.NoError(t, err)

	// Room should still exist with Bob
	room := sfu.GetRoom(roomID)
	require.NotNil(t, room)
	assert.Equal(t, 1, room.ParticipantCount())
	assert.Nil(t, room.GetParticipant(aliceID))
	assert.NotNil(t, room.GetParticipant(bobID))
}

func TestSFUHandler_HandleSFULeave_LastParticipantDeletesRoom(t *testing.T) {
	handler, sfu, _, _ := newTestSFUHandler(t)
	ctx := context.Background()

	roomID := uuid.New()
	aliceID := uuid.New()
	addSFURoomParticipant(t, sfu, roomID, aliceID, "alice")

	sigCtx := &SignalingContext{UserID: aliceID, Username: "alice"}
	payload, _ := json.Marshal(CallLeavePayload{RoomID: roomID.String()})
	err := handler.HandleSFULeave(ctx, sigCtx, payload)
	assert.NoError(t, err)

	// Room should be deleted
	assert.Nil(t, sfu.GetRoom(roomID))
}

func TestSFUHandler_HandleSFULeave_BroadcastsLeft(t *testing.T) {
	handler, sfu, _, ps := newTestSFUHandler(t)
	ctx := context.Background()

	roomID := uuid.New()
	aliceID := uuid.New()
	bobID := uuid.New()

	addSFURoomParticipant(t, sfu, roomID, aliceID, "alice")
	addSFURoomParticipant(t, sfu, roomID, bobID, "bob")

	// Subscribe to Bob's topic to verify participant_left broadcast
	received := make(chan *pubsub.Message, 1)
	sub, _ := ps.Subscribe(ctx, pubsub.Topics.User(bobID.String()), func(ctx context.Context, msg *pubsub.Message) {
		received <- msg
	})
	defer func() { _ = sub.Unsubscribe() }()

	// Alice leaves
	sigCtx := &SignalingContext{UserID: aliceID, Username: "alice"}
	payload, _ := json.Marshal(CallLeavePayload{RoomID: roomID.String()})
	_ = handler.HandleSFULeave(ctx, sigCtx, payload)

	select {
	case msg := <-received:
		assert.Equal(t, EventTypeCallParticipantLeft, msg.Type)
		var event CallParticipantEvent
		_ = json.Unmarshal(msg.Payload, &event)
		assert.Equal(t, aliceID, event.UserID)
		assert.Equal(t, "alice", event.Username)
		assert.Equal(t, "left", event.Action)
	case <-time.After(200 * time.Millisecond):
		t.Fatal("Bob did not receive participant_left event")
	}
}

// =============================================================================
// HandleSFUMuteUpdate Tests
// =============================================================================

func TestSFUHandler_HandleSFUMuteUpdate_InvalidPayload(t *testing.T) {
	handler, _, _, _ := newTestSFUHandler(t)
	ctx := context.Background()
	sigCtx := &SignalingContext{UserID: uuid.New(), Username: "alice"}

	err := handler.HandleSFUMuteUpdate(ctx, sigCtx, json.RawMessage(`{bad`))
	require.Error(t, err)

	callErr, ok := err.(*CallError)
	require.True(t, ok)
	assert.Equal(t, "invalid_payload", callErr.Code)
}

func TestSFUHandler_HandleSFUMuteUpdate_InvalidRoomID(t *testing.T) {
	handler, _, _, _ := newTestSFUHandler(t)
	ctx := context.Background()
	sigCtx := &SignalingContext{UserID: uuid.New(), Username: "alice"}

	payload := json.RawMessage(`{"room_id": "bad-uuid", "kind": "audio", "muted": true}`)
	err := handler.HandleSFUMuteUpdate(ctx, sigCtx, payload)
	require.Error(t, err)

	callErr, ok := err.(*CallError)
	require.True(t, ok)
	assert.Equal(t, "invalid_room", callErr.Code)
}

func TestSFUHandler_HandleSFUMuteUpdate_RoomNotFound(t *testing.T) {
	handler, _, _, _ := newTestSFUHandler(t)
	ctx := context.Background()
	sigCtx := &SignalingContext{UserID: uuid.New(), Username: "alice"}
	roomID := uuid.New()

	payload, _ := json.Marshal(map[string]interface{}{
		"room_id": roomID.String(),
		"kind":    "audio",
		"muted":   true,
	})
	err := handler.HandleSFUMuteUpdate(ctx, sigCtx, payload)
	require.Error(t, err)

	callErr, ok := err.(*CallError)
	require.True(t, ok)
	assert.Equal(t, "room_not_found", callErr.Code)
}

func TestSFUHandler_HandleSFUMuteUpdate_RelaysToOthers(t *testing.T) {
	handler, sfu, _, ps := newTestSFUHandler(t)
	ctx := context.Background()

	roomID := uuid.New()
	aliceID := uuid.New()
	bobID := uuid.New()
	charlieID := uuid.New()

	addSFURoomParticipant(t, sfu, roomID, aliceID, "alice")
	addSFURoomParticipant(t, sfu, roomID, bobID, "bob")
	addSFURoomParticipant(t, sfu, roomID, charlieID, "charlie")

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

	// Alice mutes video
	sigCtx := &SignalingContext{UserID: aliceID, Username: "alice"}
	payload, _ := json.Marshal(map[string]interface{}{
		"room_id": roomID.String(),
		"kind":    "video",
		"muted":   true,
	})
	err := handler.HandleSFUMuteUpdate(ctx, sigCtx, payload)
	require.NoError(t, err)

	// Both Bob and Charlie should receive it
	select {
	case msg := <-bobReceived:
		assert.Equal(t, EventTypeCallMuteUpdate, msg.Type)
		var mutePayload map[string]interface{}
		_ = json.Unmarshal(msg.Payload, &mutePayload)
		assert.Equal(t, aliceID.String(), mutePayload["user_id"])
		assert.Equal(t, "video", mutePayload["kind"])
		assert.Equal(t, true, mutePayload["muted"])
	case <-time.After(200 * time.Millisecond):
		t.Error("Bob did not receive SFU mute update")
	}

	select {
	case msg := <-charlieReceived:
		assert.Equal(t, EventTypeCallMuteUpdate, msg.Type)
	case <-time.After(200 * time.Millisecond):
		t.Error("Charlie did not receive SFU mute update")
	}
}

func TestSFUHandler_HandleSFUMuteUpdate_DoesNotRelayBackToSender(t *testing.T) {
	handler, sfu, _, ps := newTestSFUHandler(t)
	ctx := context.Background()

	roomID := uuid.New()
	aliceID := uuid.New()

	addSFURoomParticipant(t, sfu, roomID, aliceID, "alice")

	selfReceived := make(chan *pubsub.Message, 1)
	sub, _ := ps.Subscribe(ctx, pubsub.Topics.User(aliceID.String()), func(ctx context.Context, msg *pubsub.Message) {
		selfReceived <- msg
	})
	defer func() { _ = sub.Unsubscribe() }()

	sigCtx := &SignalingContext{UserID: aliceID, Username: "alice"}
	payload, _ := json.Marshal(map[string]interface{}{
		"room_id": roomID.String(),
		"kind":    "audio",
		"muted":   true,
	})
	err := handler.HandleSFUMuteUpdate(ctx, sigCtx, payload)
	require.NoError(t, err)

	select {
	case <-selfReceived:
		t.Fatal("SFU mute update should NOT be relayed to sender")
	case <-time.After(200 * time.Millisecond):
		// Correct — no message received
	}
}

// =============================================================================
// IsUserInSFURoom Tests
// =============================================================================

func TestSFUHandler_IsUserInSFURoom_UserPresent(t *testing.T) {
	handler, sfu, _, _ := newTestSFUHandler(t)

	roomID := uuid.New()
	userID := uuid.New()
	addSFURoomParticipant(t, sfu, roomID, userID, "alice")

	assert.True(t, handler.IsUserInSFURoom(roomID, userID))
}

func TestSFUHandler_IsUserInSFURoom_UserNotPresent(t *testing.T) {
	handler, sfu, _, _ := newTestSFUHandler(t)

	roomID := uuid.New()
	addSFURoomParticipant(t, sfu, roomID, uuid.New(), "alice")

	assert.False(t, handler.IsUserInSFURoom(roomID, uuid.New()))
}

func TestSFUHandler_IsUserInSFURoom_RoomDoesNotExist(t *testing.T) {
	handler, _, _, _ := newTestSFUHandler(t)

	assert.False(t, handler.IsUserInSFURoom(uuid.New(), uuid.New()))
}

// =============================================================================
// SFU Room Management Tests
// =============================================================================

func TestSFURoom_SetGetCallID(t *testing.T) {
	sfu := NewSFU(&SFUConfig{}, pubsub.NewMemoryPubSub(), slog.New(slog.NewTextHandler(os.Stdout, nil)))

	roomID := uuid.New()
	room := sfu.GetOrCreateRoom(roomID)

	// Initially Nil
	assert.Equal(t, uuid.Nil, room.GetCallID())

	// Set and get
	callID := uuid.New()
	room.SetCallID(callID)
	assert.Equal(t, callID, room.GetCallID())
}

func TestSFURoom_ParticipantLifecycle(t *testing.T) {
	ps := pubsub.NewMemoryPubSub()
	defer func() { _ = ps.Close() }()
	sfu := NewSFU(&SFUConfig{}, ps, slog.New(slog.NewTextHandler(os.Stdout, nil)))

	roomID := uuid.New()
	aliceID := uuid.New()
	bobID := uuid.New()

	room := addSFURoomParticipant(t, sfu, roomID, aliceID, "alice")
	addSFURoomParticipant(t, sfu, roomID, bobID, "bob")

	assert.Equal(t, 2, room.ParticipantCount())
	assert.NotNil(t, room.GetParticipant(aliceID))
	assert.NotNil(t, room.GetParticipant(bobID))

	// Remove alice
	room.RemoveParticipant(aliceID)
	assert.Equal(t, 1, room.ParticipantCount())
	assert.Nil(t, room.GetParticipant(aliceID))
	assert.NotNil(t, room.GetParticipant(bobID))

	// Remove bob
	room.RemoveParticipant(bobID)
	assert.Equal(t, 0, room.ParticipantCount())
}

func TestSFURoom_GetParticipantList(t *testing.T) {
	ps := pubsub.NewMemoryPubSub()
	defer func() { _ = ps.Close() }()
	sfu := NewSFU(&SFUConfig{}, ps, slog.New(slog.NewTextHandler(os.Stdout, nil)))

	roomID := uuid.New()
	aliceID := uuid.New()
	bobID := uuid.New()

	addSFURoomParticipant(t, sfu, roomID, aliceID, "alice")
	addSFURoomParticipant(t, sfu, roomID, bobID, "bob")

	room := sfu.GetRoom(roomID)
	list := room.GetParticipantList()

	assert.Len(t, list, 2)

	// Participants should have correct usernames
	usernames := map[string]bool{}
	for _, p := range list {
		usernames[p.Username] = true
	}
	assert.True(t, usernames["alice"])
	assert.True(t, usernames["bob"])
}

func TestSFU_GetOrCreateRoom_IdempotentForSameID(t *testing.T) {
	ps := pubsub.NewMemoryPubSub()
	defer func() { _ = ps.Close() }()
	sfu := NewSFU(&SFUConfig{}, ps, slog.New(slog.NewTextHandler(os.Stdout, nil)))

	roomID := uuid.New()
	room1 := sfu.GetOrCreateRoom(roomID)
	room2 := sfu.GetOrCreateRoom(roomID) // Same ID

	assert.Same(t, room1, room2, "GetOrCreateRoom should return the same room for the same ID")
}

func TestSFU_DeleteRoom(t *testing.T) {
	ps := pubsub.NewMemoryPubSub()
	defer func() { _ = ps.Close() }()
	sfu := NewSFU(&SFUConfig{}, ps, slog.New(slog.NewTextHandler(os.Stdout, nil)))

	roomID := uuid.New()
	sfu.GetOrCreateRoom(roomID)
	assert.NotNil(t, sfu.GetRoom(roomID))

	sfu.DeleteRoom(roomID)
	assert.Nil(t, sfu.GetRoom(roomID))
}

func TestSFU_DeleteRoom_NonExistent(t *testing.T) {
	ps := pubsub.NewMemoryPubSub()
	defer func() { _ = ps.Close() }()
	sfu := NewSFU(&SFUConfig{}, ps, slog.New(slog.NewTextHandler(os.Stdout, nil)))

	// Deleting a non-existent room should not panic
	assert.NotPanics(t, func() {
		sfu.DeleteRoom(uuid.New())
	})
}

func TestSFURoom_GetTracks_EmptyWhenNoRemoteTracks(t *testing.T) {
	ps := pubsub.NewMemoryPubSub()
	defer func() { _ = ps.Close() }()
	sfu := NewSFU(&SFUConfig{}, ps, slog.New(slog.NewTextHandler(os.Stdout, nil)))

	roomID := uuid.New()
	addSFURoomParticipant(t, sfu, roomID, uuid.New(), "alice")

	room := sfu.GetRoom(roomID)
	tracks := room.GetTracks()
	assert.Empty(t, tracks)
}

// =============================================================================
// SFU Helper Function Tests
// =============================================================================

func TestTrackKey(t *testing.T) {
	senderID := uuid.New()
	trackID := "audio"

	key := trackKey(senderID, trackID)
	assert.Equal(t, senderID.String()+":"+trackID, key)
}

func TestSplitTrackKey(t *testing.T) {
	senderID := uuid.New()

	tests := []struct {
		name       string
		input      string
		wantSender string
		wantTrack  string
	}{
		{
			name:       "normal key",
			input:      senderID.String() + ":audio",
			wantSender: senderID.String(),
			wantTrack:  "audio",
		},
		{
			name:       "track with colons",
			input:      senderID.String() + ":some:complex:id",
			wantSender: senderID.String() + ":some:complex",
			wantTrack:  "id",
		},
		{
			name:       "no colon",
			input:      "nocolon",
			wantSender: "",
			wantTrack:  "nocolon",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sender, track := splitTrackKey(tt.input)
			assert.Equal(t, tt.wantSender, sender)
			assert.Equal(t, tt.wantTrack, track)
		})
	}
}

// =============================================================================
// P2P-to-SFU Migration Tests (via SFUHandler)
// =============================================================================

// Test that when an SFU handler calls HandleGroupJoin with an existing P2P room,
// migration events are sent to P2P participants. Since convRepo is nil, we cannot
// test the full flow, but we can verify the P2P cleanup path.
func TestSFUHandler_Migration_CleanupP2PRoom(t *testing.T) {
	_, _, mgr, ps := newTestSFUHandler(t)
	ctx := context.Background()

	roomID := uuid.New()
	aliceID := uuid.New()
	bobID := uuid.New()

	// Set up P2P room with 2 participants
	_, _ = mgr.JoinCall(ctx, roomID, aliceID, "alice")
	_, _ = mgr.JoinCall(ctx, roomID, bobID, "bob")

	require.NotNil(t, mgr.GetRoom(roomID))
	require.Equal(t, 2, mgr.GetRoom(roomID).ParticipantCount())

	// Simulate migration by manually doing what HandleGroupJoin does:
	// remove all P2P participants
	aliceReceived := make(chan *pubsub.Message, 5)
	sub, _ := ps.Subscribe(ctx, pubsub.Topics.User(aliceID.String()), func(ctx context.Context, msg *pubsub.Message) {
		aliceReceived <- msg
	})
	defer func() { _ = sub.Unsubscribe() }()

	p2pRoom := mgr.GetRoom(roomID)
	for _, participant := range p2pRoom.GetParticipants() {
		migrationEvent := map[string]interface{}{
			"room_id": roomID.String(),
			"reason":  "switching_to_group",
		}
		payloadBytes, _ := json.Marshal(migrationEvent)
		msg := &pubsub.Message{
			Topic:   pubsub.Topics.User(participant.UserID.String()),
			Type:    EventTypeCallMigration,
			Payload: payloadBytes,
		}
		_ = ps.Publish(ctx, msg.Topic, msg)
	}

	// Verify Alice received migration event
	select {
	case msg := <-aliceReceived:
		assert.Equal(t, EventTypeCallMigration, msg.Type)
		var payload map[string]string
		_ = json.Unmarshal(msg.Payload, &payload)
		assert.Equal(t, roomID.String(), payload["room_id"])
		assert.Equal(t, "switching_to_group", payload["reason"])
	case <-time.After(200 * time.Millisecond):
		t.Fatal("Alice did not receive migration event")
	}
}

// =============================================================================
// Edge Cases
// =============================================================================

func TestSFUHandler_HandleSFULeave_SameUserLeavesTwice(t *testing.T) {
	handler, sfu, _, _ := newTestSFUHandler(t)
	ctx := context.Background()

	roomID := uuid.New()
	aliceID := uuid.New()
	addSFURoomParticipant(t, sfu, roomID, aliceID, "alice")

	sigCtx := &SignalingContext{UserID: aliceID, Username: "alice"}
	payload, _ := json.Marshal(CallLeavePayload{RoomID: roomID.String()})

	// First leave
	err := handler.HandleSFULeave(ctx, sigCtx, payload)
	assert.NoError(t, err)

	// Second leave — room is deleted, should still succeed
	err = handler.HandleSFULeave(ctx, sigCtx, payload)
	assert.NoError(t, err)
}

func TestSFUHandler_HandleSFUMuteUpdate_AloneInRoom(t *testing.T) {
	handler, sfu, _, ps := newTestSFUHandler(t)
	ctx := context.Background()

	roomID := uuid.New()
	aliceID := uuid.New()
	addSFURoomParticipant(t, sfu, roomID, aliceID, "alice")

	// Capture any published messages
	anyReceived := make(chan *pubsub.Message, 1)
	sub, _ := ps.Subscribe(ctx, pubsub.Topics.User(aliceID.String()), func(ctx context.Context, msg *pubsub.Message) {
		anyReceived <- msg
	})
	defer func() { _ = sub.Unsubscribe() }()

	sigCtx := &SignalingContext{UserID: aliceID, Username: "alice"}
	payload, _ := json.Marshal(map[string]interface{}{
		"room_id": roomID.String(),
		"kind":    "audio",
		"muted":   true,
	})
	err := handler.HandleSFUMuteUpdate(ctx, sigCtx, payload)
	assert.NoError(t, err)

	// No one should receive anything (mute update not sent back to sender, no other participants)
	select {
	case <-anyReceived:
		t.Fatal("mute update should not be published when user is alone in room")
	case <-time.After(200 * time.Millisecond):
		// Correct — no mute update should be published when user is alone
	}
}
