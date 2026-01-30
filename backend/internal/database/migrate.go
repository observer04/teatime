package database

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// EnsureSchema checks for core tables and applies the initial migration if needed.
func EnsureSchema(ctx context.Context, db *DB, migrationsDir string) error {
	var regclass *string
	err := db.Pool.QueryRow(ctx, "SELECT to_regclass('public.users')").Scan(&regclass)
	if err != nil {
		return fmt.Errorf("check users table: %w", err)
	}
	if regclass != nil {
		return nil
	}

	path := filepath.Join(migrationsDir, "000001_init_schema.up.sql")
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read migration file: %w", err)
	}

	sql := strings.TrimSpace(string(content))
	if sql == "" {
		return fmt.Errorf("migration file is empty: %s", path)
	}

	if _, err := db.Pool.Exec(ctx, sql); err != nil {
		return fmt.Errorf("apply initial schema: %w", err)
	}

	return nil
}
