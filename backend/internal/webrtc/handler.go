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

	// Check if this is a new call or joining existing
	room := h.manager.GetRoom(roomID)
	isNewCall := room == nil || room.ParticipantCount() == 0

	// Join the call
	room, err = h.manager.JoinCall(ctx, roomID, sigCtx.UserID, sigCtx.Username)
	if err != nil {
		return nil, &CallError{Code: "join_failed", Message: err.Error()}
	}

	// If this is a new call, create call log and notify other members
	if isNewCall && h.callRepo != nil {
		// Determine call type from payload (default to video)
		callType := database.CallTypeVideo
		
		// Create call log
		callLog, err := h.callRepo.CreateCallLog(ctx, roomID, sigCtx.UserID, callType)
		if err != nil {
			h.logger.Error("failed to create call log", "error", err)
		} else {
			// Add initiator as participant
			h.callRepo.AddParticipant(ctx, callLog.ID, sigCtx.UserID)
			
			// Store call ID in room for later reference
			room.SetCallID(callLog.ID)
			
			// Notify other conversation members about incoming call
			h.broadcastIncomingCall(ctx, roomID, callLog.ID, sigCtx, callType)
		}
	} else if room.GetCallID() != uuid.Nil && h.callRepo != nil {
		// Joining existing call - add as participant and start call if needed
		h.callRepo.AddParticipant(ctx, room.GetCallID(), sigCtx.UserID)
		
		// If this is the second person, start the call
		if room.ParticipantCount() == 2 {
			h.callRepo.StartCall(ctx, room.GetCallID())
		}
	}

	// Return config with ICE servers and current participants
	return &CallConfigPayload{
		RoomID:       roomID,
		ICEServers:   h.manager.GetConfig().GetICEServers(),
		Participants: room.GetParticipants(),
	}, nil
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

	h.manager.LeaveCall(ctx, roomID, sigCtx.UserID, sigCtx.Username)
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

// CallError represents an error during call handling
type CallError struct {
	Code    string
	Message string
}

func (e *CallError) Error() string {
	return e.Message
}
