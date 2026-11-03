package checks_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/LaurenteEber/tupaqnin/internal/checks"
	"github.com/LaurenteEber/tupaqnin/internal/graph"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// powerProject is a section whose figure «3,14 × 104» a decision covers.
func powerProject(t *testing.T, text string) (*store.DB, store.Object) {
	t.Helper()
	ctx := context.Background()
	db, err := store.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := db.Writer.ExecContext(ctx, `INSERT INTO project (id, created_at) VALUES ('p1', '2026-11-03T00:00:00.000Z')`); err != nil {
		t.Fatalf("seed project: %v", err)
	}
	var sec store.Object
	err = db.Write(ctx, "p1", func(tx *store.Tx) error {
		ev := store.EventInput{Type: store.EventStateChanged, Actor: "agent"}
		var err error
		sec, err = tx.CreateObject(store.ObjectSection, "draft", map[string]any{"title": "S", "text": text}, ev)
		if err != nil {
			return err
		}
		dec, err := tx.CreateObject(store.ObjectDecision, "human", map[string]any{"statement": "Usar el conteo."}, ev)
		if err != nil {
			return err
		}
		fig, err := tx.CreateObject(store.ObjectFigure, "", map[string]any{"section_id": sec.ID, "anchor": "3,14 × 104"}, ev)
		if err != nil {
			return err
		}
		_, err = tx.AddEdge(store.EdgeCovers, fig.ID, dec.ID, "", nil, store.EventInput{Type: store.EventEdge, Actor: "agent"})
		return err
	})
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	return db, sec
}

const powerBase = "El conteo fue de 3,14 × 104 partículas por litro en total."

func figureReasons(t *testing.T, db *store.DB, sec store.Object, text string) []graph.SuspiciousReason {
	t.Helper()
	edited := sec
	edited.Body = []byte(`{"title":"S","text":` + quote(text) + `}`)
	all, err := checks.SuspiciousReasons(context.Background(), db, "p1", checks.SuspectEdit{Section: edited, BaseText: powerBase})
	if err != nil {
		t.Fatalf("SuspiciousReasons: %v", err)
	}
	var out []graph.SuspiciousReason
	for _, r := range all {
		if r.Kind == graph.SuspiciousFigureAnchor {
			out = append(out, r)
		}
	}
	return out
}

// C.14, B-104: correcting a lost superscript is not deleting a cited figure, while
// changing the figure still is.
func TestB104_CorrectingAPowerDoesNotCountAsDeletingAFigure(t *testing.T) {
	db, sec := powerProject(t, powerBase)
	for _, text := range []string{
		"El conteo fue de 3,14 × 10⁴ partículas por litro en total.",
		"El conteo fue de 3,14 × 10^4 partículas por litro en total.",
	} {
		if got := figureReasons(t, db, sec, text); len(got) != 0 {
			t.Fatalf("%q fired: %+v", text, got)
		}
	}
	if got := figureReasons(t, db, sec, "El conteo fue de 3,14 × 10⁵ partículas por litro en total."); len(got) != 1 {
		t.Fatalf("10⁵: %+v, want the rule to fire", got)
	}
	if got := figureReasons(t, db, sec, "El conteo fue alto en total."); len(got) != 1 {
		t.Fatalf("a deleted figure: %+v, want the rule to fire", got)
	}
}

// C.14, B-104: the corrected power is still covered by its figure's decision, and
// another one is a figure without a decision.
func TestB104_ACorrectedPowerKeepsItsDecision(t *testing.T) {
	without := func(text string) []string {
		t.Helper()
		db, sec := powerProject(t, text)
		got, err := graph.FiguresWithoutDecision(context.Background(), db, "p1", sec.ID)
		if err != nil {
			t.Fatal(err)
		}
		return got
	}
	if got := without("El conteo fue de 3,14 × 10⁴ partículas."); len(got) != 0 {
		t.Fatalf("the corrected power: %v, want none", got)
	}
	if got := without("El conteo fue de 3,14 × 10⁵ partículas."); len(got) != 1 || got[0] != "10⁵" {
		t.Fatalf("another power: %v, want 10⁵", got)
	}
}

func quote(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}
