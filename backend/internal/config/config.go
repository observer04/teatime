package config

import (
	"fmt"
	"os"
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
