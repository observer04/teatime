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

// GetMemberRole returns a user's role in a conversation (returns error if not a member)
func (r *ConversationRepository) GetMemberRole(ctx context.Context, convID, userID uuid.UUID) (domain.MemberRole, error) {
	var role domain.MemberRole
	err := r.db.Pool.QueryRow(ctx, `
		SELECT role FROM conversation_members 
		WHERE conversation_id = $1 AND user_id = $2
	`, convID, userID).Scan(&role)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", domain.ErrNotMember
	}
	return role, err
}

// UpdateTitle updates a group conversation's title
func (r *ConversationRepository) UpdateTitle(ctx context.Context, convID uuid.UUID, title string) error {
	result, err := r.db.Pool.Exec(ctx, `
		UPDATE conversations 
		SET title = $2, updated_at = NOW()
		WHERE id = $1 AND type = 'group'
	`, convID, title)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return domain.ErrConversationNotFound
	}
	return nil
}

// GetMemberCount returns the number of members in a conversation
func (r *ConversationRepository) GetMemberCount(ctx context.Context, convID uuid.UUID) (int, error) {
	var count int
	err := r.db.Pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM conversation_members WHERE conversation_id = $1
	`, convID).Scan(&count)
	return count, err
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
		INSERT INTO messages (id, conversation_id, sender_id, body_text, attachment_id, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, msg.ID, msg.ConversationID, msg.SenderID, msg.BodyText, msg.AttachmentID, msg.CreatedAt)

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
			SELECT m.id, m.conversation_id, m.sender_id, m.body_text, m.attachment_id, m.created_at,
			       u.id, u.username, u.display_name, u.avatar_url
			FROM messages m
			LEFT JOIN users u ON u.id = m.sender_id
			WHERE m.conversation_id = $1 AND m.created_at < $2
			ORDER BY m.created_at DESC
			LIMIT $3
		`, convID, before, limit)
	} else {
		rows, err = r.db.Pool.Query(ctx, `
			SELECT m.id, m.conversation_id, m.sender_id, m.body_text, m.attachment_id, m.created_at,
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
			&m.ID, &m.ConversationID, &senderID, &m.BodyText, &m.AttachmentID, &m.CreatedAt,
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

// ============================================================================
// Starred Messages
// ============================================================================

// StarMessage adds a message to user's starred list
func (r *ConversationRepository) StarMessage(ctx context.Context, userID, messageID uuid.UUID) error {
	_, err := r.db.Pool.Exec(ctx, `
		INSERT INTO starred_messages (user_id, message_id)
		VALUES ($1, $2)
		ON CONFLICT DO NOTHING
	`, userID, messageID)
	return err
}

// UnstarMessage removes a message from user's starred list
func (r *ConversationRepository) UnstarMessage(ctx context.Context, userID, messageID uuid.UUID) error {
	_, err := r.db.Pool.Exec(ctx, `
		DELETE FROM starred_messages WHERE user_id = $1 AND message_id = $2
	`, userID, messageID)
	return err
}

// GetStarredMessages returns all starred messages for a user
func (r *ConversationRepository) GetStarredMessages(ctx context.Context, userID uuid.UUID, limit int) ([]domain.Message, error) {
	rows, err := r.db.Pool.Query(ctx, `
		SELECT m.id, m.conversation_id, m.sender_id, m.body_text, m.created_at,
		       u.id, u.username, u.display_name, u.avatar_url
		FROM starred_messages sm
		JOIN messages m ON m.id = sm.message_id
		LEFT JOIN users u ON u.id = m.sender_id
		WHERE sm.user_id = $1
		ORDER BY sm.starred_at DESC
		LIMIT $2
	`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []domain.Message
	for rows.Next() {
		var m domain.Message
		var senderID *uuid.UUID
		var userIDPtr *uuid.UUID
		var username, displayName, avatarURL *string

		err := rows.Scan(
			&m.ID, &m.ConversationID, &senderID, &m.BodyText, &m.CreatedAt,
			&userIDPtr, &username, &displayName, &avatarURL,
		)
		if err != nil {
			return nil, err
		}
		m.SenderID = senderID
		if userIDPtr != nil {
			m.Sender = &domain.PublicUser{
				ID:          *userIDPtr,
				Username:    *username,
				DisplayName: stringValue(displayName),
				AvatarURL:   stringValue(avatarURL),
			}
		}
		messages = append(messages, m)
	}
	return messages, rows.Err()
}

// IsMessageStarred checks if a message is starred by a user
func (r *ConversationRepository) IsMessageStarred(ctx context.Context, userID, messageID uuid.UUID) (bool, error) {
	var exists bool
	err := r.db.Pool.QueryRow(ctx, `
		SELECT EXISTS(SELECT 1 FROM starred_messages WHERE user_id = $1 AND message_id = $2)
	`, userID, messageID).Scan(&exists)
	return exists, err
}

// ============================================================================
// Message Search
// ============================================================================

// SearchMessages performs full-text search on messages within a conversation
func (r *ConversationRepository) SearchMessages(ctx context.Context, convID uuid.UUID, query string, limit int) ([]domain.Message, error) {
	rows, err := r.db.Pool.Query(ctx, `
		SELECT m.id, m.conversation_id, m.sender_id, m.body_text, m.created_at,
		       u.id, u.username, u.display_name, u.avatar_url,
		       ts_rank(m.search_vector, plainto_tsquery('english', $2)) as rank
		FROM messages m
		LEFT JOIN users u ON u.id = m.sender_id
		WHERE m.conversation_id = $1 
		  AND m.search_vector @@ plainto_tsquery('english', $2)
		ORDER BY rank DESC, m.created_at DESC
		LIMIT $3
	`, convID, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []domain.Message
	for rows.Next() {
		var m domain.Message
		var senderID *uuid.UUID
		var userIDPtr *uuid.UUID
		var username, displayName, avatarURL *string
		var rank float64

		err := rows.Scan(
			&m.ID, &m.ConversationID, &senderID, &m.BodyText, &m.CreatedAt,
			&userIDPtr, &username, &displayName, &avatarURL,
			&rank,
		)
		if err != nil {
			return nil, err
		}
		m.SenderID = senderID
		if userIDPtr != nil {
			m.Sender = &domain.PublicUser{
				ID:          *userIDPtr,
				Username:    *username,
				DisplayName: stringValue(displayName),
				AvatarURL:   stringValue(avatarURL),
			}
		}
		messages = append(messages, m)
	}
	return messages, rows.Err()
}

// SearchAllMessages searches across all conversations the user is a member of
func (r *ConversationRepository) SearchAllMessages(ctx context.Context, userID uuid.UUID, query string, limit int) ([]domain.Message, error) {
	rows, err := r.db.Pool.Query(ctx, `
		SELECT m.id, m.conversation_id, m.sender_id, m.body_text, m.created_at,
		       u.id, u.username, u.display_name, u.avatar_url,
		       ts_rank(m.search_vector, plainto_tsquery('english', $2)) as rank
		FROM messages m
		LEFT JOIN users u ON u.id = m.sender_id
		JOIN conversation_members cm ON cm.conversation_id = m.conversation_id AND cm.user_id = $1
		WHERE m.search_vector @@ plainto_tsquery('english', $2)
		ORDER BY rank DESC, m.created_at DESC
		LIMIT $3
	`, userID, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []domain.Message
	for rows.Next() {
		var m domain.Message
		var senderID *uuid.UUID
		var userIDPtr *uuid.UUID
		var username, displayName, avatarURL *string
		var rank float64

		err := rows.Scan(
			&m.ID, &m.ConversationID, &senderID, &m.BodyText, &m.CreatedAt,
			&userIDPtr, &username, &displayName, &avatarURL,
			&rank,
		)
		if err != nil {
			return nil, err
		}
		m.SenderID = senderID
		if userIDPtr != nil {
			m.Sender = &domain.PublicUser{
				ID:          *userIDPtr,
				Username:    *username,
				DisplayName: stringValue(displayName),
				AvatarURL:   stringValue(avatarURL),
			}
		}
		messages = append(messages, m)
	}
	return messages, rows.Err()
}

// ============================================================================
// Archive Conversations
// ============================================================================

// ArchiveConversation marks a conversation as archived for a user
func (r *ConversationRepository) ArchiveConversation(ctx context.Context, convID uuid.UUID) error {
	_, err := r.db.Pool.Exec(ctx, `
		UPDATE conversations SET archived_at = NOW() WHERE id = $1
	`, convID)
	return err
}

// UnarchiveConversation unarchives a conversation
func (r *ConversationRepository) UnarchiveConversation(ctx context.Context, convID uuid.UUID) error {
	_, err := r.db.Pool.Exec(ctx, `
		UPDATE conversations SET archived_at = NULL WHERE id = $1
	`, convID)
	return err
}

// GetArchivedConversations returns all archived conversations for a user
func (r *ConversationRepository) GetArchivedConversations(ctx context.Context, userID uuid.UUID) ([]domain.Conversation, error) {
	rows, err := r.db.Pool.Query(ctx, `
		SELECT c.id, c.type, c.title, c.created_by, c.created_at, c.updated_at, c.archived_at
		FROM conversations c
		JOIN conversation_members cm ON cm.conversation_id = c.id
		WHERE cm.user_id = $1 AND c.archived_at IS NOT NULL
		ORDER BY c.archived_at DESC
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
			&c.CreatedBy, &c.CreatedAt, &c.UpdatedAt, &c.ArchivedAt,
		)
		if err != nil {
			return nil, err
		}
		conversations = append(conversations, c)
	}
	return conversations, rows.Err()
}

