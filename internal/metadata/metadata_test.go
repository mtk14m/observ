package metadata_test

import (
	"database/sql"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/mtk14n/obsrv/internal/metadata"
)

func TestMigrationsArePerComponent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "obsrv.db")
	db, err := metadata.Open(t.Context(), path)
	if err != nil {
		t.Fatal(err)
	}
	if err := metadata.Migrate(t.Context(), db, "alert", []string{"CREATE TABLE a (id INTEGER)"}); err != nil {
		t.Fatal(err)
	}
	if err := metadata.Migrate(t.Context(), db, "auth", []string{"CREATE TABLE u (id INTEGER)"}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(t.Context(), "INSERT INTO a (id) VALUES (1)"); err != nil {
		t.Fatal(err)
	}
	_ = db.Close()

	// One more alert migration: only it runs, and auth is untouched.
	db, err = metadata.Open(t.Context(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	if err := metadata.Migrate(t.Context(), db, "alert", []string{"CREATE TABLE a (id INTEGER)", "CREATE TABLE b (id INTEGER)"}); err != nil {
		t.Fatalf("second alert migration: %v", err)
	}
	if err := metadata.Migrate(t.Context(), db, "auth", []string{"CREATE TABLE u (id INTEGER)"}); err != nil {
		t.Fatalf("auth already applied: %v", err)
	}
	var n int
	if err := db.QueryRowContext(t.Context(), "SELECT count(*) FROM a").Scan(&n); err != nil || n != 1 {
		t.Errorf("rows in a = %d, %v; want 1", n, err)
	}
}

func TestFailedMigrationIsRolledBack(t *testing.T) {
	path := filepath.Join(t.TempDir(), "obsrv.db")
	db, _ := metadata.Open(t.Context(), path)
	defer func() { _ = db.Close() }()
	if err := metadata.Migrate(t.Context(), db, "x", []string{"CREATE TABLE a (id INTEGER)", "NOT SQL"}); err == nil {
		t.Fatal("broken migration returned nil error")
	}
	if err := metadata.Migrate(t.Context(), db, "x", []string{"CREATE TABLE a (id INTEGER)"}); err != nil {
		t.Fatalf("the valid migration must have been kept: %v", err)
	}
}

// Databases created before migrations were per component recorded the
// alerting migrations in schema_migrations(version).
func TestUpgradesTheLegacyMigrationTable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "obsrv.db")
	legacy, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatal(err)
	}
	for _, stmt := range []string{
		"CREATE TABLE schema_migrations (version INTEGER PRIMARY KEY)",
		"INSERT INTO schema_migrations (version) VALUES (1)",
		"CREATE TABLE a (id INTEGER)",
	} {
		if _, err := legacy.ExecContext(t.Context(), stmt); err != nil {
			t.Fatal(err)
		}
	}
	_ = legacy.Close()

	db, err := metadata.Open(t.Context(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	// Migration 1 of "alert" was already applied: it must not run again.
	if err := metadata.Migrate(t.Context(), db, "alert", []string{"CREATE TABLE a (id INTEGER)"}); err != nil {
		t.Errorf("legacy alert migration re-ran: %v", err)
	}
}
