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

// Search godoc
//
//	@Summary		Search users
//	@Description	Search for users by username
//	@Tags			users
//	@Produce		json
//	@Param			q	query		string	true	"Search query (min 2 chars)"
//	@Param			limit	query		int	false	"Result limit (default 20, max 50)"
//	@Success		200	{object}	object{users=[]interface{},count=int}
//	@Failure		400	{object}	map[string]string
//	@Router			/users/search [get]
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

// GetByUsername godoc
//
//	@Summary		Get user by username
//	@Description	Retrieve user profile by username
//	@Tags			users
//	@Produce		json
//	@Param			username	path		string	true	"Username"
//	@Success		200	{object}	interface{}
//	@Failure		400	{object}	map[string]string
//	@Failure		404	{object}	map[string]string
//	@Router			/users/{username} [get]
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

// UpdateProfile godoc
//
//	@Summary		Update profile
//	@Description	Update your display name and avatar
//	@Tags			users
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			request	body		object{display_name=string,avatar_url=string}	true	"Profile updates"
//	@Success		200	{object}	interface{}
//	@Failure		400	{object}	map[string]string
//	@Failure		401	{object}	map[string]string
//	@Router			/users/me [put]
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

// GetMe godoc
//
//	@Summary		Get your profile
//	@Description	Retrieve your own user profile
//	@Tags			users
//	@Produce		json
//	@Security		BearerAuth
//	@Success		200	{object}	interface{}
//	@Failure		401	{object}	map[string]string
//	@Router			/users/me [get]
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

// UpdatePreferences godoc
//
//	@Summary		Update preferences
//	@Description	Update privacy preferences (online status, read receipts)
//	@Tags			users
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			request	body		object{show_online_status=bool,read_receipts_enabled=bool}	true	"Preferences"
//	@Success		200	{object}	interface{}
//	@Failure		400	{object}	map[string]string
//	@Failure		401	{object}	map[string]string
//	@Router			/users/me/preferences [patch]
func (h *UserHandler) UpdatePreferences(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.GetUserID(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var input struct {
		ShowOnlineStatus    bool `json:"show_online_status"`
		ReadReceiptsEnabled bool `json:"read_receipts_enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.users.UpdatePreferences(r.Context(), userID, input.ShowOnlineStatus, input.ReadReceiptsEnabled); err != nil {
		h.logger.Error("update preferences failed", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to update preferences")
		return
	}

	// Return updated user
	user, err := h.users.GetByID(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}

	writeJSON(w, http.StatusOK, user)
}

// DeleteAccount godoc
//
//	@Summary		Delete account
//	@Description	Permanently delete your account and all associated data
//	@Tags			users
//	@Produce		json
//	@Security		BearerAuth
//	@Success		200	{object}	map[string]string
//	@Failure		401	{object}	map[string]string
//	@Failure		500	{object}	map[string]string
//	@Router			/users/me [delete]
func (h *UserHandler) DeleteAccount(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.GetUserID(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	if err := h.users.DeleteUser(r.Context(), userID); err != nil {
		h.logger.Error("delete account failed", "error", err, "user_id", userID)
		writeError(w, http.StatusInternalServerError, "failed to delete account")
		return
	}

	h.logger.Info("account deleted", "user_id", userID)
	writeJSON(w, http.StatusOK, map[string]string{"message": "account deleted successfully"})
}

