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
type CallICECandidatePayload struct {
	RoomID    string `json:"room_id"`
	TargetID  string `json:"target_id"`
	Candidate string `json:"candidate"`
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
}

// CallErrorPayload is sent when an error occurs
type CallErrorPayload struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
