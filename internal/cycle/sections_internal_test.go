package cycle

import (
	"context"
	"testing"

	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// K-17: without the «aquí se concluye» mark no role is set, and the
// next step can propose it (K-2); with the mark it stays confirmed.
func TestK17_UnmarkedSectionGetsTheRoleTheNextStepProposes(t *testing.T) {
	ctx := context.Background()
	db, err := store.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := db.Writer.ExecContext(ctx,
		`INSERT INTO project (id, created_at) VALUES ('p1', '2026-10-30T00:00:00.000Z')`); err != nil {
		t.Fatalf("seed project: %v", err)
	}
	unmarked, err := CreateSection(ctx, db, "p1", "Recomendaciones", "", "")
	if err != nil {
		t.Fatalf("CreateSection: %v", err)
	}
	marked, err := CreateSection(ctx, db, "p1", "Conclusión", "conclusion", "")
	if err != nil {
		t.Fatalf("CreateSection: %v", err)
	}
	var applied [2]bool
	if err := db.Write(ctx, "p1", func(tx *store.Tx) error {
		var err error
		if applied[0], err = proposeRole(tx, unmarked.Label, unmarked.ID, "recommendation", "turn-1"); err != nil {
			return err
		}
		applied[1], err = proposeRole(tx, marked.Label, marked.ID, "other", "turn-1")
		return err
	}); err != nil {
		t.Fatalf("proposeRole: %v", err)
	}
	if !applied[0] || applied[1] {
		t.Fatalf("applied: got %v, want [true false]", applied)
	}
	obj, _ := db.GetObject(ctx, "p1", unmarked.Label)
	role, _ := bodyOf(obj)["role"].(map[string]any)
	if bodyStr(role, "value") != "recommendation" || bodyStr(role, "state") != "proposed" {
		t.Fatalf("unmarked role: got %v, want recommendation proposed", role)
	}
}
