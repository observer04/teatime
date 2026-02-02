package database

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
)

// CallStatus represents the status of a call
type CallStatus string

const (
	CallStatusRinging   CallStatus = "ringing"
	CallStatusActive    CallStatus = "active"
	CallStatusEnded     CallStatus = "ended"
	CallStatusMissed    CallStatus = "missed"
	CallStatusDeclined  CallStatus = "declined"
	CallStatusCancelled CallStatus = "cancelled"
)

// CallType represents the type of call
type CallType string

const (
	CallTypeVideo CallType = "video"
	CallTypeAudio CallType = "audio"
)

// CallLog represents a call log entry
type CallLog struct {
	ID              uuid.UUID  `json:"id"`
	ConversationID  uuid.UUID  `json:"conversation_id"`
	InitiatorID     uuid.UUID  `json:"initiator_id"`
	CallType        CallType   `json:"call_type"`
	Status          CallStatus `json:"status"`
	StartedAt       *time.Time `json:"started_at,omitempty"`
	EndedAt         *time.Time `json:"ended_at,omitempty"`
	DurationSeconds int        `json:"duration_seconds"`
	CreatedAt       time.Time  `json:"created_at"`

	// Populated from joins
	InitiatorUsername string            `json:"initiator_username,omitempty"`
	ConversationTitle string            `json:"conversation_title,omitempty"`
	ConversationType  string            `json:"conversation_type,omitempty"`
	OtherUser         *UserSummary      `json:"other_user,omitempty"` // For DMs
	Participants      []CallParticipant `json:"participants,omitempty"`
}

// CallParticipant represents a user who joined a call
type CallParticipant struct {
	UserID   uuid.UUID  `json:"user_id"`
	Username string     `json:"username"`
	JoinedAt time.Time  `json:"joined_at"`
	LeftAt   *time.Time `json:"left_at,omitempty"`
}

// UserSummary is a minimal user representation
type UserSummary struct {
	ID        uuid.UUID `json:"id"`
	Username  string    `json:"username"`
	AvatarURL *string   `json:"avatar_url,omitempty"`
}

// CallRepository handles call-related database operations
type CallRepository struct {
	db *DB
}

// NewCallRepository creates a new CallRepository
func NewCallRepository(db *DB) *CallRepository {
	return &CallRepository{db: db}
}

