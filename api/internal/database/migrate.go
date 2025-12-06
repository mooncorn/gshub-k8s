package database

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// migrationLockID is a unique identifier for the advisory lock
// Using a hash of "gshub_migrations" to avoid collisions
const migrationLockID = 7283945628 // arbitrary unique number

// Migrate runs all pending database migrations from the specified directory
func (db *DB) Migrate(ctx context.Context, migrationsDir string) error {
	// Acquire advisory lock to prevent concurrent migrations from multiple pods
	// This blocks until the lock is available
	_, err := db.Pool.Exec(ctx, "SELECT pg_advisory_lock($1)", migrationLockID)
	if err != nil {
		return fmt.Errorf("failed to acquire migration lock: %w", err)
	}
	defer db.Pool.Exec(ctx, "SELECT pg_advisory_unlock($1)", migrationLockID)

	// Create migrations tracking table if it doesn't exist
	_, err = db.Pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version VARCHAR(255) PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create schema_migrations table: %w", err)
	}

	// Get list of applied migrations
	rows, err := db.Pool.Query(ctx, "SELECT version FROM schema_migrations ORDER BY version")
	if err != nil {
		return fmt.Errorf("failed to query applied migrations: %w", err)
	}
	defer rows.Close()

	applied := make(map[string]bool)
	for rows.Next() {
		var version string
		if err := rows.Scan(&version); err != nil {
			return fmt.Errorf("failed to scan migration version: %w", err)
		}
		applied[version] = true
	}

	// Read migration files from directory
	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		return fmt.Errorf("failed to read migrations directory: %w", err)
	}

	// Collect migration files (only numbered migrations like 00001_xxx.sql)
	var migrations []string
	for _, entry := range entries {
		name := entry.Name()
		if !entry.IsDir() && strings.HasSuffix(name, ".sql") && len(name) >= 5 {
			// Check if filename starts with a number (migration file pattern)
			if name[0] >= '0' && name[0] <= '9' {
				migrations = append(migrations, name)
			}
		}
	}

	// Sort migrations by name (they're numbered, so alphabetical order works)
	sort.Strings(migrations)

	// Apply pending migrations
	appliedCount := 0
	for _, filename := range migrations {
		if applied[filename] {
			continue
		}

		// Read migration file
		content, err := os.ReadFile(filepath.Join(migrationsDir, filename))
		if err != nil {
			return fmt.Errorf("failed to read migration %s: %w", filename, err)
		}

		// Execute migration in a transaction
		tx, err := db.Pool.Begin(ctx)
		if err != nil {
			return fmt.Errorf("failed to begin transaction for %s: %w", filename, err)
		}

		_, err = tx.Exec(ctx, string(content))
		if err != nil {
			tx.Rollback(ctx)
			return fmt.Errorf("failed to execute migration %s: %w", filename, err)
		}

		// Record migration as applied
		_, err = tx.Exec(ctx, "INSERT INTO schema_migrations (version) VALUES ($1)", filename)
		if err != nil {
			tx.Rollback(ctx)
			return fmt.Errorf("failed to record migration %s: %w", filename, err)
		}

		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("failed to commit migration %s: %w", filename, err)
		}

		appliedCount++
		fmt.Printf("Applied migration: %s\n", filename)
	}

	if appliedCount == 0 {
		fmt.Println("No new migrations to apply")
	} else {
		fmt.Printf("Applied %d migration(s)\n", appliedCount)
	}

	return nil
}
