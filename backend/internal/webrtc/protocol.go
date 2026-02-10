package webrtc

import "github.com/google/uuid"

// WebSocket event types for call signaling
const (
	EventTypeCallJoin              = "call.join"
	EventTypeCallLeave             = "call.leave"
	EventTypeCallOffer             = "call.offer"
	EventTypeCallAnswer            = "call.answer"
	EventTypeCallICECandidate      = "call.ice_candidate"
	EventTypeCallParticipantJoined = "call.participant_joined"
	EventTypeCallParticipantLeft   = "call.participant_left"
	EventTypeCallConfig            = "call.config"
	EventTypeCallError             = "call.error"
	// Incoming call events
	EventTypeCallIncoming   = "call.incoming"    // Sent to other members when someone starts a call
	EventTypeCallAccepted   = "call.accepted"    // Sent when someone accepts the call
	EventTypeCallDeclined   = "call.declined"    // Sent when someone declines the call
	EventTypeCallCancelled  = "call.cancelled"   // Sent when caller cancels before answer
	EventTypeCallEnded      = "call.ended"       // Sent when call ends
	EventTypeCallReady      = "call.ready"       // Sent when participant is ready for offer
	EventTypeCallMuteUpdate = "call.mute_update" // Sent when participant toggles mute/video
	EventTypeCallMigration  = "call.migration"   // Sent when P2P call migrates to SFU

	// SFU Events
	// Note: EventTypeSFUJoin exists for completeness but the frontend always sends
	// EventTypeCallJoin which is auto-routed to SFU by the hub when sfuHandler is available.
	EventTypeSFUJoin       = "sfu.join"
	EventTypeSFULeave      = "sfu.leave"
	EventTypeSFUOffer      = "sfu.offer"
	EventTypeSFUAnswer     = "sfu.answer"
	EventTypeSFUCandidate  = "sfu.candidate"
	EventTypeSFUTracks     = "sfu.tracks"
	EventTypeSFUMuteUpdate = "sfu.mute_update"
)

// CallJoinPayload is sent by client to join a call
type CallJoinPayload struct {
	RoomID string `json:"room_id"` // conversation_id
}

// CallLeavePayload is sent by client to leave a call
type CallLeavePayload struct {
	RoomID string `json:"room_id"`
}

// CallOfferPayload contains an SDP offer for WebRTC
type CallOfferPayload struct {
	RoomID   string `json:"room_id"`
	TargetID string `json:"target_id"` // user to send offer to
	SDP      string `json:"sdp"`
}

// CallAnswerPayload contains an SDP answer for WebRTC
type CallAnswerPayload struct {
	RoomID   string `json:"room_id"`
	TargetID string `json:"target_id"`
	SDP      string `json:"sdp"`
}

// CallICECandidatePayload contains ICE candidate info
// CallICECandidatePayload contains ICE candidate info
type CallICECandidatePayload struct {
	RoomID    string      `json:"room_id"`
	TargetID  string      `json:"target_id"`
	Candidate interface{} `json:"candidate"`
}

// CallParticipantEvent is sent when someone joins/leaves
type CallParticipantEvent struct {
	RoomID   uuid.UUID `json:"room_id"`
	UserID   uuid.UUID `json:"user_id"`
	Username string    `json:"username"`
	Action   string    `json:"action"` // "joined" or "left"
}

// CallConfigPayload is sent to client after joining
type CallConfigPayload struct {
	RoomID       uuid.UUID     `json:"room_id"`
	ICEServers   []ICEServer   `json:"ice_servers"`
	Participants []Participant `json:"participants"`
	IsInitiator  bool          `json:"is_initiator"`
}

// CallErrorPayload is sent when an error occurs
type CallErrorPayload struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// CallIncomingPayload is sent to other conversation members when a call starts
type CallIncomingPayload struct {
	CallID           uuid.UUID `json:"call_id"`
	ConversationID   uuid.UUID `json:"conversation_id"`
	ConversationName string    `json:"conversation_name,omitempty"`
	CallerID         uuid.UUID `json:"caller_id"`
	CallerName       string    `json:"caller_name"`
	CallerAvatar     string    `json:"caller_avatar,omitempty"`
	CallType         string    `json:"call_type"` // "video" or "audio"
	IsGroup          bool      `json:"is_group"`
}

// CallAcceptedPayload is sent when someone accepts the call
type CallAcceptedPayload struct {
	CallID uuid.UUID `json:"call_id"`
	UserID uuid.UUID `json:"user_id"`
}

// CallDeclinedPayload is sent when someone declines the call
type CallDeclinedPayload struct {
	CallID uuid.UUID `json:"call_id"`
	UserID uuid.UUID `json:"user_id"`
}

// CallCancelledPayload is sent when caller cancels before anyone answers
type CallCancelledPayload struct {
	CallID   uuid.UUID `json:"call_id"`
	CallerID uuid.UUID `json:"caller_id"`
}

// CallEndedPayload is sent when call ends
type CallEndedPayload struct {
	CallID          uuid.UUID `json:"call_id"`
	DurationSeconds int       `json:"duration_seconds"`
}
