package migrate

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

func RunMigrations(ctx context.Context, pool *pgxpool.Pool, migrationsDir string) error {
	if _, err := pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version VARCHAR(255) PRIMARY KEY,
			applied_at TIMESTAMPTZ DEFAULT NOW()
		)
	`); err != nil {
		return fmt.Errorf("create schema_migrations table: %w", err)
	}

	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		return fmt.Errorf("read migrations dir %s: %w", migrationsDir, err)
	}

	var sqlFiles []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".sql") {
			sqlFiles = append(sqlFiles, entry.Name())
		}
	}
	sort.Strings(sqlFiles)

	if len(sqlFiles) == 0 {
		slog.Warn("no migration files found", "dir", migrationsDir)
		return nil
	}

	applied := 0
	skipped := 0

	for _, name := range sqlFiles {
		var exists bool
		err := pool.QueryRow(ctx,
			"SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version = $1)", name,
		).Scan(&exists)
		if err != nil {
			return fmt.Errorf("check migration %s: %w", name, err)
		}

		if exists {
			skipped++
			continue
		}

		content, err := os.ReadFile(fmt.Sprintf("%s/%s", migrationsDir, name))
		if err != nil {
			return fmt.Errorf("read migration %s: %w", name, err)
		}

		slog.Info("applying migration", "migration", name)

		if _, err := pool.Exec(ctx, string(content)); err != nil {
			return fmt.Errorf("apply migration %s: %w", name, err)
		}

		if _, err := pool.Exec(ctx,
			"INSERT INTO schema_migrations (version) VALUES ($1)", name,
		); err != nil {
			return fmt.Errorf("record migration %s: %w", name, err)
		}

		applied++
		slog.Info("migration applied", "migration", name)
	}

	slog.Info("migrations complete", "applied", applied, "skipped", skipped)
	return nil
}