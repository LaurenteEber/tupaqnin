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

// p89Seed opens a project with sec-1 («Inicio. Cierre.») and reads the
// step that creates a new section and edits sec-1.
func p89Seed(t *testing.T) (context.Context, *store.DB, store.Object, json.RawMessage) {
	t.Helper()
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
	t.Cleanup(func() { beforeApplyWrite = func() {} })
	return ctx, db, sec, output
}

// P-89: the person changes sec-1's text between the step's validation
// and its write; the step is rejected as stale (B-11) and nothing of it
// is written: the text stays the person's and no new section is born.
func TestP89_ATextChangeBetweenValidationAndWriteRejectsTheStep(t *testing.T) {
	ctx, db, sec, output := p89Seed(t)
	beforeApplyWrite = func() {
		err := db.Write(ctx, "p1", func(tx *store.Tx) error {
			_, err := tx.UpdateObject(sec.ID, 1, "draft",
				map[string]any{"title": "Síntesis", "text": "Inicio. Otro cierre."},
				store.EventInput{Type: store.EventEdit, Actor: "human"})
			return err
		})
		if err != nil {
			t.Errorf("human edit: %v", err)
		}
	}
	applied, err := NewGate(db).ApplyStep(ctx, "p1", "turn-1", output)
	if err != nil {
		t.Fatalf("ApplyStep: %v", err)
	}
	if applied.Applied || !strings.Contains(strings.Join(applied.Errors, " "), "sec-1 changed since the version you read") {
		t.Fatalf("applied: %+v, want the step rejected as stale", applied)
	}
	obj, _ := db.GetObject(ctx, "p1", sec.ID)
	var b map[string]any
	_ = json.Unmarshal(obj.Body, &b)
	if b["text"] != "Inicio. Otro cierre." || obj.CurrentVersion != 2 {
		t.Fatalf("sec-1 v%d %v, want the person's text untouched", obj.CurrentVersion, b)
	}
	if sections, _ := db.ListObjects(ctx, "p1", store.ObjectSection); len(sections) != 1 {
		t.Fatalf("sections: %d, want 1 (the new one rolled back)", len(sections))
	}
}

// P-89 (B-14): a signoff of sec-1 starts between the step's validation
// and its write. The write re-checks it in its transaction: the step is
// rejected with the signing message and sec-1's text is not touched.
func TestP89_ASignoffStartedBetweenValidationAndWriteRejectsTheStep(t *testing.T) {
	ctx, db, sec, output := p89Seed(t)
	beforeApplyWrite = func() {
		open, err := NewGate(db).RequestSignoff(ctx, "p1", sec.Label)
		if err != nil || !open.Opened {
			t.Errorf("RequestSignoff: %+v %v, want opened", open, err)
		}
	}
	applied, err := NewGate(db).ApplyStep(ctx, "p1", "turn-1", output)
	if err != nil {
		t.Fatalf("ApplyStep: %v", err)
	}
	if applied.Applied || !strings.Contains(strings.Join(applied.Errors, " "), "sec-1 started being signed by the person") {
		t.Fatalf("applied: %+v, want the step rejected: sec-1 is being signed", applied)
	}
	obj, _ := db.GetObject(ctx, "p1", sec.ID)
	var b map[string]any
	_ = json.Unmarshal(obj.Body, &b)
	if b["text"] != "Inicio. Cierre." {
		t.Fatalf("sec-1 text %q, want it untouched while being signed", b["text"])
	}
	if sections, _ := db.ListObjects(ctx, "p1", store.ObjectSection); len(sections) != 1 {
		t.Fatalf("sections: %d, want 1 (the new one rolled back)", len(sections))
	}
}
