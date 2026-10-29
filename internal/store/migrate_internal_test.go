package store

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
)

// TestB8_Migration0002KeepsEveryEdgeAndAllowsTheNextVersion starts from
// a database migrated before 0002 (0001 and 0003), with a current and an
// old edge row, then opens it: 0002 keeps both rows and lets the same
// edge id take its next version.
func TestB8_Migration0002KeepsEveryEdgeAndAllowsTheNextVersion(t *testing.T) {
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
	}{{1, "0001_init.sql"}, {3, "0003_project_name.sql"}} {
		body, err := migrationsFS.ReadFile("migrations/" + m.file)
		if err != nil {
			t.Fatalf("read %s: %v", m.file, err)
		}
		if err := applyMigration(ctx, raw, m.version, string(body)); err != nil {
			t.Fatalf("apply %s: %v", m.file, err)
		}
	}
	for _, q := range []string{
		`INSERT INTO project (id, created_at) VALUES ('p1', '2026-10-29T00:00:00.000Z')`,
		`INSERT INTO object (project_id, id, label, type, state, current_version) VALUES ('p1', 'o1', 'dec-1', 'decision', 'agent', 1)`,
		`INSERT INTO object (project_id, id, label, type, state, current_version) VALUES ('p1', 'o2', 'sec-1', 'section', 'draft', 1)`,
		`INSERT INTO version (project_id, object_id, version, body, created_at) VALUES ('p1', 'o1', 1, '{}', '2026-10-29T00:00:00.000Z')`,
		`INSERT INTO version (project_id, object_id, version, body, created_at) VALUES ('p1', 'o2', 1, '{}', '2026-10-29T00:00:00.000Z')`,
		`INSERT INTO edge (project_id, id, type, from_id, to_id, version, anchor, current) VALUES ('p1', 'e1', 'used_in', 'o1', 'o2', 1, NULL, 1)`,
		`INSERT INTO edge (project_id, id, type, from_id, to_id, version, anchor, current) VALUES ('p1', 'e0', 'used_in', 'o1', 'o2', 1, 'x', 0)`,
	} {
		if _, err := raw.ExecContext(ctx, q); err != nil {
			t.Fatalf("seed %q: %v", q, err)
		}
	}
	_ = raw.Close()

	db, err := Open(ctx, dir)
	if err != nil {
		t.Fatalf("Open (applies 0002): %v", err)
	}
	defer func() { _ = db.Close() }()
	var rows int
	if err := db.Reader.QueryRowContext(ctx, `SELECT COUNT(*) FROM edge`).Scan(&rows); err != nil || rows != 2 {
		t.Fatalf("edges after 0002: %d, %v; want 2", rows, err)
	}
	var anchor sql.NullString
	if err := db.Reader.QueryRowContext(ctx, `SELECT anchor FROM edge WHERE id = 'e0'`).Scan(&anchor); err != nil || anchor.String != "x" {
		t.Fatalf("old edge after 0002: %q, %v", anchor.String, err)
	}
	if err := db.Write(ctx, "p1", func(tx *Tx) error {
		_, err := tx.ChangeEdge("e1", "o2", "now", EventInput{Type: EventEdge, Actor: "agent"})
		return err
	}); err != nil {
		t.Fatalf("ChangeEdge after 0002: %v", err)
	}
}
