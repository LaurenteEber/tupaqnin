package cycle_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/LaurenteEber/tupaqnin/internal/cycle"
	"github.com/LaurenteEber/tupaqnin/internal/graph"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

func history(t *testing.T, db *store.DB, block int) []graph.Entry {
	t.Helper()
	entries, err := graph.BlockHistory(context.Background(), db, "p1", "sec-1", block)
	if err != nil {
		t.Fatalf("BlockHistory: %v", err)
	}
	return entries
}

// B-10: after an agent step and your edit, the block shows both, in
// order (newest first).
func TestB10_StepAndYourEditShowInTheBlockInOrder(t *testing.T) {
	db := seedStepProject(t)
	step := json.RawMessage(`{"triage_line":"Construir.","closing_risk":"none","note":"Sumo una frase.",
		"new_text":[{"section_id":"sec-1","base_version":1,"anchor":"Cierre.","operation":"insert_after","text":" El agente suma una frase sobre filtros."}]}`)
	if applied := applyRecorded(t, db, step); !applied.Applied {
		t.Fatalf("step: %v", applied.Errors)
	}
	ctx := context.Background()
	if err := db.Write(ctx, "p1", func(tx *store.Tx) error {
		sec, _ := tx.GetObject(ctx, "p1", "sec-1")
		var b map[string]any
		_ = json.Unmarshal(sec.Body, &b)
		b["text"] = b["text"].(string) + " La persona agrega una frase larga propia."
		_, err := tx.UpdateObject(sec.ID, sec.CurrentVersion, sec.State, b, store.EventInput{Type: store.EventEdit, Actor: "human"})
		return err
	}); err != nil {
		t.Fatalf("edit: %v", err)
	}
	entries := history(t, db, 1)
	if len(entries) < 2 || entries[0].Actor != "human" || entries[0].Version != 3 ||
		entries[1].Actor != "agent" || entries[1].Version != 2 {
		t.Fatalf("history: %+v, want your edit (v3) then the step (v2)", entries)
	}
}

func decisionEntries(entries []graph.Entry) map[string]*graph.DecisionEntry {
	out := map[string]*graph.DecisionEntry{}
	for _, e := range entries {
		if e.Kind == graph.EntryDecision {
			out[e.Decision.Label] = e.Decision
		}
	}
	return out
}

// A linked figure leads to its decision with its contrast.
func TestD7_LinkedFigureLeadsToItsDecisionWithItsContrast(t *testing.T) {
	db := cleanCardProject(t)
	d := decisionEntries(history(t, db, 1))["dec-3"]
	if d == nil || d.Figure != "60 veces" || d.Statement != "Comparar por veces." ||
		d.InsteadOf != "Comparar por conteos crudos." || d.WouldChooseIf == "" || d.Discarded {
		t.Fatalf("dec-3 entry: %+v", d)
	}
}

// A replaced decision shows as discarded, next to yours with its
// «porque».
func TestD7_ReplacedDecisionShowsAsDiscarded(t *testing.T) {
	db := cleanCardProject(t)
	if _, err := cycle.NewGate(db).AnswerCard(context.Background(), "p1", "card-1", cycle.OptionOther, "Porque el comité pidió conteos."); err != nil {
		t.Fatalf("AnswerCard: %v", err)
	}
	got := decisionEntries(history(t, db, 1))
	if got["dec-4"] == nil || got["dec-4"].Discarded || got["dec-4"].Reason != "Porque el comité pidió conteos." {
		t.Fatalf("your decision: %+v", got["dec-4"])
	}
	if got["dec-3"] == nil || !got["dec-3"].Discarded {
		t.Fatalf("replaced decision: %+v, want discarded", got["dec-3"])
	}
}

func TestB10_BlockOutOfRangeIsNotFound(t *testing.T) {
	db := seedStepProject(t)
	if _, err := graph.BlockHistory(context.Background(), db, "p1", "sec-1", 9); err == nil {
		t.Fatal("block 9 of a one-block section: want an error")
	}
}

// P-60 (pantallas P5, D7): the case 5 paragraph with «1,5» shows the
// comparison rule the step declared for its section and the datum 1,5
// with the calculation source and its passage (A-3).
func TestP60_Case5ParagraphShowsTheRuleAndTheCalculationSource(t *testing.T) {
	db := seedStepProject(t)
	calc, err := os.ReadFile(filepath.Join("..", "demo", "case5", "template", "sources", "calculo-comparacion.md"))
	if err != nil {
		t.Fatalf("read the case 5 calculation: %v", err)
	}
	ctx := context.Background()
	if err := db.Write(ctx, "p1", func(tx *store.Tx) error {
		src, err := tx.CreateObject(store.ObjectSource, "", map[string]any{
			"kind": "file", "filename": "calculo-comparacion.md", "text": string(calc),
		}, store.EventInput{Type: store.EventStateChanged, Actor: "human"})
		if err != nil {
			return err
		}
		return tx.RecordDelivery(store.Delivery{Via: store.ViaRead, ObjectID: src.ID, Version: src.CurrentVersion})
	}); err != nil {
		t.Fatalf("seed source: %v", err)
	}
	step := json.RawMessage(`{"triage_line":"Construir.","closing_risk":"none","note":"Escribo la comparación.",
		"new_decisions":[{"id":"new-1","statement":"Comparar los conteos desde el mismo tamaño mínimo de partícula.",
			"instead_of":"Comparar los conteos crudos.","would_choose_if":"Todos los estudios usaran el mismo filtro.",
			"assumption":false,"used_in_sections":["sec-1"]}],
		"new_text":[{"section_id":"sec-1","base_version":1,"anchor":"Cierre.","operation":"insert_after",
			"text":" La embotellada trae 1,5 veces más partículas que la de grifo."}],
		"used_sources":["src-2"]}`)
	applied := applyRecorded(t, db, step)
	if !applied.Applied {
		t.Fatalf("step: %v", applied.Errors)
	}
	entries := history(t, db, 1)
	rule := decisionEntries(entries)[applied.Labels["new-1"]]
	if rule == nil || rule.Statement != "Comparar los conteos desde el mismo tamaño mínimo de partícula." ||
		rule.InsteadOf != "Comparar los conteos crudos." {
		t.Fatalf("the rule is not in the paragraph's history: %+v", entries)
	}
	var datum *graph.DatumEntry
	for _, e := range entries {
		if e.Kind == graph.EntryDatum {
			datum = e.Datum
		}
	}
	if datum == nil || datum.Value != "1,5" || datum.Source != "calculo-comparacion.md" ||
		datum.Passage != "Embotellada frente a grifo: 1,5 veces." {
		t.Fatalf("datum entry: %+v, want 1,5 from calculo-comparacion.md with its passage", datum)
	}
}
