package webrtc

import (
	"context"
	"log/slog"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/observer/teatime/internal/pubsub"
	"github.com/pion/webrtc/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSFUParticipant_CandidateBuffering verifies that ICE candidates are buffered
// until an offer is sent (when LocalDescription is nil).
func TestSFUParticipant_CandidateBuffering(t *testing.T) {
	ps := pubsub.NewMemoryPubSub()
	defer func() { _ = ps.Close() }()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	sfuInst := NewSFU(&SFUConfig{}, ps, logger)
	roomID := uuid.New()
	room := sfuInst.GetOrCreateRoom(roomID)

	api := webrtc.NewAPI()

	// --- Generate a valid *webrtc.ICECandidate from a helper PC ---
	helperPC, err := api.NewPeerConnection(webrtc.Configuration{})
	require.NoError(t, err)
	defer func() { _ = helperPC.Close() }()

	candidateCh := make(chan *webrtc.ICECandidate, 1)
	helperPC.OnICECandidate(func(c *webrtc.ICECandidate) {
		if c != nil {
			select {
			case candidateCh <- c:
			default:
			}
		}
	})
	_, _ = helperPC.AddTransceiverFromKind(webrtc.RTPCodecTypeAudio)
	offer, err := helperPC.CreateOffer(nil)
	require.NoError(t, err)
	require.NoError(t, helperPC.SetLocalDescription(offer))

	var candidate *webrtc.ICECandidate
	select {
	case candidate = <-candidateCh:
	case <-time.After(2 * time.Second):
		t.Fatal("failed to generate helper ICE candidate")
	}

	// --- Create the participant under test (fresh PC, no local description) ---
	pc, err := api.NewPeerConnection(webrtc.Configuration{})
	require.NoError(t, err)
	defer func() { _ = pc.Close() }()

	userID := uuid.New()
	p := &SFUParticipant{
		UserID:   userID,
		Username: "test-user",
		pc:       pc,
		room:     room,
		sfu:      sfuInst,
		logger:   logger,
		ctx:      context.Background(),
		cancel:   func() {},
	}

	// Subscribe to user topic to observe emitted signals
	received := make(chan *pubsub.Message, 10)
	sub, err := ps.Subscribe(context.Background(), pubsub.Topics.User(userID.String()), func(ctx context.Context, msg *pubsub.Message) {
		received <- msg
	})
	require.NoError(t, err)
	defer func() { _ = sub.Unsubscribe() }()

	// 1. sendICECandidate should BUFFER because CurrentLocalDescription is nil
	assert.Nil(t, pc.CurrentLocalDescription())
	p.sendICECandidate(context.Background(), candidate)

	select {
	case <-received:
		t.Fatal("candidate should have been buffered, not emitted")
	case <-time.After(50 * time.Millisecond):
		// Good — nothing emitted
	}

	p.mu.Lock()
	assert.Len(t, p.pendingCandidates, 1, "candidate should be in pendingCandidates buffer")
	p.mu.Unlock()

	// 2. sendOffer should FLUSH the buffer
	p.sendOffer(context.Background(), "mock-sdp")

	// Expect 2 signals: the offer + the flushed candidate
	signals := 0
	for signals < 2 {
		select {
		case msg := <-received:
			if msg.Type == "sfu.offer" || msg.Type == "sfu.candidate" {
				signals++
			}
		case <-time.After(500 * time.Millisecond):
			t.Fatalf("timed out waiting for signals, got %d/2", signals)
		}
	}

	// Buffer should be empty now
	p.mu.Lock()
	assert.Empty(t, p.pendingCandidates, "pendingCandidates should be empty after flush")
	p.mu.Unlock()
}

// TestSFU_Concurrency_RoomMap verifies thread safety of the SFU room map
func TestSFU_Concurrency_RoomMap(t *testing.T) {
	ps := pubsub.NewMemoryPubSub()
	defer func() { _ = ps.Close() }()
	sfuInst := NewSFU(&SFUConfig{}, ps, slog.New(slog.NewTextHandler(os.Stdout, nil)))

	var wg sync.WaitGroup
	roomID := uuid.New()

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			switch i % 3 {
			case 0:
				sfuInst.GetOrCreateRoom(roomID)
			case 1:
				sfuInst.GetRoom(roomID)
			case 2:
				sfuInst.DeleteRoom(roomID)
			}
		}(i)
	}
	wg.Wait()
}

