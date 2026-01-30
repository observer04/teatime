package domain

import "errors"

// Domain errors - use these for consistent error handling
var (
	// Auth errors
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrUserNotFound       = errors.New("user not found")
	ErrEmailTaken         = errors.New("email already registered")
	ErrUsernameTaken      = errors.New("username already taken")
	ErrTokenExpired       = errors.New("token has expired")
	ErrTokenRevoked       = errors.New("token has been revoked")
	ErrTokenInvalid       = errors.New("invalid token")

	// Conversation errors
	ErrConversationNotFound = errors.New("conversation not found")
	ErrNotMember            = errors.New("user is not a member of this conversation")
	ErrAlreadyMember        = errors.New("user is already a member")
	ErrCannotRemoveAdmin    = errors.New("cannot remove the last admin")

	// Message errors
	ErrMessageNotFound = errors.New("message not found")
	ErrEmptyMessage    = errors.New("message cannot be empty")

	// Block errors
	ErrUserBlocked = errors.New("user has blocked you")
	ErrSelfBlock   = errors.New("cannot block yourself")
)
