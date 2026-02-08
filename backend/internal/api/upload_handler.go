package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/observer/teatime/internal/auth"
	"github.com/observer/teatime/internal/database"
	"github.com/observer/teatime/internal/domain"
	"github.com/observer/teatime/internal/storage"
)

type UploadHandler struct {
	attachmentRepo   *database.AttachmentRepository
	conversationRepo *database.ConversationRepository
	r2Storage        *storage.R2Storage
	maxUploadBytes   int64
	allowedMimeTypes []string
	r2Bucket         string
}

func NewUploadHandler(
	attachmentRepo *database.AttachmentRepository,
	conversationRepo *database.ConversationRepository,
	r2Storage *storage.R2Storage,
	maxUploadBytes int64,
	r2Bucket string,
) *UploadHandler {
	return &UploadHandler{
		attachmentRepo:   attachmentRepo,
		conversationRepo: conversationRepo,
		r2Storage:        r2Storage,
		maxUploadBytes:   maxUploadBytes,
		r2Bucket:         r2Bucket,
		allowedMimeTypes: []string{
			"image/", "video/", "audio/",
			"application/pdf",
			"application/msword",
			"application/vnd.openxmlformats-officedocument",
			"text/plain",
		},
	}
}

// InitUpload godoc
//
//	@Summary		Initialize file upload
//	@Description	Request a presigned URL for uploading a file to R2
//	@Tags			uploads
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			request	body		domain.UploadInitRequest	true	"Upload initialization request"
//	@Success		200		{object}	domain.UploadInitResponse	"Presigned upload URL generated"
//	@Failure		400		{object}	map[string]string	"Invalid input"
//	@Failure		403		{object}	map[string]string	"Not a member of conversation"
//	@Failure		401		{object}	map[string]string	"Unauthorized"
//	@Router			/uploads/init [post]
func (h *UploadHandler) InitUpload(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID, ok := auth.GetUserID(ctx)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req domain.UploadInitRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	// Validate request
	if req.ConversationID == "" || req.Filename == "" || req.MimeType == "" || req.SizeBytes <= 0 {
		http.Error(w, "missing required fields", http.StatusBadRequest)
		return
	}

	// Check file size
	if req.SizeBytes > h.maxUploadBytes {
		http.Error(w, fmt.Sprintf("file too large (max %d bytes)", h.maxUploadBytes), http.StatusBadRequest)
		return
	}

	// Check mime type
	if !h.isMimeTypeAllowed(req.MimeType) {
		http.Error(w, "file type not allowed", http.StatusBadRequest)
		return
	}

	// Parse conversation ID
	convID, err := uuid.Parse(req.ConversationID)
	if err != nil {
		http.Error(w, "invalid conversation_id", http.StatusBadRequest)
		return
	}

	// Verify user is a member of the conversation
	isMember, err := h.conversationRepo.IsMember(ctx, convID, userID)
	if err != nil {
		http.Error(w, "failed to verify membership", http.StatusInternalServerError)
		return
	}
	if !isMember {
		http.Error(w, "not a member of this conversation", http.StatusForbidden)
		return
	}

	// Generate attachment ID and object key
	attachmentID := uuid.New().String()
	objectKey := h.generateObjectKey(req.ConversationID, attachmentID, req.Filename)

	// Create attachment record in DB
	attachment := &domain.Attachment{
		ID:             attachmentID,
		UploaderID:     userID.String(),
		ConversationID: req.ConversationID,
		Bucket:         h.r2Bucket,
		ObjectKey:      objectKey,
		Filename:       req.Filename,
		MimeType:       req.MimeType,
		SizeBytes:      req.SizeBytes,
		Status:         domain.AttachmentStatusUploading,
		CreatedAt:      time.Now(),
	}

	if err := h.attachmentRepo.CreateAttachment(ctx, attachment); err != nil {
		http.Error(w, "failed to create attachment record", http.StatusInternalServerError)
		return
	}

	// Generate presigned PUT URL (15 minutes expiry)
	presignedURL, err := h.r2Storage.GeneratePresignedPutURL(ctx, objectKey, req.MimeType, 15*time.Minute)
	if err != nil {
		http.Error(w, "failed to generate upload URL", http.StatusInternalServerError)
		return
	}

	// Return response
	resp := domain.UploadInitResponse{
		AttachmentID: attachmentID,
		ObjectKey:    objectKey,
		PresignedURL: presignedURL,
		RequiredHeaders: map[string]string{
			"Content-Type": req.MimeType,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// CompleteUpload godoc
//
//	@Summary		Complete file upload
//	@Description	Mark a file upload as complete after successfully uploading to R2
//	@Tags			uploads
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			request	body		domain.UploadCompleteRequest	true	"Upload completion request"
//	@Success		200		{object}	map[string]string	"Upload completed"
//	@Failure		400		{object}	map[string]string	"Invalid input"
//	@Failure		403		{object}	map[string]string	"Not authorized"
//	@Failure		404		{object}	map[string]string	"Attachment not found"
//	@Router			/uploads/complete [post]
func (h *UploadHandler) CompleteUpload(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID, ok := auth.GetUserID(ctx)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req domain.UploadCompleteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error":   "invalid request",
			"details": err.Error(),
		})
		return
	}

	if req.AttachmentID == "" {
		http.Error(w, "attachment_id required", http.StatusBadRequest)
		return
	}

	// Get attachment
	attachment, err := h.attachmentRepo.GetAttachmentByID(ctx, req.AttachmentID)
	if err != nil {
		// Return detailed error for debugging
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error":         "attachment not found",
			"attachment_id": req.AttachmentID,
			"details":       err.Error(),
		})
		return
	}

	// Verify uploader
	if attachment.UploaderID != userID.String() {
		http.Error(w, "not authorized", http.StatusForbidden)
		return
	}

	// Mark as ready
	if err := h.attachmentRepo.MarkAttachmentReady(ctx, req.AttachmentID, req.SHA256); err != nil {
		http.Error(w, "failed to mark attachment ready", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"status":        "completed",
		"attachment_id": req.AttachmentID,
	})
}

