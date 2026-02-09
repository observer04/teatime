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
	"github.com/observer/teatime/internal/websocket"
)

// ConversationHandler handles conversation and message endpoints
type ConversationHandler struct {
	convs       *database.ConversationRepository
	users       *database.UserRepository
	broadcaster websocket.RoomBroadcaster
	logger      *slog.Logger
}

func NewConversationHandler(convs *database.ConversationRepository, users *database.UserRepository, broadcaster websocket.RoomBroadcaster, logger *slog.Logger) *ConversationHandler {
	return &ConversationHandler{
		convs:       convs,
		users:       users,
		broadcaster: broadcaster,
		logger:      logger,
	}
}

// CreateConversation godoc
//
//	@Summary		Create conversation
//	@Description	Create a new direct message or group conversation
//	@Tags			conversations
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			request	body		object{type=string,title=string,member_ids=[]string}	true	"Conversation details"
//	@Success		201	{object}	domain.Conversation
//	@Failure		400	{object}	map[string]string
//	@Failure		401	{object}	map[string]string
//	@Router			/conversations [post]
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

// ListConversations godoc
//
//	@Summary		List conversations
//	@Description	Get all conversations for the authenticated user
//	@Tags			conversations
//	@Produce		json
//	@Security		BearerAuth
//	@Param			archived	query		bool	false	"Include archived conversations"
//	@Success		200	{object}	object{conversations=[]domain.Conversation,count=int}
//	@Failure		401	{object}	map[string]string
//	@Router			/conversations [get]
func (h *ConversationHandler) ListConversations(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.GetUserID(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	// Check for archived parameter
	if r.URL.Query().Get("archived") == "true" {
		conversations, err := h.convs.GetArchivedConversations(r.Context(), userID)
		if err != nil {
			h.logger.Error("list archived conversations failed", "error", err)
			writeError(w, http.StatusInternalServerError, "failed to list archived conversations")
			return
		}
		if conversations == nil {
			conversations = []domain.Conversation{}
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"conversations": conversations,
			"count":         len(conversations),
		})
		return
	}

	// Get conversations with details (unread count, last message, etc.)
	conversations, err := h.convs.GetUserConversationsWithDetails(r.Context(), userID)
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

// GetConversation godoc
//
//	@Summary		Get conversation details
//	@Description	Get details of a specific conversation including members
//	@Tags			conversations
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id	path		string	true	"Conversation ID"
//	@Success		200	{object}	domain.Conversation
//	@Failure		401	{object}	map[string]string
//	@Failure		404	{object}	map[string]string
//	@Router			/conversations/{id} [get]
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

// AddMember godoc
//
//	@Summary		Add member to conversation
//	@Description	Add a new member to a group conversation
//	@Tags			conversations
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id	path		string	true	"Conversation ID"
//	@Param			request	body		object{user_id=string}	true	"User to add"
//	@Success		200	{object}	map[string]string
//	@Failure		400	{object}	map[string]string
//	@Failure		401	{object}	map[string]string
//	@Router			/conversations/{id}/members [post]
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

	// Get new member's username for broadcast
	newMember, err := h.users.GetByID(r.Context(), newMemberID)
	if err == nil && h.broadcaster != nil {
		if err := h.broadcaster.BroadcastMemberJoined(r.Context(), convID, newMemberID, newMember.Username, string(domain.MemberRoleMember), userID); err != nil {
			h.logger.Error("failed to broadcast member joined", "error", err)
		}
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "member added"})
}

