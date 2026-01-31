package domain

import "time"

// AttachmentStatus represents the upload status
type AttachmentStatus string

const (
	AttachmentStatusUploading AttachmentStatus = "uploading"
	AttachmentStatusReady     AttachmentStatus = "ready"
	AttachmentStatusError     AttachmentStatus = "error"
)

// Attachment represents a file uploaded to R2
type Attachment struct {
	ID             string           `json:"id"`
	UploaderID     string           `json:"uploader_id"`
	ConversationID string           `json:"conversation_id"`
	Bucket         string           `json:"bucket"`
	ObjectKey      string           `json:"object_key"`
	Filename       string           `json:"filename"`
	MimeType       string           `json:"mime_type"`
	SizeBytes      int64            `json:"size_bytes"`
	SHA256         *string          `json:"sha256,omitempty"`
	Status         AttachmentStatus `json:"status"`
	CreatedAt      time.Time        `json:"created_at"`
	CompletedAt    *time.Time       `json:"completed_at,omitempty"`
}

// UploadInitRequest is the request to initialize an upload
type UploadInitRequest struct {
	ConversationID string `json:"conversation_id"`
	Filename       string `json:"filename"`
	MimeType       string `json:"mime_type"`
	SizeBytes      int64  `json:"size_bytes"`
}

// UploadInitResponse is the response from upload init
type UploadInitResponse struct {
	AttachmentID    string            `json:"attachment_id"`
	ObjectKey       string            `json:"object_key"`
	PresignedURL    string            `json:"presigned_url"`
	RequiredHeaders map[string]string `json:"required_headers,omitempty"`
}

// UploadCompleteRequest is the request to finalize an upload
type UploadCompleteRequest struct {
	AttachmentID string `json:"attachment_id"`
	SHA256       string `json:"sha256,omitempty"`
}

// AttachmentDownloadResponse contains the download URL
type AttachmentDownloadResponse struct {
	AttachmentID string `json:"attachment_id"`
	Filename     string `json:"filename"`
	MimeType     string `json:"mime_type"`
	SizeBytes    int64  `json:"size_bytes"`
	DownloadURL  string `json:"download_url"`
}
