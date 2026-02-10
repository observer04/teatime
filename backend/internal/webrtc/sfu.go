package webrtc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/observer/teatime/internal/pubsub"
	"github.com/pion/rtcp"
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

type SFUConfig struct {
	ICEServers []webrtc.ICEServer
}

type SFURoom struct {
	mu           sync.RWMutex
	ID           uuid.UUID
	participants map[uuid.UUID]*SFUParticipant
	callID       uuid.UUID
	logger       *slog.Logger
}

type SFUParticipant struct {
	mu           sync.RWMutex
	UserID       uuid.UUID
	Username     string
	pc           *webrtc.PeerConnection
	localTracks  map[string]*webrtc.TrackLocalStaticRTP
	remoteTracks map[string]*webrtc.TrackRemote
	room         *SFURoom
	sfu          *SFU
	logger       *slog.Logger

	// Renegotiation handling
	isNegotiating      bool
	negotiationPending bool

	// Lifecycle management
	ctx    context.Context
	cancel context.CancelFunc

	// Candidate Buffering
	pendingCandidates []*webrtc.ICECandidate

	// Track subscriptions (Sender side)
	subscribers   map[string][]*webrtc.TrackLocalStaticRTP // trackID -> list of subscribers
	subscribersMu sync.RWMutex

	// Track subscriptions (Receiver side) - to clean up on leave
	subscriptions map[string]uuid.UUID // trackID -> senderID
}

type TrackInfo struct {
	ID       string `json:"id"`
	Kind     string `json:"kind"`
	UserID   string `json:"user_id"`
	Username string `json:"username"`
}

func NewSFU(config *SFUConfig, ps pubsub.PubSub, logger *slog.Logger) *SFU {
	return &SFU{
		rooms:  make(map[uuid.UUID]*SFURoom),
		config: config,
		pubsub: ps,
		logger: logger.With("component", "sfu"),
	}
}

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
	return room
}

func (s *SFU) GetRoom(roomID uuid.UUID) *SFURoom {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.rooms[roomID]
}

func (s *SFU) DeleteRoom(roomID uuid.UUID) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.rooms, roomID)
}

// requestKeyframe relays a PLI to the original sender of a track
func (s *SFU) requestKeyframe(trackID string, roomID uuid.UUID) {
	room := s.GetRoom(roomID)
	if room == nil {
		return
	}

	room.mu.RLock()
	defer room.mu.RUnlock()

	// Find the participant who owns this track (the sender)
	for _, p := range room.participants {
		p.mu.RLock()
		if track, ok := p.remoteTracks[trackID]; ok {
			// Found the sender! Send PLI to them.
			// Only log if Debug level is enabled to avoid spam
			// p.logger.Debug("Relaying PLI for track", "track_id", trackID)
			
			_ = p.pc.WriteRTCP([]rtcp.Packet{
				&rtcp.PictureLossIndication{MediaSSRC: uint32(track.SSRC())},
			})
			p.mu.RUnlock()
			return
		}
		p.mu.RUnlock()
	}
}

func (r *SFURoom) SetCallID(callID uuid.UUID) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.callID = callID
}

func (r *SFURoom) GetCallID() uuid.UUID {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.callID
}

// GetTracks returns actual track info from participants for mapping
func (r *SFURoom) GetTracks() []TrackInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var tracks []TrackInfo
	for _, p := range r.participants {
		p.mu.RLock()
		for _, track := range p.remoteTracks {
			tracks = append(tracks, TrackInfo{
				ID:       track.ID(),       // The REAL WebRTC Track ID
				Kind:     track.Kind().String(),
				UserID:   p.UserID.String(),
				Username: p.Username,
			})
		}
		p.mu.RUnlock()
	}
	return tracks
}

