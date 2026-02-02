// Package webrtc provides WebRTC functionality for video/audio calls.
// This file implements the SFU (Selective Forwarding Unit) for group calls.
// The SFU receives media from each participant and forwards it to all others.
//
// Architecture:
// - For 1:1 calls: Use P2P mesh (existing handler.go/manager.go)
// - For group calls (3+ participants): Use SFU (this file)
//
// The SFU creates a server-side PeerConnection for each participant.
// When a participant sends media, the SFU forwards it to all other participants.
package webrtc

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"

	"github.com/google/uuid"
	"github.com/observer/teatime/internal/pubsub"
	"github.com/pion/webrtc/v3"
)

// SFU manages server-side WebRTC connections for group calls
type SFU struct {
	mu     sync.RWMutex
	rooms  map[uuid.UUID]*SFURoom
	config *SFUConfig
	pubsub pubsub.PubSub
	logger *slog.Logger
}

// SFUConfig holds configuration for the SFU
type SFUConfig struct {
	ICEServers []webrtc.ICEServer
}

// SFURoom represents a group call room managed by the SFU
type SFURoom struct {
	mu           sync.RWMutex
	ID           uuid.UUID
	participants map[uuid.UUID]*SFUParticipant
	logger       *slog.Logger
}

// SFUParticipant represents a participant in an SFU room
type SFUParticipant struct {
	mu           sync.RWMutex
	UserID       uuid.UUID
	Username     string
	pc           *webrtc.PeerConnection
	localTracks  map[string]*webrtc.TrackLocalStaticRTP // Tracks we're sending to this participant
	remoteTracks map[string]*webrtc.TrackRemote         // Tracks received from this participant
	room         *SFURoom
	sfu          *SFU
	logger       *slog.Logger
}

// TrackInfo describes a media track
type TrackInfo struct {
	ID       string `json:"id"`
	Kind     string `json:"kind"` // "audio" or "video"
	UserID   string `json:"user_id"`
	Username string `json:"username"`
}

// NewSFU creates a new SFU instance
func NewSFU(config *SFUConfig, ps pubsub.PubSub, logger *slog.Logger) *SFU {
	return &SFU{
		rooms:  make(map[uuid.UUID]*SFURoom),
		config: config,
		pubsub: ps,
		logger: logger.With("component", "sfu"),
	}
}

// GetRoom returns an SFU room if it exists
func (s *SFU) GetRoom(roomID uuid.UUID) *SFURoom {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.rooms[roomID]
}

// GetOrCreateRoom gets an existing room or creates a new one
func (s *SFU) GetOrCreateRoom(roomID uuid.UUID) *SFURoom {
	s.mu.Lock()
	defer s.mu.Unlock()

	if room, ok := s.rooms[roomID]; ok {
		return room
	}

	room := &SFURoom{
		ID:           roomID,
		participants: make(map[uuid.UUID]*SFUParticipant),
		logger:       s.logger.With("room_id", roomID),
	}
	s.rooms[roomID] = room
	s.logger.Info("created SFU room", "room_id", roomID)
	return room
}

// DeleteRoom removes an empty room
func (s *SFU) DeleteRoom(roomID uuid.UUID) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.rooms, roomID)
	s.logger.Info("deleted SFU room", "room_id", roomID)
}