// CreateCallLog creates a new call log entry
func (r *CallRepository) CreateCallLog(ctx context.Context, conversationID, initiatorID uuid.UUID, callType CallType) (*CallLog, error) {
	call := &CallLog{
		ID:             uuid.New(),
		ConversationID: conversationID,
		InitiatorID:    initiatorID,
		CallType:       callType,
		Status:         CallStatusRinging,
		CreatedAt:      time.Now(),
	}

	query := `
		INSERT INTO call_logs (id, conversation_id, initiator_id, call_type, status, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`

	_, err := r.db.Pool.Exec(ctx, query,
		call.ID, call.ConversationID, call.InitiatorID, call.CallType, call.Status, call.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	return call, nil
}

// UpdateCallStatus updates the status of a call
func (r *CallRepository) UpdateCallStatus(ctx context.Context, callID uuid.UUID, status CallStatus) error {
	query := `UPDATE call_logs SET status = $2 WHERE id = $1`
	_, err := r.db.Pool.Exec(ctx, query, callID, status)
	return err
}

// StartCall marks a call as active with a start time
func (r *CallRepository) StartCall(ctx context.Context, callID uuid.UUID) error {
	query := `UPDATE call_logs SET status = 'active', started_at = NOW() WHERE id = $1`
	_, err := r.db.Pool.Exec(ctx, query, callID)
	return err
}

// EndCall marks a call as ended and calculates duration
func (r *CallRepository) EndCall(ctx context.Context, callID uuid.UUID) error {
	query := `
		UPDATE call_logs 
		SET status = 'ended', 
		    ended_at = NOW(),
		    duration_seconds = CASE 
		        WHEN started_at IS NOT NULL THEN EXTRACT(EPOCH FROM (NOW() - started_at))::INTEGER
		        ELSE 0
		    END
		WHERE id = $1
	`
	_, err := r.db.Pool.Exec(ctx, query, callID)
	return err
}

// AddParticipant adds a participant to a call
func (r *CallRepository) AddParticipant(ctx context.Context, callID, userID uuid.UUID) error {
	query := `
		INSERT INTO call_participants (call_id, user_id, joined_at)
		VALUES ($1, $2, NOW())
		ON CONFLICT (call_id, user_id) DO UPDATE SET joined_at = NOW(), left_at = NULL
	`
	_, err := r.db.Pool.Exec(ctx, query, callID, userID)
	return err
}

// RemoveParticipant marks a participant as having left the call
func (r *CallRepository) RemoveParticipant(ctx context.Context, callID, userID uuid.UUID) error {
	query := `UPDATE call_participants SET left_at = NOW() WHERE call_id = $1 AND user_id = $2`
	_, err := r.db.Pool.Exec(ctx, query, callID, userID)
	return err
}

// GetCallLog retrieves a call log by ID
func (r *CallRepository) GetCallLog(ctx context.Context, callID uuid.UUID) (*CallLog, error) {
	query := `
		SELECT 
			cl.id, cl.conversation_id, cl.initiator_id, cl.call_type, cl.status,
			cl.started_at, cl.ended_at, cl.duration_seconds, cl.created_at,
			u.username as initiator_username,
			c.title as conversation_title, c.type as conversation_type
		FROM call_logs cl
		JOIN users u ON u.id = cl.initiator_id
		JOIN conversations c ON c.id = cl.conversation_id
		WHERE cl.id = $1
	`

	var call CallLog
	var startedAt, endedAt sql.NullTime
	var convTitle sql.NullString

	err := r.db.Pool.QueryRow(ctx, query, callID).Scan(
		&call.ID, &call.ConversationID, &call.InitiatorID, &call.CallType, &call.Status,
		&startedAt, &endedAt, &call.DurationSeconds, &call.CreatedAt,
		&call.InitiatorUsername, &convTitle, &call.ConversationType,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	if startedAt.Valid {
		call.StartedAt = &startedAt.Time
	}
	if endedAt.Valid {
		call.EndedAt = &endedAt.Time
	}
	if convTitle.Valid {
		call.ConversationTitle = convTitle.String
	}

	return &call, nil
}

// GetActiveCallForConversation finds an active/ringing call for a conversation
func (r *CallRepository) GetActiveCallForConversation(ctx context.Context, conversationID uuid.UUID) (*CallLog, error) {
	query := `
		SELECT id, conversation_id, initiator_id, call_type, status, 
		       started_at, ended_at, duration_seconds, created_at
		FROM call_logs
		WHERE conversation_id = $1 AND status IN ('ringing', 'active')
		ORDER BY created_at DESC
		LIMIT 1
	`

	var call CallLog
	var startedAt, endedAt sql.NullTime

	err := r.db.Pool.QueryRow(ctx, query, conversationID).Scan(
		&call.ID, &call.ConversationID, &call.InitiatorID, &call.CallType, &call.Status,
		&startedAt, &endedAt, &call.DurationSeconds, &call.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil // No active call
		}
		return nil, err
	}

	if startedAt.Valid {
		call.StartedAt = &startedAt.Time
	}
	if endedAt.Valid {
		call.EndedAt = &endedAt.Time
	}

	return &call, nil
}

// GetUserCallHistory retrieves call history for a user
func (r *CallRepository) GetUserCallHistory(ctx context.Context, userID uuid.UUID, limit, offset int) ([]CallLog, error) {
	query := `
		SELECT DISTINCT ON (cl.id)
			cl.id, cl.conversation_id, cl.initiator_id, cl.call_type, cl.status,
			cl.started_at, cl.ended_at, cl.duration_seconds, cl.created_at,
			u.username as initiator_username,
			c.title as conversation_title, c.type as conversation_type
		FROM call_logs cl
		JOIN users u ON u.id = cl.initiator_id
		JOIN conversations c ON c.id = cl.conversation_id
		LEFT JOIN call_participants cp ON cp.call_id = cl.id
		WHERE cl.initiator_id = $1 
		   OR cp.user_id = $1
		   OR EXISTS (
		       SELECT 1 FROM conversation_members cm 
		       WHERE cm.conversation_id = cl.conversation_id AND cm.user_id = $1
		   )
		ORDER BY cl.id, cl.created_at DESC
		LIMIT $2 OFFSET $3
	`

	// Need a separate query to properly order by created_at DESC
	wrapperQuery := `
		SELECT * FROM (` + query + `) sub
		ORDER BY created_at DESC
	`

	rows, err := r.db.Pool.Query(ctx, wrapperQuery, userID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var calls []CallLog
	for rows.Next() {
		var call CallLog
		var startedAt, endedAt sql.NullTime
		var convTitle sql.NullString

		if err := rows.Scan(
			&call.ID, &call.ConversationID, &call.InitiatorID, &call.CallType, &call.Status,
			&startedAt, &endedAt, &call.DurationSeconds, &call.CreatedAt,
			&call.InitiatorUsername, &convTitle, &call.ConversationType,
		); err != nil {
			return nil, err
		}

		if startedAt.Valid {
			call.StartedAt = &startedAt.Time
		}
		if endedAt.Valid {
			call.EndedAt = &endedAt.Time
		}
		if convTitle.Valid {
			call.ConversationTitle = convTitle.String
		}

		calls = append(calls, call)
	}

	return calls, nil
}

// GetUserCallHistoryWithDetails retrieves call history with other user info for DMs
func (r *CallRepository) GetUserCallHistoryWithDetails(ctx context.Context, userID uuid.UUID, limit, offset int) ([]CallLog, error) {
	calls, err := r.GetUserCallHistory(ctx, userID, limit, offset)
	if err != nil {
		return nil, err
	}

	// For each DM call, fetch the other user's info
	for i := range calls {
		if calls[i].ConversationType == "dm" {
			otherUser, err := r.getOtherUserInDM(ctx, calls[i].ConversationID, userID)
			if err == nil {
				calls[i].OtherUser = otherUser
			}
		}
	}

	return calls, nil
}

func (r *CallRepository) getOtherUserInDM(ctx context.Context, conversationID, currentUserID uuid.UUID) (*UserSummary, error) {
	query := `
		SELECT u.id, u.username, u.avatar_url
		FROM conversation_members cm
		JOIN users u ON u.id = cm.user_id
		WHERE cm.conversation_id = $1 AND cm.user_id != $2
		LIMIT 1
	`

	var user UserSummary
	var avatarURL sql.NullString

	err := r.db.Pool.QueryRow(ctx, query, conversationID, currentUserID).Scan(
		&user.ID, &user.Username, &avatarURL,
	)
	if err != nil {
		return nil, err
	}

	if avatarURL.Valid {
		user.AvatarURL = &avatarURL.String
	}

	return &user, nil
}

// IsCallActive checks if a specific call is still active/ringing
func (r *CallRepository) IsCallActive(ctx context.Context, callID uuid.UUID) (bool, error) {
	query := `
		SELECT EXISTS(
			SELECT 1 FROM call_logs 
			WHERE id = $1 AND status IN ('ringing', 'active')
		)
	`

	var exists bool
	err := r.db.Pool.QueryRow(ctx, query, callID).Scan(&exists)
	return exists, err
}

// GetMissedCallCount returns the count of missed calls for a user since a given time
func (r *CallRepository) GetMissedCallCount(ctx context.Context, userID uuid.UUID, since time.Time) (int, error) {
	query := `
		SELECT COUNT(DISTINCT cl.id)
		FROM call_logs cl
		JOIN conversation_members cm ON cm.conversation_id = cl.conversation_id
		WHERE cm.user_id = $1 
		  AND cl.initiator_id != $1
		  AND cl.status = 'missed'
		  AND cl.created_at > $2
	`

	var count int
	err := r.db.Pool.QueryRow(ctx, query, userID, since).Scan(&count)
	return count, err
}
