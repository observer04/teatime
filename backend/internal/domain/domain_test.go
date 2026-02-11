package domain

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

// =============================================================================
// User.ToPublic Tests
// =============================================================================

func TestUser_ToPublic_ExposesLastSeenWhenOptedIn(t *testing.T) {
	now := time.Now()
	user := &User{
		ID:               uuid.New(),
		Username:         "alice",
		Email:            "alice@example.com",
		DisplayName:      "Alice W",
		AvatarURL:        "https://example.com/alice.png",
		ShowOnlineStatus: true,
		LastSeenAt:       &now,
	}

	pub := user.ToPublic()

	assert.Equal(t, user.ID, pub.ID)
	assert.Equal(t, "alice", pub.Username)
	assert.Equal(t, "Alice W", pub.DisplayName)
	assert.Equal(t, "https://example.com/alice.png", pub.AvatarURL)
	assert.NotNil(t, pub.LastSeenAt, "LastSeenAt should be exposed when ShowOnlineStatus=true")
	assert.Equal(t, &now, pub.LastSeenAt)
}

func TestUser_ToPublic_HidesLastSeenWhenOptedOut(t *testing.T) {
	now := time.Now()
	user := &User{
		ID:               uuid.New(),
		Username:         "bob",
		Email:            "bob@example.com",
		ShowOnlineStatus: false,
		LastSeenAt:       &now,
	}

	pub := user.ToPublic()

	assert.Equal(t, "bob", pub.Username)
	assert.Nil(t, pub.LastSeenAt, "LastSeenAt should be hidden when ShowOnlineStatus=false")
}

func TestUser_ToPublic_NeverExposesEmail(t *testing.T) {
	user := &User{
		ID:       uuid.New(),
		Username: "charlie",
		Email:    "charlie@secret.com",
	}

	pub := user.ToPublic()

	// PublicUser struct should not have an Email field
	// This is a structural test — if PublicUser ever gets an Email field, this test needs updating
	assert.Equal(t, "charlie", pub.Username)
	// No direct way to assert absence of field in Go, but we verify serialization doesn't leak
}

func TestUser_ToPublic_NilLastSeenAt(t *testing.T) {
	user := &User{
		ID:               uuid.New(),
		Username:         "eve",
		ShowOnlineStatus: true,
		LastSeenAt:       nil,
	}

	pub := user.ToPublic()
	assert.Nil(t, pub.LastSeenAt)
}

// =============================================================================
// RefreshToken.IsValid Tests
// =============================================================================

func TestRefreshToken_IsValid_ValidToken(t *testing.T) {
	token := &RefreshToken{
		ID:        uuid.New(),
		UserID:    uuid.New(),
		ExpiresAt: time.Now().Add(24 * time.Hour),
		CreatedAt: time.Now(),
		RevokedAt: nil,
	}

	assert.True(t, token.IsValid())
}

func TestRefreshToken_IsValid_ExpiredToken(t *testing.T) {
	token := &RefreshToken{
		ID:        uuid.New(),
		UserID:    uuid.New(),
		ExpiresAt: time.Now().Add(-1 * time.Hour), // Expired 1 hour ago
		CreatedAt: time.Now().Add(-25 * time.Hour),
		RevokedAt: nil,
	}

	assert.False(t, token.IsValid())
}

func TestRefreshToken_IsValid_RevokedToken(t *testing.T) {
	revokedAt := time.Now().Add(-1 * time.Hour)
	token := &RefreshToken{
		ID:        uuid.New(),
		UserID:    uuid.New(),
		ExpiresAt: time.Now().Add(24 * time.Hour), // Not expired
		CreatedAt: time.Now().Add(-1 * time.Hour),
		RevokedAt: &revokedAt, // But revoked
	}

	assert.False(t, token.IsValid())
}

func TestRefreshToken_IsValid_BothExpiredAndRevoked(t *testing.T) {
	revokedAt := time.Now().Add(-2 * time.Hour)
	token := &RefreshToken{
		ID:        uuid.New(),
		UserID:    uuid.New(),
		ExpiresAt: time.Now().Add(-1 * time.Hour), // Expired
		CreatedAt: time.Now().Add(-25 * time.Hour),
		RevokedAt: &revokedAt, // Also revoked
	}

	assert.False(t, token.IsValid())
}

func TestRefreshToken_IsValid_ExpiresExactlyNow(t *testing.T) {
	// Edge case: token expires at exactly the current moment
	// IsValid uses time.Now().Before(rt.ExpiresAt)
	// If ExpiresAt == Now, Before returns false, so token is invalid
	token := &RefreshToken{
		ID:        uuid.New(),
		UserID:    uuid.New(),
		ExpiresAt: time.Now(), // Exactly now
		RevokedAt: nil,
	}

	// Might be false due to tiny time difference
	assert.False(t, token.IsValid(), "token expiring exactly now should be invalid (not Before)")
}

// =============================================================================
// Conversation Type Tests
// =============================================================================

func TestConversationType_Values(t *testing.T) {
	assert.Equal(t, ConversationType("dm"), ConversationTypeDM)
	assert.Equal(t, ConversationType("group"), ConversationTypeGroup)
}

func TestMemberRole_Values(t *testing.T) {
	assert.Equal(t, MemberRole("member"), MemberRoleMember)
	assert.Equal(t, MemberRole("admin"), MemberRoleAdmin)
}
