package cycle_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/LaurenteEber/tupaqnin/internal/checks"
	"github.com/LaurenteEber/tupaqnin/internal/cycle"
	"github.com/LaurenteEber/tupaqnin/internal/graph"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// Piece 2.2 (backlog-r2): the list of changes, the warnings (rule 4)
// and «escrita con lo anterior».

// seedChangesProject creates, in this order: sec-1 «Síntesis» and
// sec-2 «Conclusiones» (draft, with text), src-1 (used by sec-2) and
// dec-1 (the agent's, used in both). The using edges keep what they
// used (B-45).
func seedChangesProject(t *testing.T) *store.DB {
	t.Helper()
	db := openTemp(t)
	ctx := context.Background()
	human := store.EventInput{Type: store.EventStateChanged, Actor: "human"}
	edge := store.EventInput{Type: store.EventEdge, Actor: "agent"}
	err := db.Write(ctx, "p1", func(tx *store.Tx) error {
		syn, err := tx.CreateObject(store.ObjectSection, "draft", map[string]any{
			"title": "Síntesis", "text": "La síntesis compara por promedio. Cierre.",
			"role": map[string]any{"value": "other", "state": "confirmed"}, "open_changes": []any{},
		}, human)
		if err != nil {
			return err
		}
		con, err := tx.CreateObject(store.ObjectSection, "draft", map[string]any{
			"title": "Conclusiones", "text": "La embotellada trae más partículas. Fin.",
			"role": map[string]any{"value": "other", "state": "confirmed"}, "open_changes": []any{},
		}, human)
		if err != nil {
			return err
		}
		src, err := tx.CreateObject(store.ObjectSource, "", map[string]any{
			"kind": "calculation", "filename": "calculo.md", "text": "Promedio: 60.",
			"fingerprint": "fp-1",
		}, human)
		if err != nil {
			return err
		}
		dec, err := tx.CreateObject(store.ObjectDecision, "agent", map[string]any{
			"statement": "Comparar por promedio.", "instead_of": "Comparar por mediana.",
			"would_choose_if": "Hubiera valores extremos.", "assumption": false, "origin": "agent",
		}, store.EventInput{Type: store.EventStateChanged, Actor: "agent"})
		if err != nil {
			return err
		}
		for _, sec := range []store.Object{syn, con} {
			if _, err := tx.AddEdge(store.EdgeUsedIn, dec.ID, sec.ID, "", graph.UsageOf(dec), edge); err != nil {
				return err
			}
		}
		_, err = tx.AddEdge(store.EdgeUses, con.ID, src.ID, "", graph.UsageOf(src), edge)
		return err
	})
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	return db
}

func openChangeLabels(t *testing.T, db *store.DB, label string) []string {
	t.Helper()
	sec, err := db.GetObject(context.Background(), "p1", label)
	if err != nil {
		t.Fatalf("GetObject %s: %v", label, err)
	}
	out := []string{}
	for _, c := range graph.OpenChanges(sec) {
		out = append(out, c.Label)
	}
	return out
}

// supersedeInSynthesis is an agent step that rewrites «Síntesis» and
// replaces dec-1 with a new decision used there.
const supersedeInSynthesis = `{"stretch":"build","triage_line":"Construir.","closing_risk":"none","note":"n",
	"new_decisions":[{"id":"new-1","statement":"Comparar por mediana.","instead_of":"Comparar por promedio.",
		"would_choose_if":"No hubiera valores extremos.","assumption":false,"used_in_sections":["sec-1"],"supersedes":"dec-1"}],
	"new_text":[{"section_id":"sec-1","base_version":1,"anchor":"Cierre.","claim_origin":"material","operation":"replace","text":"Se compara por mediana."}]}`

