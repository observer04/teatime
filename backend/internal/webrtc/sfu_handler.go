// Package webrtc provides WebRTC functionality for video/audio calls.
// This file handles signaling messages for SFU-based group calls.
package webrtc

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/google/uuid"
	"github.com/observer/teatime/internal/database"
	"github.com/observer/teatime/internal/pubsub"
)

// Additional event types for SFU
const (
	EventTypeSFUOffer     = "sfu.offer"     // SFU sends offer to participant
	EventTypeSFUAnswer    = "sfu.answer"    // Participant sends answer to SFU
	EventTypeSFUCandidate = "sfu.candidate" // ICE candidate exchange with SFU
	EventTypeSFUTracks    = "sfu.tracks"    // Track list update
)

// SFUHandler processes signaling messages for group calls
type SFUHandler struct {
	sfu      *SFU
	p2pMgr   *Manager // P2P manager for 1:1 calls
	convRepo *database.ConversationRepository
	callRepo *database.CallRepository
	pubsub   pubsub.PubSub
	logger   *slog.Logger
}

// NewSFUHandler creates a new SFU handler
func NewSFUHandler(
	sfu *SFU,
	p2pMgr *Manager,
	convRepo *database.ConversationRepository,
	callRepo *database.CallRepository,
	ps pubsub.PubSub,
	logger *slog.Logger,
) *SFUHandler {
	return &SFUHandler{
		sfu:      sfu,
		p2pMgr:   p2pMgr,
		convRepo: convRepo,
		callRepo: callRepo,
		pubsub:   ps,
		logger:   logger.With("component", "sfu_handler"),
	}
}

// SFUJoinPayload is the payload for joining a group call
type SFUJoinPayload struct {
	RoomID   string `json:"room_id"`
	IsGroup  bool   `json:"is_group"`  // True for group calls (use SFU), false for P2P
	CallType string `json:"call_type"` // "video" or "audio"
}

// SFUOfferPayload contains SDP offer/answer for SFU
type SFUOfferPayload struct {
	RoomID string `json:"room_id"`
	SDP    string `json:"sdp"`
}

// SFUCandidatePayload contains ICE candidate for SFU
type SFUCandidatePayload struct {
	RoomID    string `json:"room_id"`
	Candidate string `json:"candidate"`
}

// SFUConfigPayload is sent to client after joining a group call
type SFUConfigPayload struct {
	RoomID       uuid.UUID     `json:"room_id"`
	ICEServers   []ICEServer   `json:"ice_servers"`
	Participants []Participant `json:"participants"`
	Mode         string        `json:"mode"` // "sfu" or "p2p"
}

// SFUTracksPayload contains track information
type SFUTracksPayload struct {
	RoomID string      `json:"room_id"`
	Tracks []TrackInfo `json:"tracks"`
}

// HandleGroupJoin processes a group call join request
// This determines whether to use SFU or P2P based on the conversation type and participant count
func (h *SFUHandler) HandleGroupJoin(ctx context.Context, sigCtx *SignalingContext, payload json.RawMessage) (*SFUConfigPayload, error) {
	var p SFUJoinPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return nil, &CallError{Code: "invalid_payload", Message: "Invalid join payload"}
	}

	roomID, err := uuid.Parse(p.RoomID)
	if err != nil {
		return nil, &CallError{Code: "invalid_room", Message: "Invalid room ID"}
	}

	// Verify user is member of the conversation
	isMember, err := h.convRepo.IsMember(ctx, roomID, sigCtx.UserID)
	if err != nil || !isMember {
		return nil, &CallError{Code: "not_member", Message: "Not a member of this conversation"}
	}

	// Get conversation to check if it's a group
	conv, err := h.convRepo.GetByID(ctx, roomID)
	if err != nil {
		return nil, &CallError{Code: "not_found", Message: "Conversation not found"}
	}

	// For group conversations (3+ members) or explicit group flag, use SFU
	isGroup := p.IsGroup || conv.Type == "group" || len(conv.Members) > 2

	if isGroup {
		return h.joinSFU(ctx, sigCtx, roomID, p.CallType)
	}

	// For 1:1 calls, use P2P (existing logic)
	return h.joinP2P(ctx, sigCtx, roomID, p.CallType)
}