// RemoveMember godoc
//
//	@Summary		Remove member from conversation
//	@Description	Remove a member from a group conversation
//	@Tags			conversations
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id	path		string	true	"Conversation ID"
//	@Param			userId	path		string	true	"User ID to remove"
//	@Success		200	{object}	map[string]string
//	@Failure		401	{object}	map[string]string
//	@Router			/conversations/{id}/members/{userId} [delete]
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

	// Get target user's info before removal for broadcast
	targetUser, _ := h.users.GetByID(r.Context(), targetUserID)
	targetUsername := ""
	if targetUser != nil {
		targetUsername = targetUser.Username
	}

	// Check caller is member and get their role
	callerRole, err := h.convs.GetMemberRole(r.Context(), convID, userID)
	if err != nil {
		if errors.Is(err, domain.ErrNotMember) {
			writeError(w, http.StatusForbidden, "not a member of this conversation")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to check membership")
		return
	}

	// Can remove self, or admin can remove others
	if userID != targetUserID && callerRole != domain.MemberRoleAdmin {
		writeError(w, http.StatusForbidden, "only admins can remove other members")
		return
	}

	if err := h.convs.RemoveMember(r.Context(), convID, targetUserID); err != nil {
		h.logger.Error("remove member failed", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to remove member")
		return
	}

	// Broadcast member removal
	if h.broadcaster != nil {
		if err := h.broadcaster.BroadcastMemberLeft(r.Context(), convID, targetUserID, targetUsername, userID); err != nil {
			h.logger.Error("failed to broadcast member left", "error", err)
		}
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "member removed"})
}

