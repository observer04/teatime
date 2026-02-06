package auth

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/observer/teatime/internal/domain"
)

// UserRepository interface for auth operations
type UserRepository interface {
	Create(ctx context.Context, user *domain.User, passwordHash string) error
	GetByEmail(ctx context.Context, email string) (*domain.User, error)
	GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error)
	GetPasswordHash(ctx context.Context, userID uuid.UUID) (string, error)
	EmailExists(ctx context.Context, email string) (bool, error)
	UsernameExists(ctx context.Context, username string) (bool, error)

	CreateRefreshToken(ctx context.Context, userID uuid.UUID, token string, expiresAt time.Time) (uuid.UUID, error)
	GetRefreshToken(ctx context.Context, token string) (*domain.RefreshToken, error)
	RevokeRefreshToken(ctx context.Context, tokenID uuid.UUID) error
	RevokeAllUserTokens(ctx context.Context, userID uuid.UUID) error
}

// Service handles authentication logic
type Service struct {
	users  UserRepository
	tokens *TokenService
}

// NewService creates an auth service
func NewService(users UserRepository, tokens *TokenService) *Service {
	return &Service{
		users:  users,
		tokens: tokens,
	}
}

// TokenPair holds both access and refresh tokens
type TokenPair struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"-"` // Never in JSON body, goes to cookie
	ExpiresAt    time.Time `json:"expires_at"`
}

// RegisterInput for user registration
type RegisterInput struct {
	Email    string `json:"email"`
	Username string `json:"username"`
	Password string `json:"password"`
}

// Register creates a new user account
func (s *Service) Register(ctx context.Context, input RegisterInput) (*domain.User, *TokenPair, error) {
	// Validate input
	if err := validateEmail(input.Email); err != nil {
		return nil, nil, err
	}
	if err := validateUsername(input.Username); err != nil {
		return nil, nil, err
	}
	if err := validatePassword(input.Password); err != nil {
		return nil, nil, err
	}

	// Check email uniqueness
	exists, err := s.users.EmailExists(ctx, input.Email)
	if err != nil {
		return nil, nil, fmt.Errorf("check email: %w", err)
	}
	if exists {
		return nil, nil, domain.ErrEmailTaken
	}

	// Check username uniqueness
	exists, err = s.users.UsernameExists(ctx, input.Username)
	if err != nil {
		return nil, nil, fmt.Errorf("check username: %w", err)
	}
	if exists {
		return nil, nil, domain.ErrUsernameTaken
	}

	// Hash password
	hash, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, nil, fmt.Errorf("hash password: %w", err)
	}

	// Create user
	user := &domain.User{
		ID:        uuid.New(),
		Email:     strings.ToLower(input.Email),
		Username:  strings.ToLower(input.Username),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := s.users.Create(ctx, user, string(hash)); err != nil {
		return nil, nil, fmt.Errorf("create user: %w", err)
	}

	// Generate tokens
	tokens, err := s.generateTokenPair(ctx, user)
	if err != nil {
		return nil, nil, err
	}

	return user, tokens, nil
}

// LoginInput for user login
type LoginInput struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// Login authenticates a user
func (s *Service) Login(ctx context.Context, input LoginInput) (*domain.User, *TokenPair, error) {
	// Find user by email
	user, err := s.users.GetByEmail(ctx, strings.ToLower(input.Email))
	if err != nil {
		if errors.Is(err, domain.ErrUserNotFound) {
			return nil, nil, domain.ErrInvalidCredentials
		}
		return nil, nil, fmt.Errorf("find user: %w", err)
	}

	// Get password hash
	hash, err := s.users.GetPasswordHash(ctx, user.ID)
	if err != nil {
		return nil, nil, fmt.Errorf("get password: %w", err)
	}

	// Compare password
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(input.Password)); err != nil {
		return nil, nil, domain.ErrInvalidCredentials
	}

	// Generate tokens
	tokens, err := s.generateTokenPair(ctx, user)
	if err != nil {
		return nil, nil, err
	}

	return user, tokens, nil
}

// Refresh generates new tokens using a refresh token
func (s *Service) Refresh(ctx context.Context, refreshToken string) (*domain.User, *TokenPair, error) {
	// Get stored token
	storedToken, err := s.users.GetRefreshToken(ctx, refreshToken)
	if err != nil {
		return nil, nil, domain.ErrTokenInvalid
	}

	// Validate token
	if !storedToken.IsValid() {
		if storedToken.RevokedAt != nil {
			return nil, nil, domain.ErrTokenRevoked
		}
		return nil, nil, domain.ErrTokenExpired
	}

	// Revoke old token (rotation)
	if err := s.users.RevokeRefreshToken(ctx, storedToken.ID); err != nil {
		return nil, nil, fmt.Errorf("revoke old token: %w", err)
	}

	// Get user
	user, err := s.users.GetByID(ctx, storedToken.UserID)
	if err != nil {
		return nil, nil, fmt.Errorf("get user: %w", err)
	}

	// Generate new tokens
	tokens, err := s.generateTokenPair(ctx, user)
	if err != nil {
		return nil, nil, err
	}

	return user, tokens, nil
}

// Logout revokes a refresh token
func (s *Service) Logout(ctx context.Context, refreshToken string) error {
	storedToken, err := s.users.GetRefreshToken(ctx, refreshToken)
	if err != nil {
		// Already invalid, consider it logged out
		return nil
	}
	return s.users.RevokeRefreshToken(ctx, storedToken.ID)
}

// LogoutAll revokes all refresh tokens for a user
func (s *Service) LogoutAll(ctx context.Context, userID uuid.UUID) error {
	return s.users.RevokeAllUserTokens(ctx, userID)
}

// ValidateToken validates an access token and returns claims
func (s *Service) ValidateToken(tokenString string) (*Claims, error) {
	return s.tokens.ValidateAccessToken(tokenString)
}

// generateTokenPair creates both access and refresh tokens
func (s *Service) generateTokenPair(ctx context.Context, user *domain.User) (*TokenPair, error) {
	// Generate access token
	accessToken, expiresAt, err := s.tokens.GenerateAccessToken(user.ID, user.Username)
	if err != nil {
		return nil, fmt.Errorf("generate access token: %w", err)
	}

	// Generate refresh token
	refreshToken, refreshExpiresAt, err := s.tokens.GenerateRefreshToken()
	if err != nil {
		return nil, fmt.Errorf("generate refresh token: %w", err)
	}

	// Store refresh token
	_, err = s.users.CreateRefreshToken(ctx, user.ID, refreshToken, refreshExpiresAt)
	if err != nil {
		return nil, fmt.Errorf("store refresh token: %w", err)
	}

	return &TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresAt:    expiresAt,
	}, nil
}

// RefreshTokenTTL returns refresh token duration for cookie
func (s *Service) RefreshTokenTTL() time.Duration {
	return s.tokens.RefreshTokenTTL()
}

// GenerateAccessToken creates an access token (for OAuth flow)
func (s *Service) GenerateAccessToken(userID uuid.UUID, username string) (string, error) {
	token, _, err := s.tokens.GenerateAccessToken(userID, username)
	return token, err
}

// GenerateRefreshToken creates a refresh token (for OAuth flow)
func (s *Service) GenerateRefreshToken() (string, time.Time, error) {
	return s.tokens.GenerateRefreshToken()
}

// ============================================================================
// Validation helpers
// ============================================================================

var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
var usernameRegex = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_]{2,31}$`)

func validateEmail(email string) error {
	if !emailRegex.MatchString(email) {
		return errors.New("invalid email format")
	}
	return nil
}

func validateUsername(username string) error {
	if !usernameRegex.MatchString(username) {
		return errors.New("username must be 3-32 characters, start with letter, contain only letters, numbers, underscore")
	}
	return nil
}

func validatePassword(password string) error {
	if len(password) < 8 {
		return errors.New("password must be at least 8 characters")
	}

	var hasUpper, hasLower, hasNumber bool
	for _, c := range password {
		switch {
		case unicode.IsUpper(c):
			hasUpper = true
		case unicode.IsLower(c):
			hasLower = true
		case unicode.IsNumber(c):
			hasNumber = true
		}
	}

	if !hasUpper || !hasLower || !hasNumber {
		return errors.New("password must contain uppercase, lowercase, and number")
	}

	return nil
}
