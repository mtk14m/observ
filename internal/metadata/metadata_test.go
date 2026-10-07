package metadata_test

import (
	"path/filepath"
	"testing"

	"github.com/mtk14n/obsrv/internal/metadata"
)

func TestOpenAppliesMigrationsOnce(t *testing.T) {
	path := filepath.Join(t.TempDir(), "obsrv.db")
	migrations := []string{
		"CREATE TABLE a (id INTEGER PRIMARY KEY)",
		"CREATE TABLE b (id INTEGER PRIMARY KEY)",
	}
	db, err := metadata.Open(t.Context(), path, migrations)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(t.Context(), "INSERT INTO a (id) VALUES (1)"); err != nil {
		t.Fatal(err)
	}
	_ = db.Close()

	// Reopening with one more migration only applies the new one and keeps data.
	db, err = metadata.Open(t.Context(), path, append(migrations, "CREATE TABLE c (id INTEGER PRIMARY KEY)"))
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer func() { _ = db.Close() }()
	var n int
	if err := db.QueryRowContext(t.Context(), "SELECT count(*) FROM a").Scan(&n); err != nil || n != 1 {
		t.Errorf("rows in a = %d, %v; want 1 (data kept)", n, err)
	}
	if _, err := db.ExecContext(t.Context(), "INSERT INTO c (id) VALUES (1)"); err != nil {
		t.Errorf("new migration not applied: %v", err)
	}
}

func TestFailedMigrationIsRolledBack(t *testing.T) {
	path := filepath.Join(t.TempDir(), "obsrv.db")
	if _, err := metadata.Open(t.Context(), path, []string{"CREATE TABLE a (id INTEGER)", "NOT SQL"}); err == nil {
		t.Fatal("Open with a broken migration returned nil error")
	}
	db, err := metadata.Open(t.Context(), path, []string{"CREATE TABLE a (id INTEGER)"})
	if err != nil {
		t.Fatalf("the valid migration must have been kept: %v", err)
	}
	_ = db.Close()
}