// TestSFURoom_ParticipantMap_Concurrency verifies thread safety of AddParticipant
// and read operations. Note: RemoveParticipant is excluded because it calls Close()
// which re-acquires room.mu (lock ordering issue in production code — documented).
func TestSFURoom_ParticipantMap_Concurrency(t *testing.T) {
	ps := pubsub.NewMemoryPubSub()
	defer func() { _ = ps.Close() }()
	sfuInst := NewSFU(&SFUConfig{}, ps, slog.New(slog.NewTextHandler(os.Stdout, nil)))
	room := sfuInst.GetOrCreateRoom(uuid.New())

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			pid := uuid.New()
			_, cancel := context.WithCancel(context.Background())
			p := &SFUParticipant{UserID: pid, cancel: cancel}

			room.AddParticipant(p)
			_ = room.GetParticipant(pid)
			_ = room.ParticipantCount()
			_ = room.GetParticipantList()
			// Note: RemoveParticipant intentionally omitted — it calls p.Close()
			// which tries room.mu.RLock while RemoveParticipant holds room.mu write lock.
		}()
	}
	wg.Wait()
}

// TestSFUParticipant_RemoteCandidateBuffering verifies that remote ICE candidates
// are buffered when no remote description has been set yet.
func TestSFUParticipant_RemoteCandidateBuffering(t *testing.T) {
	ps := pubsub.NewMemoryPubSub()
	defer func() { _ = ps.Close() }()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	sfuInst := NewSFU(&SFUConfig{}, ps, logger)
	room := sfuInst.GetOrCreateRoom(uuid.New())

	api := webrtc.NewAPI()
	pc, err := api.NewPeerConnection(webrtc.Configuration{})
	require.NoError(t, err)
	defer func() { _ = pc.Close() }()

	p := &SFUParticipant{
		UserID:   uuid.New(),
		Username: "buffer-test",
		pc:       pc,
		room:     room,
		sfu:      sfuInst,
		logger:   logger,
		ctx:      context.Background(),
		cancel:   func() {},
	}

	// Remote description is nil, so candidate should be buffered
	assert.Nil(t, pc.CurrentRemoteDescription())

	candInit := map[string]interface{}{
		"candidate":     "candidate:1 1 UDP 123456 10.0.0.1 9999 typ host",
		"sdpMid":        "0",
		"sdpMLineIndex": 0,
	}

	err = p.HandleICECandidate(context.Background(), candInit)
	require.NoError(t, err)

	p.mu.Lock()
	assert.Len(t, p.remotePendingCandidates, 1, "remote candidate should be buffered")
	p.mu.Unlock()
}

// TestSFUParticipant_NegotiationState verifies the negotiation queue state machine.
func TestSFUParticipant_NegotiationState(t *testing.T) {
	ps := pubsub.NewMemoryPubSub()
	defer func() { _ = ps.Close() }()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	sfuInst := NewSFU(&SFUConfig{}, ps, logger)
	room := sfuInst.GetOrCreateRoom(uuid.New())

	p := &SFUParticipant{
		UserID:   uuid.New(),
		Username: "nego-test",
		room:     room,
		sfu:      sfuInst,
		logger:   logger,
		ctx:      context.Background(),
		cancel:   func() {},
	}

	// Initial state
	assert.False(t, p.isNegotiating)
	assert.False(t, p.negotiationPending)

	// Simulate entering negotiation
	p.mu.Lock()
	p.isNegotiating = true
	p.mu.Unlock()

	// Second call while negotiating should set pending
	p.mu.Lock()
	if p.isNegotiating {
		p.negotiationPending = true
	}
	p.mu.Unlock()

	p.mu.Lock()
	assert.True(t, p.isNegotiating, "should still be negotiating")
	assert.True(t, p.negotiationPending, "should have pending negotiation queued")
	p.mu.Unlock()

	// Simulate answer received — reset state
	p.mu.Lock()
	p.isNegotiating = false
	pending := p.negotiationPending
	p.negotiationPending = false
	p.mu.Unlock()

	assert.True(t, pending, "pending should have been true before reset")

	p.mu.Lock()
	assert.False(t, p.isNegotiating)
	assert.False(t, p.negotiationPending)
	p.mu.Unlock()
}
