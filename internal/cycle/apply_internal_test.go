package cycle

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// TestS2_TheWholeStepGoesInOneTransaction: the step creates a new
// section and edits sec-1; the person saves sec-1 between the read and
// the write, so the edit hits a stale version (B-11). Nothing of the
// step stays: no new section, no event, and the step is rejected.
func TestS2_TheWholeStepGoesInOneTransaction(t *testing.T) {
	ctx := context.Background()
	db, err := store.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := db.Writer.ExecContext(ctx,
		`INSERT INTO project (id, created_at) VALUES ('p1', '2026-10-29T00:00:00.000Z')`); err != nil {
		t.Fatalf("seed project: %v", err)
	}
	var sec store.Object
	if err := db.Write(ctx, "p1", func(tx *store.Tx) error {
		sec, err = tx.CreateObject(store.ObjectSection, "draft",
			map[string]any{"title": "Síntesis", "text": "Inicio. Cierre."},
			store.EventInput{Type: store.EventStateChanged, Actor: "human"})
		return err
	}); err != nil {
		t.Fatalf("seed section: %v", err)
	}
	output, err := os.ReadFile(filepath.Join("testdata", "steps", "new-section-and-edit.json"))
	if err != nil {
		t.Fatalf("read output: %v", err)
	}

	beforeApplyWrite = func() {
		err := db.Write(ctx, "p1", func(tx *store.Tx) error {
			_, err := tx.UpdateObject(sec.ID, 1, "draft",
				map[string]any{"title": "Síntesis", "text": "Inicio. Cierre. Escrito por la persona."},
				store.EventInput{Type: store.EventEdit, Actor: "human"})
			return err
		})
		if err != nil {
			t.Errorf("human edit: %v", err)
		}
	}
	t.Cleanup(func() { beforeApplyWrite = func() {} })
	before, _ := db.EventsSince(ctx, "p1", 0)

	applied, err := NewGate(db).ApplyStep(ctx, "p1", "turn-1", json.RawMessage(output))
	if err != nil {
		t.Fatalf("ApplyStep: %v", err)
	}
	if applied.Applied || !strings.Contains(strings.Join(applied.Errors, " "), "sec-1 changed since the version you read") {
		t.Fatalf("applied: %+v, want rejected as stale", applied)
	}
	sections, _ := db.ListObjects(ctx, "p1", store.ObjectSection)
	if len(sections) != 1 {
		t.Fatalf("sections: %d, want 1 (the new one rolled back)", len(sections))
	}
	after, _ := db.EventsSince(ctx, "p1", 0)
	if len(after) != len(before)+1 { // only the person's edit
		t.Fatalf("events: %d before, %d after; want only the person's edit added", len(before), len(after))
	}
}
