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

// CallHandler processes WebRTC signaling messages from WebSocket
type CallHandler struct {
	manager  *Manager
	convRepo *database.ConversationRepository
	callRepo *database.CallRepository
	pubsub   pubsub.PubSub
	logger   *slog.Logger
}

// NewCallHandler creates a new call handler
func NewCallHandler(mgr *Manager, convRepo *database.ConversationRepository, callRepo *database.CallRepository, ps pubsub.PubSub, logger *slog.Logger) *CallHandler {
	return &CallHandler{
		manager:  mgr,
		convRepo: convRepo,
		callRepo: callRepo,
		pubsub:   ps,
		logger:   logger,
	}
}

// SignalingContext provides user context for call handling
type SignalingContext struct {
	UserID   uuid.UUID
	Username string
}

// HandleJoin processes a call.join message
func (h *CallHandler) HandleJoin(ctx context.Context, sigCtx *SignalingContext, payload json.RawMessage) (*CallConfigPayload, error) {
	var p CallJoinPayload
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

	// Join the call first - this is atomic
	room, err := h.manager.JoinCall(ctx, roomID, sigCtx.UserID, sigCtx.Username)
	if err != nil {
		return nil, &CallError{Code: "join_failed", Message: err.Error()}
	}

	// Check if this user is the call initiator (first participant)
	// We determine this by checking if the room has a CallID already set
	existingCallID := room.GetCallID()
	isInitiator := existingCallID == uuid.Nil

	// If there's an existing call ID, verify it's still active in the database
	// This handles cases where the room persisted but the call ended (e.g., due to disconnects)
	if !isInitiator && h.callRepo != nil {
		isActive, err := h.callRepo.IsCallActive(ctx, existingCallID)
		if err != nil {
			h.logger.Error("failed to check if call is active", "error", err, "call_id", existingCallID)
		} else if !isActive {
			// Call ended but room still exists - clear the CallID and treat as new call
			h.logger.Info("existing call ID found but call is no longer active, treating as new call",
				"room_id", roomID,
				"old_call_id", existingCallID)
			room.SetCallID(uuid.Nil)
			existingCallID = uuid.Nil
			isInitiator = true
		}
	}

	h.logger.Info("user joining call",
		"room_id", roomID,
		"user_id", sigCtx.UserID,
		"username", sigCtx.Username,
		"is_initiator", isInitiator,
		"existing_call_id", existingCallID,
		"participant_count", room.ParticipantCount())

	if isInitiator && h.callRepo != nil {
		// This is the call initiator - create call log and notify others
		callType := database.CallTypeVideo

		callLog, err := h.callRepo.CreateCallLog(ctx, roomID, sigCtx.UserID, callType)
		if err != nil {
			h.logger.Error("failed to create call log", "error", err)
		} else {
			// Add initiator as participant
			_ = h.callRepo.AddParticipant(ctx, callLog.ID, sigCtx.UserID)

			// Store call ID in room for later reference
			room.SetCallID(callLog.ID)

			// Notify other conversation members about incoming call
			h.broadcastIncomingCall(ctx, roomID, callLog.ID, sigCtx, callType)
		}
	} else if existingCallID != uuid.Nil && h.callRepo != nil {
		// Joining existing call - add as participant and start call if needed
		_ = h.callRepo.AddParticipant(ctx, existingCallID, sigCtx.UserID)

		// If this is the second person, mark call as started
		if room.ParticipantCount() == 2 {
			_ = h.callRepo.StartCall(ctx, existingCallID)
		}
	}

	// Return config with ICE servers and current participants
	config := &CallConfigPayload{
		RoomID:       roomID,
		ICEServers:   h.manager.GetConfig().GetICEServers(),
		Participants: room.GetParticipants(),
	}
	
	h.logger.Info("sending call config",
		"room_id", roomID,
		"user_id", sigCtx.UserID,
		"participant_count", len(config.Participants),
		"participants", config.Participants)
	
	return config, nil
}

