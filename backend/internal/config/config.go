package config

import (
	"fmt"
	"os"
	"strings"
)

// Config holds all application configuration.
// We use a struct (not globals) so it's testable and explicit.
type Config struct {
	// Server
	ServerAddr string
	Env        string // "development" or "production"

	// Database
	DatabaseURL string

	// Auth (will be populated later)
	JWTSigningKey  string
	GitHubClientID string
	GitHubSecret   string

	// URLs
	AppBaseURL string
	APIBaseURL string

	// Static files
	StaticDir string

	// WebRTC / TURN
	ICESTUNURLs  []string
	ICETURNURLs  []string
	TURNUsername string
	TURNPassword string

	// R2 / File Storage
	R2AccountID       string
	R2AccessKeyID     string
	R2SecretAccessKey string
	R2Bucket          string
	R2Endpoint        string
	MaxUploadBytes    int64

	// Redis (for PubSub horizontal scaling)
	RedisURL   string // e.g., "redis://localhost:6379"
	PubSubType string // "memory" or "redis"

	// Google OAuth
	GoogleClientID     string
	GoogleClientSecret string
	GoogleRedirectURL  string // OAuth callback URL
	OAuthEnabled       bool   // Feature flag for OAuth
}

// Load reads configuration from environment variables.
// In production, these come from the host. In dev, from .env via docker-compose.
func Load() (*Config, error) {
	cfg := &Config{
		ServerAddr:  getEnvOrDefault("SERVER_ADDR", "0.0.0.0:8080"),
		Env:         getEnvOrDefault("APP_ENV", "development"),
		DatabaseURL: getEnvOrDefault("DATABASE_URL", "postgres://teatime:teatime@localhost:5432/teatime?sslmode=disable"),
		AppBaseURL:  getEnvOrDefault("APP_BASE_URL", "http://localhost:5173"),
		APIBaseURL:  getEnvOrDefault("API_BASE_URL", "http://localhost:8080"),
	}

	// These are optional in Stage 0, required later
	cfg.JWTSigningKey = os.Getenv("JWT_SIGNING_KEY")
	cfg.GitHubClientID = os.Getenv("GITHUB_CLIENT_ID")
	cfg.GitHubSecret = os.Getenv("GITHUB_CLIENT_SECRET")
	cfg.StaticDir = os.Getenv("STATIC_DIR")

	// WebRTC / TURN configuration
	cfg.ICESTUNURLs = splitEnv("ICE_STUN_URLS", "stun:stun.l.google.com:19302")
	cfg.ICETURNURLs = splitEnv("ICE_TURN_URLS", "")
	cfg.TURNUsername = os.Getenv("TURN_USERNAME")
	cfg.TURNPassword = os.Getenv("TURN_PASSWORD")

	// R2 / File Storage configuration
	cfg.R2AccountID = os.Getenv("R2_ACCOUNT_ID")
	cfg.R2AccessKeyID = os.Getenv("R2_ACCESS_KEY_ID")
	cfg.R2SecretAccessKey = os.Getenv("R2_SECRET_ACCESS_KEY")
	cfg.R2Bucket = os.Getenv("R2_BUCKET")
	cfg.R2Endpoint = getEnvOrDefault("R2_ENDPOINT", fmt.Sprintf("https://%s.r2.cloudflarestorage.com", cfg.R2AccountID))
	cfg.MaxUploadBytes = 100 * 1024 * 1024 // 100MB default

	// Redis / PubSub configuration
	cfg.RedisURL = os.Getenv("REDIS_URL")
	cfg.PubSubType = getEnvOrDefault("PUBSUB_TYPE", "memory") // "memory" or "redis"

	// Google OAuth configuration
	cfg.GoogleClientID = os.Getenv("GOOGLE_CLIENT_ID")
	cfg.GoogleClientSecret = os.Getenv("GOOGLE_CLIENT_SECRET")
	cfg.GoogleRedirectURL = getEnvOrDefault("GOOGLE_REDIRECT_URL", cfg.APIBaseURL+"/auth/google/callback")
	cfg.OAuthEnabled = cfg.GoogleClientID != "" && cfg.GoogleClientSecret != ""

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

func (c *Config) validate() error {
	if c.DatabaseURL == "" {
		return fmt.Errorf("DATABASE_URL is required")
	}
	return nil
}

func (c *Config) IsDevelopment() bool {
	return c.Env == "development"
}

func getEnvOrDefault(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

// splitEnv splits a comma-separated env var into a slice
func splitEnv(key, defaultVal string) []string {
	val := os.Getenv(key)
	if val == "" {
		val = defaultVal
	}
	if val == "" {
		return nil
	}
	parts := strings.Split(val, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}
