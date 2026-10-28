package store_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/LaurenteEber/tupaqnin/internal/store"
)

func openTemp(t *testing.T) *store.DB {
	t.Helper()
	db, err := store.Open(context.Background(), t.TempDir())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func TestT8_OpenTwiceDoesNotReapplyMigrations(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()

	first, err := store.Open(ctx, dir)
	if err != nil {
		t.Fatalf("Open (first): %v", err)
	}
	if err := first.Close(); err != nil {
		t.Fatalf("Close (first): %v", err)
	}

	second, err := store.Open(ctx, dir)
	if err != nil {
		t.Fatalf("Open (second): %v", err)
	}
	defer func() { _ = second.Close() }()

	var applied int
	if err := second.Reader.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM schema_version`).Scan(&applied); err != nil {
		t.Fatalf("count schema_version: %v", err)
	}
	if applied != 1 {
		t.Fatalf("schema_version rows: got %d, want 1 (no reapplied migrations)", applied)
	}
}

func TestT8_EachConnectionHasWALAndForeignKeys(t *testing.T) {
	db := openTemp(t)
	ctx := context.Background()

	pools := map[string]*sql.DB{"writer": db.Writer, "reader": db.Reader}
	for name, pool := range pools {
		var journalMode string
		if err := pool.QueryRowContext(ctx, `PRAGMA journal_mode`).Scan(&journalMode); err != nil {
			t.Fatalf("%s: PRAGMA journal_mode: %v", name, err)
		}
		if journalMode != "wal" {
			t.Errorf("%s: journal_mode = %q, want %q", name, journalMode, "wal")
		}
		var foreignKeys int
		if err := pool.QueryRowContext(ctx, `PRAGMA foreign_keys`).Scan(&foreignKeys); err != nil {
			t.Fatalf("%s: PRAGMA foreign_keys: %v", name, err)
		}
		if foreignKeys != 1 {
			t.Errorf("%s: foreign_keys = %d, want 1", name, foreignKeys)
		}
	}
}

func TestT8_ObjectLabelUniquePerProjectAndEventSeqUniquePerProject(t *testing.T) {
	db := openTemp(t)
	ctx := context.Background()

	mustExec := func(query string, args ...any) {
		t.Helper()
		if _, err := db.Writer.ExecContext(ctx, query, args...); err != nil {
			t.Fatalf("exec %q: %v", query, err)
		}
	}
	mustFail := func(query string, args ...any) {
		t.Helper()
		if _, err := db.Writer.ExecContext(ctx, query, args...); err == nil {
			t.Fatalf("exec %q: got no error, want a uniqueness failure", query)
		}
	}

	mustExec(`INSERT INTO project (id, created_at) VALUES ('p1', '2026-10-28T00:00:00.000Z')`)
	mustExec(`INSERT INTO object (project_id, id, label, type, state, current_version)
		VALUES ('p1', 'o1', 'dec-1', 'decision', 'agent', 1)`)
	mustFail(`INSERT INTO object (project_id, id, label, type, state, current_version)
		VALUES ('p1', 'o2', 'dec-1', 'decision', 'agent', 1)`)

	mustExec(`INSERT INTO event (project_id, seq, type, created_at)
		VALUES ('p1', 1, 'human_message', '2026-10-28T00:00:00.000Z')`)
	mustFail(`INSERT INTO event (project_id, seq, type, created_at)
		VALUES ('p1', 1, 'human_message', '2026-10-28T00:00:00.000Z')`)
}
