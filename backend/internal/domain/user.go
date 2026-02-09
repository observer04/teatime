package domain

import (
	"time"

	"github.com/google/uuid"
)

// User represents a registered user
type User struct {
	ID                  uuid.UUID  `json:"id"`
	Username            string     `json:"username"`
	Email               string     `json:"email,omitempty"` // omit in public responses
	DisplayName         string     `json:"display_name,omitempty"`
	AvatarURL           string     `json:"avatar_url,omitempty"`
	ShowOnlineStatus    bool       `json:"show_online_status"`
	ReadReceiptsEnabled bool       `json:"read_receipts_enabled"`
	LastSeenAt          *time.Time `json:"last_seen_at,omitempty"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
}

// PublicUser is the safe-to-expose version of User
type PublicUser struct {
	ID          uuid.UUID  `json:"id"`
	Username    string     `json:"username"`
	DisplayName string     `json:"display_name,omitempty"`
	AvatarURL   string     `json:"avatar_url,omitempty"`
	IsOnline    bool       `json:"is_online,omitempty"`    // Only set if user allows showing online status
	LastSeenAt  *time.Time `json:"last_seen_at,omitempty"` // Only set if user allows showing online status
}

func (u *User) ToPublic() PublicUser {
	pub := PublicUser{
		ID:          u.ID,
		Username:    u.Username,
		DisplayName: u.DisplayName,
		AvatarURL:   u.AvatarURL,
	}
	// Only expose presence info if user has opted in
	if u.ShowOnlineStatus {
		pub.LastSeenAt = u.LastSeenAt
	}
	return pub
}

// Credentials stores password hash separately from user
type Credentials struct {
	UserID       uuid.UUID `json:"-"`
	PasswordHash string    `json:"-"`
	CreatedAt    time.Time `json:"-"`
	UpdatedAt    time.Time `json:"-"`
}

// RefreshToken for JWT rotation
type RefreshToken struct {
	ID        uuid.UUID  `json:"id"`
	UserID    uuid.UUID  `json:"user_id"`
	TokenHash string     `json:"-"` // never expose
	ExpiresAt time.Time  `json:"expires_at"`
	CreatedAt time.Time  `json:"created_at"`
	RevokedAt *time.Time `json:"revoked_at,omitempty"`
}

func (rt *RefreshToken) IsValid() bool {
	return rt.RevokedAt == nil && time.Now().Before(rt.ExpiresAt)
}
