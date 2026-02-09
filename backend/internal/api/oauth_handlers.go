package api

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"strings"

	"github.com/google/uuid"
	"github.com/observer/teatime/internal/auth"
	"github.com/observer/teatime/internal/database"
	"github.com/observer/teatime/internal/domain"
)

// OAuthHandlers handles OAuth-related API endpoints
type OAuthHandlers struct {
	oauthService *auth.OAuthService
	authService  *auth.Service
	userRepo     *database.UserRepository
	appBaseURL   string
	logger       *slog.Logger
}

// NewOAuthHandlers creates a new OAuth handlers instance
func NewOAuthHandlers(
	oauthService *auth.OAuthService,
	authService *auth.Service,
	userRepo *database.UserRepository,
	appBaseURL string,
) *OAuthHandlers {
	return &OAuthHandlers{
		oauthService: oauthService,
		authService:  authService,
		userRepo:     userRepo,
		appBaseURL:   appBaseURL,
		logger:       slog.Default().With("component", "oauth-handlers"),
	}
}

// HandleGoogleAuth initiates the Google OAuth flow
func (h *OAuthHandlers) HandleGoogleAuth(w http.ResponseWriter, r *http.Request) {
	authURL, state, err := h.oauthService.GetAuthURL()
	if err != nil {
		h.logger.Error("failed to generate auth URL", "error", err)
		h.redirectWithError(w, r, "Failed to initiate login")
		return
	}

	h.logger.Info("redirecting to Google OAuth", "state", state[:8]+"...")

	http.Redirect(w, r, authURL, http.StatusTemporaryRedirect)
}

// HandleGoogleCallback handles the OAuth callback from Google
func (h *OAuthHandlers) HandleGoogleCallback(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Check for error from Google
	if errParam := r.URL.Query().Get("error"); errParam != "" {
		h.logger.Warn("OAuth error from Google", "error", errParam)
		h.redirectWithError(w, r, "Authentication cancelled")
		return
	}

	// Validate state parameter (CSRF protection)
	state := r.URL.Query().Get("state")
	if !h.oauthService.ValidateState(state) {
		h.logger.Warn("invalid OAuth state", "state", state[:8]+"...")
		h.redirectWithError(w, r, "Invalid authentication state")
		return
	}

	// Get authorization code
	code := r.URL.Query().Get("code")
	if code == "" {
		h.logger.Warn("missing authorization code")
		h.redirectWithError(w, r, "Missing authorization code")
		return
	}

	// Exchange code for user info
	googleUser, err := h.oauthService.ExchangeCode(ctx, code)
	if err != nil {
		h.logger.Error("failed to exchange code", "error", err)
		h.redirectWithError(w, r, "Failed to authenticate with Google")
		return
	}

	// Check if email is verified
	if !googleUser.VerifiedEmail {
		h.logger.Warn("unverified Google email", "email", googleUser.Email)
		h.redirectWithError(w, r, "Please verify your Google email first")
		return
	}

	// Try to find existing user by OAuth identity
	user, err := h.userRepo.GetUserByOAuthProvider(ctx, "google", googleUser.ID)
	if err != nil && !errors.Is(err, domain.ErrUserNotFound) {
		h.logger.Error("failed to lookup OAuth user", "error", err)
		h.redirectWithError(w, r, "Database error")
		return
	}

	var needsUsername bool

	if user == nil {
		// No OAuth identity found, check if user exists by email
		user, err = h.userRepo.GetByEmail(ctx, googleUser.Email)
		if err != nil && !errors.Is(err, domain.ErrUserNotFound) {
			h.logger.Error("failed to lookup user by email", "error", err)
			h.redirectWithError(w, r, "Database error")
			return
		}

		if user != nil {
			// User exists with this email, link the OAuth identity
			h.logger.Info("linking OAuth to existing user", "user_id", user.ID, "email", googleUser.Email)
			if err := h.userRepo.CreateOAuthIdentity(ctx, user.ID, "google", googleUser.ID); err != nil {
				h.logger.Error("failed to link OAuth identity", "error", err)
				h.redirectWithError(w, r, "Failed to link account")
				return
			}
		} else {
			// New user - create account
			h.logger.Info("creating new OAuth user", "email", googleUser.Email, "name", googleUser.Name)

			// Generate a temporary username (user will be prompted to change it)
			tempUsername := h.generateTempUsername(googleUser.Name)

			user = &domain.User{
				ID:          uuid.New(),
				Username:    tempUsername,
				Email:       googleUser.Email,
				DisplayName: googleUser.Name,
				AvatarURL:   googleUser.Picture,
			}

			if err := h.userRepo.CreateUserWithOAuth(ctx, user, "google", googleUser.ID); err != nil {
				h.logger.Error("failed to create OAuth user", "error", err)
				h.redirectWithError(w, r, "Failed to create account")
				return
			}

			needsUsername = true
		}
	}

	// Generate JWT tokens
	accessToken, err := h.authService.GenerateAccessToken(user.ID, user.Username)
	if err != nil {
		h.logger.Error("failed to generate access token", "error", err)
		h.redirectWithError(w, r, "Failed to generate session")
		return
	}

	refreshToken, expiresAt, err := h.authService.GenerateRefreshToken()
	if err != nil {
		h.logger.Error("failed to generate refresh token", "error", err)
		h.redirectWithError(w, r, "Failed to generate session")
		return
	}

	// Store refresh token
	if _, err := h.userRepo.CreateRefreshToken(ctx, user.ID, refreshToken, expiresAt); err != nil {
		h.logger.Error("failed to store refresh token", "error", err)
		h.redirectWithError(w, r, "Failed to create session")
		return
	}

	// Set refresh token as HTTP-only cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    refreshToken,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		Expires:  expiresAt,
	})

	h.logger.Info("OAuth login successful", "user_id", user.ID, "email", user.Email)

	// Redirect to frontend with tokens in URL hash
	redirectURL := fmt.Sprintf("%s/#oauth_token=%s&needs_username=%t",
		h.appBaseURL,
		accessToken,
		needsUsername,
	)

	http.Redirect(w, r, redirectURL, http.StatusTemporaryRedirect)
}