// A-1, rule 4: changing the decision of «Síntesis» that «Conclusiones»
// uses leaves a chg-n in «Conclusiones» and moves it to stale, with
// usage_change entered; «Síntesis», which the step wrote, gets nothing.
func TestA1_ChangedDecisionOfSynthesisLeavesAChangeInConclusions(t *testing.T) {
	db := seedChangesProject(t)
	if applied := applyRecorded(t, db, json.RawMessage(supersedeInSynthesis)); !applied.Applied {
		t.Fatalf("ApplyStep: %+v", applied)
	}
	if got := openChangeLabels(t, db, "sec-2"); strings.Join(got, ",") != "chg-1" {
		t.Fatalf("sec-2 open changes: %v, want chg-1", got)
	}
	sec, _ := db.GetObject(context.Background(), "p1", "sec-2")
	if sec.State != "stale" {
		t.Fatalf("sec-2 state %q, want stale", sec.State)
	}
	if c := graph.OpenChanges(sec)[0]; c.Kind != "decision" || c.About != "dec-1" {
		t.Fatalf("change: %+v, want decision dec-1", c)
	}
	if got := openChangeLabels(t, db, "sec-1"); len(got) != 0 {
		t.Fatalf("sec-1 open changes: %v, want none (the step wrote it)", got)
	}
	if st, _ := status(t, db, "sec-2"); st != graph.StatusStale {
		t.Fatalf("SectionStatus sec-2: %s, want stale", st)
	}
	var entered bool
	for _, ev := range eventsOfType(t, db, store.EventUsageChange) {
		var data map[string]any
		_ = json.Unmarshal(ev.Data, &data)
		entered = entered || data["kind"] == checks.UsageEntered && data["change"] == "chg-1" && data["section"] == "sec-2"
	}
	if !entered {
		t.Fatal("no usage_change entered for chg-1 in sec-2")
	}
	if data := lastStepEnded(t, db); data["usage_changes"] == nil {
		t.Fatalf("step_ended without usage_changes: %v", data)
	}
}

// A-1, rule 4 on a signed section: the same change moves it to
// changed_since_signoff and enters its list (estados §1).
func TestA1_ChangedDecisionMovesASignedSectionToChangedSinceSignoff(t *testing.T) {
	db := seedChangesProject(t)
	ctx := context.Background()
	err := db.Write(ctx, "p1", func(tx *store.Tx) error {
		sec, err := tx.GetObject(ctx, "p1", "sec-2")
		if err != nil {
			return err
		}
		var b map[string]any
		_ = json.Unmarshal(sec.Body, &b)
		if _, err := tx.UpdateObject(sec.ID, sec.CurrentVersion, "signed", b,
			store.EventInput{Type: store.EventStateChanged, Actor: "human"}); err != nil {
			return err
		}
		dec, err := tx.GetObject(ctx, "p1", "dec-1")
		if err != nil {
			return err
		}
		var db map[string]any
		_ = json.Unmarshal(dec.Body, &db)
		if _, err := tx.UpdateObject(dec.ID, dec.CurrentVersion, "revoked", db,
			store.EventInput{Type: store.EventStateChanged, Actor: "human"}); err != nil {
			return err
		}
		_, err = checks.RunRules(ctx, tx, "p1", checks.Change{Kind: checks.YourEdit, Changed: []string{dec.ID}, Actor: "human"})
		return err
	})
	if err != nil {
		t.Fatalf("revoke: %v", err)
	}
	sec, _ := db.GetObject(ctx, "p1", "sec-2")
	if sec.State != "changed_since_signoff" || len(graph.OpenChanges(sec)) != 1 {
		t.Fatalf("sec-2: %s %v, want changed_since_signoff with one change", sec.State, graph.OpenChanges(sec))
	}
	if sec1, _ := db.GetObject(ctx, "p1", "sec-1"); sec1.State != "stale" {
		t.Fatalf("sec-1 state %q, want stale", sec1.State)
	}
}

// B-45: only content counts as a change; confirming a decision or
// marking it critical warns nobody.
func TestB45_StateChangeOfADecisionDoesNotWarn(t *testing.T) {
	db := seedChangesProject(t)
	ctx := context.Background()
	err := db.Write(ctx, "p1", func(tx *store.Tx) error {
		dec, err := tx.GetObject(ctx, "p1", "dec-1")
		if err != nil {
			return err
		}
		var b map[string]any
		_ = json.Unmarshal(dec.Body, &b)
		b["human_marked_critical"] = true
		if _, err := tx.UpdateObject(dec.ID, dec.CurrentVersion, "confirmed", b,
			store.EventInput{Type: store.EventStateChanged, Actor: "human"}); err != nil {
			return err
		}
		findings, err := checks.Warn(ctx, tx, "human", "", []string{dec.ID}, nil)
		if len(findings) != 0 {
			t.Errorf("findings: %+v, want none", findings)
		}
		return err
	})
	if err != nil {
		t.Fatalf("confirm: %v", err)
	}
	for _, l := range []string{"sec-1", "sec-2"} {
		if got := openChangeLabels(t, db, l); len(got) != 0 {
			t.Fatalf("%s open changes: %v, want none", l, got)
		}
	}
}

