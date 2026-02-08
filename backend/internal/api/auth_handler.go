package api

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/observer/teatime/internal/auth"
	"github.com/observer/teatime/internal/domain"
)

// AuthHandler handles authentication endpoints
type AuthHandler struct {
	auth   *auth.Service
	logger *slog.Logger
}

func NewAuthHandler(authService *auth.Service, logger *slog.Logger) *AuthHandler {
	return &AuthHandler{
		auth:   authService,
		logger: logger,
	}
}

// Register godoc
//
//	@Summary		Register a new user
//	@Description	Create a new user account with username, email, and password
//	@Tags			auth
//	@Accept			json
//	@Produce		json
//	@Param			request	body		auth.RegisterInput	true	"Registration details"
//	@Success		201		{object}	map[string]interface{}	"User created successfully"
//	@Failure		400		{object}	map[string]string	"Invalid input"
//	@Failure		409		{object}	map[string]string	"Username or email already exists"
//	@Router			/auth/register [post]
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var input auth.RegisterInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	user, tokens, err := h.auth.Register(r.Context(), input)
	if err != nil {
		h.handleAuthError(w, err)
		return
	}

	// Set refresh token cookie
	h.setRefreshTokenCookie(w, tokens.RefreshToken)

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"user":         user.ToPublic(),
		"access_token": tokens.AccessToken,
		"expires_at":   tokens.ExpiresAt,
	})
}

// Login godoc
//
//	@Summary		Login
//	@Description	Authenticate user with email/username and password
//	@Tags			auth
//	@Accept			json
//	@Produce		json
//	@Param			request	body		auth.LoginInput	true	"Login credentials"
//	@Success		200		{object}	map[string]interface{}	"Login successful"
//	@Failure		400		{object}	map[string]string	"Invalid input"
//	@Failure		401		{object}	map[string]string	"Invalid credentials"
//	@Router			/auth/login [post]
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var input auth.LoginInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	user, tokens, err := h.auth.Login(r.Context(), input)
	if err != nil {
		h.handleAuthError(w, err)
		return
	}

	// Set refresh token cookie
	h.setRefreshTokenCookie(w, tokens.RefreshToken)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"user":         user.ToPublic(),
		"access_token": tokens.AccessToken,
		"expires_at":   tokens.ExpiresAt,
	})
}

// Refresh godoc
//
//	@Summary		Refresh token
//	@Description	Get a new access token using refresh token from cookie
//	@Tags			auth
//	@Produce		json
//	@Success		200	{object}	object{user=interface{},access_token=string,expires_at=int}
//	@Failure		401	{object}	map[string]string
//	@Router			/auth/refresh [post]
func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	// Get refresh token from cookie
	cookie, err := r.Cookie("refresh_token")
	if err != nil {
		writeError(w, http.StatusUnauthorized, "refresh token required")
		return
	}

	user, tokens, err := h.auth.Refresh(r.Context(), cookie.Value)
	if err != nil {
		h.handleAuthError(w, err)
		return
	}

	// Set new refresh token cookie
	h.setRefreshTokenCookie(w, tokens.RefreshToken)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"user":         user.ToPublic(),
		"access_token": tokens.AccessToken,
		"expires_at":   tokens.ExpiresAt,
	})
}

// Logout godoc
//
//	@Summary		Logout
//	@Description	Invalidate refresh token and clear cookies
//	@Tags			auth
//	@Produce		json
//	@Success		200	{object}	map[string]string
//	@Router			/auth/logout [post]
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	// Get refresh token from cookie
	cookie, err := r.Cookie("refresh_token")
	if err == nil {
		_ = h.auth.Logout(r.Context(), cookie.Value)
	}

	// Clear cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	})

	writeJSON(w, http.StatusOK, map[string]string{"status": "logged out"})
}

// Me godoc
//
//	@Summary		Get authenticated user
//	@Description	Get info about the currently authenticated user
//	@Tags			auth
//	@Produce		json
//	@Security		BearerAuth
//	@Success		200	{object}	object{id=string,username=string}
//	@Failure		401	{object}	map[string]string
//	@Router			/auth/me [get]
func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.GetUserID(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	// In a real implementation, we'd fetch fresh user data
	// For now, return info from token
	username, _ := auth.GetUsername(r.Context())

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"id":       userID,
		"username": username,
	})
}

func (h *AuthHandler) setRefreshTokenCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    token,
		Path:     "/",
		MaxAge:   int(h.auth.RefreshTokenTTL().Seconds()),
		HttpOnly: true,
		Secure:   true, // Set to false for local development without HTTPS
		SameSite: http.SameSiteLaxMode,
	})
}

func (h *AuthHandler) handleAuthError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrInvalidCredentials):
		writeError(w, http.StatusUnauthorized, "invalid email or password")
	case errors.Is(err, domain.ErrEmailTaken):
		writeError(w, http.StatusConflict, "email already registered")
	case errors.Is(err, domain.ErrUsernameTaken):
		writeError(w, http.StatusConflict, "username already taken")
	case errors.Is(err, domain.ErrTokenInvalid):
		writeError(w, http.StatusUnauthorized, "invalid token")
	case errors.Is(err, domain.ErrTokenExpired):
		writeError(w, http.StatusUnauthorized, "token expired")
	case errors.Is(err, domain.ErrTokenRevoked):
		writeError(w, http.StatusUnauthorized, "token revoked")
	default:
		h.logger.Error("auth error", "error", err)
		writeError(w, http.StatusBadRequest, err.Error())
	}
}
