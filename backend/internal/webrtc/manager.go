// Package webrtc provides WebRTC video calling functionality using Pion SFU.
// This is implemented as a package within the main application (modular monolith)
// to share auth and HTTP context with the rest of the server.
package webrtc

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"

	"github.com/google/uuid"
	"github.com/observer/teatime/internal/pubsub"
)

// ICEServer represents a STUN/TURN server configuration
type ICEServer struct {
	URLs       []string `json:"urls"`
	Username   string   `json:"username,omitempty"`
	Credential string   `json:"credential,omitempty"`
}

// Config holds WebRTC-related configuration
type Config struct {
	STUNURLs     []string // e.g., ["stun:stun.l.google.com:19302"]
	TURNURLs     []string // e.g., ["turn:your-server:3478"]
	TURNUsername string
	TURNPassword string
}

// GetICEServers returns the ICE server configuration for clients
func (c *Config) GetICEServers() []ICEServer {
	servers := make([]ICEServer, 0, 2)

	if len(c.STUNURLs) > 0 {
		servers = append(servers, ICEServer{URLs: c.STUNURLs})
	}

	if len(c.TURNURLs) > 0 && c.TURNUsername != "" {
		servers = append(servers, ICEServer{
			URLs:       c.TURNURLs,
			Username:   c.TURNUsername,
			Credential: c.TURNPassword,
		})
	}

	return servers
}

// Participant represents a user in a call
type Participant struct {
	UserID   uuid.UUID `json:"user_id"`
	Username string    `json:"username"`
	// PeerConnection will be added when Pion is integrated
}

// Room represents an active video call
type Room struct {
	ID           uuid.UUID `json:"id"` // Same as conversation ID
	CallID       uuid.UUID `json:"call_id"` // Reference to call_logs entry
	Participants map[uuid.UUID]*Participant
	mu           sync.RWMutex
	createdAt    int64
}

// NewRoom creates a new call room
func NewRoom(id uuid.UUID) *Room {
	return &Room{
		ID:           id,
		Participants: make(map[uuid.UUID]*Participant),
	}
}

// SetCallID sets the database call log ID
func (r *Room) SetCallID(callID uuid.UUID) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.CallID = callID
}

// GetCallID returns the database call log ID
func (r *Room) GetCallID() uuid.UUID {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.CallID
}

// AddParticipant adds a user to the room
func (r *Room) AddParticipant(userID uuid.UUID, username string) *Participant {
	r.mu.Lock()
	defer r.mu.Unlock()

	p := &Participant{
		UserID:   userID,
		Username: username,
	}
	r.Participants[userID] = p
	return p
}

// RemoveParticipant removes a user from the room
func (r *Room) RemoveParticipant(userID uuid.UUID) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.Participants, userID)
}

// GetParticipants returns a copy of the participants list
func (r *Room) GetParticipants() []Participant {
	r.mu.RLock()
	defer r.mu.RUnlock()

	participants := make([]Participant, 0, len(r.Participants))
	for _, p := range r.Participants {
		participants = append(participants, *p)
	}
	return participants
}

// ParticipantCount returns the number of participants
func (r *Room) ParticipantCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.Participants)
}

// Manager handles WebRTC call rooms
type Manager struct {
	rooms  map[uuid.UUID]*Room
	mu     sync.RWMutex
	config *Config
	pubsub pubsub.PubSub
	logger *slog.Logger
}

// NewManager creates a new WebRTC manager
func NewManager(cfg *Config, ps pubsub.PubSub, logger *slog.Logger) *Manager {
	return &Manager{
		rooms:  make(map[uuid.UUID]*Room),
		config: cfg,
		pubsub: ps,
		logger: logger,
	}
}

// GetOrCreateRoom gets an existing room or creates a new one
func (m *Manager) GetOrCreateRoom(roomID uuid.UUID) *Room {
	m.mu.Lock()
	defer m.mu.Unlock()

	if room, ok := m.rooms[roomID]; ok {
		return room
	}

	room := NewRoom(roomID)
	m.rooms[roomID] = room
	return room
}

// GetRoom returns a room if it exists
func (m *Manager) GetRoom(roomID uuid.UUID) *Room {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.rooms[roomID]
}

// DeleteRoom removes an empty room
func (m *Manager) DeleteRoom(roomID uuid.UUID) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.rooms, roomID)
}

// JoinCall adds a user to a call and notifies other participants
func (m *Manager) JoinCall(ctx context.Context, roomID, userID uuid.UUID, username string) (*Room, error) {
	room := m.GetOrCreateRoom(roomID)
	
	// Get existing participants before adding the new one
	existingParticipants := room.GetParticipants()
	
	room.AddParticipant(userID, username)

	// Notify other participants via pubsub (send to each user's topic)
	event := CallParticipantEvent{
		RoomID:   roomID,
		UserID:   userID,
		Username: username,
		Action:   "joined",
	}

	payloadBytes, _ := json.Marshal(event)
	
	// Send to each existing participant's user topic
	for _, p := range existingParticipants {
		if p.UserID == userID {
			continue // Skip the user who just joined
		}
		msg := &pubsub.Message{
			Topic:   pubsub.Topics.User(p.UserID.String()),
			Type:    EventTypeCallParticipantJoined,
			Payload: payloadBytes,
		}
		m.pubsub.Publish(ctx, msg.Topic, msg)
	}

	m.logger.Info("user joined call", "room_id", roomID, "user_id", userID)
	return room, nil
}

// LeaveCall removes a user from a call
func (m *Manager) LeaveCall(ctx context.Context, roomID, userID uuid.UUID, username string) {
	room := m.GetRoom(roomID)
	if room == nil {
		return
	}

	// Get remaining participants before removing this user
	remainingParticipants := room.GetParticipants()
	
	room.RemoveParticipant(userID)

	// Notify remaining participants via pubsub (send to each user's topic)
	event := CallParticipantEvent{
		RoomID:   roomID,
		UserID:   userID,
		Username: username,
		Action:   "left",
	}

	payloadBytes, _ := json.Marshal(event)
	
	// Send to each remaining participant's user topic
	for _, p := range remainingParticipants {
		if p.UserID == userID {
			continue // Skip the user who is leaving
		}
		msg := &pubsub.Message{
			Topic:   pubsub.Topics.User(p.UserID.String()),
			Type:    EventTypeCallParticipantLeft,
			Payload: payloadBytes,
		}
		m.pubsub.Publish(ctx, msg.Topic, msg)
	}

	// Clean up empty rooms
	if room.ParticipantCount() == 0 {
		m.DeleteRoom(roomID)
		m.logger.Info("call ended (no participants)", "room_id", roomID)
	}

	m.logger.Info("user left call", "room_id", roomID, "user_id", userID)
}

// GetConfig returns the ICE server configuration
func (m *Manager) GetConfig() *Config {
	return m.config
}

// GetActiveRooms returns a list of active room IDs (for monitoring)
func (m *Manager) GetActiveRooms() []uuid.UUID {
	m.mu.RLock()
	defer m.mu.RUnlock()

	rooms := make([]uuid.UUID, 0, len(m.rooms))
	for id := range m.rooms {
		rooms = append(rooms, id)
	}
	return rooms
}