// redirectWithError redirects to the frontend with an error message
func (h *OAuthHandlers) redirectWithError(w http.ResponseWriter, r *http.Request, message string) {
	redirectURL := fmt.Sprintf("%s/#oauth_error=%s", h.appBaseURL, message)
	http.Redirect(w, r, redirectURL, http.StatusTemporaryRedirect)
}

// generateTempUsername creates a temporary username from the user's name
func (h *OAuthHandlers) generateTempUsername(name string) string {
	// Remove special characters and spaces
	reg := regexp.MustCompile(`[^a-zA-Z0-9]`)
	base := reg.ReplaceAllString(strings.ToLower(name), "")

	// Limit length
	if len(base) > 20 {
		base = base[:20]
	}
	if len(base) < 3 {
		base = "user"
	}

	// Add random suffix to ensure uniqueness
	suffix := make([]byte, 4)
	_, _ = rand.Read(suffix)
	return fmt.Sprintf("%s_%s", base, hex.EncodeToString(suffix))
}

// HandleSetUsername allows OAuth users to set their username
func (h *OAuthHandlers) HandleSetUsername(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get user ID from context (set by auth middleware)
	userID, ok := auth.GetUserID(ctx)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req struct {
		Username string `json:"username"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Validate username
	if len(req.Username) < 3 || len(req.Username) > 32 {
		writeError(w, http.StatusBadRequest, "username must be 3-32 characters")
		return
	}

	// Check username format (alphanumeric and underscores only)
	if matched, _ := regexp.MatchString(`^[a-zA-Z0-9_]+$`, req.Username); !matched {
		writeError(w, http.StatusBadRequest, "username can only contain letters, numbers, and underscores")
		return
	}

	// Check if username is taken
	exists, err := h.userRepo.UsernameExists(ctx, req.Username)
	if err != nil {
		h.logger.Error("failed to check username", "error", err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	if exists {
		writeError(w, http.StatusConflict, "username already taken")
		return
	}

	// Update username
	if err := h.userRepo.UpdateUsername(ctx, userID, req.Username); err != nil {
		h.logger.Error("failed to update username", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to update username")
		return
	}

	// Get updated user
	user, err := h.userRepo.GetByID(ctx, userID)
	if err != nil {
		h.logger.Error("failed to get user", "error", err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}

	// Generate new access token with updated username
	accessToken, err := h.authService.GenerateAccessToken(user.ID, user.Username)
	if err != nil {
		h.logger.Error("failed to generate access token", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to generate token")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"access_token": accessToken,
		"user":         user.ToPublic(),
	})
}