// JoinRoom adds a participant to an SFU room and creates their PeerConnection
func (s *SFU) JoinRoom(ctx context.Context, roomID, userID uuid.UUID, username string) (*SFUParticipant, error) {
	room := s.GetOrCreateRoom(roomID)

	// Create WebRTC PeerConnection for this participant
	config := webrtc.Configuration{
		ICEServers: s.config.ICEServers,
	}

	pc, err := webrtc.NewPeerConnection(config)
	if err != nil {
		return nil, err
	}

	participant := &SFUParticipant{
		UserID:       userID,
		Username:     username,
		pc:           pc,
		localTracks:  make(map[string]*webrtc.TrackLocalStaticRTP),
		remoteTracks: make(map[string]*webrtc.TrackRemote),
		room:         room,
		sfu:          s,
		logger:       room.logger.With("user_id", userID, "username", username),
	}

	// Set up track handler - when this participant sends media, forward to others
	pc.OnTrack(func(remoteTrack *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
		participant.handleIncomingTrack(ctx, remoteTrack, receiver)
	})

	// Handle ICE candidates
	pc.OnICECandidate(func(candidate *webrtc.ICECandidate) {
		if candidate == nil {
			return
		}
		participant.sendICECandidate(ctx, candidate)
	})

	// Handle connection state changes
	pc.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		participant.logger.Info("connection state changed", "state", state.String())
		if state == webrtc.PeerConnectionStateFailed || state == webrtc.PeerConnectionStateClosed {
			// Clean up on disconnect
			room.RemoveParticipant(userID)
			if room.ParticipantCount() == 0 {
				s.DeleteRoom(roomID)
			}
		}
	})

	// Add participant to room
	room.AddParticipant(participant)

	// Add existing tracks from other participants to this new participant
	room.mu.RLock()
	for _, other := range room.participants {
		if other.UserID == userID {
			continue
		}
		// Add other participant's tracks to this participant's connection
		other.mu.RLock()
		for _, remoteTrack := range other.remoteTracks {
			participant.subscribeToTrack(other.UserID, other.Username, remoteTrack)
		}
		other.mu.RUnlock()
	}
	room.mu.RUnlock()

	participant.logger.Info("participant joined SFU room")
	return participant, nil
}

// handleIncomingTrack processes media received from a participant
func (p *SFUParticipant) handleIncomingTrack(ctx context.Context, remoteTrack *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
	p.logger.Info("received track", "kind", remoteTrack.Kind().String(), "track_id", remoteTrack.ID())

	p.mu.Lock()
	p.remoteTracks[remoteTrack.ID()] = remoteTrack
	p.mu.Unlock()

	// Forward this track to all other participants
	p.room.mu.RLock()
	for _, other := range p.room.participants {
		if other.UserID == p.UserID {
			continue
		}
		other.subscribeToTrack(p.UserID, p.Username, remoteTrack)
	}
	p.room.mu.RUnlock()

	// Read RTP packets and forward to all subscribers
	go p.forwardTrack(ctx, remoteTrack)
}

// subscribeToTrack adds a remote track to this participant's connection
func (p *SFUParticipant) subscribeToTrack(senderID uuid.UUID, senderName string, remoteTrack *webrtc.TrackRemote) {
	// Create a local track to send to this participant
	localTrack, err := webrtc.NewTrackLocalStaticRTP(
		remoteTrack.Codec().RTPCodecCapability,
		remoteTrack.ID(),
		remoteTrack.StreamID(),
	)
	if err != nil {
		p.logger.Error("failed to create local track", "error", err)
		return
	}

	// Add the track to our peer connection
	sender, err := p.pc.AddTrack(localTrack)
	if err != nil {
		p.logger.Error("failed to add track", "error", err)
		return
	}

	// Handle RTCP packets (for things like PLI/NACK)
	go func() {
		rtcpBuf := make([]byte, 1500)
		for {
			if _, _, rtcpErr := sender.Read(rtcpBuf); rtcpErr != nil {
				return
			}
		}
	}()

	p.mu.Lock()
	p.localTracks[remoteTrack.ID()] = localTrack
	p.mu.Unlock()

	p.logger.Info("subscribed to track", "from_user", senderName, "track_id", remoteTrack.ID())
}

// forwardTrack reads RTP packets from a remote track and writes to all local tracks
func (p *SFUParticipant) forwardTrack(ctx context.Context, remoteTrack *webrtc.TrackRemote) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		rtp, _, err := remoteTrack.ReadRTP()
		if err != nil {
			p.logger.Debug("track read ended", "track_id", remoteTrack.ID(), "error", err)
			return
		}

		// Forward to all other participants
		p.room.mu.RLock()
		for _, other := range p.room.participants {
			if other.UserID == p.UserID {
				continue
			}
			other.mu.RLock()
			if localTrack, ok := other.localTracks[remoteTrack.ID()]; ok {
				if err := localTrack.WriteRTP(rtp); err != nil {
					other.logger.Debug("failed to write RTP", "error", err)
				}
			}
			other.mu.RUnlock()
		}
		p.room.mu.RUnlock()
	}
}