// JoinRoom adds a participant
func (s *SFU) JoinRoom(ctx context.Context, roomID, userID uuid.UUID, username string) (*SFUParticipant, error) {
	room := s.GetOrCreateRoom(roomID)

	// Create a dedicated context for this participant that survives the request
	pCtx, pCancel := context.WithCancel(context.Background())

	// Codec Enforcement (VP8/Opus)
	m := &webrtc.MediaEngine{}
	if err := m.RegisterCodec(webrtc.RTPCodecParameters{
		RTPCodecCapability: webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeVP8, ClockRate: 90000, Channels: 0, SDPFmtpLine: "", RTCPFeedback: nil},
		PayloadType:        96,
	}, webrtc.RTPCodecTypeVideo); err != nil {
		pCancel()
		return nil, err
	}
	if err := m.RegisterCodec(webrtc.RTPCodecParameters{
		RTPCodecCapability: webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeOpus, ClockRate: 48000, Channels: 2, SDPFmtpLine: "", RTCPFeedback: nil},
		PayloadType:        111,
	}, webrtc.RTPCodecTypeAudio); err != nil {
		pCancel()
		return nil, err
	}

	api := webrtc.NewAPI(webrtc.WithMediaEngine(m), webrtc.WithSettingEngine(webrtc.SettingEngine{}))
	
	config := webrtc.Configuration{ICEServers: s.config.ICEServers}
	pc, err := api.NewPeerConnection(config)
	if err != nil {
		pCancel()
		return nil, err
	}

	// Allow receiving audio/video
	for _, kind := range []webrtc.RTPCodecType{webrtc.RTPCodecTypeVideo, webrtc.RTPCodecTypeAudio} {
		if _, err := pc.AddTransceiverFromKind(kind, webrtc.RTPTransceiverInit{
			Direction: webrtc.RTPTransceiverDirectionRecvonly,
		}); err != nil {
			pc.Close()
			pCancel()
			return nil, err
		}
	}

	participant := &SFUParticipant{
		UserID:       userID,
		Username:     username,
		pc:           pc,
		localTracks:   make(map[string]*webrtc.TrackLocalStaticRTP),
		remoteTracks:  make(map[string]*webrtc.TrackRemote),
		subscribers:   make(map[string][]*webrtc.TrackLocalStaticRTP),
		subscriptions: make(map[string]uuid.UUID),
		room:          room,
		sfu:           s,
		logger:        room.logger.With("user_id", userID),
		ctx:           pCtx, // Use this for forwardTrack
		cancel:        pCancel,
	}

	pc.OnTrack(func(remoteTrack *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
		// Use the Participant's Long-Lived Context, NOT reqCtx
		participant.handleIncomingTrack(participant.ctx, remoteTrack, receiver)
	})

	pc.OnICECandidate(func(candidate *webrtc.ICECandidate) {
		if candidate != nil {
			// Use the Participant's Context
			participant.sendICECandidate(participant.ctx, candidate)
		}
	})

	pc.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		if state == webrtc.PeerConnectionStateFailed || state == webrtc.PeerConnectionStateClosed {
			room.RemoveParticipant(userID)
			if room.ParticipantCount() == 0 {
				s.DeleteRoom(roomID)
			}
		}
	})

	room.AddParticipant(participant)

	// Subscribe to existing tracks
	room.mu.RLock()
	for _, other := range room.participants {
		if other.UserID == userID {
			continue
		}
		other.mu.RLock()
		for _, remoteTrack := range other.remoteTracks {
			// Don't negotiate yet; the initial offer covers this
			participant.subscribeToTrack(ctx, other.UserID, remoteTrack, false)
		}
		other.mu.RUnlock()
	}
	room.mu.RUnlock()

	return participant, nil
}

func (p *SFUParticipant) handleIncomingTrack(ctx context.Context, remoteTrack *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
	p.mu.Lock()
	p.remoteTracks[remoteTrack.ID()] = remoteTrack
	p.mu.Unlock()

	// Forward to others
	p.room.mu.RLock()
	for _, other := range p.room.participants {
		if other.UserID == p.UserID {
			continue
		}
		// True = Trigger negotiation because connection is already established
		other.subscribeToTrack(ctx, p.UserID, remoteTrack, true)
	}
	p.room.mu.RUnlock()

	go p.forwardTrack(ctx, remoteTrack)
}

// AddSubscriber adds a subscriber for a specific track
func (p *SFUParticipant) AddSubscriber(trackID string, sub *webrtc.TrackLocalStaticRTP) {
	p.subscribersMu.Lock()
	defer p.subscribersMu.Unlock()
	p.subscribers[trackID] = append(p.subscribers[trackID], sub)
}

// RemoveSubscriber removes a subscriber
func (p *SFUParticipant) RemoveSubscriber(trackID string, sub *webrtc.TrackLocalStaticRTP) {
	p.subscribersMu.Lock()
	defer p.subscribersMu.Unlock()

	subs := p.subscribers[trackID]
	for i, s := range subs {
		if s == sub {
			// Remove element
			p.subscribers[trackID] = append(subs[:i], subs[i+1:]...)
			return
		}
	}
}