// UpdateConversation godoc
//
//	@Summary		Update conversation
//	@Description	Update conversation title or settings
//	@Tags			conversations
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id	path		string	true	"Conversation ID"
//	@Param			request	body		object{title=string}	true	"Update details"
//	@Success		200	{object}	domain.Conversation
//	@Failure		400	{object}	map[string]string
//	@Failure		401	{object}	map[string]string
//	@Router			/conversations/{id} [patch]
func (h *ConversationHandler) UpdateConversation(w http.ResponseWriter, r *http.Request) {
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
		Title string `json:"title"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Validate title
	if input.Title == "" {
		writeError(w, http.StatusBadRequest, "title is required")
		return
	}
	if len(input.Title) > 100 {
		writeError(w, http.StatusBadRequest, "title too long (max 100)")
		return
	}

	// Check caller is admin
	callerRole, err := h.convs.GetMemberRole(r.Context(), convID, userID)
	if err != nil {
		if errors.Is(err, domain.ErrNotMember) {
			writeError(w, http.StatusForbidden, "not a member of this conversation")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to check membership")
		return
	}

	if callerRole != domain.MemberRoleAdmin {
		writeError(w, http.StatusForbidden, "only admins can rename the group")
		return
	}

	if err := h.convs.UpdateTitle(r.Context(), convID, input.Title); err != nil {
		if errors.Is(err, domain.ErrConversationNotFound) {
			writeError(w, http.StatusNotFound, "conversation not found")
			return
		}
		h.logger.Error("update conversation failed", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to update conversation")
		return
	}

	// Broadcast the title update
	if h.broadcaster != nil {
		if err := h.broadcaster.BroadcastRoomUpdated(r.Context(), convID, input.Title, userID); err != nil {
			h.logger.Error("failed to broadcast room updated", "error", err)
		}
	}

	// Fetch updated conversation
	conv, err := h.convs.GetByID(r.Context(), convID)
	if err != nil {
		h.logger.Error("fetch conversation failed", "error", err)
		writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
		return
	}

	writeJSON(w, http.StatusOK, conv)
}

// ============================================================================
// Messages
// ============================================================================

// GetMessages godoc
//
//	@Summary		Get messages
//	@Description	Get messages from a conversation with pagination
//	@Tags			messages
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id	path		string	true	"Conversation ID"
//	@Param			before	query		string	false	"Cursor for pagination"
//	@Param			limit	query		int	false	"Number of messages (default 50)"
//	@Success		200	{object}	object{messages=[]domain.Message,has_more=bool}
//	@Failure		401	{object}	map[string]string
//	@Router			/conversations/{id}/messages [get]
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

	// Populate receipt statuses for messages sent by the current user
	if len(messages) > 0 {
		// Collect message IDs sent by this user
		var ownMsgIDs []uuid.UUID
		for _, msg := range messages {
			if msg.SenderID != nil && *msg.SenderID == userID {
				ownMsgIDs = append(ownMsgIDs, msg.ID)
			}
		}

		// Get receipt statuses in bulk
		if len(ownMsgIDs) > 0 {
			statuses, err := h.convs.GetMessageReceiptStatuses(r.Context(), ownMsgIDs)
			if err != nil {
				h.logger.Warn("failed to get receipt statuses", "error", err)
			} else {
				// Apply statuses to messages
				for i := range messages {
					if messages[i].SenderID != nil && *messages[i].SenderID == userID {
						if status, ok := statuses[messages[i].ID]; ok {
							messages[i].ReceiptStatus = status
						} else {
							messages[i].ReceiptStatus = "sent"
						}
					}
				}
			}
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"messages": messages,
		"count":    len(messages),
	})
}

// SendMessage godoc
//
//	@Summary		Send message
//	@Description	Send a new message to a conversation
//	@Tags			messages
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id	path		string	true	"Conversation ID"
//	@Param			request	body		object{body_text=string,attachment_id=string}	true	"Message content"
//	@Success		201	{object}	domain.Message
//	@Failure		400	{object}	map[string]string
//	@Failure		401	{object}	map[string]string
//	@Router			/conversations/{id}/messages [post]
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

// BlockUser godoc
//
//	@Summary		Block user
//	@Description	Block a user from contacting you
//	@Tags			users
//	@Produce		json
//	@Security		BearerAuth
//	@Param			username	path		string	true	"Username to block"
//	@Success		200	{object}	map[string]string
//	@Failure		400	{object}	map[string]string
//	@Failure		401	{object}	map[string]string
//	@Router			/blocks/{username} [post]
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

// UnblockUser godoc
//
//	@Summary		Unblock user
//	@Description	Remove a user from your block list
//	@Tags			users
//	@Produce		json
//	@Security		BearerAuth
//	@Param			username	path		string	true	"Username to unblock"
//	@Success		200	{object}	map[string]string
//	@Failure		400	{object}	map[string]string
//	@Failure		401	{object}	map[string]string
//	@Router			/blocks/{username} [delete]
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

// ============================================================================
// Starred Messages
// ============================================================================

// StarMessage godoc
//
//	@Summary		Star message
//	@Description	Add a message to your starred list
//	@Tags			messages
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id	path		string	true	"Message ID"
//	@Success		200	{object}	map[string]string
//	@Failure		400	{object}	map[string]string
//	@Failure		401	{object}	map[string]string
//	@Router			/messages/{id}/star [post]
func (h *ConversationHandler) StarMessage(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.GetUserID(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	messageID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid message ID")
		return
	}

	// Get the message to check membership
	msg, err := h.convs.GetMessageByID(r.Context(), messageID)
	if err != nil {
		if errors.Is(err, domain.ErrMessageNotFound) {
			writeError(w, http.StatusNotFound, "message not found")
			return
		}
		h.logger.Error("get message failed", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to get message")
		return
	}

	// Check membership
	isMember, err := h.convs.IsMember(r.Context(), msg.ConversationID, userID)
	if err != nil || !isMember {
		writeError(w, http.StatusForbidden, "not a member of this conversation")
		return
	}

	if err := h.convs.StarMessage(r.Context(), userID, messageID); err != nil {
		h.logger.Error("star message failed", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to star message")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "message starred"})
}

// UnstarMessage godoc
//
//	@Summary		Unstar message
//	@Description	Remove a message from your starred list
//	@Tags			messages
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id	path		string	true	"Message ID"
//	@Success		200	{object}	map[string]string
//	@Failure		400	{object}	map[string]string
//	@Failure		401	{object}	map[string]string
//	@Router			/messages/{id}/star [delete]
func (h *ConversationHandler) UnstarMessage(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.GetUserID(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	messageID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid message ID")
		return
	}

	if err := h.convs.UnstarMessage(r.Context(), userID, messageID); err != nil {
		h.logger.Error("unstar message failed", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to unstar message")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "message unstarred"})
}

// DeleteMessage godoc
//
//	@Summary		Delete message
//	@Description	Delete a message you sent (only the sender or group admin can delete)
//	@Tags			messages
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id	path		string	true	"Message ID"
//	@Success		200	{object}	map[string]string
//	@Failure		400	{object}	map[string]string
//	@Failure		401	{object}	map[string]string
//	@Failure		403	{object}	map[string]string
//	@Failure		404	{object}	map[string]string
//	@Router			/messages/{id} [delete]
func (h *ConversationHandler) DeleteMessage(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.GetUserID(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	messageID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid message ID")
		return
	}

	// Get message to check ownership and conversation
	msg, err := h.convs.GetMessageByID(r.Context(), messageID)
	if err != nil {
		if errors.Is(err, domain.ErrMessageNotFound) {
			writeError(w, http.StatusNotFound, "message not found")
			return
		}
		h.logger.Error("get message failed", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to get message")
		return
	}

	// Check permissions: sender can delete their own message, or admin can delete any in the group
	canDelete := false
	if msg.SenderID != nil && *msg.SenderID == userID {
		canDelete = true
	} else {
		// Check if user is admin of the conversation
		role, err := h.convs.GetMemberRole(r.Context(), msg.ConversationID, userID)
		if err == nil && role == domain.MemberRoleAdmin {
			canDelete = true
		}
	}

	if !canDelete {
		writeError(w, http.StatusForbidden, "you can only delete your own messages")
		return
	}

	// Delete the message
	if err := h.convs.DeleteMessage(r.Context(), messageID); err != nil {
		h.logger.Error("delete message failed", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to delete message")
		return
	}

	// Broadcast deletion to room
	if h.broadcaster != nil {
		if err := h.broadcaster.BroadcastMessageDeleted(r.Context(), messageID, msg.ConversationID, userID); err != nil {
			h.logger.Error("failed to broadcast message deletion", "error", err)
		}
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "message deleted"})
}

// GetStarredMessages godoc
//
//	@Summary		Get starred messages
//	@Description	Retrieve all messages you've starred
//	@Tags			messages
//	@Produce		json
//	@Security		BearerAuth
//	@Param			limit	query		int	false	"Result limit (default 50)"
//	@Success		200	{object}	object{messages=[]domain.Message,count=int}
//	@Failure		401	{object}	map[string]string
//	@Router			/messages/starred [get]
func (h *ConversationHandler) GetStarredMessages(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.GetUserID(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	limit := 50
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}

	messages, err := h.convs.GetStarredMessages(r.Context(), userID, limit)
	if err != nil {
		h.logger.Error("get starred messages failed", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to get starred messages")
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

// ============================================================================
// Message Search
// ============================================================================

// SearchMessages godoc
//
//	@Summary		Search messages in conversation
//	@Description	Full-text search within a specific conversation
//	@Tags			messages
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id	path		string	true	"Conversation ID"
//	@Param			q	query		string	true	"Search query"
//	@Param			limit	query		int	false	"Result limit (default 20)"
//	@Success		200	{object}	object{messages=[]domain.Message,count=int,query=string}
//	@Failure		400	{object}	map[string]string
//	@Failure		401	{object}	map[string]string
//	@Router			/conversations/{id}/messages/search [get]
func (h *ConversationHandler) SearchMessages(w http.ResponseWriter, r *http.Request) {
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

	query := r.URL.Query().Get("q")
	if query == "" {
		writeError(w, http.StatusBadRequest, "search query is required")
		return
	}

	limit := 50
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}

	messages, err := h.convs.SearchMessages(r.Context(), convID, query, limit)
	if err != nil {
		h.logger.Error("search messages failed", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to search messages")
		return
	}

	if messages == nil {
		messages = []domain.Message{}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"messages": messages,
		"count":    len(messages),
		"query":    query,
	})
}

// SearchAllMessages godoc
//
//	@Summary		Search all messages
//	@Description	Full-text search across all conversations you're in
//	@Tags			messages
//	@Produce		json
//	@Security		BearerAuth
//	@Param			q	query		string	true	"Search query"
//	@Param			limit	query		int	false	"Result limit (default 50)"
//	@Success		200	{object}	object{messages=[]domain.Message,count=int,query=string}
//	@Failure		400	{object}	map[string]string
//	@Failure		401	{object}	map[string]string
//	@Router			/messages/search [get]
func (h *ConversationHandler) SearchAllMessages(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.GetUserID(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	query := r.URL.Query().Get("q")
	if query == "" {
		writeError(w, http.StatusBadRequest, "search query is required")
		return
	}

	limit := 50
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}

	messages, err := h.convs.SearchAllMessages(r.Context(), userID, query, limit)
	if err != nil {
		h.logger.Error("search all messages failed", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to search messages")
		return
	}

	if messages == nil {
		messages = []domain.Message{}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"messages": messages,
		"count":    len(messages),
		"query":    query,
	})
}

// ============================================================================
// Archive
// ============================================================================

// ArchiveConversation godoc
//
//	@Summary		Archive conversation
//	@Description	Move a conversation to the archive
//	@Tags			conversations
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id	path		string	true	"Conversation ID"
//	@Success		200	{object}	map[string]string
//	@Failure		400	{object}	map[string]string
//	@Failure		401	{object}	map[string]string
//	@Router			/conversations/{id}/archive [post]
func (h *ConversationHandler) ArchiveConversation(w http.ResponseWriter, r *http.Request) {
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

	if err := h.convs.ArchiveConversation(r.Context(), convID); err != nil {
		h.logger.Error("archive conversation failed", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to archive conversation")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "conversation archived"})
}

// UnarchiveConversation godoc
//
//	@Summary		Unarchive conversation
//	@Description	Restore a conversation from the archive
//	@Tags			conversations
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id	path		string	true	"Conversation ID"
//	@Success		200	{object}	map[string]string
//	@Failure		400	{object}	map[string]string
//	@Failure		401	{object}	map[string]string
//	@Router			/conversations/{id}/unarchive [post]
func (h *ConversationHandler) UnarchiveConversation(w http.ResponseWriter, r *http.Request) {
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

	if err := h.convs.UnarchiveConversation(r.Context(), convID); err != nil {
		h.logger.Error("unarchive conversation failed", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to unarchive conversation")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "conversation unarchived"})
}

// ============================================================================
// Read Status
// ============================================================================

// MarkConversationRead godoc
//
//	@Summary		Mark conversation as read
//	@Description	Mark a conversation as read up to a specific message
//	@Tags			conversations
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id	path		string	true	"Conversation ID"
//	@Param			request	body		object{message_id=string}	false	"Last read message"
//	@Success		200	{object}	map[string]string
//	@Failure		400	{object}	map[string]string
//	@Failure		401	{object}	map[string]string
//	@Router			/conversations/{id}/read [post]
func (h *ConversationHandler) MarkConversationRead(w http.ResponseWriter, r *http.Request) {
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

	// Optional: include last message ID
	var input struct {
		MessageID string `json:"message_id"`
	}
	_ = json.NewDecoder(r.Body).Decode(&input)

	var messageID *uuid.UUID
	if input.MessageID != "" {
		id, err := uuid.Parse(input.MessageID)
		if err == nil {
			messageID = &id
		}
	}

	if err := h.convs.MarkConversationRead(r.Context(), convID, userID, messageID); err != nil {
		h.logger.Error("mark conversation read failed", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to mark conversation read")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "marked as read"})
}

// MarkAllConversationsRead godoc
//
//	@Summary		Mark all conversations as read
//	@Description	Mark all your conversations as read
//	@Tags			conversations
//	@Produce		json
//	@Security		BearerAuth
//	@Success		200	{object}	map[string]string
//	@Failure		401	{object}	map[string]string
//	@Router			/conversations/mark-all-read [post]
func (h *ConversationHandler) MarkAllConversationsRead(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.GetUserID(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	if err := h.convs.MarkAllConversationsRead(r.Context(), userID); err != nil {
		h.logger.Error("mark all read failed", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to mark all read")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "all conversations marked as read"})
}
