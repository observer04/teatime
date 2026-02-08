package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

// GoogleUser represents user info returned from Google OAuth
type GoogleUser struct {
	ID            string `json:"id"`
	Email         string `json:"email"`
	VerifiedEmail bool   `json:"verified_email"`
	Name          string `json:"name"`
	GivenName     string `json:"given_name"`
	FamilyName    string `json:"family_name"`
	Picture       string `json:"picture"`
}

// OAuthService handles Google OAuth flow
type OAuthService struct {
	config *oauth2.Config
	logger *slog.Logger

	// State token store (in-memory for now, expires after 10 minutes)
	states   map[string]time.Time
	statesMu sync.Mutex
}

// NewOAuthService creates a new OAuth service
func NewOAuthService(clientID, clientSecret, redirectURL string) *OAuthService {
	config := &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  redirectURL,
		Scopes: []string{
			"https://www.googleapis.com/auth/userinfo.email",
			"https://www.googleapis.com/auth/userinfo.profile",
		},
		Endpoint: google.Endpoint,
	}

	svc := &OAuthService{
		config: config,
		logger: slog.Default().With("component", "oauth"),
		states: make(map[string]time.Time),
	}

	// Start cleanup goroutine
	go svc.cleanupExpiredStates()

	return svc
}

// GetAuthURL generates the Google OAuth authorization URL
func (s *OAuthService) GetAuthURL() (string, string, error) {
	state, err := s.generateState()
	if err != nil {
		return "", "", fmt.Errorf("failed to generate state: %w", err)
	}

	url := s.config.AuthCodeURL(state, oauth2.AccessTypeOffline)
	return url, state, nil
}

// ValidateState checks if the state parameter is valid
func (s *OAuthService) ValidateState(state string) bool {
	s.statesMu.Lock()
	defer s.statesMu.Unlock()

	expiresAt, ok := s.states[state]
	if !ok {
		return false
	}

	// Delete the state (one-time use)
	delete(s.states, state)

	// Check if expired
	return time.Now().Before(expiresAt)
}

// ExchangeCode exchanges the authorization code for Google user info
func (s *OAuthService) ExchangeCode(ctx context.Context, code string) (*GoogleUser, error) {
	// Exchange code for token
	token, err := s.config.Exchange(ctx, code)
	if err != nil {
		s.logger.Error("failed to exchange code", "error", err)
		return nil, fmt.Errorf("failed to exchange code: %w", err)
	}

	// Fetch user info using the access token
	client := s.config.Client(ctx, token)
	resp, err := client.Get("https://www.googleapis.com/oauth2/v2/userinfo")
	if err != nil {
		s.logger.Error("failed to fetch user info", "error", err)
		return nil, fmt.Errorf("failed to fetch user info: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		s.logger.Error("user info request failed", "status", resp.StatusCode, "body", string(body))
		return nil, fmt.Errorf("user info request failed: %s", resp.Status)
	}

	var user GoogleUser
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		s.logger.Error("failed to decode user info", "error", err)
		return nil, fmt.Errorf("failed to decode user info: %w", err)
	}

	s.logger.Info("successfully fetched Google user info",
		"google_id", user.ID,
		"email", user.Email,
		"name", user.Name,
	)

	return &user, nil
}

// generateState creates a cryptographically secure random state string
func (s *OAuthService) generateState() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}

	state := base64.URLEncoding.EncodeToString(b)

	// Store state with expiration
	s.statesMu.Lock()
	s.states[state] = time.Now().Add(10 * time.Minute)
	s.statesMu.Unlock()

	return state, nil
}

// cleanupExpiredStates periodically removes expired states
func (s *OAuthService) cleanupExpiredStates() {
	ticker := time.NewTicker(5 * time.Minute)
	for range ticker.C {
		s.statesMu.Lock()
		now := time.Now()
		for state, expiresAt := range s.states {
			if now.After(expiresAt) {
				delete(s.states, state)
			}
		}
		s.statesMu.Unlock()
	}
}
