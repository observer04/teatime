package database

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// EnsureSchema applies all pending migrations in the migrations directory.
// It creates a schema_migrations table to track applied versions.
func EnsureSchema(ctx context.Context, db *DB, migrationsDir string) error {
	// 1. Create migrations table if not exists
	_, err := db.Pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version BIGINT PRIMARY KEY,
			applied_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
		);
	`)
	if err != nil {
		return fmt.Errorf("create schema_migrations table: %w", err)
	}

	// 2. Check if users table exists (for legacy compatibility)
	// If users table exists but schema_migrations doesn't (or is empty), 
	// we assume the initial migration (000001) is already applied.
	var userTableExists bool
	err = db.Pool.QueryRow(ctx, "SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_schema = 'public' AND table_name = 'users')").Scan(&userTableExists)
	if err != nil {
		return fmt.Errorf("check user table existence: %w", err)
	}

	// 3. Read and sort migration files
	slog.Info("reading migrations directory", "dir", migrationsDir)
	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		return fmt.Errorf("read migrations directory: %w", err)
	}

	var files []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".up.sql") {
			files = append(files, entry.Name())
		}
	}
	sort.Strings(files)
	slog.Info("found migration files", "count", len(files), "files", files)

	// 4. Apply migrations
	for _, file := range files {
		// Extract version (e.g., "000001" from "000001_init_schema.up.sql")
		parts := strings.Split(file, "_")
		if len(parts) == 0 {
			continue
		}
		
		version, err := strconv.ParseInt(parts[0], 10, 64)
		if err != nil {
			slog.Warn("skipping migration file with invalid version format", "file", file)
			continue
		}

		// Check if already applied
		var applied bool
		err = db.Pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version = $1)", version).Scan(&applied)
		if err != nil {
			return fmt.Errorf("check migration version %d: %w", version, err)
		}

		slog.Info("checking migration", "version", version, "applied", applied, "file", file)

		if applied {
			continue
		}

		// Legacy check: If version is 1 and users table exists, mark as applied without running
		if version == 1 && userTableExists {
			slog.Info("marking initial migration as applied (legacy)", "version", version)
			if _, err := db.Pool.Exec(ctx, "INSERT INTO schema_migrations (version) VALUES ($1)", version); err != nil {
				return fmt.Errorf("mark legacy migration %d: %w", version, err)
			}
			continue
		}

		// Run migration
		slog.Info("applying migration", "file", file, "version", version)
		path := filepath.Join(migrationsDir, file)
		content, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read migration file %s: %w", file, err)
		}

		tx, err := db.Pool.Begin(ctx)
		if err != nil {
			return fmt.Errorf("begin transaction: %w", err)
		}

		if _, err := tx.Exec(ctx, string(content)); err != nil {
			if rbErr := tx.Rollback(ctx); rbErr != nil {
				slog.Error("rollback failed", "error", rbErr)
			}
			return fmt.Errorf("execute migration %s: %w", file, err)
		}

		if _, err := tx.Exec(ctx, "INSERT INTO schema_migrations (version) VALUES ($1)", version); err != nil {
			if rbErr := tx.Rollback(ctx); rbErr != nil {
				slog.Error("rollback failed", "error", rbErr)
			}
			return fmt.Errorf("record migration %s: %w", file, err)
		}

		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit migration %s: %w", file, err)
		}
		slog.Info("migration applied successfully", "version", version)
	}

	return nil
}
