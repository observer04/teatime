package webrtc

import (
	"context"
	"log/slog"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/observer/teatime/internal/pubsub"
)

func TestRoom_AddRemoveParticipant(t *testing.T) {
	room := NewRoom(uuid.New())

	userID1 := uuid.New()
	userID2 := uuid.New()

	// Add first participant
	p1 := room.AddParticipant(userID1, "alice")
	if p1.Username != "alice" {
		t.Errorf("got username %q, want %q", p1.Username, "alice")
	}

	if room.ParticipantCount() != 1 {
		t.Errorf("got count %d, want 1", room.ParticipantCount())
	}

	// Add second participant
	room.AddParticipant(userID2, "bob")
	if room.ParticipantCount() != 2 {
		t.Errorf("got count %d, want 2", room.ParticipantCount())
	}

	// Remove first participant
	room.RemoveParticipant(userID1)
	if room.ParticipantCount() != 1 {
		t.Errorf("got count %d after remove, want 1", room.ParticipantCount())
	}

	// Check remaining participants
	participants := room.GetParticipants()
	if len(participants) != 1 {
		t.Fatalf("got %d participants, want 1", len(participants))
	}
	if participants[0].Username != "bob" {
		t.Errorf("remaining participant is %q, want bob", participants[0].Username)
	}
}

func TestManager_GetOrCreateRoom(t *testing.T) {
	ps := pubsub.NewMemoryPubSub()
	defer ps.Close()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	cfg := &Config{
		STUNURLs: []string{"stun:stun.l.google.com:19302"},
	}

	mgr := NewManager(cfg, ps, logger)

	roomID := uuid.New()

	// First call should create
	room1 := mgr.GetOrCreateRoom(roomID)
	if room1 == nil {
		t.Fatal("GetOrCreateRoom returned nil")
	}
	if room1.ID != roomID {
		t.Errorf("room ID %v != expected %v", room1.ID, roomID)
	}

	// Second call should return same room
	room2 := mgr.GetOrCreateRoom(roomID)
	if room1 != room2 {
		t.Error("GetOrCreateRoom returned different room")
	}
}

func TestManager_JoinLeaveCall(t *testing.T) {
	ps := pubsub.NewMemoryPubSub()
	defer ps.Close()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	cfg := &Config{
		STUNURLs: []string{"stun:stun.l.google.com:19302"},
	}

	mgr := NewManager(cfg, ps, logger)
	ctx := context.Background()

	roomID := uuid.New()
	userID := uuid.New()

	// Join call
	room, err := mgr.JoinCall(ctx, roomID, userID, "testuser")
	if err != nil {
		t.Fatalf("JoinCall failed: %v", err)
	}

	if room.ParticipantCount() != 1 {
		t.Errorf("participant count %d, want 1", room.ParticipantCount())
	}

	// Verify room is tracked
	rooms := mgr.GetActiveRooms()
	if len(rooms) != 1 {
		t.Errorf("active rooms count %d, want 1", len(rooms))
	}

	// Leave call
	mgr.LeaveCall(ctx, roomID, userID, "testuser")

	// Room should be deleted when empty
	if mgr.GetRoom(roomID) != nil {
		t.Error("room should be deleted after last participant leaves")
	}

	rooms = mgr.GetActiveRooms()
	if len(rooms) != 0 {
		t.Errorf("active rooms count %d, want 0", len(rooms))
	}
}

func TestConfig_GetICEServers(t *testing.T) {
	tests := []struct {
		name     string
		cfg      Config
		wantLen  int
		wantTURN bool
	}{
		{
			name:    "STUN only",
			cfg:     Config{STUNURLs: []string{"stun:stun.google.com:19302"}},
			wantLen: 1,
		},
		{
			name: "STUN and TURN",
			cfg: Config{
				STUNURLs:     []string{"stun:stun.google.com:19302"},
				TURNURLs:     []string{"turn:turn.example.com:3478"},
				TURNUsername: "user",
				TURNPassword: "pass",
			},
			wantLen:  2,
			wantTURN: true,
		},
		{
			name: "TURN without credentials ignored",
			cfg: Config{
				STUNURLs: []string{"stun:stun.google.com:19302"},
				TURNURLs: []string{"turn:turn.example.com:3478"},
				// No username/password
			},
			wantLen: 1, // Only STUN
		},
		{
			name:    "Empty config",
			cfg:     Config{},
			wantLen: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			servers := tt.cfg.GetICEServers()
			if len(servers) != tt.wantLen {
				t.Errorf("got %d servers, want %d", len(servers), tt.wantLen)
			}

			if tt.wantTURN && len(servers) >= 2 {
				turnServer := servers[1]
				if turnServer.Username != tt.cfg.TURNUsername {
					t.Errorf("TURN username %q != %q", turnServer.Username, tt.cfg.TURNUsername)
				}
			}
		})
	}
}