// broadcastIncomingCall notifies other conversation members about an incoming call
func (h *CallHandler) broadcastIncomingCall(ctx context.Context, conversationID, callID uuid.UUID, caller *SignalingContext, callType database.CallType) {
	// Get conversation details (includes members)
	conv, err := h.convRepo.GetByID(ctx, conversationID)
	if err != nil {
		h.logger.Error("failed to get conversation for call notification", "error", err)
		return
	}

	h.logger.Info("broadcasting incoming call",
		"conversation_id", conversationID,
		"call_id", callID,
		"caller_id", caller.UserID,
		"caller_name", caller.Username,
		"member_count", len(conv.Members),
		"conv_type", conv.Type)

	incomingPayload := CallIncomingPayload{
		CallID:           callID,
		ConversationID:   conversationID,
		CallerID:         caller.UserID,
		CallerName:       caller.Username,
		CallType:         string(callType),
		IsGroup:          conv.Type == domain.ConversationTypeGroup,
		ConversationName: conv.Title,
	}

	payloadBytes, _ := json.Marshal(incomingPayload)

	// Notify all members except the caller
	for _, member := range conv.Members {
		h.logger.Debug("checking member for notification",
			"member_id", member.UserID,
			"caller_id", caller.UserID,
			"is_caller", member.UserID == caller.UserID)

		if member.UserID == caller.UserID {
			continue
		}

		topic := pubsub.Topics.User(member.UserID.String())
		h.logger.Info("sending call.incoming to user",
			"user_id", member.UserID,
			"topic", topic)

		msg := &pubsub.Message{
			Topic:   topic,
			Type:    EventTypeCallIncoming,
			Payload: payloadBytes,
		}
		if err := h.pubsub.Publish(ctx, msg.Topic, msg); err != nil {
			h.logger.Error("failed to send incoming call notification", "user_id", member.UserID, "error", err)
		} else {
			h.logger.Info("successfully published call.incoming", "user_id", member.UserID)
		}
	}
}

// HandleLeave processes a call.leave message
func (h *CallHandler) HandleLeave(ctx context.Context, sigCtx *SignalingContext, payload json.RawMessage) error {
	var p CallLeavePayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return &CallError{Code: "invalid_payload", Message: "Invalid leave payload"}
	}

	roomID, err := uuid.Parse(p.RoomID)
	if err != nil {
		return &CallError{Code: "invalid_room", Message: "Invalid room ID"}
	}

	// Get room before leaving to check if it will become empty
	room := h.manager.GetRoom(roomID)
	var callID uuid.UUID
	if room != nil {
		callID = room.GetCallID()
	}

	h.manager.LeaveCall(ctx, roomID, sigCtx.UserID, sigCtx.Username)

	// If the room was deleted (became empty), end the call in the database
	if room != nil && h.manager.GetRoom(roomID) == nil && callID != uuid.Nil && h.callRepo != nil {
		h.logger.Info("ending call in database", "call_id", callID)
		_ = h.callRepo.EndCall(ctx, callID)
	}

	return nil
}

// HandleOffer relays an SDP offer to target participant
func (h *CallHandler) HandleOffer(ctx context.Context, sigCtx *SignalingContext, payload json.RawMessage) error {
	var p CallOfferPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return &CallError{Code: "invalid_payload", Message: "Invalid offer payload"}
	}

	roomID, err := uuid.Parse(p.RoomID)
	if err != nil {
		return &CallError{Code: "invalid_room", Message: "Invalid room ID"}
	}

	targetID, err := uuid.Parse(p.TargetID)
	if err != nil {
		return &CallError{Code: "invalid_target", Message: "Invalid target ID"}
	}

	h.logger.Info("relaying offer", "from", sigCtx.UserID, "to", targetID, "room", roomID)

	// Verify both users are in the call
	room := h.manager.GetRoom(roomID)
	if room == nil {
		return &CallError{Code: "no_call", Message: "No active call in this room"}
	}

	// Relay the offer to target user via pubsub
	relayPayload := map[string]interface{}{
		"room_id":   roomID.String(),
		"from_id":   sigCtx.UserID.String(),
		"from_name": sigCtx.Username,
		"sdp":       p.SDP,
	}
	payloadBytes, _ := json.Marshal(relayPayload)

	msg := &pubsub.Message{
		Topic:   pubsub.Topics.User(targetID.String()),
		Type:    EventTypeCallOffer,
		Payload: payloadBytes,
	}
	return h.pubsub.Publish(ctx, msg.Topic, msg)
}