// A-1, rule 7 of the gate: a step that declares updated without
// touching the section is rejected in that close alone, with a message
// for the agent; the rest of the step applies. A step that writes the
// section closes it, and the empty list returns it to draft.
func TestA1_UpdatedWithoutTouchingTheSectionIsRejectedInThatClose(t *testing.T) {
	db := seedChangesProject(t)
	applyRecorded(t, db, json.RawMessage(supersedeInSynthesis))
	sec1, _ := db.GetObject(context.Background(), "p1", "sec-1")
	out := `{"stretch":"build","triage_line":"Construir.","closing_risk":"none","note":"n",
		"handled_changes":[{"change_id":"chg-1","result":"updated"}],
		"new_text":[{"section_id":"sec-1","base_version":` + itoa(sec1.CurrentVersion) + `,"anchor":"Se compara por mediana.","claim_origin":"material","operation":"replace","text":"Se compara por la mediana."}]}`
	applied := applyRecorded(t, db, json.RawMessage(out))
	if !applied.Applied || len(applied.Rejected) != 1 || !strings.Contains(applied.Rejected[0], "chg-1") ||
		!strings.Contains(applied.Rejected[0], "sec-2") {
		t.Fatalf("ApplyStep: %+v, want the close of chg-1 rejected with a message", applied)
	}
	if text := bodyText(t, db, "sec-1"); text != "La síntesis compara por promedio. Se compara por la mediana." {
		t.Fatalf("sec-1 text %q: the rest of the step must apply", text)
	}
	if got := openChangeLabels(t, db, "sec-2"); strings.Join(got, ",") != "chg-1" {
		t.Fatalf("sec-2 open changes: %v, want chg-1 still open", got)
	}
	notApplied, _ := lastStepEnded(t, db)["not_applied"].(map[string]any)
	if notApplied["handled_changes"] == nil {
		t.Fatalf("step_ended not_applied without handled_changes: %v", notApplied)
	}

	out = `{"stretch":"build","triage_line":"Construir.","closing_risk":"none","note":"n",
		"handled_changes":[{"change_id":"chg-1","result":"updated"}],
		"used_decisions":["dec-2"],
		"new_text":[{"section_id":"sec-2","base_version":` + itoa(version(t, db, "sec-2")) + `,"anchor":"Fin.","claim_origin":"material","operation":"replace","text":"Comparada por mediana."}]}`
	if applied := applyRecorded(t, db, json.RawMessage(out)); !applied.Applied || len(applied.Rejected) != 0 {
		t.Fatalf("ApplyStep that writes sec-2: %+v", applied)
	}
	if got := openChangeLabels(t, db, "sec-2"); len(got) != 0 {
		t.Fatalf("sec-2 open changes: %v, want none", got)
	}
	if st, _ := status(t, db, "sec-2"); st != graph.StatusReady && st != graph.StatusDraft {
		t.Fatalf("sec-2 state %s, want draft (or ready)", st)
	}
}

// T-1, rule 7: reviewed_unchanged with «ok» does not close, from the
// agent or from you; with a valid «porque» it does.
func TestT1_ReviewedUnchangedWithOkDoesNotClose(t *testing.T) {
	db := seedChangesProject(t)
	applyRecorded(t, db, json.RawMessage(supersedeInSynthesis))
	out := `{"stretch":"build","triage_line":"Revisar.","closing_risk":"none","note":"n",
		"handled_changes":[{"change_id":"chg-1","result":"reviewed_unchanged","reason":"ok"}]}`
	if applied := applyRecorded(t, db, json.RawMessage(out)); len(applied.Rejected) != 1 {
		t.Fatalf("ApplyStep: %+v, want the close rejected", applied)
	}
	gate := cycle.NewGate(db)
	if err := gate.CloseChange(context.Background(), "p1", "chg-1", checks.UsageReviewedUnchanged, "ok"); !errors.Is(err, cycle.ErrInvalidReason) {
		t.Fatalf("CloseChange with ok: %v, want ErrInvalidReason", err)
	}
	if got := openChangeLabels(t, db, "sec-2"); strings.Join(got, ",") != "chg-1" {
		t.Fatalf("sec-2 open changes: %v, want chg-1 still open", got)
	}
	if err := gate.CloseChange(context.Background(), "p1", "chg-1", checks.UsageReviewedUnchanged,
		"La conclusión no depende del promedio."); err != nil {
		t.Fatalf("CloseChange with a reason: %v", err)
	}
	sec, _ := db.GetObject(context.Background(), "p1", "sec-2")
	if sec.State != "draft" || len(graph.OpenChanges(sec)) != 0 {
		t.Fatalf("sec-2: %s %v, want draft with the list empty", sec.State, graph.OpenChanges(sec))
	}
}

