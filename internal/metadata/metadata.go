// Package metadata opens obsrv's embedded SQLite database, which holds
// small, mutable state: alert rules and events, users and sessions.
// Telemetry itself never goes there.
package metadata

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite" // registers the pure-Go "sqlite" driver
)

// Open opens (or creates) the database at path.
func Open(ctx context.Context, path string) (*sql.DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return nil, fmt.Errorf("metadata: %w", err)
	}
	dsn := "file:" + path + "?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("metadata: open: %w", err)
	}
	db.SetMaxOpenConns(1) // SQLite allows one writer; serialising avoids SQLITE_BUSY
	if err := initMigrations(ctx, db); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

func initMigrations(ctx context.Context, db *sql.DB) error {
	if _, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS migrations (
		component TEXT NOT NULL, version INTEGER NOT NULL, PRIMARY KEY (component, version))`); err != nil {
		return fmt.Errorf("metadata: %w", err)
	}
	// Before migrations were per component, only alerting used the database
	// and recorded its versions in schema_migrations.
	var legacy int
	if err := db.QueryRowContext(ctx,
		`SELECT count(*) FROM sqlite_master WHERE type = 'table' AND name = 'schema_migrations'`).Scan(&legacy); err != nil {
		return fmt.Errorf("metadata: %w", err)
	}
	if legacy == 0 {
		return nil
	}
	if _, err := db.ExecContext(ctx, `INSERT OR IGNORE INTO migrations (component, version)
		SELECT 'alert', version FROM schema_migrations`); err != nil {
		return fmt.Errorf("metadata: upgrade legacy migrations: %w", err)
	}
	if _, err := db.ExecContext(ctx, `DROP TABLE schema_migrations`); err != nil {
		return fmt.Errorf("metadata: upgrade legacy migrations: %w", err)
	}
	return nil
}

// Migrate applies, in order, the migrations of a component that have not
// been applied yet. Each runs in a transaction. Migrations are append-only:
// never edit or reorder a released one.
func Migrate(ctx context.Context, db *sql.DB, component string, migrations []string) error {
	var applied int
	if err := db.QueryRowContext(ctx, `SELECT coalesce(max(version), 0) FROM migrations WHERE component = ?`,
		component).Scan(&applied); err != nil {
		return fmt.Errorf("metadata: %w", err)
	}
	for i := applied; i < len(migrations); i++ {
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("metadata: %w", err)
		}
		if _, err := tx.ExecContext(ctx, migrations[i]); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("metadata: %s migration %d: %w", component, i+1, err)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO migrations (component, version) VALUES (?, ?)`, component, i+1); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("metadata: %s migration %d: %w", component, i+1, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("metadata: %s migration %d: %w", component, i+1, err)
		}
	}
	return nil
}