// HandleAnswer relays an SDP answer to target participant
func (h *CallHandler) HandleAnswer(ctx context.Context, sigCtx *SignalingContext, payload json.RawMessage) error {
	var p CallAnswerPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return &CallError{Code: "invalid_payload", Message: "Invalid answer payload"}
	}

	roomID, err := uuid.Parse(p.RoomID)
	if err != nil {
		return &CallError{Code: "invalid_room", Message: "Invalid room ID"}
	}

	targetID, err := uuid.Parse(p.TargetID)
	if err != nil {
		return &CallError{Code: "invalid_target", Message: "Invalid target ID"}
	}

	h.logger.Info("relaying answer", "from", sigCtx.UserID, "to", targetID, "room", roomID)

	// Verify room exists
	room := h.manager.GetRoom(roomID)
	if room == nil {
		return &CallError{Code: "no_call", Message: "No active call in this room"}
	}

	// Relay the answer to target user
	relayPayload := map[string]interface{}{
		"room_id":   roomID.String(),
		"from_id":   sigCtx.UserID.String(),
		"from_name": sigCtx.Username,
		"sdp":       p.SDP,
	}
	payloadBytes, _ := json.Marshal(relayPayload)

	msg := &pubsub.Message{
		Topic:   pubsub.Topics.User(targetID.String()),
		Type:    EventTypeCallAnswer,
		Payload: payloadBytes,
	}
	return h.pubsub.Publish(ctx, msg.Topic, msg)
}

// HandleICECandidate relays an ICE candidate to target participant
func (h *CallHandler) HandleICECandidate(ctx context.Context, sigCtx *SignalingContext, payload json.RawMessage) error {
	var p CallICECandidatePayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return &CallError{Code: "invalid_payload", Message: "Invalid ICE candidate payload"}
	}

	roomID, err := uuid.Parse(p.RoomID)
	if err != nil {
		return &CallError{Code: "invalid_room", Message: "Invalid room ID"}
	}

	targetID, err := uuid.Parse(p.TargetID)
	if err != nil {
		return &CallError{Code: "invalid_target", Message: "Invalid target ID"}
	}

	// Verify room exists
	room := h.manager.GetRoom(roomID)
	if room == nil {
		return &CallError{Code: "no_call", Message: "No active call in this room"}
	}

	// Relay the ICE candidate to target user
	relayPayload := map[string]interface{}{
		"room_id":   roomID.String(),
		"from_id":   sigCtx.UserID.String(),
		"candidate": p.Candidate,
	}
	payloadBytes, _ := json.Marshal(relayPayload)

	msg := &pubsub.Message{
		Topic:   pubsub.Topics.User(targetID.String()),
		Type:    EventTypeCallICECandidate,
		Payload: payloadBytes,
	}
	return h.pubsub.Publish(ctx, msg.Topic, msg)
}

// HandleDeclined processes a call.declined message
func (h *CallHandler) HandleDeclined(ctx context.Context, sigCtx *SignalingContext, payload json.RawMessage) error {
	var p struct {
		CallID         string `json:"call_id"`
		ConversationID string `json:"conversation_id"`
	}
	if err := json.Unmarshal(payload, &p); err != nil {
		return &CallError{Code: "invalid_payload", Message: "Invalid decline payload"}
	}

	callID, err := uuid.Parse(p.CallID)
	if err != nil {
		return &CallError{Code: "invalid_call_id", Message: "Invalid call ID"}
	}

	// Update call status in database
	if h.callRepo != nil {
		if err := h.callRepo.UpdateCallStatus(ctx, callID, database.CallStatusDeclined); err != nil {
			h.logger.Error("failed to update call status to declined", "error", err, "call_id", callID)
		}
	}

	// Get call log to find the caller
	call, err := h.callRepo.GetCallLog(ctx, callID)
	if err != nil {
		h.logger.Error("failed to get call log for decline", "error", err, "call_id", callID)
		return &CallError{Code: "call_not_found", Message: "Call not found"}
	}

	// Notify the caller that the call was declined
	// The caller is the one who initiated the call (call.InitiatorID)
	// But we should verify if the current user is actually a participant/callee?
	// For now, just notifying the caller is sufficient.

	relayPayload := map[string]interface{}{
		"call_id":         callID.String(),
		"conversation_id": p.ConversationID,
		"decliner_id":     sigCtx.UserID.String(),
		"decliner_name":   sigCtx.Username,
	}
	payloadBytes, _ := json.Marshal(relayPayload)

	// Send to caller
	callerTopic := pubsub.Topics.User(call.InitiatorID.String())
	msg := &pubsub.Message{
		Topic:   callerTopic,
		Type:    EventTypeCallDeclined,
		Payload: payloadBytes,
	}
	
	h.logger.Info("relaying call declined", "from", sigCtx.UserID, "to", call.InitiatorID)
	
	return h.pubsub.Publish(ctx, msg.Topic, msg)
}

// CallError represents an error during call handling
type CallError struct {
	Code    string
	Message string
}

func (e *CallError) Error() string {
	return e.Message
}