// E-4: with a change open, asking to sign lists it and opens no
// signoff; editing something else does not free the section; updated
// after your edit empties the list and the section returns to draft.
func TestE4_OpenChangeBlocksSignoffAndEmptyListReturnsToDraft(t *testing.T) {
	db := seedChangesProject(t)
	applyRecorded(t, db, json.RawMessage(supersedeInSynthesis))
	gate := cycle.NewGate(db)
	ctx := context.Background()

	open, err := gate.RequestSignoff(ctx, "p1", "sec-2")
	if err != nil {
		t.Fatalf("RequestSignoff: %v", err)
	}
	if open.Opened || strings.Join(open.OpenChanges, ",") != "chg-1" {
		t.Fatalf("RequestSignoff: %+v, want chg-1 listed and no signoff", open)
	}
	if err := gate.CloseChange(ctx, "p1", "chg-1", checks.UsageUpdated, ""); !errors.Is(err, cycle.ErrSectionNotEdited) {
		t.Fatalf("updated before an edit: %v, want ErrSectionNotEdited", err)
	}
	yourEdit(t, db, "sec-1", "Otra sección, editada.") // another section
	if err := gate.CloseChange(ctx, "p1", "chg-1", checks.UsageUpdated, ""); !errors.Is(err, cycle.ErrSectionNotEdited) {
		t.Fatalf("updated after editing another section: %v, want ErrSectionNotEdited", err)
	}
	yourEdit(t, db, "sec-2", "La embotellada trae más partículas, por mediana. Fin.")
	if st, _ := status(t, db, "sec-2"); st != graph.StatusStale {
		t.Fatalf("after your edit: %s, want still stale (an edit does not free it)", st)
	}
	if err := gate.CloseChange(ctx, "p1", "chg-1", checks.UsageUpdated, ""); err != nil {
		t.Fatalf("updated after your edit: %v", err)
	}
	if st, _ := status(t, db, "sec-2"); st != graph.StatusReady {
		t.Fatalf("after closing: %s, want ready (draft without pendings)", st)
	}
	if sec, _ := db.GetObject(ctx, "p1", "sec-2"); sec.State != "draft" {
		t.Fatalf("stored state %q, want draft", sec.State)
	}
}

func version(t *testing.T, db *store.DB, label string) int {
	t.Helper()
	obj, err := db.GetObject(context.Background(), "p1", label)
	if err != nil {
		t.Fatalf("GetObject %s: %v", label, err)
	}
	return obj.CurrentVersion
}

func bodyText(t *testing.T, db *store.DB, label string) string {
	t.Helper()
	obj, err := db.GetObject(context.Background(), "p1", label)
	if err != nil {
		t.Fatalf("GetObject %s: %v", label, err)
	}
	var b map[string]any
	_ = json.Unmarshal(obj.Body, &b)
	s, _ := b["text"].(string)
	return s
}

// E-4: SectionStatus counts open changes in any state: a draft with
// text, no pendings and a change in its list is not ready.
func TestE4_SectionStatusCountsOpenChangesInAnyState(t *testing.T) {
	db := seedChangesProject(t)
	ctx := context.Background()
	if st, _ := status(t, db, "sec-2"); st != graph.StatusReady {
		t.Fatalf("before: %s, want ready", st)
	}
	err := db.Write(ctx, "p1", func(tx *store.Tx) error {
		sec, err := tx.GetObject(ctx, "p1", "sec-2")
		if err != nil {
			return err
		}
		var b map[string]any
		_ = json.Unmarshal(sec.Body, &b)
		b["open_changes"] = []any{map[string]any{"label": "chg-9", "kind": "source", "about": "src-1"}}
		_, err = tx.UpdateObject(sec.ID, sec.CurrentVersion, "draft", b, store.EventInput{Type: store.EventEdit, Actor: "human"})
		return err
	})
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	if st, _ := status(t, db, "sec-2"); st != graph.StatusDraft {
		t.Fatalf("with an open change: %s, want draft (not ready)", st)
	}
}