// joinSFU handles joining via the SFU
func (h *SFUHandler) joinSFU(ctx context.Context, sigCtx *SignalingContext, roomID uuid.UUID, callType string) (*SFUConfigPayload, error) {
	h.logger.Info("user joining SFU room",
		"room_id", roomID,
		"user_id", sigCtx.UserID,
		"username", sigCtx.Username)

	participant, err := h.sfu.JoinRoom(ctx, roomID, sigCtx.UserID, sigCtx.Username)
	if err != nil {
		return nil, &CallError{Code: "join_failed", Message: err.Error()}
	}

	room := h.sfu.GetRoom(roomID)
	if room == nil {
		return nil, &CallError{Code: "room_not_found", Message: "Room not found after join"}
	}

	// Notify other participants about new joiner
	h.broadcastParticipantJoined(ctx, room, sigCtx)

	// Create initial offer to send to the participant
	offer, err := participant.CreateOffer(ctx)
	if err != nil {
		h.logger.Error("failed to create initial offer", "error", err)
	} else {
		// Send offer to participant
		h.sendOfferToParticipant(ctx, sigCtx.UserID, roomID, offer)
	}

	// Return SFU config
	iceServers := h.p2pMgr.GetConfig().GetICEServers()
	return &SFUConfigPayload{
		RoomID:       roomID,
		ICEServers:   iceServers,
		Participants: room.GetParticipantList(),
		Mode:         "sfu",
	}, nil
}

// joinP2P handles joining via P2P (delegates to existing manager)
func (h *SFUHandler) joinP2P(ctx context.Context, sigCtx *SignalingContext, roomID uuid.UUID, callType string) (*SFUConfigPayload, error) {
	h.logger.Info("user joining P2P call",
		"room_id", roomID,
		"user_id", sigCtx.UserID,
		"username", sigCtx.Username)

	// Check if this is the initiator
	room := h.p2pMgr.GetRoom(roomID)
	existingCallID := uuid.Nil
	if room != nil {
		existingCallID = room.GetCallID()
	}
	isInitiator := existingCallID == uuid.Nil

	// Join using P2P manager
	room, err := h.p2pMgr.JoinCall(ctx, roomID, sigCtx.UserID, sigCtx.Username)
	if err != nil {
		return nil, &CallError{Code: "join_failed", Message: err.Error()}
	}

	// Handle call log creation for initiator
	if isInitiator && h.callRepo != nil {
		ct := database.CallTypeVideo
		if callType == "audio" {
			ct = database.CallTypeAudio
		}

		callLog, err := h.callRepo.CreateCallLog(ctx, roomID, sigCtx.UserID, ct)
		if err != nil {
			h.logger.Error("failed to create call log", "error", err)
		} else {
			h.callRepo.AddParticipant(ctx, callLog.ID, sigCtx.UserID)
			room.SetCallID(callLog.ID)
			// Note: broadcastIncomingCall is handled by CallHandler
		}
	} else if existingCallID != uuid.Nil && h.callRepo != nil {
		h.callRepo.AddParticipant(ctx, existingCallID, sigCtx.UserID)
		if room.ParticipantCount() == 2 {
			h.callRepo.StartCall(ctx, existingCallID)
		}
	}

	iceServers := h.p2pMgr.GetConfig().GetICEServers()
	return &SFUConfigPayload{
		RoomID:       roomID,
		ICEServers:   iceServers,
		Participants: room.GetParticipants(),
		Mode:         "p2p",
	}, nil
}

// HandleSFUOffer handles an SDP offer from the client to the SFU
func (h *SFUHandler) HandleSFUOffer(ctx context.Context, sigCtx *SignalingContext, payload json.RawMessage) error {
	var p SFUOfferPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return &CallError{Code: "invalid_payload", Message: "Invalid offer payload"}
	}

	roomID, err := uuid.Parse(p.RoomID)
	if err != nil {
		return &CallError{Code: "invalid_room", Message: "Invalid room ID"}
	}

	room := h.sfu.GetRoom(roomID)
	if room == nil {
		return &CallError{Code: "room_not_found", Message: "Room not found"}
	}

	participant := room.GetParticipant(sigCtx.UserID)
	if participant == nil {
		return &CallError{Code: "not_in_call", Message: "Not in this call"}
	}

	// Handle the offer and get answer
	answer, err := participant.HandleOffer(ctx, p.SDP)
	if err != nil {
		return &CallError{Code: "offer_failed", Message: err.Error()}
	}

	// Send answer back to participant
	h.sendAnswerToParticipant(ctx, sigCtx.UserID, roomID, answer)

	return nil
}

// HandleSFUAnswer handles an SDP answer from the client to the SFU
func (h *SFUHandler) HandleSFUAnswer(ctx context.Context, sigCtx *SignalingContext, payload json.RawMessage) error {
	var p SFUOfferPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return &CallError{Code: "invalid_payload", Message: "Invalid answer payload"}
	}

	roomID, err := uuid.Parse(p.RoomID)
	if err != nil {
		return &CallError{Code: "invalid_room", Message: "Invalid room ID"}
	}

	room := h.sfu.GetRoom(roomID)
	if room == nil {
		return &CallError{Code: "room_not_found", Message: "Room not found"}
	}

	participant := room.GetParticipant(sigCtx.UserID)
	if participant == nil {
		return &CallError{Code: "not_in_call", Message: "Not in this call"}
	}

	if err := participant.HandleAnswer(ctx, p.SDP); err != nil {
		return &CallError{Code: "answer_failed", Message: err.Error()}
	}

	return nil
}

