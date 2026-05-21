package migrate

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestMigrateEmptyDir(t *testing.T) {
	tmpDir := t.TempDir()

	pool, err := createTestPool()
	if err != nil {
		t.Skipf("database not available: %v", err)
	}
	defer pool.Close()

	ctx := context.Background()

	err = RunMigrations(ctx, pool, tmpDir)
	if err != nil {
		t.Fatalf("expected no error for empty dir, got %v", err)
	}
}

func TestMigrateIdempotent(t *testing.T) {
	tmpDir := t.TempDir()

	err := os.WriteFile(filepath.Join(tmpDir, "001_test.sql"), []byte(`
		CREATE TABLE IF NOT EXISTS test_table (id SERIAL PRIMARY KEY);
	`), 0644)
	if err != nil {
		t.Fatalf("failed to write migration: %v", err)
	}

	pool, err := createTestPool()
	if err != nil {
		t.Skipf("database not available: %v", err)
	}
	defer pool.Close()

	ctx := context.Background()

	err = RunMigrations(ctx, pool, tmpDir)
	if err != nil {
		t.Fatalf("first migration run failed: %v", err)
	}

	err = RunMigrations(ctx, pool, tmpDir)
	if err != nil {
		t.Fatalf("second migration run (idempotent) failed: %v", err)
	}

	var count int
	err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM schema_migrations").Scan(&count)
	if err != nil {
		t.Fatalf("query schema_migrations: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 migration recorded, got %d", count)
	}
}

func createTestPool() (*pgxpool.Pool, error) {
	ctx := context.Background()
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://agentid:agentid_dev@localhost:5432/agentid"
	}
	config, err := pgxpool.ParseConfig(dbURL)
	if err != nil {
		return nil, err
	}
	config.MinConns = 1
	config.MaxConns = 2
	return pgxpool.NewWithConfig(ctx, config)
}