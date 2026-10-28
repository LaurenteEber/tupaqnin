package graph_test

import (
	"context"
	"testing"

	"github.com/LaurenteEber/tupaqnin/internal/graph"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

func TestT2_LabelerTranslatesBothWaysAndRejectsUnknown(t *testing.T) {
	db, err := store.Open(context.Background(), t.TempDir())
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	ctx := context.Background()
	if _, err := db.Writer.ExecContext(ctx,
		`INSERT INTO project (id, created_at) VALUES ('p1', '2026-10-28T00:00:00.000Z')`); err != nil {
		t.Fatalf("seed project: %v", err)
	}
	var obj store.Object
	err = db.Write(ctx, "p1", func(tx *store.Tx) error {
		var err error
		obj, err = tx.CreateObject(store.ObjectDecision, "agent",
			map[string]any{"statement": "use X"},
			store.EventInput{Type: store.EventStateChanged, Actor: "human"})
		return err
	})
	if err != nil {
		t.Fatalf("CreateObject: %v", err)
	}

	lab, err := graph.NewLabeler(ctx, db, "p1")
	if err != nil {
		t.Fatalf("NewLabeler: %v", err)
	}
	label, err := lab.ToLabel(obj.ID)
	if err != nil || label != "dec-1" {
		t.Fatalf("ToLabel: got %q, %v; want dec-1", label, err)
	}
	id, err := lab.ToID("dec-1")
	if err != nil || id != obj.ID {
		t.Fatalf("ToID: got %q, %v; want the internal id", id, err)
	}
	if _, err := lab.ToLabel("no-such-id"); err == nil {
		t.Fatal("ToLabel of an unknown id: want an error")
	}
	if _, err := lab.ToID("dec-99"); err == nil {
		t.Fatal("ToID of an unknown label: want an error")
	}
	// Isolation: another project's labeler does not know this object.
	other, err := graph.NewLabeler(ctx, db, "p2")
	if err != nil {
		t.Fatalf("NewLabeler p2: %v", err)
	}
	if _, err := other.ToID("dec-1"); err == nil {
		t.Fatal("ToID in another project: want an error")
	}
}
