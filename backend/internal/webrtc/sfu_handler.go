// Package webrtc provides WebRTC functionality for video/audio calls.
// This file handles signaling messages for SFU-based group calls.
package webrtc

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/google/uuid"
	"github.com/observer/teatime/internal/database"
	"github.com/observer/teatime/internal/domain"
	"github.com/observer/teatime/internal/pubsub"
)

// Additional event types for SFU are defined in protocol.go

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
	RoomID    string      `json:"room_id"`
	Candidate interface{} `json:"candidate"`
}

// SFUConfigPayload is sent to client after joining a group call
type SFUConfigPayload struct {
	RoomID       uuid.UUID     `json:"room_id"`
	ICEServers   []ICEServer   `json:"ice_servers"`
	Participants []Participant `json:"participants"`
	Mode         string        `json:"mode"` // "sfu" or "p2p"
	SDP          string        `json:"sdp,omitempty"`
	IsInitiator  bool          `json:"is_initiator"`
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
	isGroup := p.IsGroup || conv.Type == domain.ConversationTypeGroup || len(conv.Members) > 2

	if isGroup {
		// FIX 1: Split-Brain Detection & Migration
		// Check if there is an active P2P call for this room
		p2pRoom := h.p2pMgr.GetRoom(roomID)
		if p2pRoom != nil && p2pRoom.ParticipantCount() > 0 {
			h.logger.Info("detected P2P room during group join - triggering migration", "room_id", roomID)

			// Notify P2P participants to reload/reconnect, which will now route them to SFU
			// because the conversation type is now 'group' (or implicit group)
			migrationEvent := map[string]interface{}{
				"room_id": roomID.String(),
				"reason":  "switching_to_group",
			}
			payloadBytes, _ := json.Marshal(migrationEvent)

			// Broadcast 'call.error' with specific code to force frontend reconnection
			// Or a specific 'call.migration' event if frontend supports it
			for _, participant := range p2pRoom.GetParticipants() {
				msg := &pubsub.Message{
					Topic:   pubsub.Topics.User(participant.UserID.String()),
					Type:    EventTypeCallMigration,
					Payload: payloadBytes,
				}
				if err := h.pubsub.Publish(ctx, msg.Topic, msg); err != nil {
					h.logger.Error("failed to publish migration event", "error", err, "user_id", participant.UserID)
				}
			}

			// Clean up the P2P room to prevent split-brain state
			p2pCallID := p2pRoom.GetCallID()
			p2pParticipants := p2pRoom.GetParticipants()
			for _, participant := range p2pParticipants {
				h.p2pMgr.LeaveCall(ctx, roomID, participant.UserID, participant.Username)
			}

			// End the P2P call log in the database
			if p2pCallID != uuid.Nil && h.callRepo != nil {
				h.logger.Info("ending P2P call during migration to SFU",
					"room_id", roomID, "p2p_call_id", p2pCallID)
				_ = h.callRepo.EndCall(ctx, p2pCallID)
			}
		}

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

	// Determine if this user is the call initiator (no existing call ID means they're first)
	isInitiator := room.GetCallID() == uuid.Nil

	// Call logging: create call log for initiator, add participant for joiners
	if h.callRepo != nil {
		existingCallID := room.GetCallID()

		if isInitiator {
			// Zombie call cleanup: end any dangling active calls for this room
			activeCallID, err := h.callRepo.GetActiveCallID(ctx, roomID)
			if err == nil && activeCallID != uuid.Nil {
				h.logger.Warn("found dangling active SFU call, cleaning up",
					"room_id", roomID, "call_id", activeCallID)
				_ = h.callRepo.EndCall(ctx, activeCallID)
			}

			ct := database.CallTypeVideo
			if callType == "audio" {
				ct = database.CallTypeAudio
			}
			callLog, err := h.callRepo.CreateCallLog(ctx, roomID, sigCtx.UserID, ct)
			if err != nil {
				h.logger.Error("failed to create SFU call log", "error", err)
			} else {
				_ = h.callRepo.AddParticipant(ctx, callLog.ID, sigCtx.UserID)
				room.SetCallID(callLog.ID)

				// Detect if call type is video or audio
				dbCallType := database.CallTypeVideo
				if callType == "audio" {
					dbCallType = database.CallTypeAudio
				}
				h.broadcastIncomingCall(ctx, roomID, callLog.ID, sigCtx, dbCallType)
			}
		} else {
			_ = h.callRepo.AddParticipant(ctx, existingCallID, sigCtx.UserID)
			if room.ParticipantCount() == 2 {
				_ = h.callRepo.StartCall(ctx, existingCallID)
			}
		}
	}

	// Notify other participants about new joiner
	h.broadcastParticipantJoined(ctx, room, sigCtx)

	// Create initial offer to send to the participant
	var offerSDP string
	offer, err := participant.CreateOffer(ctx)
	if err != nil {
		h.logger.Error("failed to create initial offer", "error", err)
	} else {
		offerSDP = offer
		// optimization: we include SDP in the response, so we don't need to send it via pubsub
		// h.sendOfferToParticipant(ctx, sigCtx.UserID, roomID, offer)

		// Flush pending CANDIDATES that might have been buffered during CreateOffer
		// The `CreateOffer` method in `sfu.go` sets LocalDescription, so `sendICECandidate`
		// might have buffered some if they fired before SetLocalDescription returned (unlikely given lock?
		// actually `CreateOffer` uses `pc.SetLocalDescription` internally).
		// Wait, `sfu.go` `sendOffer` method did the flushing. Since we are NOT calling `sendOffer` (which calls `sendOfferToParticipant`),
		// we need to make sure we replicate the sidebar effects of `sendOffer` if any.
		// `SFUParticipant.sendOffer` does: 1. Publish JSON 2. Flush pendingCandidates.
		// We skipped 1. We MUST do 2.

		// Accessing private field `pendingCandidates` of `SFUParticipant` is not possible directly from here if it wasn't exported.
		// But we are in `package webrtc`, so we can access it.
		// Let's create a helper or just do it if we can access the field (it's in the same package).
		// `participant` is `*SFUParticipant`.

		participant.mu.Lock()
		candidates := participant.pendingCandidates
		participant.pendingCandidates = nil
		participant.mu.Unlock()

		for _, c := range candidates {
			participant.emitCandidate(ctx, c)
		}
	}

	// Send track info so frontend can identify remote streams
	h.sendTrackInfo(ctx, sigCtx.UserID, room)

	// Return SFU config
	iceServers := h.p2pMgr.GetConfig().GetICEServers()
	return &SFUConfigPayload{
		RoomID:       roomID,
		ICEServers:   iceServers,
		Participants: room.GetParticipantList(),
		Mode:         "sfu",
		SDP:          offerSDP,
		IsInitiator:  isInitiator, // Set based on whether they created the call
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
			_ = h.callRepo.AddParticipant(ctx, callLog.ID, sigCtx.UserID)
			room.SetCallID(callLog.ID)

			// Broadcast incoming call to other members
			h.broadcastIncomingCall(ctx, roomID, callLog.ID, sigCtx, ct)
		}
	} else if existingCallID != uuid.Nil && h.callRepo != nil {
		_ = h.callRepo.AddParticipant(ctx, existingCallID, sigCtx.UserID)
		if room.ParticipantCount() == 2 {
			_ = h.callRepo.StartCall(ctx, existingCallID)
		}
	}

	iceServers := h.p2pMgr.GetConfig().GetICEServers()
	return &SFUConfigPayload{
		RoomID:       roomID,
		ICEServers:   iceServers,
		Participants: room.GetParticipants(),
		Mode:         "p2p",
		IsInitiator:  isInitiator,
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
		// Capture call ID before removing participant
		callID := room.GetCallID()

		room.RemoveParticipant(sigCtx.UserID)

		// Notify others
		h.broadcastParticipantLeft(ctx, room, sigCtx)

		// Clean up empty room and end call in DB
		if room.ParticipantCount() == 0 {
			h.sfu.DeleteRoom(roomID)

			// End the call in the database (mirrors P2P HandleLeave behavior)
			if callID != uuid.Nil && h.callRepo != nil {
				h.logger.Info("ending SFU call in database", "call_id", callID, "room_id", roomID)
				if err := h.callRepo.EndCall(ctx, callID); err != nil {
					h.logger.Error("failed to end SFU call", "error", err, "call_id", callID)
				}
			}
		}
	}

	h.logger.Info("user left SFU room", "room_id", roomID, "user_id", sigCtx.UserID)
	return nil
}