// GetAttachmentURL godoc
//
//	@Summary		Get file download URL
//	@Description	Get a presigned download URL for an attachment
//	@Tags			attachments
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id	path		string	true	"Attachment ID"
//	@Success		200	{object}	domain.AttachmentDownloadResponse	"Download URL generated"
//	@Failure		403	{object}	map[string]string	"Not authorized"
//	@Failure		404	{object}	map[string]string	"Attachment not found"
//	@Router			/attachments/{id}/url [get]
func (h *UploadHandler) GetAttachmentURL(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID, ok := auth.GetUserID(ctx)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// Get attachment ID from path
	attachmentID := strings.TrimPrefix(r.URL.Path, "/attachments/")
	attachmentID = strings.TrimSuffix(attachmentID, "/url")

	// Get attachment
	attachment, err := h.attachmentRepo.GetAttachmentByID(ctx, attachmentID)
	if err != nil {
		http.Error(w, "attachment not found", http.StatusNotFound)
		return
	}

	// Parse conversation ID
	convID, err := uuid.Parse(attachment.ConversationID)
	if err != nil {
		http.Error(w, "invalid conversation_id", http.StatusInternalServerError)
		return
	}

	// Verify user is a member of the conversation
	isMember, err := h.conversationRepo.IsMember(ctx, convID, userID)
	if err != nil {
		http.Error(w, "failed to verify membership", http.StatusInternalServerError)
		return
	}
	if !isMember {
		http.Error(w, "not authorized", http.StatusForbidden)
		return
	}

	// Generate presigned GET URL (1 hour expiry)
	downloadURL, err := h.r2Storage.GeneratePresignedGetURL(ctx, attachment.ObjectKey, 1*time.Hour)
	if err != nil {
		http.Error(w, "failed to generate download URL", http.StatusInternalServerError)
		return
	}

	resp := domain.AttachmentDownloadResponse{
		AttachmentID: attachment.ID,
		Filename:     attachment.Filename,
		MimeType:     attachment.MimeType,
		SizeBytes:    attachment.SizeBytes,
		DownloadURL:  downloadURL,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// Helper functions

func (h *UploadHandler) isMimeTypeAllowed(mimeType string) bool {
	for _, allowed := range h.allowedMimeTypes {
		if strings.HasPrefix(mimeType, allowed) {
			return true
		}
	}
	return false
}

func (h *UploadHandler) generateObjectKey(conversationID, attachmentID, filename string) string {
	// Clean filename
	ext := path.Ext(filename)
	safeFilename := fmt.Sprintf("%s%s", attachmentID, ext)

	// Format: conv/{conversation_id}/{attachment_id}.ext
	return fmt.Sprintf("conv/%s/%s", conversationID, safeFilename)
}
