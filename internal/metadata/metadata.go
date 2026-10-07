// Package metadata opens obsrv's embedded SQLite database, which holds
// small, mutable state: alert rules, notification channels, alert states
// and events, and later users. Telemetry itself never goes there.
package metadata

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite" // registers the pure-Go "sqlite" driver
)

// Open opens (or creates) the database at path and applies, in order, the
// migrations it has not applied yet. Each migration runs in a transaction.
// Migrations are append-only: never edit or reorder a released one.
func Open(ctx context.Context, path string, migrations []string) (*sql.DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return nil, fmt.Errorf("metadata: %w", err)
	}
	dsn := "file:" + path + "?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("metadata: open: %w", err)
	}
	db.SetMaxOpenConns(1) // SQLite allows one writer; serialising avoids SQLITE_BUSY
	if err := migrate(ctx, db, migrations); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

func migrate(ctx context.Context, db *sql.DB, migrations []string) error {
	if _, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (version INTEGER PRIMARY KEY)`); err != nil {
		return fmt.Errorf("metadata: %w", err)
	}
	var applied int
	if err := db.QueryRowContext(ctx, `SELECT coalesce(max(version), 0) FROM schema_migrations`).Scan(&applied); err != nil {
		return fmt.Errorf("metadata: %w", err)
	}
	for i := applied; i < len(migrations); i++ {
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("metadata: %w", err)
		}
		if _, err := tx.ExecContext(ctx, migrations[i]); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("metadata: migration %d: %w", i+1, err)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO schema_migrations (version) VALUES (?)`, i+1); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("metadata: migration %d: %w", i+1, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("metadata: migration %d: %w", i+1, err)
		}
	}
	return nil
}