func (p *SFUParticipant) subscribeToTrack(ctx context.Context, senderID uuid.UUID, remoteTrack *webrtc.TrackRemote, negotiate bool) {
	localTrack, err := webrtc.NewTrackLocalStaticRTP(
		remoteTrack.Codec().RTPCodecCapability,
		remoteTrack.ID(),
		remoteTrack.StreamID(),
	)
	if err != nil {
		p.logger.Error("failed to create local track", "error", err)
		return
	}

	sender, err := p.pc.AddTrack(localTrack)
	if err != nil {
		p.logger.Error("failed to add track", "error", err)
		return
	}

	// Read RTCP from receiver (needed for PLI)
	// Read RTCP from receiver (needed for PLI)
	go func() {
		rtcpBuf := make([]byte, 1500)
		for {
			n, _, rtcpErr := sender.Read(rtcpBuf)
			if rtcpErr != nil {
				return
			}

			// FIX 5: Handle RTCP Feedback (PLI/NACK)
			pkts, err := rtcp.Unmarshal(rtcpBuf[:n])
			if err != nil {
				continue
			}

			for _, pkt := range pkts {
				switch pkt.(type) {
				case *rtcp.PictureLossIndication, *rtcp.FullIntraRequest:
					// User needs a keyframe! Relay this request to the original sender.
					p.sfu.requestKeyframe(remoteTrack.ID(), p.room.ID)
				}
			}
		}
	}()

	p.mu.Lock()
	p.localTracks[remoteTrack.ID()] = localTrack
	p.subscriptions[remoteTrack.ID()] = senderID
	p.mu.Unlock()

	// Register with sender
	p.room.mu.RLock()
	sfuSender := p.room.participants[senderID]
	p.room.mu.RUnlock()

	if sfuSender != nil {
		sfuSender.AddSubscriber(remoteTrack.ID(), localTrack)
	}

	// FIX 3: Request Keyframe (PLI) immediately so new subscriber gets image
	p.sendPLI(remoteTrack)

	if negotiate {
		p.processNegotiation(ctx)
	}
}

// FIX 2: Negotiation Queue
func (p *SFUParticipant) processNegotiation(ctx context.Context) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.isNegotiating {
		p.negotiationPending = true
		return
	}

	p.isNegotiating = true
	p.negotiationPending = false

	go func() {
		// Small delay to debounce multiple track additions
		time.Sleep(50 * time.Millisecond)
		
		offer, err := p.CreateOffer(ctx)
		if err != nil {
			p.logger.Error("failed to create offer", "error", err)
			p.mu.Lock()
			p.isNegotiating = false
			p.mu.Unlock()
			return
		}
		p.sendOffer(ctx, offer)
	}()
}

func (p *SFUParticipant) sendPLI(track *webrtc.TrackRemote) {
	// Find the participant who owns this remote track
	p.room.mu.RLock()
	defer p.room.mu.RUnlock()

	for _, participant := range p.room.participants {
		participant.mu.RLock()
		if _, ok := participant.remoteTracks[track.ID()]; ok {
			// Found the sender, send PLI to their PC
			err := participant.pc.WriteRTCP([]rtcp.Packet{
				&rtcp.PictureLossIndication{MediaSSRC: uint32(track.SSRC())},
			})
			if err != nil {
				p.logger.Error("failed to send PLI", "error", err)
			}
			participant.mu.RUnlock()
			return
		}
		participant.mu.RUnlock()
	}
}

func (p *SFUParticipant) forwardTrack(ctx context.Context, remoteTrack *webrtc.TrackRemote) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		rtp, _, err := remoteTrack.ReadRTP()
		if err != nil {
			return
		}

		// Optimized: Use internal subscribers map, no room lock needed
		p.subscribersMu.RLock()
		// Copy subscribers to avoid holding lock during write
		targets := make([]*webrtc.TrackLocalStaticRTP, len(p.subscribers[remoteTrack.ID()]))
		copy(targets, p.subscribers[remoteTrack.ID()])
		p.subscribersMu.RUnlock()

		// Write to targets
		for _, target := range targets {
			// FIX 4: Deep Copy the packet so SSRC rewriting doesn't race
			packetCopy := *rtp
			packetCopy.Header = rtp.Header   // Shallow copy header struct
			packetCopy.Payload = rtp.Payload // Payload slice matches (safe to read shared)

			// WriteRTP will modify the Header.SSRC of packetCopy, not the original rtp
			if err := target.WriteRTP(&packetCopy); err != nil {
				if errors.Is(err, io.ErrClosedPipe) {
					// Clean up closed pipe to prevent repeated errors
					p.RemoveSubscriber(remoteTrack.ID(), target)
				}
			}
		}
	}
}

func (p *SFUParticipant) HandleAnswer(ctx context.Context, sdp string) error {
	answer := webrtc.SessionDescription{Type: webrtc.SDPTypeAnswer, SDP: sdp}
	if err := p.pc.SetRemoteDescription(answer); err != nil {
		return err
	}

	// Check if pending negotiation exists
	p.mu.Lock()
	p.isNegotiating = false
	pending := p.negotiationPending
	p.mu.Unlock()

	if pending {
		p.processNegotiation(ctx)
	}
	return nil
}

