package cycle_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/LaurenteEber/tupaqnin/internal/graph"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// writeLooseFigure is a first step that writes «7 casos» in sec-1
// without linking it: the pending «cifra sin decisión» (B-15).
func writeLooseFigure(t *testing.T, db *store.DB) store.Object {
	t.Helper()
	applied := applyRecorded(t, db, json.RawMessage(`{"triage_line": "Construir: un paso.", "closing_risk": "none", "note": "Paso de prueba.",
		"new_text": [{"section_id": "sec-1", "base_version": 1, "anchor": "Cierre.", "operation": "insert_after", "text": " Quedan 7 casos."}]}`))
	if !applied.Applied {
		t.Fatalf("first step not applied: %v", applied.Errors)
	}
	sec, _ := object(t, db, "sec-1")
	if pending, _ := graph.FiguresWithoutDecision(context.Background(), db, "p1", sec.ID); len(pending) != 1 || pending[0] != "7" {
		t.Fatalf("after the first step: figures without decision %v, want [7]", pending)
	}
	return sec
}

// P-52, B-15: «cifra sin decisión» closes «al vincularla». A later step
// links a figure already in the text, without writing it again.
func TestP52_ALaterStepLinksAFigureAlreadyInTheText(t *testing.T) {
	db := seedStepProject(t)
	sec := writeLooseFigure(t, db)
	applied := applyRecorded(t, db, json.RawMessage(`{"triage_line": "Construir: vinculo la cifra.", "closing_risk": "figures_and_calculations", "note": "Paso de prueba.",
		"new_decisions": [{"id": "new-1", "statement": "Contar los casos que quedan.", "instead_of": "Contar todos los casos.",
			"would_choose_if": "No hubiera exclusiones.", "assumption": false, "used_in_sections": ["sec-1"]}],
		"figure_links": [{"figure_anchor": "7 casos", "decision_id": "new-1"}]}`))
	if !applied.Applied {
		t.Fatalf("linking step not applied: %v", applied.Errors)
	}
	if pending, err := graph.FiguresWithoutDecision(context.Background(), db, "p1", sec.ID); err != nil || len(pending) != 0 {
		t.Fatalf("figures without decision: %v, %v; want none", pending, err)
	}
	fig, fb := object(t, db, "fig-1")
	dec, _ := object(t, db, applied.Labels["new-1"])
	if fb["section_id"] != sec.ID || fb["anchor"] != "7 casos" {
		t.Fatalf("figure body: %v", fb)
	}
	if got := edgeTypes(t, db, fig.ID)[store.EdgeCovers]; len(got) != 1 || got[0] != dec.ID {
		t.Fatalf("covers edge: %v", got)
	}
}

// P-52: a citation's claim anchor is looked up in the text after the
// step too.
func TestP52_ALaterStepCitesAPassageAlreadyInTheText(t *testing.T) {
	db := seedStepProject(t)
	applied := applyRecorded(t, db, json.RawMessage(`{"triage_line": "Construir: cito.", "closing_risk": "none", "note": "Paso de prueba.",
		"used_sources": ["src-1"],
		"citations": [{"claim_anchor": "Los estudios usan filtros.", "source_id": "src-1", "passage": "Los filtros finos retienen más partículas."}]}`))
	if !applied.Applied {
		t.Fatalf("citing step not applied: %v", applied.Errors)
	}
	cit, cb := object(t, db, "cit-1")
	sec, _ := object(t, db, "sec-1")
	src, _ := object(t, db, "src-1")
	if cb["section_id"] != sec.ID {
		t.Fatalf("citation section: %v, want sec-1", cb["section_id"])
	}
	if got := edgeTypes(t, db, cit.ID)[store.EdgeComesFrom]; len(got) != 1 || got[0] != src.ID {
		t.Fatalf("comes_from: %v", got)
	}
}

// P-52: the anchor must be unique in the project after the step; 0 or
// several appearances are the same error.
func TestP52_AnAnchorInTwoSectionsIsRejectedWithItsCount(t *testing.T) {
	db := seedStepProject(t)
	writeLooseFigure(t, db)
	requireRejected(t, applyRecorded(t, db, json.RawMessage(`{"triage_line": "Construir: otra sección.", "closing_risk": "none", "note": "Paso de prueba.",
		"proposed_roles": [{"section_id": "new-2", "role": "other", "title": "Otra"}],
		"new_decisions": [{"id": "new-1", "statement": "Contar los casos que quedan.", "instead_of": "Contar todos los casos.",
			"would_choose_if": "No hubiera exclusiones.", "assumption": false, "used_in_sections": ["sec-1"]}],
		"new_text": [{"section_id": "new-2", "base_version": 0, "anchor": "", "operation": "insert_after", "text": "Aquí también quedan 7 casos."}],
		"figure_links": [{"figure_anchor": "7 casos", "decision_id": "new-1"}]}`)), "the anchor appears 2 times")
	requireRejected(t, applyRecorded(t, db, json.RawMessage(`{"triage_line": "Construir: vinculo.", "closing_risk": "none", "note": "Paso de prueba.",
		"new_decisions": [{"id": "new-1", "statement": "Contar los casos que quedan.", "instead_of": "Contar todos los casos.",
			"would_choose_if": "No hubiera exclusiones.", "assumption": false, "used_in_sections": ["sec-1"]}],
		"figure_links": [{"figure_anchor": "9 casos", "decision_id": "new-1"}]}`)), "the anchor appears 0 times")
}

// P-52 with B-14: a link into a signed section is not applied (the
// section rejects the agent's steps); the rest of the step applies.
func TestP52_LinkIntoASignedSectionIsNotApplied(t *testing.T) {
	db := seedStepProject(t)
	writeLooseFigure(t, db)
	if _, err := db.Writer.ExecContext(context.Background(), `UPDATE object SET state = 'signed' WHERE project_id = 'p1' AND label = 'sec-1'`); err != nil {
		t.Fatalf("sign: %v", err)
	}
	applied := applyRecorded(t, db, json.RawMessage(`{"triage_line": "Construir: vinculo.", "closing_risk": "none", "note": "Paso de prueba.",
		"new_decisions": [{"id": "new-1", "statement": "Contar los casos que quedan.", "instead_of": "Contar todos los casos.",
			"would_choose_if": "No hubiera exclusiones.", "assumption": false, "used_in_sections": ["sec-1"]}],
		"figure_links": [{"figure_anchor": "7 casos", "decision_id": "new-1"}]}`))
	if !applied.Applied {
		t.Fatalf("not applied: %v", applied.Errors)
	}
	if figs, _ := db.ListObjects(context.Background(), "p1", store.ObjectFigure); len(figs) != 0 {
		t.Fatalf("figures: %d, want none in the signed section", len(figs))
	}
	ended := eventsOfType(t, db, store.EventStepEnded)
	if last := string(ended[len(ended)-1].Data); !strings.Contains(last, `"figure_links"`) {
		t.Fatalf("step_ended not_applied misses the link: %s", last)
	}
}
