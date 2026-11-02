package store

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"
)

// E.1: migration 0005 over a database with projects keeps them and their
// rows. They have no owner, so nothing removes them (B-87, B-88).
func TestT8_Migration0005KeepsProjects(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()
	raw, err := sql.Open("sqlite", "file:"+filepath.Join(dir, dbFileName)+"?_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatalf("open raw: %v", err)
	}
	if _, err := raw.ExecContext(ctx, `CREATE TABLE schema_version (version INTEGER PRIMARY KEY)`); err != nil {
		t.Fatalf("schema_version: %v", err)
	}
	for _, m := range []struct {
		version int
		file    string
	}{{1, "0001_init.sql"}, {2, "0002_edge_version_key.sql"}, {3, "0003_project_name.sql"}, {4, "0004_edge_data.sql"}} {
		body, err := migrationsFS.ReadFile("migrations/" + m.file)
		if err != nil {
			t.Fatalf("read %s: %v", m.file, err)
		}
		if err := applyMigration(ctx, raw, m.version, string(body)); err != nil {
			t.Fatalf("apply %s: %v", m.file, err)
		}
	}
	for _, q := range []string{
		`INSERT INTO project (id, name, created_at) VALUES ('p1', 'Tesis', '2026-10-29T00:00:00.000Z')`,
		`INSERT INTO object (project_id, id, label, type, state, current_version) VALUES ('p1', 'o1', 'sec-1', 'section', 'draft', 1)`,
		`INSERT INTO version (project_id, object_id, version, body, created_at) VALUES ('p1', 'o1', 1, '{}', '2026-10-29T00:00:00.000Z')`,
		`INSERT INTO event (project_id, seq, type, created_at) VALUES ('p1', 1, 'state_changed', '2026-10-29T00:00:00.000Z')`,
	} {
		if _, err := raw.ExecContext(ctx, q); err != nil {
			t.Fatalf("seed %q: %v", q, err)
		}
	}
	_ = raw.Close()

	db, err := Open(ctx, dir)
	if err != nil {
		t.Fatalf("Open (applies 0005): %v", err)
	}
	defer func() { _ = db.Close() }()

	var name string
	var account, anon sql.NullString
	if err := db.Reader.QueryRowContext(ctx, `SELECT name, account_id, anon_session_hash FROM project WHERE id = 'p1'`).
		Scan(&name, &account, &anon); err != nil || name != "Tesis" || account.Valid || anon.Valid {
		t.Fatalf("project after 0005: %q, %v, %v, %v", name, account, anon, err)
	}
	for _, table := range []string{"object", "version", "event"} {
		var n int
		if err := db.Reader.QueryRowContext(ctx, `SELECT COUNT(*) FROM "`+table+`"`).Scan(&n); err != nil || n != 1 {
			t.Fatalf("%s after 0005: %d, %v", table, n, err)
		}
	}
	// The indexes the piece names: session's key, the unique username and
	// the project's account.
	for index, onTable := range map[string]string{"account_username": "account", "project_account_id": "project", "sqlite_autoindex_session_1": "session"} {
		var tbl string
		if err := db.Reader.QueryRowContext(ctx, `SELECT tbl_name FROM sqlite_master WHERE type = 'index' AND name = ?`, index).Scan(&tbl); err != nil || tbl != onTable {
			t.Fatalf("index %s: %q, %v", index, tbl, err)
		}
	}
	if projects, _, err := db.PurgeExpired(ctx, time.Now().Add(365*24*time.Hour)); err != nil || projects != 0 {
		t.Fatalf("purge touched a project without an owner: %d, %v", projects, err)
	}
	if exists, err := db.ProjectExists(ctx, "p1"); err != nil || !exists {
		t.Fatalf("p1 after purge: %v, %v", exists, err)
	}
}
