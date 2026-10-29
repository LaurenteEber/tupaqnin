package graph_test

import (
	"context"
	"strings"
	"testing"

	"github.com/LaurenteEber/tupaqnin/internal/graph"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// originFixture: sec-1 (a conclusion), two assumption decisions (dec-1,
// dec-2) and three critical decisions of sec-1 (dec-3, dec-4, dec-5):
// two depend on dec-2, one on dec-1, when depends is true.
func originFixture(t *testing.T, depends bool) (*store.DB, store.Object) {
	t.Helper()
	db := openGraphDB(t)
	ev := store.EventInput{Type: store.EventStateChanged, Actor: "agent"}
	edge := store.EventInput{Type: store.EventEdge, Actor: "agent"}
	var sec store.Object
	err := db.Write(context.Background(), "p1", func(tx *store.Tx) error {
		var err error
		sec, err = tx.CreateObject(store.ObjectSection, "draft", map[string]any{"title": "Síntesis",
			"text": "Texto.", "role": map[string]any{"value": "conclusion", "state": "confirmed"}}, ev)
		if err != nil {
			return err
		}
		var assumptions []store.Object
		for _, s := range []string{"Uso del 10 %.", "Mismo tamaño mínimo."} {
			a, err := tx.CreateObject(store.ObjectDecision, "agent", map[string]any{"statement": s, "assumption": true}, ev)
			if err != nil {
				return err
			}
			assumptions = append(assumptions, a)
		}
		for i, target := range []int{1, 1, 0} {
			d, err := tx.CreateObject(store.ObjectDecision, "agent", map[string]any{"statement": "Decisión " + string(rune('A'+i))}, ev)
			if err != nil {
				return err
			}
			if _, err := tx.AddEdge(store.EdgeUsedIn, d.ID, sec.ID, "", edge); err != nil {
				return err
			}
			if depends {
				if _, err := tx.AddEdge(store.EdgeDependsOnAssumption, d.ID, assumptions[target].ID, "", edge); err != nil {
					return err
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	return db, sec
}

// Fichas §9, E-3: the app chooses the assumption the most critical
// decisions of the section depend on, following depends_on_assumption.
func TestN1_OriginQuestionAssumptionIsTheOneMostCriticalDecisionsDependOn(t *testing.T) {
	db, sec := originFixture(t, true)
	ctx := context.Background()
	critical, err := graph.SectionCriticalDecisions(ctx, db, "p1", sec.ID)
	if err != nil {
		t.Fatalf("SectionCriticalDecisions: %v", err)
	}
	var labels []string
	for _, d := range critical {
		labels = append(labels, d.Label)
	}
	// The assumptions are used nowhere, so only dec-3..dec-5 are
	// critical (conclusion section), in the order they arrived.
	if strings.Join(labels, ",") != "dec-3,dec-4,dec-5" {
		t.Fatalf("critical: %v", labels)
	}
	id, err := graph.OriginAssumption(ctx, db, "p1", critical)
	if err != nil {
		t.Fatalf("OriginAssumption: %v", err)
	}
	chosen, _ := db.GetObject(ctx, "p1", id)
	if chosen.Label != "dec-2" {
		t.Fatalf("origin assumption: %s, want dec-2 (two critical decisions depend on it)", chosen.Label)
	}
}

// Fichas §9: without an edge to an assumption, Wonder chooses.
func TestN1_WithoutAnAssumptionEdgeWonderChooses(t *testing.T) {
	db, sec := originFixture(t, false)
	critical, _ := graph.SectionCriticalDecisions(context.Background(), db, "p1", sec.ID)
	if id, err := graph.OriginAssumption(context.Background(), db, "p1", critical); err != nil || id != "" {
		t.Fatalf("got %q, %v; want none", id, err)
	}
}

// Rule 7 and fichas §7 (Wonder): the brief carries labels and literal
// text, never an internal id.
func TestR7_WonderBriefCarriesLabelsNeverInternalIDs(t *testing.T) {
	db, sec := originFixture(t, true)
	ctx := context.Background()
	critical, _ := graph.SectionCriticalDecisions(ctx, db, "p1", sec.ID)
	origin, _ := graph.OriginAssumption(ctx, db, "p1", critical)
	body := graph.SignoffBody{SectionID: sec.ID, SectionVersion: 1, OriginAssumptionID: origin,
		Explanation: "Mi explicación.", Cases: []graph.SignoffCase{{Case: "Un caso.", Answer: "Mi respuesta."}}}
	for _, d := range critical {
		body.Critical = append(body.Critical, graph.SignedRef{ID: d.ID, Version: 1})
	}
	var sign store.Object
	err := db.Write(ctx, "p1", func(tx *store.Tx) error {
		var err error
		sign, err = tx.CreateObject(store.ObjectSignoff, "", body, store.EventInput{Type: store.EventStateChanged, Actor: "human"})
		return err
	})
	if err != nil {
		t.Fatalf("create signoff: %v", err)
	}
	brief, err := graph.WonderBrief(ctx, db, "p1", sign.ID)
	if err != nil {
		t.Fatalf("WonderBrief: %v", err)
	}
	objects, _ := db.AllObjects(ctx, "p1")
	for _, o := range objects {
		if strings.Contains(brief, o.ID) {
			t.Fatalf("the brief carries the internal id of %s", o.Label)
		}
	}
	for _, want := range []string{"sec-1", "dec-3", "dec-4", "dec-5", "The app chose the assumption dec-2: Mismo tamaño mínimo.",
		"Mi explicación.", "Un caso.", "Mi respuesta.", "Not judged yet"} {
		if !strings.Contains(brief, want) {
			t.Fatalf("brief lacks %q:\n%s", want, brief)
		}
	}
}
