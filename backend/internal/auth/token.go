package auth

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// TokenType distinguishes access vs refresh tokens
type TokenType string

const (
	TokenTypeAccess  TokenType = "access"
	TokenTypeRefresh TokenType = "refresh"
)

// Claims represents the JWT claims
type Claims struct {
	jwt.RegisteredClaims
	UserID   uuid.UUID `json:"uid"`
	Username string    `json:"username"`
	Type     TokenType `json:"type"`
}

// TokenService handles JWT creation and validation
type TokenService struct {
	signingKey      []byte
	accessTokenTTL  time.Duration
	refreshTokenTTL time.Duration
}

// NewTokenService creates a new token service
func NewTokenService(signingKey string) (*TokenService, error) {
	if len(signingKey) < 32 {
		return nil, errors.New("signing key must be at least 32 characters")
	}
	return &TokenService{
		signingKey:      []byte(signingKey),
		accessTokenTTL:  24 * time.Hour,     // 24 hours (increased from 15m to fix frequent timeouts)
		refreshTokenTTL: 7 * 24 * time.Hour, // 7 days
	}, nil
}

// GenerateAccessToken creates a short-lived access token
func (s *TokenService) GenerateAccessToken(userID uuid.UUID, username string) (string, time.Time, error) {
	now := time.Now()
	expiresAt := now.Add(s.accessTokenTTL)

	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        uuid.NewString(),
			Subject:   userID.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			Issuer:    "teatime",
		},
		UserID:   userID,
		Username: username,
		Type:     TokenTypeAccess,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(s.signingKey)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("sign token: %w", err)
	}

	return signed, expiresAt, nil
}

// GenerateRefreshToken creates a long-lived refresh token (opaque, not JWT)
// We use opaque tokens for refresh so they can be revoked by deleting from DB
func (s *TokenService) GenerateRefreshToken() (string, time.Time, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", time.Time{}, fmt.Errorf("generate random bytes: %w", err)
	}

	token := base64.URLEncoding.EncodeToString(bytes)
	expiresAt := time.Now().Add(s.refreshTokenTTL)

	return token, expiresAt, nil
}

// ValidateAccessToken parses and validates an access token
func (s *TokenService) ValidateAccessToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return s.signingKey, nil
	})

	if err != nil {
		return nil, fmt.Errorf("parse token: %w", err)
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token claims")
	}

	if claims.Type != TokenTypeAccess {
		return nil, errors.New("not an access token")
	}

	return claims, nil
}

// AccessTokenTTL returns the access token duration (for cookie MaxAge)
func (s *TokenService) AccessTokenTTL() time.Duration {
	return s.accessTokenTTL
}

// RefreshTokenTTL returns the refresh token duration
func (s *TokenService) RefreshTokenTTL() time.Duration {
	return s.refreshTokenTTL
}