// HandleSFUCandidate handles an ICE candidate from the client
func (h *SFUHandler) HandleSFUCandidate(ctx context.Context, sigCtx *SignalingContext, payload json.RawMessage) error {
	var p SFUCandidatePayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return &CallError{Code: "invalid_payload", Message: "Invalid candidate payload"}
	}

	roomID, err := uuid.Parse(p.RoomID)
	if err != nil {
		return &CallError{Code: "invalid_room", Message: "Invalid room ID"}
	}

	room := h.sfu.GetRoom(roomID)
	if room == nil {
		return &CallError{Code: "room_not_found", Message: "Room not found"}
	}

	participant := room.GetParticipant(sigCtx.UserID)
	if participant == nil {
		return &CallError{Code: "not_in_call", Message: "Not in this call"}
	}

	if err := participant.HandleICECandidate(ctx, p.Candidate); err != nil {
		return &CallError{Code: "candidate_failed", Message: err.Error()}
	}

	return nil
}

// HandleSFULeave handles a participant leaving the SFU room
func (h *SFUHandler) HandleSFULeave(ctx context.Context, sigCtx *SignalingContext, payload json.RawMessage) error {
	var p CallLeavePayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return &CallError{Code: "invalid_payload", Message: "Invalid leave payload"}
	}

	roomID, err := uuid.Parse(p.RoomID)
	if err != nil {
		return &CallError{Code: "invalid_room", Message: "Invalid room ID"}
	}

	room := h.sfu.GetRoom(roomID)
	if room != nil {
		room.RemoveParticipant(sigCtx.UserID)

		// Notify others
		h.broadcastParticipantLeft(ctx, room, sigCtx)

		// Clean up empty room
		if room.ParticipantCount() == 0 {
			h.sfu.DeleteRoom(roomID)
		}
	}

	h.logger.Info("user left SFU room", "room_id", roomID, "user_id", sigCtx.UserID)
	return nil
}

// Helper methods for sending messages

func (h *SFUHandler) sendOfferToParticipant(ctx context.Context, userID, roomID uuid.UUID, sdp string) {
	payload := map[string]interface{}{
		"room_id": roomID.String(),
		"sdp":     sdp,
	}
	payloadBytes, _ := json.Marshal(payload)

	msg := &pubsub.Message{
		Topic:   pubsub.Topics.User(userID.String()),
		Type:    EventTypeSFUOffer,
		Payload: payloadBytes,
	}
	h.pubsub.Publish(ctx, msg.Topic, msg)
}

func (h *SFUHandler) sendAnswerToParticipant(ctx context.Context, userID, roomID uuid.UUID, sdp string) {
	payload := map[string]interface{}{
		"room_id": roomID.String(),
		"sdp":     sdp,
	}
	payloadBytes, _ := json.Marshal(payload)

	msg := &pubsub.Message{
		Topic:   pubsub.Topics.User(userID.String()),
		Type:    "sfu.answer.server", // Distinguished from client answer
		Payload: payloadBytes,
	}
	h.pubsub.Publish(ctx, msg.Topic, msg)
}

func (h *SFUHandler) broadcastParticipantJoined(ctx context.Context, room *SFURoom, joiner *SignalingContext) {
	event := CallParticipantEvent{
		RoomID:   room.ID,
		UserID:   joiner.UserID,
		Username: joiner.Username,
		Action:   "joined",
	}
	payloadBytes, _ := json.Marshal(event)

	room.mu.RLock()
	defer room.mu.RUnlock()

	for _, p := range room.participants {
		if p.UserID == joiner.UserID {
			continue
		}
		msg := &pubsub.Message{
			Topic:   pubsub.Topics.User(p.UserID.String()),
			Type:    EventTypeCallParticipantJoined,
			Payload: payloadBytes,
		}
		h.pubsub.Publish(ctx, msg.Topic, msg)
	}
}

func (h *SFUHandler) broadcastParticipantLeft(ctx context.Context, room *SFURoom, leaver *SignalingContext) {
	event := CallParticipantEvent{
		RoomID:   room.ID,
		UserID:   leaver.UserID,
		Username: leaver.Username,
		Action:   "left",
	}
	payloadBytes, _ := json.Marshal(event)

	room.mu.RLock()
	defer room.mu.RUnlock()

	for _, p := range room.participants {
		if p.UserID == leaver.UserID {
			continue
		}
		msg := &pubsub.Message{
			Topic:   pubsub.Topics.User(p.UserID.String()),
			Type:    EventTypeCallParticipantLeft,
			Payload: payloadBytes,
		}
		h.pubsub.Publish(ctx, msg.Topic, msg)
	}
}
