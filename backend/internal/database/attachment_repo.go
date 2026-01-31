package database

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/observer/teatime/internal/domain"
)

type AttachmentRepository struct {
	pool *pgxpool.Pool
}

func NewAttachmentRepository(pool *pgxpool.Pool) *AttachmentRepository {
	return &AttachmentRepository{pool: pool}
}

// CreateAttachment creates a new attachment record in uploading status
func (r *AttachmentRepository) CreateAttachment(ctx context.Context, att *domain.Attachment) error {
	query := `
		INSERT INTO attachments (id, uploader_id, conversation_id, bucket, object_key, filename, mime_type, size_bytes, status, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`
	_, err := r.pool.Exec(ctx, query,
		att.ID, att.UploaderID, att.ConversationID, att.Bucket, att.ObjectKey,
		att.Filename, att.MimeType, att.SizeBytes, att.Status, att.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to create attachment: %w", err)
	}
	return nil
}

// GetAttachmentByID retrieves an attachment by ID
func (r *AttachmentRepository) GetAttachmentByID(ctx context.Context, id string) (*domain.Attachment, error) {
	query := `
		SELECT id::text, uploader_id::text, conversation_id::text, bucket, object_key, filename, mime_type, size_bytes, sha256, status, created_at, completed_at
		FROM attachments
		WHERE id = $1
	`
	var att domain.Attachment
	fmt.Printf("DEBUG: Querying attachment with ID: %s\n", id)
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&att.ID, &att.UploaderID, &att.ConversationID, &att.Bucket, &att.ObjectKey,
		&att.Filename, &att.MimeType, &att.SizeBytes, &att.SHA256, &att.Status, &att.CreatedAt, &att.CompletedAt,
	)
	if err != nil {
		fmt.Printf("DEBUG: Query error: %v\n", err)
		return nil, fmt.Errorf("failed to get attachment: %w", err)
	}
	fmt.Printf("DEBUG: Found attachment: %s\n", att.ID)
	return &att, nil
}

// MarkAttachmentReady marks an attachment as ready after successful upload
func (r *AttachmentRepository) MarkAttachmentReady(ctx context.Context, id string, sha256 string) error {
	now := time.Now()
	query := `
		UPDATE attachments
		SET status = $1, sha256 = $2, completed_at = $3
		WHERE id = $4
	`
	_, err := r.pool.Exec(ctx, query, domain.AttachmentStatusReady, sha256, now, id)
	if err != nil {
		return fmt.Errorf("failed to mark attachment ready: %w", err)
	}
	return nil
}

// MarkAttachmentError marks an attachment as error
func (r *AttachmentRepository) MarkAttachmentError(ctx context.Context, id string) error {
	query := `
		UPDATE attachments
		SET status = $1
		WHERE id = $2
	`
	_, err := r.pool.Exec(ctx, query, domain.AttachmentStatusError, id)
	if err != nil {
		return fmt.Errorf("failed to mark attachment error: %w", err)
	}
	return nil
}

// GetAttachmentsByConversation retrieves all attachments for a conversation
func (r *AttachmentRepository) GetAttachmentsByConversation(ctx context.Context, conversationID string) ([]*domain.Attachment, error) {
	query := `
		SELECT id, uploader_id, conversation_id, bucket, object_key, filename, mime_type, size_bytes, sha256, status, created_at, completed_at
		FROM attachments
		WHERE conversation_id = $1 AND status = $2
		ORDER BY created_at DESC
	`
	rows, err := r.pool.Query(ctx, query, conversationID, domain.AttachmentStatusReady)
	if err != nil {
		return nil, fmt.Errorf("failed to get attachments: %w", err)
	}
	defer rows.Close()

	var attachments []*domain.Attachment
	for rows.Next() {
		var att domain.Attachment
		err := rows.Scan(
			&att.ID, &att.UploaderID, &att.ConversationID, &att.Bucket, &att.ObjectKey,
			&att.Filename, &att.MimeType, &att.SizeBytes, &att.SHA256, &att.Status, &att.CreatedAt, &att.CompletedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan attachment: %w", err)
		}
		attachments = append(attachments, &att)
	}

	return attachments, nil
}

// DeleteAttachment deletes an attachment record
func (r *AttachmentRepository) DeleteAttachment(ctx context.Context, id string) error {
	query := `DELETE FROM attachments WHERE id = $1`
	_, err := r.pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete attachment: %w", err)
	}
	return nil
}