func (h *SFUHandler) sendAnswerToParticipant(ctx context.Context, userID, roomID uuid.UUID, sdp string) {
	payload := map[string]interface{}{
		"room_id": roomID.String(),
		"sdp":     sdp,
	}
	payloadBytes, _ := json.Marshal(payload)

	msg := &pubsub.Message{
		Topic:   pubsub.Topics.User(userID.String()),
		Type:    EventTypeSFUAnswer,
		Payload: payloadBytes,
	}
	_ = h.pubsub.Publish(ctx, msg.Topic, msg)
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
		_ = h.pubsub.Publish(ctx, msg.Topic, msg)
	}
}

func (h *SFUHandler) sendTrackInfo(ctx context.Context, userID uuid.UUID, room *SFURoom) {
	// Get all tracks from all participants
	tracks := room.GetTracks()

	payload := SFUTracksPayload{
		RoomID: room.ID.String(),
		Tracks: tracks,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		h.logger.Error("failed to marshal track info", "error", err)
		return
	}

	// Send to the specific user
	msg := &pubsub.Message{
		Topic:   pubsub.Topics.User(userID.String()),
		Type:    EventTypeSFUTracks,
		Payload: payloadBytes,
	}
	_ = h.pubsub.Publish(ctx, msg.Topic, msg)
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
		_ = h.pubsub.Publish(ctx, msg.Topic, msg)
	}
}

