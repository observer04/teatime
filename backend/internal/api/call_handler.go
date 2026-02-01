package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/observer/teatime/internal/auth"
	"github.com/observer/teatime/internal/database"
)

// CallHandler handles call-related HTTP endpoints
type CallHandler struct {
	callRepo *database.CallRepository
	convRepo *database.ConversationRepository
	logger   *slog.Logger
}

// NewCallHandler creates a new CallHandler
func NewCallHandler(callRepo *database.CallRepository, convRepo *database.ConversationRepository, logger *slog.Logger) *CallHandler {
	return &CallHandler{
		callRepo: callRepo,
		convRepo: convRepo,
		logger:   logger,
	}
}

// GetCallHistory godoc
// @Summary Get user's call history
// @Tags calls
// @Security BearerAuth
// @Produce json
// @Param limit query int false "Limit (default 50)"
// @Param offset query int false "Offset (default 0)"
// @Success 200 {object} map[string]interface{}
// @Router /calls [get]
func (h *CallHandler) GetCallHistory(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.GetUserID(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	limit := 50
	offset := 0

	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 100 {
			limit = parsed
		}
	}
	if o := r.URL.Query().Get("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	calls, err := h.callRepo.GetUserCallHistoryWithDetails(r.Context(), userID, limit, offset)
	if err != nil {
		h.logger.Error("failed to get call history", "error", err, "user_id", userID)
		writeError(w, http.StatusInternalServerError, "Failed to get call history")
		return
	}

	if calls == nil {
		calls = []database.CallLog{}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"calls":  calls,
		"limit":  limit,
		"offset": offset,
	})
}

// GetCall godoc
// @Summary Get a specific call
// @Tags calls
// @Security BearerAuth
// @Produce json
// @Param id path string true "Call ID"
// @Success 200 {object} database.CallLog
// @Router /calls/{id} [get]
func (h *CallHandler) GetCall(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.GetUserID(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	callID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid call ID")
		return
	}

	call, err := h.callRepo.GetCallLog(r.Context(), callID)
	if err != nil {
		if err == database.ErrNotFound {
			writeError(w, http.StatusNotFound, "Call not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "Failed to get call")
		return
	}

	// Verify user is a member of the conversation
	isMember, err := h.convRepo.IsMember(r.Context(), call.ConversationID, userID)
	if err != nil || !isMember {
		writeError(w, http.StatusForbidden, "Not a member of this conversation")
		return
	}

	writeJSON(w, http.StatusOK, call)
}

// GetMissedCallCount godoc
// @Summary Get count of missed calls
// @Tags calls
// @Security BearerAuth
// @Produce json
// @Success 200 {object} map[string]int
// @Router /calls/missed/count [get]
func (h *CallHandler) GetMissedCallCount(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.GetUserID(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	// Count missed calls in the last 7 days
	count, err := h.callRepo.GetMissedCallCount(r.Context(), userID, time.Now().AddDate(0, 0, -7))
	if err != nil {
		h.logger.Error("failed to get missed call count", "error", err)
		count = 0
	}

	writeJSON(w, http.StatusOK, map[string]int{"count": count})
}

// CreateCallRequest is the request body for creating a call
type CreateCallRequest struct {
	ConversationID string `json:"conversation_id"`
	CallType       string `json:"call_type"` // "video" or "audio"
}

// CreateCall godoc
// @Summary Create a new call log
// @Tags calls
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param body body CreateCallRequest true "Call details"
// @Success 201 {object} database.CallLog
// @Router /calls [post]
func (h *CallHandler) CreateCall(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.GetUserID(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	var req CreateCallRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	conversationID, err := uuid.Parse(req.ConversationID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid conversation ID")
		return
	}

	// Verify user is a member
	isMember, err := h.convRepo.IsMember(r.Context(), conversationID, userID)
	if err != nil || !isMember {
		writeError(w, http.StatusForbidden, "Not a member of this conversation")
		return
	}

	callType := database.CallTypeVideo
	if req.CallType == "audio" {
		callType = database.CallTypeAudio
	}

	call, err := h.callRepo.CreateCallLog(r.Context(), conversationID, userID, callType)
	if err != nil {
		h.logger.Error("failed to create call", "error", err)
		writeError(w, http.StatusInternalServerError, "Failed to create call")
		return
	}

	writeJSON(w, http.StatusCreated, call)
}

// UpdateCallRequest is the request body for updating a call
type UpdateCallRequest struct {
	Status string `json:"status"` // "active", "ended", "missed", "declined", "cancelled"
}

// UpdateCall godoc
// @Summary Update a call's status
// @Tags calls
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path string true "Call ID"
// @Param body body UpdateCallRequest true "Call status"
// @Success 200 {object} map[string]string
// @Router /calls/{id} [patch]
func (h *CallHandler) UpdateCall(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.GetUserID(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	callID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid call ID")
		return
	}

	var req UpdateCallRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Get the call to verify access
	call, err := h.callRepo.GetCallLog(r.Context(), callID)
	if err != nil {
		if err == database.ErrNotFound {
			writeError(w, http.StatusNotFound, "Call not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "Failed to get call")
		return
	}

	// Verify user is a member of the conversation
	isMember, err := h.convRepo.IsMember(r.Context(), call.ConversationID, userID)
	if err != nil || !isMember {
		writeError(w, http.StatusForbidden, "Not authorized")
		return
	}

	var status database.CallStatus
	switch req.Status {
	case "active":
		if err := h.callRepo.StartCall(r.Context(), callID); err != nil {
			writeError(w, http.StatusInternalServerError, "Failed to start call")
			return
		}
	case "ended":
		if err := h.callRepo.EndCall(r.Context(), callID); err != nil {
			writeError(w, http.StatusInternalServerError, "Failed to end call")
			return
		}
	case "missed":
		status = database.CallStatusMissed
	case "declined":
		status = database.CallStatusDeclined
	case "cancelled":
		status = database.CallStatusCancelled
	default:
		writeError(w, http.StatusBadRequest, "Invalid status")
		return
	}

	if status != "" {
		if err := h.callRepo.UpdateCallStatus(r.Context(), callID, status); err != nil {
			writeError(w, http.StatusInternalServerError, "Failed to update call")
			return
		}
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}
