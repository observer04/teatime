package database

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/observer/teatime/internal/domain"
)

// UserRepository handles user data access
type UserRepository struct {
	db *DB
}

func NewUserRepository(db *DB) *UserRepository {
	return &UserRepository{db: db}
}

// Create creates a new user with credentials
func (r *UserRepository) Create(ctx context.Context, user *domain.User, passwordHash string) error {
	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Insert user
	_, err = tx.Exec(ctx, `
		INSERT INTO users (id, username, email, display_name, avatar_url)
		VALUES ($1, $2, $3, $4, $5)
	`, user.ID, user.Username, user.Email, user.DisplayName, user.AvatarURL)
	if err != nil {
		return err
	}

	// Insert credentials
	_, err = tx.Exec(ctx, `
		INSERT INTO credentials (user_id, password_hash)
		VALUES ($1, $2)
	`, user.ID, passwordHash)
	if err != nil {
		return err
	}

	return tx.Commit(ctx)
}

// GetByID finds a user by ID
func (r *UserRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	user := &domain.User{}
	err := r.db.Pool.QueryRow(ctx, `
		SELECT id, username, email, display_name, avatar_url, created_at, updated_at
		FROM users WHERE id = $1
	`, id).Scan(
		&user.ID, &user.Username, &user.Email,
		&user.DisplayName, &user.AvatarURL,
		&user.CreatedAt, &user.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrUserNotFound
	}
	return user, err
}

// GetByEmail finds a user by email
func (r *UserRepository) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	user := &domain.User{}
	err := r.db.Pool.QueryRow(ctx, `
		SELECT id, username, email, display_name, avatar_url, created_at, updated_at
		FROM users WHERE email = $1
	`, email).Scan(
		&user.ID, &user.Username, &user.Email,
		&user.DisplayName, &user.AvatarURL,
		&user.CreatedAt, &user.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrUserNotFound
	}
	return user, err
}

// GetByUsername finds a user by username
func (r *UserRepository) GetByUsername(ctx context.Context, username string) (*domain.User, error) {
	user := &domain.User{}
	err := r.db.Pool.QueryRow(ctx, `
		SELECT id, username, email, display_name, avatar_url, created_at, updated_at
		FROM users WHERE username = $1
	`, username).Scan(
		&user.ID, &user.Username, &user.Email,
		&user.DisplayName, &user.AvatarURL,
		&user.CreatedAt, &user.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrUserNotFound
	}
	return user, err
}

// GetPasswordHash retrieves the password hash for a user
func (r *UserRepository) GetPasswordHash(ctx context.Context, userID uuid.UUID) (string, error) {
	var hash string
	err := r.db.Pool.QueryRow(ctx, `
		SELECT password_hash FROM credentials WHERE user_id = $1
	`, userID).Scan(&hash)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", domain.ErrUserNotFound
	}
	return hash, err
}

// EmailExists checks if email is already registered
func (r *UserRepository) EmailExists(ctx context.Context, email string) (bool, error) {
	var exists bool
	err := r.db.Pool.QueryRow(ctx, `
		SELECT EXISTS(SELECT 1 FROM users WHERE email = $1)
	`, email).Scan(&exists)
	return exists, err
}

// UsernameExists checks if username is taken
func (r *UserRepository) UsernameExists(ctx context.Context, username string) (bool, error) {
	var exists bool
	err := r.db.Pool.QueryRow(ctx, `
		SELECT EXISTS(SELECT 1 FROM users WHERE username = $1)
	`, username).Scan(&exists)
	return exists, err
}

