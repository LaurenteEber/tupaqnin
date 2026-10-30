package memory_test

import (
	"context"
	"strings"
	"testing"

	"github.com/LaurenteEber/tupaqnin/internal/checks"
	"github.com/LaurenteEber/tupaqnin/internal/graph"
	"github.com/LaurenteEber/tupaqnin/internal/memory"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// A-1 (backlog-r2 2.2): the change rule 4 enters in a section's list
// reaches the principal by layer 3 of the startup, by its label, with no
// internal id.
func TestA1_OpenChangeLabelsReachThePrincipalAtStartup(t *testing.T) {
	db, seeded := seeded(t)
	ctx := context.Background()
	sectionLabel := seeded.SectionLabels[0]
	var srcID string
	err := db.Write(ctx, "p1", func(tx *store.Tx) error {
		src, err := tx.CreateObject(store.ObjectSource, "", map[string]any{
			"kind": "calculation", "filename": "calculo.md", "text": "Promedio: 60.", "fingerprint": "fp-1",
		}, store.EventInput{Type: store.EventStateChanged, Actor: "human"})
		if err != nil {
			return err
		}
		srcID = src.ID
		sec, err := tx.GetObject(ctx, "p1", sectionLabel)
		if err != nil {
			return err
		}
		if _, err := tx.AddEdge(store.EdgeUses, sec.ID, src.ID, "", graph.UsageOf(src),
			store.EventInput{Type: store.EventEdge, Actor: "agent"}); err != nil {
			return err
		}
		if _, err := tx.UpdateObject(src.ID, src.CurrentVersion, "", map[string]any{
			"kind": "calculation", "filename": "calculo.md", "text": "Promedio: 61.", "fingerprint": "fp-2",
		}, store.EventInput{Type: store.EventSourceNewVersion, Actor: "human"}); err != nil {
			return err
		}
		_, err = checks.Warn(ctx, tx, "human", "", []string{src.ID}, nil)
		return err
	})
	if err != nil {
		t.Fatalf("new source version: %v", err)
	}
	sec, _ := db.GetObject(ctx, "p1", sectionLabel)
	changes := graph.OpenChanges(sec)
	if len(changes) != 1 {
		t.Fatalf("open changes: %+v, want one", changes)
	}
	brief, err := memory.New(db).Startup(ctx, "p1", memory.StartupNewSession, 1048576, fakeCount)
	if err != nil {
		t.Fatalf("Startup: %v", err)
	}
	core := string(brief.Input[0])
	if want := "Open changes " + sectionLabel + ": " + changes[0].Label; !strings.Contains(core, want) {
		t.Fatalf("core does not carry %q", want)
	}
	if strings.Contains(briefText(brief), srcID) {
		t.Fatal("the startup carries the source's internal id")
	}
}
