package checks_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LaurenteEber/tupaqnin/internal/checks"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

func checkerProject(t *testing.T) (*store.DB, checks.Change) {
	t.Helper()
	ctx := context.Background()
	db, err := store.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := db.Writer.ExecContext(ctx, `INSERT INTO project (id, created_at) VALUES ('p1', '2026-10-31T00:00:00.000Z')`); err != nil {
		t.Fatalf("seed project: %v", err)
	}
	var change checks.Change
	err = db.Write(ctx, "p1", func(tx *store.Tx) error {
		human := store.EventInput{Type: store.EventStateChanged, Actor: "human"}
		if _, err := tx.CreateObject(store.ObjectRequest, "", map[string]any{"text": "Escribe."}, human); err != nil {
			return err
		}
		sec, err := tx.CreateObject(store.ObjectSection, "draft", map[string]any{"title": "S", "text": "Nuevo texto."}, human)
		if err != nil {
			return err
		}
		change = checks.Change{Kind: checks.AgentStep, Sections: []string{sec.ID},
			Passages: []checks.Passage{{Section: sec.ID, Text: "Nuevo texto."}}}
		return nil
	})
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	return db, change
}

// Fichas §7, B-44: the fixed shape the app checks beyond the schema:
// one verdict for each probe, and a contradiction or doubt carried by
// its candidate with the question and options.
func TestB44_TheCheckerShapeNeedsEveryProbeAndACandidateForEachContradiction(t *testing.T) {
	db, change := checkerProject(t)
	schema, err := os.ReadFile(filepath.Join("..", "cycle", "schema", "checker_output.schema.json"))
	if err != nil {
		t.Fatalf("read schema: %v", err)
	}
	c := checks.NewChecker(nil, "", schema)
	probes, err := checks.Probes(context.Background(), db, "p1", change)
	if err != nil || len(probes) != 1 || probes[0].Protected != "req-1" {
		t.Fatalf("Probes: %v, %v", probes, err)
	}
	for _, tc := range []struct {
		name, out, want string
	}{
		{"valid", `{"probes":[{"protected":"req-1","verdict":"no","passage":null}],"candidates":[]}`, ""},
		{"missing probe", `{"probes":[],"candidates":[]}`, "the probe req-1 has no verdict"},
		{"unknown probe", `{"probes":[{"protected":"req-1","verdict":"no","passage":null},{"protected":"dec-9","verdict":"no","passage":null}],"candidates":[]}`, "dec-9 is not one of the probes"},
		{"contradiction without candidate", `{"probes":[{"protected":"req-1","verdict":"contradicts","passage":"Nuevo texto."}],"candidates":[]}`, "no contradiction or doubtful_term candidate names it"},
		{"candidate without options", `{"probes":[{"protected":"req-1","verdict":"doubt","passage":"Nuevo texto."}],"candidates":[{"type":"contradiction","section":"sec-1","new_passage":"Nuevo texto.","clashing_passage":"Escribe.","protected":"req-1","citation":null,"question":null,"options":null}]}`, "carries its question and two options"},
		{"section outside the change", `{"probes":[{"protected":"req-1","verdict":"no","passage":null}],"candidates":[{"type":"process_narrative","section":"sec-2","new_passage":"x","clashing_passage":null,"protected":null,"citation":null,"question":null,"options":null}]}`, "sec-2 is not a section of this change"},
		{"schema", `{"probes":[{"protected":"req-1","verdict":"maybe","passage":null}],"candidates":[]}`, "/probes/0/verdict"},
	} {
		errs, _, err := c.Validate(context.Background(), db, "p1", change, probes, tc.out)
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		got := strings.Join(errs, "; ")
		if (tc.want == "") != (got == "") || !strings.Contains(got, tc.want) {
			t.Fatalf("%s: errors %q, want %q", tc.name, got, tc.want)
		}
	}
}
