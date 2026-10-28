package store_test

import (
	"context"
	"testing"
)

func TestT8_ReaderConnectionCannotWrite(t *testing.T) {
	db := openTemp(t)
	ctx := context.Background()

	if _, err := db.Reader.ExecContext(ctx,
		`INSERT INTO project (id, created_at) VALUES ('p-read', '2026-10-28T00:00:00.000Z')`); err == nil {
		t.Fatal("reader INSERT: got no error, want a read-only failure")
	}

	// Reading through the same pool still works.
	var count int
	if err := db.Reader.QueryRowContext(ctx, `SELECT COUNT(*) FROM project`).Scan(&count); err != nil {
		t.Fatalf("reader SELECT: %v", err)
	}
}
