package api

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/observer/teatime/internal/auth"
	"github.com/observer/teatime/internal/database"
	"github.com/observer/teatime/internal/domain"
)

// ConversationHandler handles conversation and message endpoints
type ConversationHandler struct {
	convs  *database.ConversationRepository
	users  *database.UserRepository
	logger *slog.Logger
}

func NewConversationHandler(convs *database.ConversationRepository, users *database.UserRepository, logger *slog.Logger) *ConversationHandler {
	return &ConversationHandler{
		convs:  convs,
		users:  users,
		logger: logger,
	}
}

// CreateConversation handles POST /conversations
func (h *ConversationHandler) CreateConversation(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.GetUserID(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var input struct {
		Type      string   `json:"type"`       // "dm" or "group"
		Title     string   `json:"title"`      // for groups only
		MemberIDs []string `json:"member_ids"` // UUIDs of other members
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Validate type
	convType := domain.ConversationType(strings.ToLower(input.Type))
	if convType != domain.ConversationTypeDM && convType != domain.ConversationTypeGroup {
		writeError(w, http.StatusBadRequest, "type must be 'dm' or 'group'")
		return
	}

	// Parse member IDs
	memberIDs := []uuid.UUID{userID} // Always include creator
	for _, idStr := range input.MemberIDs {
		id, err := uuid.Parse(idStr)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid member ID: "+idStr)
			return
		}
		if id != userID { // Don't add creator twice
			memberIDs = append(memberIDs, id)
		}
	}

	// DM validation
	if convType == domain.ConversationTypeDM {
		if len(memberIDs) != 2 {
			writeError(w, http.StatusBadRequest, "DM must have exactly 2 members")
			return
		}
		// Check if DM already exists
		existing, err := h.convs.FindDMBetween(r.Context(), memberIDs[0], memberIDs[1])
		if err != nil {
			h.logger.Error("find DM failed", "error", err)
			writeError(w, http.StatusInternalServerError, "failed to check existing DM")
			return
		}
		if existing != nil {
			writeJSON(w, http.StatusOK, existing)
			return
		}

		// Check blocks
		blocked, err := h.convs.IsBlocked(r.Context(), memberIDs[0], memberIDs[1])
		if err != nil {
			h.logger.Error("check block failed", "error", err)
		}
		if blocked {
			writeError(w, http.StatusForbidden, "cannot create DM with this user")
			return
		}
	}

	// Group validation
	if convType == domain.ConversationTypeGroup {
		if len(memberIDs) < 2 {
			writeError(w, http.StatusBadRequest, "group must have at least 2 members")
			return
		}
		if len(memberIDs) > 100 {
			writeError(w, http.StatusBadRequest, "group cannot exceed 100 members")
			return
		}
		if input.Title == "" {
			writeError(w, http.StatusBadRequest, "group title is required")
			return
		}
		if len(input.Title) > 100 {
			writeError(w, http.StatusBadRequest, "title too long (max 100)")
			return
		}
	}

	// Create conversation
	conv := &domain.Conversation{
		ID:        uuid.New(),
		Type:      convType,
		Title:     input.Title,
		CreatedBy: &userID,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := h.convs.Create(r.Context(), conv, memberIDs); err != nil {
		h.logger.Error("create conversation failed", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to create conversation")
		return
	}

	// Fetch with members
	conv, err := h.convs.GetByID(r.Context(), conv.ID)
	if err != nil {
		h.logger.Error("fetch conversation failed", "error", err)
	}

	writeJSON(w, http.StatusCreated, conv)
}

// ListConversations handles GET /conversations
func (h *ConversationHandler) ListConversations(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.GetUserID(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	conversations, err := h.convs.GetUserConversations(r.Context(), userID)
	if err != nil {
		h.logger.Error("list conversations failed", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list conversations")
		return
	}

	if conversations == nil {
		conversations = []domain.Conversation{}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"conversations": conversations,
		"count":         len(conversations),
	})
}

// GetConversation handles GET /conversations/{id}
func (h *ConversationHandler) GetConversation(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.GetUserID(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	convID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid conversation ID")
		return
	}

	// Check membership
	isMember, err := h.convs.IsMember(r.Context(), convID, userID)
	if err != nil {
		h.logger.Error("check membership failed", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to check membership")
		return
	}
	if !isMember {
		writeError(w, http.StatusForbidden, "not a member of this conversation")
		return
	}

	conv, err := h.convs.GetByID(r.Context(), convID)
	if err != nil {
		if errors.Is(err, domain.ErrConversationNotFound) {
			writeError(w, http.StatusNotFound, "conversation not found")
			return
		}
		h.logger.Error("get conversation failed", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to get conversation")
		return
	}

	writeJSON(w, http.StatusOK, conv)
}

// AddMember handles POST /conversations/{id}/members
func (h *ConversationHandler) AddMember(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.GetUserID(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	convID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid conversation ID")
		return
	}

	var input struct {
		UserID string `json:"user_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	newMemberID, err := uuid.Parse(input.UserID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user ID")
		return
	}

	// Check caller is member
	isMember, err := h.convs.IsMember(r.Context(), convID, userID)
	if err != nil || !isMember {
		writeError(w, http.StatusForbidden, "not a member of this conversation")
		return
	}

	// Get conversation to check it's a group
	conv, err := h.convs.GetByID(r.Context(), convID)
	if err != nil {
		writeError(w, http.StatusNotFound, "conversation not found")
		return
	}
	if conv.Type != domain.ConversationTypeGroup {
		writeError(w, http.StatusBadRequest, "cannot add members to DM")
		return
	}

	// Add member
	if err := h.convs.AddMember(r.Context(), convID, newMemberID, domain.MemberRoleMember); err != nil {
		h.logger.Error("add member failed", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to add member")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "member added"})
}

// RemoveMember handles DELETE /conversations/{id}/members/{userId}
func (h *ConversationHandler) RemoveMember(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.GetUserID(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	convID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid conversation ID")
		return
	}

	targetUserID, err := uuid.Parse(r.PathValue("userId"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user ID")
		return
	}

	// Check caller is member
	isMember, err := h.convs.IsMember(r.Context(), convID, userID)
	if err != nil || !isMember {
		writeError(w, http.StatusForbidden, "not a member of this conversation")
		return
	}

	// Can only remove self, or if admin (TODO: add admin check)
	if userID != targetUserID {
		writeError(w, http.StatusForbidden, "can only remove yourself from group")
		return
	}

	if err := h.convs.RemoveMember(r.Context(), convID, targetUserID); err != nil {
		h.logger.Error("remove member failed", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to remove member")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "member removed"})
}

// ============================================================================
// Messages
// ============================================================================

// GetMessages handles GET /conversations/{id}/messages?before=<timestamp>&limit=50
func (h *ConversationHandler) GetMessages(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.GetUserID(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	convID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid conversation ID")
		return
	}

	// Check membership
	isMember, err := h.convs.IsMember(r.Context(), convID, userID)
	if err != nil || !isMember {
		writeError(w, http.StatusForbidden, "not a member of this conversation")
		return
	}

	// Parse pagination
	var before *time.Time
	if beforeStr := r.URL.Query().Get("before"); beforeStr != "" {
		t, err := time.Parse(time.RFC3339, beforeStr)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid 'before' timestamp")
			return
		}
		before = &t
	}

	limit := 50
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}

	messages, err := h.convs.GetMessages(r.Context(), convID, before, limit)
	if err != nil {
		h.logger.Error("get messages failed", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to get messages")
		return
	}

	if messages == nil {
		messages = []domain.Message{}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"messages": messages,
		"count":    len(messages),
	})
}

// SendMessage handles POST /conversations/{id}/messages
func (h *ConversationHandler) SendMessage(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.GetUserID(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	convID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid conversation ID")
		return
	}

	var input struct {
		BodyText string `json:"body_text"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Validate message
	input.BodyText = strings.TrimSpace(input.BodyText)
	if input.BodyText == "" {
		writeError(w, http.StatusBadRequest, "message cannot be empty")
		return
	}
	if len(input.BodyText) > 10000 {
		writeError(w, http.StatusBadRequest, "message too long (max 10000 chars)")
		return
	}

	// Check membership
	isMember, err := h.convs.IsMember(r.Context(), convID, userID)
	if err != nil || !isMember {
		writeError(w, http.StatusForbidden, "not a member of this conversation")
		return
	}

	// Create message
	msg := &domain.Message{
		ID:             uuid.New(),
		ConversationID: convID,
		SenderID:       &userID,
		BodyText:       input.BodyText,
		CreatedAt:      time.Now(),
	}

	if err := h.convs.CreateMessage(r.Context(), msg); err != nil {
		h.logger.Error("create message failed", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to send message")
		return
	}

	// Get sender info
	user, _ := h.users.GetByID(r.Context(), userID)
	if user != nil {
		pub := user.ToPublic()
		msg.Sender = &pub
	}

	writeJSON(w, http.StatusCreated, msg)
}

// ============================================================================
// Blocking
// ============================================================================

// BlockUser handles POST /blocks/{username}
func (h *ConversationHandler) BlockUser(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.GetUserID(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	username := r.PathValue("username")
	targetUser, err := h.users.GetByUsername(r.Context(), username)
	if err != nil {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}

	if targetUser.ID == userID {
		writeError(w, http.StatusBadRequest, "cannot block yourself")
		return
	}

	if err := h.convs.Block(r.Context(), userID, targetUser.ID); err != nil {
		h.logger.Error("block user failed", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to block user")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "user blocked"})
}

// UnblockUser handles DELETE /blocks/{username}
func (h *ConversationHandler) UnblockUser(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.GetUserID(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	username := r.PathValue("username")
	targetUser, err := h.users.GetByUsername(r.Context(), username)
	if err != nil {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}

	if err := h.convs.Unblock(r.Context(), userID, targetUser.ID); err != nil {
		h.logger.Error("unblock user failed", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to unblock user")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "user unblocked"})
}
