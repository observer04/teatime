package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/observer/teatime/internal/auth"
	"github.com/observer/teatime/internal/database"
)

// UserHandler handles user-related endpoints
type UserHandler struct {
	users  *database.UserRepository
	logger *slog.Logger
}

func NewUserHandler(users *database.UserRepository, logger *slog.Logger) *UserHandler {
	return &UserHandler{
		users:  users,
		logger: logger,
	}
}

// Search handles GET /users/search?q=...
func (h *UserHandler) Search(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if len(query) < 2 {
		writeError(w, http.StatusBadRequest, "query must be at least 2 characters")
		return
	}

	limitStr := r.URL.Query().Get("limit")
	limit := 20
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 50 {
			limit = l
		}
	}

	users, err := h.users.SearchByUsername(r.Context(), query, limit)
	if err != nil {
		h.logger.Error("search users failed", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to search users")
		return
	}

	// Convert to public users
	publicUsers := make([]interface{}, len(users))
	for i, u := range users {
		publicUsers[i] = u.ToPublic()
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"users": publicUsers,
		"count": len(users),
	})
}

// GetByUsername handles GET /users/{username}
func (h *UserHandler) GetByUsername(w http.ResponseWriter, r *http.Request) {
	username := r.PathValue("username")
	if username == "" {
		writeError(w, http.StatusBadRequest, "username required")
		return
	}

	user, err := h.users.GetByUsername(r.Context(), username)
	if err != nil {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}

	writeJSON(w, http.StatusOK, user.ToPublic())
}

// UpdateProfile handles PUT /users/me
func (h *UserHandler) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.GetUserID(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var input struct {
		DisplayName string `json:"display_name"`
		AvatarURL   string `json:"avatar_url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Validate
	if len(input.DisplayName) > 100 {
		writeError(w, http.StatusBadRequest, "display name too long (max 100)")
		return
	}
	if len(input.AvatarURL) > 500 {
		writeError(w, http.StatusBadRequest, "avatar URL too long")
		return
	}

	// Get current user
	user, err := h.users.GetByID(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}

	// Update fields
	user.DisplayName = input.DisplayName
	user.AvatarURL = input.AvatarURL

	if err := h.users.Update(r.Context(), user); err != nil {
		h.logger.Error("update user failed", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to update profile")
		return
	}

	writeJSON(w, http.StatusOK, user.ToPublic())
}

// GetMe handles GET /users/me - returns full user info
func (h *UserHandler) GetMe(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.GetUserID(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	user, err := h.users.GetByID(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}

	writeJSON(w, http.StatusOK, user)
}
