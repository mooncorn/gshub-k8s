package database

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

var (
	testPool      *pgxpool.Pool
	testContainer *postgres.PostgresContainer
)

// TestMain sets up the test database and runs all tests
func TestMain(m *testing.M) {
	ctx := context.Background()

	// Start PostgreSQL container
	container, connStr, err := setupPostgresContainer(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to start PostgreSQL container: %v\n", err)
		os.Exit(1)
	}
	testContainer = container

	// Create connection pool
	pool, err := pgxpool.New(ctx, connStr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create connection pool: %v\n", err)
		testContainer.Terminate(ctx)
		os.Exit(1)
	}
	testPool = pool

	// Run migrations
	if err := runMigrations(ctx, pool); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to run migrations: %v\n", err)
		pool.Close()
		testContainer.Terminate(ctx)
		os.Exit(1)
	}

	// Run tests
	code := m.Run()

	// Cleanup
	pool.Close()
	if err := testContainer.Terminate(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to terminate container: %v\n", err)
	}

	os.Exit(code)
}

// setupPostgresContainer starts a PostgreSQL container for testing
func setupPostgresContainer(ctx context.Context) (*postgres.PostgresContainer, string, error) {
	container, err := postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("testdb"),
		postgres.WithUsername("testuser"),
		postgres.WithPassword("testpass"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second),
		),
	)
	if err != nil {
		return nil, "", fmt.Errorf("failed to start container: %w", err)
	}

	connStr, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		return nil, "", fmt.Errorf("failed to get connection string: %w", err)
	}

	return container, connStr, nil
}

// runMigrations executes all migration SQL files in order
func runMigrations(ctx context.Context, pool *pgxpool.Pool) error {
	// Get path to migrations directory - go up two directories from database to api, then to migrations
	migrationsDir := filepath.Join("..", "..", "migrations")

	// Read all .sql files from migrations directory
	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		return fmt.Errorf("failed to read migrations directory: %w", err)
	}

	// Collect and sort migration files (they're already sorted by name due to numbering)
	// Only include files that match the migration naming pattern (00001_xxx.sql)
	var migrationFiles []string
	for _, entry := range entries {
		name := entry.Name()
		if !entry.IsDir() && filepath.Ext(name) == ".sql" && len(name) >= 5 && name[0:5] >= "00001" && name[0:5] <= "99999" {
			migrationFiles = append(migrationFiles, name)
		}
	}

	if len(migrationFiles) == 0 {
		return fmt.Errorf("no migration files found in %s", migrationsDir)
	}

	// Execute each migration in order
	for _, filename := range migrationFiles {
		migrationPath := filepath.Join(migrationsDir, filename)
		sqlBytes, err := os.ReadFile(migrationPath)
		if err != nil {
			return fmt.Errorf("failed to read migration file %s: %w", filename, err)
		}

		// Execute migration
		_, err = pool.Exec(ctx, string(sqlBytes))
		if err != nil {
			return fmt.Errorf("failed to execute migration %s: %w", filename, err)
		}
	}

	return nil
}

// setupTest creates a new transaction for test isolation
// Returns a DB instance and a cleanup function that rolls back the transaction
func setupTest(t *testing.T) (*DB, func()) {
	t.Helper()

	ctx := context.Background()
	tx, err := testPool.Begin(ctx)
	require.NoError(t, err, "failed to begin transaction")

	cleanup := func() {
		tx.Rollback(ctx)
	}

	return &DB{Pool: tx}, cleanup
}

// Helper functions for generating test data

var rng = rand.New(rand.NewSource(time.Now().UnixNano()))

// RandomString generates a random alphanumeric string of given length
func RandomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[rng.Intn(len(charset))]
	}
	return string(b)
}

// RandomEmail generates a random email address
func RandomEmail() string {
	return fmt.Sprintf("%s@test.com", RandomString(10))
}

// RandomServerID generates a random server ID
func RandomServerID() string {
	return fmt.Sprintf("srv-%s", RandomString(8))
}

// RandomSubdomain generates a random subdomain
func RandomSubdomain() string {
	return RandomString(12)
}
