package webrtc

import (
	"context"
	"encoding/json"
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
// until an offer is sent (simulating the behavior when LocalDescription is not yet set).
func TestSFUParticipant_CandidateBuffering(t *testing.T) {
	// Setup dependencies
	ps := pubsub.NewMemoryPubSub()
	defer func() { _ = ps.Close() }()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	sfu := NewSFU(&SFUConfig{}, ps, logger)
	roomID := uuid.New()
	room := sfu.GetOrCreateRoom(roomID)

	// Create a real PeerConnection (cheapest way to satisfy struct)
	api := webrtc.NewAPI()
	pc, err := api.NewPeerConnection(webrtc.Configuration{})
	require.NoError(t, err)
	defer func() { _ = pc.Close() }()

	params := webrtc.ICECandidateInit{
		Candidate:     "candidate:1 1 UDP 123456 192.168.1.1 12345 typ host",
		SDPMid:        func(s string) *string { return &s }("audio"),
		SDPMLineIndex: func(i uint16) *uint16 { return &i }(0),
	}
	candidate := &webrtc.ICECandidate{
		Foundation: "1",
		Priority:   1,
		IP:         "192.168.1.1",
		Protocol:   webrtc.ICEProtocolUdp,
		Port:       12345,
		Typ:        webrtc.ICECandidateTypeHost,
		Component:  1,
	}

	// Manually construct participant
	userID := uuid.New()
	p := &SFUParticipant{
		UserID:   userID,
		Username: "test-user",
		pc:       pc,
		room:     room,
		sfu:      s,
		logger:   logger,
		ctx:      context.Background(),
		cancel:   func() {},
	}

	// Sub to user topic to catch emitted candidates
	received := make(chan *pubsub.Message, 10)
	sub, _ := ps.Subscribe(context.Background(), pubsub.Topics.User(userID.String()), func(ctx context.Context, msg *pubsub.Message) {
		received <- msg
	})
	defer func() { _ = sub.Unsubscribe() }()

	// 1. Send Candidate - Should be BUFFERED because CurrentLocalDescription is nil
	assert.Nil(t, pc.CurrentLocalDescription())
	p.sendICECandidate(context.Background(), candidate)

	// Verify NO message received
	select {
	case <-received:
		t.Fatal("candidate should have been buffered, not emitted")
	case <-time.After(50 * time.Millisecond):
		// Correct
	}

	// Verify it's in the buffer (accessing private field via same-package test)
	p.mu.Lock()
	assert.Len(t, p.pendingCandidates, 1)
	p.mu.Unlock()

	// 2. Send Offer - Should FLUSH buffer
	// We don't actually need to set LocalDescription on PC for this test to pass,
	// because `sendOffer` manually checks `pendingCandidates` and flushes them.
	// But in real flow, CreateOffer would have set it.
	// Here we just test that `sendOffer` triggers the flush.
	p.sendOffer(context.Background(), "mock-sdp")

	// Verify we get 2 signals: 1 offer, 1 candidate
	signals := 0
	for signals < 2 {
		select {
		case msg := <-received:
			if msg.Type == "sfu.offer" {
				signals++
			} else if msg.Type == "sfu.candidate" {
				signals++
				// Verify candidate content
				var payload map[string]interface{}
				_ = json.Unmarshal(msg.Payload, &payload)
				candMap, ok := payload["candidate"].(map[string]interface{})
				require.True(t, ok)
				assert.Contains(t, candMap["candidate"], "candidate:1")
			}
		case <-time.After(200 * time.Millisecond):
			t.Fatal("timed out waiting for offer and candidate flush")
		}
	}

	// Buffer should be empty now
	p.mu.Lock()
	assert.Len(t, p.pendingCandidates, 0)
	p.mu.Unlock()
}

// TestSFU_Concurrency_RoomMap verifies thread safety of the room map
func TestSFU_Concurrency_RoomMap(t *testing.T) {
	ps := pubsub.NewMemoryPubSub()
	defer func() { _ = ps.Close() }()
	sfu := NewSFU(&SFUConfig{}, ps, slog.New(slog.NewTextHandler(os.Stdout, nil)))

	var wg sync.WaitGroup
	roomID := uuid.New()

	// 100 goroutines trying to get/create/delete same room
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if i%3 == 0 {
				sfu.GetOrCreateRoom(roomID)
			} else if i%3 == 1 {
				sfu.GetRoom(roomID)
			} else {
				// Deletes might race with creates, effectively testing lock contention
				sfu.DeleteRoom(roomID)
			}
		}(i)
	}
	wg.Wait()
}

// TestSFURoom_ParticipantMap_Concurrency verifies thread safety of participant map
func TestSFURoom_ParticipantMap_Concurrency(t *testing.T) {
	ps := pubsub.NewMemoryPubSub()
	defer func() { _ = ps.Close() }()
	sfu := NewSFU(&SFUConfig{}, ps, slog.New(slog.NewTextHandler(os.Stdout, nil)))
	room := sfu.GetOrCreateRoom(uuid.New())

	var wg sync.WaitGroup

	// 100 goroutines adding/removing participants
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			pid := uuid.New()
			p := &SFUParticipant{UserID: pid}

			room.AddParticipant(p)
			_ = room.GetParticipant(pid)
			_ = room.ParticipantCount()
			_ = room.GetParticipantList()
			room.RemoveParticipant(pid)
		}(i)
	}
	wg.Wait()
}
