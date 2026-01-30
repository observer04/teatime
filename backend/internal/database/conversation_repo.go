package database

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/observer/teatime/internal/domain"
)

// ConversationRepository handles conversation and message data access
type ConversationRepository struct {
	db *DB
}

func NewConversationRepository(db *DB) *ConversationRepository {
	return &ConversationRepository{db: db}
}

// Create creates a new conversation with initial members
func (r *ConversationRepository) Create(ctx context.Context, conv *domain.Conversation, memberIDs []uuid.UUID) error {
	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// Insert conversation
	_, err = tx.Exec(ctx, `
		INSERT INTO conversations (id, type, title, created_by)
		VALUES ($1, $2, $3, $4)
	`, conv.ID, conv.Type, conv.Title, conv.CreatedBy)
	if err != nil {
		return err
	}

	// Insert members
	for _, userID := range memberIDs {
		role := domain.MemberRoleMember
		if conv.CreatedBy != nil && *conv.CreatedBy == userID {
			role = domain.MemberRoleAdmin
		}
		_, err = tx.Exec(ctx, `
			INSERT INTO conversation_members (conversation_id, user_id, role)
			VALUES ($1, $2, $3)
		`, conv.ID, userID, role)
		if err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

// GetByID retrieves a conversation with its members
func (r *ConversationRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Conversation, error) {
	conv := &domain.Conversation{}
	err := r.db.Pool.QueryRow(ctx, `
		SELECT id, type, title, created_by, created_at, updated_at
		FROM conversations WHERE id = $1
	`, id).Scan(
		&conv.ID, &conv.Type, &conv.Title,
		&conv.CreatedBy, &conv.CreatedAt, &conv.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrConversationNotFound
	}
	if err != nil {
		return nil, err
	}

	// Fetch members with user info
	rows, err := r.db.Pool.Query(ctx, `
		SELECT cm.conversation_id, cm.user_id, cm.role, cm.joined_at,
		       u.id, u.username, u.display_name, u.avatar_url
		FROM conversation_members cm
		JOIN users u ON u.id = cm.user_id
		WHERE cm.conversation_id = $1
	`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var m domain.ConversationMember
		var user domain.PublicUser
		err := rows.Scan(
			&m.ConversationID, &m.UserID, &m.Role, &m.JoinedAt,
			&user.ID, &user.Username, &user.DisplayName, &user.AvatarURL,
		)
		if err != nil {
			return nil, err
		}
		m.User = &user
		conv.Members = append(conv.Members, m)
	}

	return conv, rows.Err()
}

// GetUserConversations returns all conversations for a user
func (r *ConversationRepository) GetUserConversations(ctx context.Context, userID uuid.UUID) ([]domain.Conversation, error) {
	rows, err := r.db.Pool.Query(ctx, `
		SELECT c.id, c.type, c.title, c.created_by, c.created_at, c.updated_at
		FROM conversations c
		JOIN conversation_members cm ON cm.conversation_id = c.id
		WHERE cm.user_id = $1
		ORDER BY c.updated_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var conversations []domain.Conversation
	for rows.Next() {
		var c domain.Conversation
		err := rows.Scan(
			&c.ID, &c.Type, &c.Title,
			&c.CreatedBy, &c.CreatedAt, &c.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		conversations = append(conversations, c)
	}
	return conversations, rows.Err()
}

// IsMember checks if a user is a member of a conversation
func (r *ConversationRepository) IsMember(ctx context.Context, convID, userID uuid.UUID) (bool, error) {
	var exists bool
	err := r.db.Pool.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM conversation_members 
			WHERE conversation_id = $1 AND user_id = $2
		)
	`, convID, userID).Scan(&exists)
	return exists, err
}

// AddMember adds a user to a conversation
func (r *ConversationRepository) AddMember(ctx context.Context, convID, userID uuid.UUID, role domain.MemberRole) error {
	_, err := r.db.Pool.Exec(ctx, `
		INSERT INTO conversation_members (conversation_id, user_id, role)
		VALUES ($1, $2, $3)
		ON CONFLICT DO NOTHING
	`, convID, userID, role)
	return err
}

// RemoveMember removes a user from a conversation
func (r *ConversationRepository) RemoveMember(ctx context.Context, convID, userID uuid.UUID) error {
	_, err := r.db.Pool.Exec(ctx, `
		DELETE FROM conversation_members 
		WHERE conversation_id = $1 AND user_id = $2
	`, convID, userID)
	return err
}

// FindDMBetween finds an existing DM conversation between two users
func (r *ConversationRepository) FindDMBetween(ctx context.Context, user1, user2 uuid.UUID) (*domain.Conversation, error) {
	var convID uuid.UUID
	err := r.db.Pool.QueryRow(ctx, `
		SELECT c.id FROM conversations c
		WHERE c.type = 'dm'
		AND EXISTS (SELECT 1 FROM conversation_members WHERE conversation_id = c.id AND user_id = $1)
		AND EXISTS (SELECT 1 FROM conversation_members WHERE conversation_id = c.id AND user_id = $2)
		AND (SELECT COUNT(*) FROM conversation_members WHERE conversation_id = c.id) = 2
	`, user1, user2).Scan(&convID)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil // No DM exists, not an error
	}
	if err != nil {
		return nil, err
	}
	return r.GetByID(ctx, convID)
}

// ============================================================================
// Message Operations
// ============================================================================

// CreateMessage creates a new message
func (r *ConversationRepository) CreateMessage(ctx context.Context, msg *domain.Message) error {
	_, err := r.db.Pool.Exec(ctx, `
		INSERT INTO messages (id, conversation_id, sender_id, body_text, created_at)
		VALUES ($1, $2, $3, $4, $5)
	`, msg.ID, msg.ConversationID, msg.SenderID, msg.BodyText, msg.CreatedAt)

	if err == nil {
		// Update conversation's updated_at
		r.db.Pool.Exec(ctx, `
			UPDATE conversations SET updated_at = NOW() WHERE id = $1
		`, msg.ConversationID)
	}
	return err
}

// GetMessages retrieves messages with cursor pagination (before timestamp)
func (r *ConversationRepository) GetMessages(ctx context.Context, convID uuid.UUID, before *time.Time, limit int) ([]domain.Message, error) {
	var rows pgx.Rows
	var err error

	if before != nil {
		rows, err = r.db.Pool.Query(ctx, `
			SELECT m.id, m.conversation_id, m.sender_id, m.body_text, m.created_at,
			       u.id, u.username, u.display_name, u.avatar_url
			FROM messages m
			LEFT JOIN users u ON u.id = m.sender_id
			WHERE m.conversation_id = $1 AND m.created_at < $2
			ORDER BY m.created_at DESC
			LIMIT $3
		`, convID, before, limit)
	} else {
		rows, err = r.db.Pool.Query(ctx, `
			SELECT m.id, m.conversation_id, m.sender_id, m.body_text, m.created_at,
			       u.id, u.username, u.display_name, u.avatar_url
			FROM messages m
			LEFT JOIN users u ON u.id = m.sender_id
			WHERE m.conversation_id = $1
			ORDER BY m.created_at DESC
			LIMIT $2
		`, convID, limit)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []domain.Message
	for rows.Next() {
		var m domain.Message
		var senderID *uuid.UUID
		var userID *uuid.UUID
		var username, displayName, avatarURL *string

		err := rows.Scan(
			&m.ID, &m.ConversationID, &senderID, &m.BodyText, &m.CreatedAt,
			&userID, &username, &displayName, &avatarURL,
		)
		if err != nil {
			return nil, err
		}
		m.SenderID = senderID
		if userID != nil {
			m.Sender = &domain.PublicUser{
				ID:          *userID,
				Username:    *username,
				DisplayName: stringValue(displayName),
				AvatarURL:   stringValue(avatarURL),
			}
		}
		messages = append(messages, m)
	}
	return messages, rows.Err()
}

func stringValue(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// ============================================================================
// Block Operations
// ============================================================================

// Block creates a block relationship
func (r *ConversationRepository) Block(ctx context.Context, blockerID, blockedID uuid.UUID) error {
	_, err := r.db.Pool.Exec(ctx, `
		INSERT INTO blocks (blocker_id, blocked_id)
		VALUES ($1, $2)
		ON CONFLICT DO NOTHING
	`, blockerID, blockedID)
	return err
}

// Unblock removes a block relationship
func (r *ConversationRepository) Unblock(ctx context.Context, blockerID, blockedID uuid.UUID) error {
	_, err := r.db.Pool.Exec(ctx, `
		DELETE FROM blocks WHERE blocker_id = $1 AND blocked_id = $2
	`, blockerID, blockedID)
	return err
}

// IsBlocked checks if user1 has blocked user2 OR user2 has blocked user1
func (r *ConversationRepository) IsBlocked(ctx context.Context, user1, user2 uuid.UUID) (bool, error) {
	var exists bool
	err := r.db.Pool.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM blocks 
			WHERE (blocker_id = $1 AND blocked_id = $2)
			   OR (blocker_id = $2 AND blocked_id = $1)
		)
	`, user1, user2).Scan(&exists)
	return exists, err
}