// HandleSFUMuteUpdate processes a call.mute_update message for SFU group calls
func (h *SFUHandler) HandleSFUMuteUpdate(ctx context.Context, sigCtx *SignalingContext, payload json.RawMessage) error {
	var p struct {
		RoomID string `json:"room_id"`
		Kind   string `json:"kind"`
		Muted  bool   `json:"muted"`
	}
	if err := json.Unmarshal(payload, &p); err != nil {
		return &CallError{Code: "invalid_payload", Message: "Invalid mute update payload"}
	}

	roomID, err := uuid.Parse(p.RoomID)
	if err != nil {
		return &CallError{Code: "invalid_room", Message: "Invalid room ID"}
	}

	room := h.sfu.GetRoom(roomID)
	if room == nil {
		return &CallError{Code: "room_not_found", Message: "Room not found"}
	}

	// Relay mute update to other participants in the SFU room
	relayPayload := map[string]interface{}{
		"room_id": roomID.String(),
		"user_id": sigCtx.UserID.String(),
		"kind":    p.Kind,
		"muted":   p.Muted,
	}
	payloadBytes, _ := json.Marshal(relayPayload)

	room.mu.RLock()
	defer room.mu.RUnlock()

	for _, participant := range room.participants {
		if participant.UserID == sigCtx.UserID {
			continue
		}
		msg := &pubsub.Message{
			Topic:   pubsub.Topics.User(participant.UserID.String()),
			Type:    EventTypeCallMuteUpdate,
			Payload: payloadBytes,
		}
		_ = h.pubsub.Publish(ctx, msg.Topic, msg)
	}

	return nil
}

// IsUserInSFURoom checks if a user is in an SFU room
func (h *SFUHandler) IsUserInSFURoom(roomID, userID uuid.UUID) bool {
	room := h.sfu.GetRoom(roomID)
	if room == nil {
		return false
	}
	return room.GetParticipant(userID) != nil
}

func (h *SFUHandler) broadcastIncomingCall(ctx context.Context, conversationID, callID uuid.UUID, caller *SignalingContext, callType database.CallType) {
	// Get all conversation members
	members, err := h.convRepo.GetByID(ctx, conversationID)
	if err != nil {
		h.logger.Error("failed to get conversation members for broadcast", "error", err)
		return
	}

	isGroup := false
	if len(members.Members) > 2 {
		isGroup = true
	}

	incomingPayload := CallIncomingPayload{
		CallID:           callID,
		ConversationID:   conversationID,
		ConversationName: members.Title, // might be empty for DMs
		CallerID:         caller.UserID,
		CallerName:       caller.Username,
		CallType:         string(callType),
		IsGroup:          isGroup,
	}

	payloadBytes, err := json.Marshal(incomingPayload)
	if err != nil {
		h.logger.Error("failed to marshal incoming call payload", "error", err)
		return
	}

	for _, member := range members.Members {
		// Don't send to caller
		if member.UserID == caller.UserID {
			continue
		}

		msg := &pubsub.Message{
			Topic:   pubsub.Topics.User(member.UserID.String()),
			Type:    EventTypeCallIncoming,
			Payload: payloadBytes,
		}

		if err := h.pubsub.Publish(ctx, msg.Topic, msg); err != nil {
			h.logger.Error("failed to publish incoming call event", "error", err, "target_user", member.UserID)
		}
	}
}