// SearchByUsername searches users by username prefix
func (r *UserRepository) SearchByUsername(ctx context.Context, query string, limit int) ([]domain.User, error) {
	rows, err := r.db.Pool.Query(ctx, `
		SELECT id, username, email, display_name, avatar_url, created_at, updated_at
		FROM users 
		WHERE username ILIKE $1 || '%'
		ORDER BY username
		LIMIT $2
	`, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []domain.User
	for rows.Next() {
		var u domain.User
		err := rows.Scan(
			&u.ID, &u.Username, &u.Email,
			&u.DisplayName, &u.AvatarURL,
			&u.CreatedAt, &u.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

// Update updates user profile fields
func (r *UserRepository) Update(ctx context.Context, user *domain.User) error {
	_, err := r.db.Pool.Exec(ctx, `
		UPDATE users 
		SET display_name = $2, avatar_url = $3, updated_at = NOW()
		WHERE id = $1
	`, user.ID, user.DisplayName, user.AvatarURL)
	return err
}

// ============================================================================
// Refresh Token Operations
// ============================================================================

// hashToken creates a SHA-256 hash of a token
func hashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

// CreateRefreshToken stores a new refresh token (hashed)
func (r *UserRepository) CreateRefreshToken(ctx context.Context, userID uuid.UUID, token string, expiresAt time.Time) (uuid.UUID, error) {
	id := uuid.New()
	tokenHash := hashToken(token)

	_, err := r.db.Pool.Exec(ctx, `
		INSERT INTO refresh_tokens (id, user_id, token_hash, expires_at)
		VALUES ($1, $2, $3, $4)
	`, id, userID, tokenHash, expiresAt)

	return id, err
}

// GetRefreshToken retrieves a refresh token by its raw value
func (r *UserRepository) GetRefreshToken(ctx context.Context, token string) (*domain.RefreshToken, error) {
	tokenHash := hashToken(token)
	rt := &domain.RefreshToken{}

	err := r.db.Pool.QueryRow(ctx, `
		SELECT id, user_id, token_hash, expires_at, created_at, revoked_at
		FROM refresh_tokens WHERE token_hash = $1
	`, tokenHash).Scan(
		&rt.ID, &rt.UserID, &rt.TokenHash,
		&rt.ExpiresAt, &rt.CreatedAt, &rt.RevokedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrTokenInvalid
	}
	return rt, err
}

// RevokeRefreshToken marks a refresh token as revoked
func (r *UserRepository) RevokeRefreshToken(ctx context.Context, tokenID uuid.UUID) error {
	_, err := r.db.Pool.Exec(ctx, `
		UPDATE refresh_tokens SET revoked_at = NOW() WHERE id = $1
	`, tokenID)
	return err
}

// RevokeAllUserTokens revokes all refresh tokens for a user (logout everywhere)
func (r *UserRepository) RevokeAllUserTokens(ctx context.Context, userID uuid.UUID) error {
	_, err := r.db.Pool.Exec(ctx, `
		UPDATE refresh_tokens SET revoked_at = NOW() 
		WHERE user_id = $1 AND revoked_at IS NULL
	`, userID)
	return err
}

// ============================================================================
// OAuth Identity Operations
// ============================================================================

// GetUserByOAuthProvider finds a user by their OAuth provider and provider user ID
func (r *UserRepository) GetUserByOAuthProvider(ctx context.Context, provider, providerUserID string) (*domain.User, error) {
	user := &domain.User{}
	err := r.db.Pool.QueryRow(ctx, `
		SELECT u.id, u.username, u.email, u.display_name, u.avatar_url, u.created_at, u.updated_at
		FROM users u
		INNER JOIN oauth_identities oi ON u.id = oi.user_id
		WHERE oi.provider = $1 AND oi.provider_user_id = $2
	`, provider, providerUserID).Scan(
		&user.ID, &user.Username, &user.Email,
		&user.DisplayName, &user.AvatarURL,
		&user.CreatedAt, &user.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrUserNotFound
	}
	return user, err
}

// CreateOAuthIdentity links an OAuth provider to an existing user
func (r *UserRepository) CreateOAuthIdentity(ctx context.Context, userID uuid.UUID, provider, providerUserID string) error {
	_, err := r.db.Pool.Exec(ctx, `
		INSERT INTO oauth_identities (user_id, provider, provider_user_id)
		VALUES ($1, $2, $3)
		ON CONFLICT (provider, provider_user_id) DO NOTHING
	`, userID, provider, providerUserID)
	return err
}

// CreateUserWithOAuth creates a new user via OAuth (without password)
// The user will need to set a username later
func (r *UserRepository) CreateUserWithOAuth(ctx context.Context, user *domain.User, provider, providerUserID string) error {
	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Insert user
	_, err = tx.Exec(ctx, `
		INSERT INTO users (id, username, email, display_name, avatar_url)
		VALUES ($1, $2, $3, $4, $5)
	`, user.ID, user.Username, user.Email, user.DisplayName, user.AvatarURL)
	if err != nil {
		return err
	}

	// Insert OAuth identity
	_, err = tx.Exec(ctx, `
		INSERT INTO oauth_identities (user_id, provider, provider_user_id)
		VALUES ($1, $2, $3)
	`, user.ID, provider, providerUserID)
	if err != nil {
		return err
	}

	return tx.Commit(ctx)
}

// HasOAuthIdentity checks if a user has an OAuth identity linked
func (r *UserRepository) HasOAuthIdentity(ctx context.Context, userID uuid.UUID, provider string) (bool, error) {
	var exists bool
	err := r.db.Pool.QueryRow(ctx, `
		SELECT EXISTS(SELECT 1 FROM oauth_identities WHERE user_id = $1 AND provider = $2)
	`, userID, provider).Scan(&exists)
	return exists, err
}

// UpdateUsername updates just the username for a user (for OAuth users setting their username)
func (r *UserRepository) UpdateUsername(ctx context.Context, userID uuid.UUID, username string) error {
	_, err := r.db.Pool.Exec(ctx, `
		UPDATE users SET username = $2, updated_at = NOW() WHERE id = $1
	`, userID, username)
	return err
}