// Helpers for offer/candidate/close remain similar...
func (p *SFUParticipant) sendICECandidate(ctx context.Context, candidate *webrtc.ICECandidate) {
	p.mu.Lock()
	// FIX 13: Candidate Race - Buffer if offer not sent (no local description)
	if p.pc.CurrentLocalDescription() == nil {
		p.pendingCandidates = append(p.pendingCandidates, candidate)
		p.mu.Unlock()
		return
	}
	p.mu.Unlock()

	p.emitCandidate(ctx, candidate)
}

func (p *SFUParticipant) emitCandidate(ctx context.Context, candidate *webrtc.ICECandidate) {
	candidateJSON, _ := json.Marshal(candidate.ToJSON())
	payload := map[string]interface{}{
		"room_id":   p.room.ID.String(),
		"from_id":   "server",
		"candidate": string(candidateJSON),
	}
	bytes, _ := json.Marshal(payload)
	msg := &pubsub.Message{
		Topic:   pubsub.Topics.User(p.UserID.String()),
		Type:    "sfu.candidate", // Matches handler constant
		Payload: bytes,
	}
	_ = p.sfu.pubsub.Publish(ctx, msg.Topic, msg)
}

func (p *SFUParticipant) sendOffer(ctx context.Context, sdp string) {
	payload := map[string]interface{}{"room_id": p.room.ID.String(), "sdp": sdp}
	bytes, _ := json.Marshal(payload)
	msg := &pubsub.Message{
		Topic:   pubsub.Topics.User(p.UserID.String()),
		Type:    "sfu.offer", // Matches handler constant
		Payload: bytes,
	}
	_ = p.sfu.pubsub.Publish(ctx, msg.Topic, msg)

	// FIX 13: Flush pending candidates after offer is sent
	p.mu.Lock()
	candidates := p.pendingCandidates
	p.pendingCandidates = nil
	p.mu.Unlock()

	for _, c := range candidates {
		p.emitCandidate(ctx, c)
	}
}

func (p *SFUParticipant) HandleICECandidate(ctx context.Context, cand string) error {
	var i webrtc.ICECandidateInit
	if err := json.Unmarshal([]byte(cand), &i); err != nil {
		return fmt.Errorf("failed to unmarshal candidate: %w", err)
	}
	return p.pc.AddICECandidate(i)
}

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

func (p *SFUParticipant) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.cancel() // FIX 11: Kill all forwardTrack loops

	// Clean up subscriptions from upstream senders
	p.room.mu.RLock()
	for trackID, senderID := range p.subscriptions {
		if sender := p.room.participants[senderID]; sender != nil {
			if localTrack, ok := p.localTracks[trackID]; ok {
				sender.RemoveSubscriber(trackID, localTrack)
			}
		}
	}
	p.room.mu.RUnlock()

	if p.pc != nil {
		return p.pc.Close()
	}
	return nil
}

// HandleOffer handles an offer from the client (renegotiation initiated by client)
func (p *SFUParticipant) HandleOffer(ctx context.Context, sdp string) (string, error) {
	offer := webrtc.SessionDescription{Type: webrtc.SDPTypeOffer, SDP: sdp}
	if err := p.pc.SetRemoteDescription(offer); err != nil {
		return "", err
	}

	answer, err := p.pc.CreateAnswer(nil)
	if err != nil {
		return "", err
	}

	if err := p.pc.SetLocalDescription(answer); err != nil {
		return "", err
	}

	return answer.SDP, nil
}

func (r *SFURoom) AddParticipant(p *SFUParticipant) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.participants[p.UserID] = p
}

func (r *SFURoom) RemoveParticipant(u uuid.UUID) {
	r.mu.Lock()
	p, ok := r.participants[u]
	if ok {
		delete(r.participants, u)
	}
	r.mu.Unlock()

	if ok && p != nil {
		p.Close()
		
		// Trigger room deletion check if empty
		r.mu.RLock()
		count := len(r.participants)
		r.mu.RUnlock()
		
		if count == 0 {
			// Room is empty.
			// Ideally we would delete the room here, but the Manager handles cleanup
			// via OnConnectionStateChange or explicit checks.
		}
	}
}

func (r *SFURoom) ParticipantCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.participants)
}

func (r *SFURoom) GetParticipant(u uuid.UUID) *SFUParticipant {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.participants[u]
}

func (r *SFURoom) GetParticipantList() []Participant {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var list []Participant
	for _, p := range r.participants {
		list = append(list, Participant{UserID: p.UserID, Username: p.Username})
	}
	return list
}