// ============================================================================
// Read Status / Unread Tracking
// ============================================================================

// MarkConversationRead updates the read status for a user in a conversation
func (r *ConversationRepository) MarkConversationRead(ctx context.Context, convID, userID uuid.UUID, messageID *uuid.UUID) error {
	_, err := r.db.Pool.Exec(ctx, `
		INSERT INTO conversation_read_status (conversation_id, user_id, last_read_at, last_read_message_id)
		VALUES ($1, $2, NOW(), $3)
		ON CONFLICT (conversation_id, user_id)
		DO UPDATE SET last_read_at = NOW(), last_read_message_id = EXCLUDED.last_read_message_id
	`, convID, userID, messageID)
	return err
}

// MarkAllConversationsRead marks all conversations as read for a user
func (r *ConversationRepository) MarkAllConversationsRead(ctx context.Context, userID uuid.UUID) error {
	_, err := r.db.Pool.Exec(ctx, `
		INSERT INTO conversation_read_status (conversation_id, user_id, last_read_at)
		SELECT cm.conversation_id, $1, NOW()
		FROM conversation_members cm
		WHERE cm.user_id = $1
		ON CONFLICT (conversation_id, user_id)
		DO UPDATE SET last_read_at = NOW()
	`, userID)
	return err
}

// GetUnreadCount returns the unread message count for a user in a conversation
func (r *ConversationRepository) GetUnreadCount(ctx context.Context, convID, userID uuid.UUID) (int, error) {
	var count int
	err := r.db.Pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM messages m
		WHERE m.conversation_id = $1
		  AND m.created_at > COALESCE(
		      (SELECT last_read_at FROM conversation_read_status 
		       WHERE conversation_id = $1 AND user_id = $2),
		      '1970-01-01'::timestamptz
		  )
		  AND m.sender_id != $2
	`, convID, userID).Scan(&count)
	return count, err
}

// GetUserConversationsWithDetails returns all conversations for a user with unread counts and last message
func (r *ConversationRepository) GetUserConversationsWithDetails(ctx context.Context, userID uuid.UUID) ([]domain.Conversation, error) {
	rows, err := r.db.Pool.Query(ctx, `
		WITH last_messages AS (
			SELECT DISTINCT ON (conversation_id)
				conversation_id, id, sender_id, body_text, created_at
			FROM messages
			ORDER BY conversation_id, created_at DESC
		),
		unread_counts AS (
			SELECT 
				m.conversation_id,
				COUNT(*) as unread_count
			FROM messages m
			LEFT JOIN conversation_read_status rs ON rs.conversation_id = m.conversation_id AND rs.user_id = $1
			WHERE m.created_at > COALESCE(rs.last_read_at, '1970-01-01'::timestamptz)
			  AND m.sender_id != $1
			GROUP BY m.conversation_id
		),
		member_counts AS (
			SELECT conversation_id, COUNT(*) as member_count
			FROM conversation_members
			GROUP BY conversation_id
		)
		SELECT 
			c.id, c.type, c.title, c.created_by, c.created_at, c.updated_at, c.archived_at,
			COALESCE(uc.unread_count, 0) as unread_count,
			COALESCE(mc.member_count, 0) as member_count,
			lm.id, lm.sender_id, lm.body_text, lm.created_at
		FROM conversations c
		JOIN conversation_members cm ON cm.conversation_id = c.id
		LEFT JOIN last_messages lm ON lm.conversation_id = c.id
		LEFT JOIN unread_counts uc ON uc.conversation_id = c.id
		LEFT JOIN member_counts mc ON mc.conversation_id = c.id
		WHERE cm.user_id = $1 AND c.archived_at IS NULL
		ORDER BY COALESCE(lm.created_at, c.created_at) DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var conversations []domain.Conversation
	for rows.Next() {
		var c domain.Conversation
		var lastMsgID, lastMsgSenderID *uuid.UUID
		var lastMsgBody *string
		var lastMsgCreatedAt *time.Time

		err := rows.Scan(
			&c.ID, &c.Type, &c.Title,
			&c.CreatedBy, &c.CreatedAt, &c.UpdatedAt, &c.ArchivedAt,
			&c.UnreadCount, &c.MemberCount,
			&lastMsgID, &lastMsgSenderID, &lastMsgBody, &lastMsgCreatedAt,
		)
		if err != nil {
			return nil, err
		}

		// Populate last message if exists
		if lastMsgID != nil {
			c.LastMessage = &domain.Message{
				ID:             *lastMsgID,
				ConversationID: c.ID,
				SenderID:       lastMsgSenderID,
				BodyText:       stringValue(lastMsgBody),
				CreatedAt:      *lastMsgCreatedAt,
			}
		}

		conversations = append(conversations, c)
	}

	// For DM conversations, fetch the other user
	for i := range conversations {
		if conversations[i].Type == domain.ConversationTypeDM {
			otherUser, err := r.GetOtherDMUser(ctx, conversations[i].ID, userID)
			if err == nil && otherUser != nil {
				conversations[i].OtherUser = otherUser
			}
		}
	}

	return conversations, rows.Err()
}

// GetOtherDMUser returns the other user in a DM conversation
func (r *ConversationRepository) GetOtherDMUser(ctx context.Context, convID, userID uuid.UUID) (*domain.PublicUser, error) {
	var user domain.PublicUser
	err := r.db.Pool.QueryRow(ctx, `
		SELECT u.id, u.username, u.display_name, u.avatar_url
		FROM conversation_members cm
		JOIN users u ON u.id = cm.user_id
		WHERE cm.conversation_id = $1 AND cm.user_id != $2
		LIMIT 1
	`, convID, userID).Scan(&user.ID, &user.Username, &user.DisplayName, &user.AvatarURL)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// GetMessageByID returns a message by ID
func (r *ConversationRepository) GetMessageByID(ctx context.Context, messageID uuid.UUID) (*domain.Message, error) {
	var m domain.Message
	var senderID *uuid.UUID
	err := r.db.Pool.QueryRow(ctx, `
		SELECT id, conversation_id, sender_id, body_text, created_at
		FROM messages WHERE id = $1
	`, messageID).Scan(&m.ID, &m.ConversationID, &senderID, &m.BodyText, &m.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrMessageNotFound
	}
	if err != nil {
		return nil, err
	}
	m.SenderID = senderID
	return &m, nil
}