// sendICECandidate sends an ICE candidate to the participant via pubsub
func (p *SFUParticipant) sendICECandidate(ctx context.Context, candidate *webrtc.ICECandidate) {
	candidateJSON, err := json.Marshal(candidate.ToJSON())
	if err != nil {
		return
	}

	payload := map[string]interface{}{
		"room_id":   p.room.ID.String(),
		"from_id":   "server", // SFU is the sender
		"candidate": string(candidateJSON),
	}
	payloadBytes, _ := json.Marshal(payload)

	msg := &pubsub.Message{
		Topic:   pubsub.Topics.User(p.UserID.String()),
		Type:    EventTypeCallICECandidate,
		Payload: payloadBytes,
	}
	p.sfu.pubsub.Publish(ctx, msg.Topic, msg)
}

// HandleOffer processes an SDP offer from the participant
func (p *SFUParticipant) HandleOffer(ctx context.Context, sdp string) (string, error) {
	offer := webrtc.SessionDescription{
		Type: webrtc.SDPTypeOffer,
		SDP:  sdp,
	}

	if err := p.pc.SetRemoteDescription(offer); err != nil {
		return "", err
	}

	// Create answer
	answer, err := p.pc.CreateAnswer(nil)
	if err != nil {
		return "", err
	}

	if err := p.pc.SetLocalDescription(answer); err != nil {
		return "", err
	}

	return answer.SDP, nil
}

// HandleAnswer processes an SDP answer from the participant
func (p *SFUParticipant) HandleAnswer(ctx context.Context, sdp string) error {
	answer := webrtc.SessionDescription{
		Type: webrtc.SDPTypeAnswer,
		SDP:  sdp,
	}

	return p.pc.SetRemoteDescription(answer)
}

// HandleICECandidate adds an ICE candidate from the participant
func (p *SFUParticipant) HandleICECandidate(ctx context.Context, candidateJSON string) error {
	var candidate webrtc.ICECandidateInit
	if err := json.Unmarshal([]byte(candidateJSON), &candidate); err != nil {
		return err
	}
	return p.pc.AddICECandidate(candidate)
}

// CreateOffer creates an SDP offer to send to the participant
func (p *SFUParticipant) CreateOffer(ctx context.Context) (string, error) {
	offer, err := p.pc.CreateOffer(nil)
	if err != nil {
		return "", err
	}

	if err := p.pc.SetLocalDescription(offer); err != nil {
		return "", err
	}

	return offer.SDP, nil
}

// Close closes the participant's connection
func (p *SFUParticipant) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.pc != nil {
		return p.pc.Close()
	}
	return nil
}

// AddParticipant adds a participant to the room
func (r *SFURoom) AddParticipant(p *SFUParticipant) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.participants[p.UserID] = p
}

// RemoveParticipant removes a participant from the room
func (r *SFURoom) RemoveParticipant(userID uuid.UUID) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if p, ok := r.participants[userID]; ok {
		p.Close()
		delete(r.participants, userID)
	}
}

// GetParticipant returns a participant by ID
func (r *SFURoom) GetParticipant(userID uuid.UUID) *SFUParticipant {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.participants[userID]
}

// ParticipantCount returns the number of participants
func (r *SFURoom) ParticipantCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.participants)
}

// GetParticipantList returns info about all participants
func (r *SFURoom) GetParticipantList() []Participant {
	r.mu.RLock()
	defer r.mu.RUnlock()

	list := make([]Participant, 0, len(r.participants))
	for _, p := range r.participants {
		list = append(list, Participant{
			UserID:   p.UserID,
			Username: p.Username,
		})
	}
	return list
}
