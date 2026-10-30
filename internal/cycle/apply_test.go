package cycle_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LaurenteEber/tupaqnin/internal/cycle"
	"github.com/LaurenteEber/tupaqnin/internal/graph"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// seedStepProject creates, in this order so their labels are fixed:
// req-1, crit-1, sec-1 (version 1), src-1, dec-1 (the human's) and
// dec-2 (the agent's, used in sec-1).
func seedStepProject(t *testing.T) *store.DB {
	t.Helper()
	db := openTemp(t)
	human := store.EventInput{Type: store.EventStateChanged, Actor: "human"}
	err := db.Write(context.Background(), "p1", func(tx *store.Tx) error {
		req, err := tx.CreateObject(store.ObjectRequest, "", map[string]any{"text": "Escribe la síntesis."}, human)
		if err != nil {
			return err
		}
		if _, err := tx.CreateObject(store.ObjectCriterion, "current",
			map[string]any{"text": "Comparar conteos.", "request_id": req.ID}, human); err != nil {
			return err
		}
		sec, err := tx.CreateObject(store.ObjectSection, "draft", map[string]any{
			"title": "Síntesis", "text": "La síntesis compara estudios. Los estudios usan filtros. Cierre.",
			"role": map[string]any{"value": "other", "state": "confirmed"}, "open_changes": []any{},
		}, human)
		if err != nil {
			return err
		}
		src, err := tx.CreateObject(store.ObjectSource, "", map[string]any{
			"kind": "file", "filename": "estudio.md",
			"text": "En 2023 los conteos fueron 60 veces mayores. Los filtros finos retienen más partículas.",
		}, human)
		if err != nil {
			return err
		}
		// The principal already read it (R-3): these tests are about
		// what a read source supports.
		if err := tx.RecordDelivery(store.Delivery{Via: store.ViaRead, ObjectID: src.ID, Version: src.CurrentVersion}); err != nil {
			return err
		}
		if _, err := tx.CreateObject(store.ObjectDecision, "human", map[string]any{
			"statement": "Usar el informe del comité.", "origin": "human",
		}, human); err != nil {
			return err
		}
		dec, err := tx.CreateObject(store.ObjectDecision, "agent", map[string]any{
			"statement": "Comparar por promedio.", "instead_of": "Comparar por mediana.",
			"would_choose_if": "Hubiera valores extremos.", "assumption": false, "origin": "agent",
		}, store.EventInput{Type: store.EventStateChanged, Actor: "agent"})
		if err != nil {
			return err
		}
		_, err = tx.AddEdge(store.EdgeUsedIn, dec.ID, sec.ID, "", store.EventInput{Type: store.EventEdge, Actor: "agent"})
		return err
	})
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	return db
}

func stepFile(t *testing.T, name string) json.RawMessage {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "steps", name+".json"))
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return raw
}

// applyRecorded runs a recorded output through EndStep and ApplyStep,
// as the runner does.
func applyRecorded(t *testing.T, db *store.DB, output json.RawMessage) cycle.ApplyResult {
	t.Helper()
	gate := cycle.NewGate(db)
	res, err := gate.EndStep(context.Background(), "p1", "turn-1", 1, output)
	if err != nil || res.Status != cycle.StatusAccepted {
		t.Fatalf("EndStep: %+v, %v (the recorded output must have the fixed shape)", res, err)
	}
	applied, err := gate.ApplyStep(context.Background(), "p1", "turn-1", output)
	if err != nil {
		t.Fatalf("ApplyStep: %v", err)
	}
	return applied
}

func object(t *testing.T, db *store.DB, label string) (store.Object, map[string]any) {
	t.Helper()
	obj, err := db.GetObject(context.Background(), "p1", label)
	if err != nil {
		t.Fatalf("GetObject %s: %v", label, err)
	}
	var b map[string]any
	_ = json.Unmarshal(obj.Body, &b)
	return obj, b
}

func requireRejected(t *testing.T, applied cycle.ApplyResult, want string) {
	t.Helper()
	if applied.Applied {
		t.Fatalf("step applied, want rejected with %q", want)
	}
	if !strings.Contains(strings.Join(applied.Errors, "\n"), want) {
		t.Fatalf("errors: %v, want one containing %q", applied.Errors, want)
	}
}

func edgeTypes(t *testing.T, db *store.DB, fromID string) map[store.EdgeType][]string {
	t.Helper()
	edges, err := db.EdgesFrom(context.Background(), "p1", fromID)
	if err != nil {
		t.Fatalf("EdgesFrom: %v", err)
	}
	out := map[store.EdgeType][]string{}
	for _, e := range edges {
		out[e.Type] = append(out[e.Type], e.ToID)
	}
	return out
}

func TestS3R2_AnchorThatDoesNotAppearIsRejectedWithItsCount(t *testing.T) {
	db := seedStepProject(t)
	requireRejected(t, applyRecorded(t, db, stepFile(t, "anchor-missing")), "the anchor appears 0 times in sec-1 version 1")
	if events := eventsOfType(t, db, store.EventStepEnded); len(events) != 0 {
		t.Fatalf("a rejected step wrote %d step_ended", len(events))
	}
}

func TestS3R2_RepeatedAnchorIsRejectedWithItsCount(t *testing.T) {
	db := seedStepProject(t)
	requireRejected(t, applyRecorded(t, db, stepFile(t, "anchor-repeated")), "the anchor appears 2 times in sec-1 version 1")
}

func TestS3R4_NewNInTheTextIsRejected(t *testing.T) {
	db := seedStepProject(t)
	requireRejected(t, applyRecorded(t, db, stepFile(t, "new-n-in-text")), "new-1 is not valid inside the text")
}

func TestS3R6_SupersedesPointingToAHumanDecisionIsRejected(t *testing.T) {
	db := seedStepProject(t)
	requireRejected(t, applyRecorded(t, db, stepFile(t, "supersedes-human")), "dec-1 is a decision of the human")
	if obj, _ := object(t, db, "dec-1"); obj.State != "human" {
		t.Fatalf("dec-1 state: %s, want human (nothing applied)", obj.State)
	}
}

func TestS3R6_SupersedingItsOwnDecisionLeavesTheOldOneSuperseded(t *testing.T) {
	db := seedStepProject(t)
	applied := applyRecorded(t, db, stepFile(t, "supersedes-agent"))
	if !applied.Applied || applied.Labels["new-1"] != "dec-3" {
		t.Fatalf("applied: %+v", applied)
	}
	if old, _ := object(t, db, "dec-2"); old.State != "superseded" {
		t.Fatalf("dec-2 state: %s, want superseded", old.State)
	}
	newDec, _ := object(t, db, "dec-3")
	old, _ := object(t, db, "dec-2")
	if got := edgeTypes(t, db, newDec.ID)[store.EdgeSupersedes]; len(got) != 1 || got[0] != old.ID {
		t.Fatalf("supersedes edge: %v", got)
	}
}

// A-3, B-15, B-20 (b): a figure with its link creates covers; a number
// literal in a used source becomes a datum linked by comes_from; a loose
// number leaves the pending «cifra sin decisión».
func TestA3_FigureLinkCreatesCoversDatumComesFromAndLooseNumberIsPending(t *testing.T) {
	db := seedStepProject(t)
	applied := applyRecorded(t, db, stepFile(t, "figure-datum-loose"))
	if !applied.Applied {
		t.Fatalf("not applied: %v", applied.Errors)
	}
	sec, b := object(t, db, "sec-1")
	if !strings.Contains(b["text"].(string), "Cierre. La comparación da 60 veces en 2023 y deja 7 casos.") {
		t.Fatalf("section text: %q", b["text"])
	}
	dec, _ := object(t, db, applied.Labels["new-1"])
	fig, fb := object(t, db, "fig-1")
	if fb["anchor"] != "60 veces" || fb["value"] != "60" || fb["section_id"] != sec.ID {
		t.Fatalf("figure body: %v", fb)
	}
	if got := edgeTypes(t, db, fig.ID)[store.EdgeCovers]; len(got) != 1 || got[0] != dec.ID {
		t.Fatalf("covers edge: %v", got)
	}
	if got := edgeTypes(t, db, dec.ID)[store.EdgeUsedIn]; len(got) != 1 || got[0] != sec.ID {
		t.Fatalf("used_in edges of the decision: %v, want only sec-1 once", got)
	}
	src, _ := object(t, db, "src-1")
	dat, db2 := object(t, db, "dat-1")
	if db2["value"] != "2023" {
		t.Fatalf("datum body: %v", db2)
	}
	if got := edgeTypes(t, db, dat.ID)[store.EdgeComesFrom]; len(got) != 1 || got[0] != src.ID {
		t.Fatalf("datum comes_from: %v", got)
	}
	if got := edgeTypes(t, db, sec.ID)[store.EdgeUses]; len(got) != 1 || got[0] != src.ID {
		t.Fatalf("uses edge: %v", got)
	}
	pending, err := graph.FiguresWithoutDecision(context.Background(), db, "p1", sec.ID)
	if err != nil || len(pending) != 1 || pending[0] != "7" {
		t.Fatalf("figures without decision: %v, %v; want [7]", pending, err)
	}
}

// Fichas §3, glossary §9: a citation whose passage is literal in its
// source gets comes_from; one whose passage is not is created without
// it and leaves «fuente que no sostiene». Neither rejects the step.
func TestA2_CitationNotLiteralInItsSourceHasNoComesFromAndIsPending(t *testing.T) {
	db := seedStepProject(t)
	applied := applyRecorded(t, db, stepFile(t, "citations"))
	if !applied.Applied {
		t.Fatalf("not applied: %v", applied.Errors)
	}
	src, _ := object(t, db, "src-1")
	good, _ := object(t, db, "cit-1")
	bad, _ := object(t, db, "cit-2")
	if got := edgeTypes(t, db, good.ID)[store.EdgeComesFrom]; len(got) != 1 || got[0] != src.ID {
		t.Fatalf("cit-1 comes_from: %v", got)
	}
	if got := edgeTypes(t, db, bad.ID)[store.EdgeComesFrom]; len(got) != 0 {
		t.Fatalf("cit-2 comes_from: %v, want none", got)
	}
	sec, _ := object(t, db, "sec-1")
	pending, err := graph.SourcesNotSupporting(context.Background(), db, "p1", sec.ID)
	if err != nil || len(pending) != 1 || pending[0] != "cit-2" {
		t.Fatalf("sources not supporting: %v, %v; want [cit-2]", pending, err)
	}
}

func TestB11_StepOnAChangedSectionIsRejected(t *testing.T) {
	db := seedStepProject(t)
	requireRejected(t, applyRecorded(t, db, stepFile(t, "stale")), "sec-1 changed since the version you read")
}

// T-4 (2), T-3, B-17: several texts of one section go back to front; each
// new-n gets its label, recorded in step_ended with what slice 1 does not
// apply.
func TestT4_TwoEditsApplyBackToFrontAndStepEndedCarriesLabelsAndNotApplied(t *testing.T) {
	db := seedStepProject(t)
	applied := applyRecorded(t, db, stepFile(t, "two-edits"))
	if !applied.Applied || !applied.Work {
		t.Fatalf("applied: %+v", applied)
	}
	if _, b := object(t, db, "sec-1"); b["text"] != "El resumen compara estudios. Los estudios usan filtros. Fin." {
		t.Fatalf("section text: %q", b["text"])
	}
	if applied.Labels["new-1"] != "sec-2" || applied.Labels["new-2"] != "dec-3" {
		t.Fatalf("labels: %v", applied.Labels)
	}
	newSec, nb := object(t, db, "sec-2")
	if nb["title"] != "Conclusiones" || nb["role"].(map[string]any)["state"] != "proposed" {
		t.Fatalf("new section body: %v", nb)
	}
	dec, _ := object(t, db, "dec-3")
	if got := edgeTypes(t, db, dec.ID)[store.EdgeUsedIn]; len(got) != 1 || got[0] != newSec.ID {
		t.Fatalf("used_in to the new section: %v", got)
	}
	if oqs, _ := db.ListObjects(context.Background(), "p1", store.ObjectOpenQuestion); len(oqs) != 0 {
		t.Fatalf("open questions created: %d, want 0 (slice 1 does not apply them)", len(oqs))
	}
	ended := eventsOfType(t, db, store.EventStepEnded)
	if len(ended) != 1 {
		t.Fatalf("step_ended: %d, want 1", len(ended))
	}
	data := eventData(t, ended[0])
	labels, _ := data["labels"].(map[string]any)
	notApplied, _ := data["not_applied"].(map[string]any)
	if labels["new-1"] != "sec-2" || labels["new-2"] != "dec-3" || notApplied["proposed_open_questions"] == nil {
		t.Fatalf("step_ended data: %v", data)
	}
}

func TestS3R3_InvisibleCharactersAndRawHTMLAreRejected(t *testing.T) {
	db := seedStepProject(t)
	out := strings.Replace(string(stepFile(t, "stale")), `"base_version": 0`, `"base_version": 1`, 1)
	invisible := strings.Replace(out, `"Fin."`, `"Fin​."`, 1)
	requireRejected(t, applyRecorded(t, db, json.RawMessage(invisible)), "invisible characters")
	html := strings.Replace(out, `"Fin."`, `"<b>Fin.</b>"`, 1)
	requireRejected(t, applyRecorded(t, db, json.RawMessage(html)), "raw HTML")
}

func TestS3R5_DecisionWithBlankContrastIsRejected(t *testing.T) {
	db := seedStepProject(t)
	out := strings.Replace(string(stepFile(t, "supersedes-agent")), `"Comparar por conteos crudos."`, `"   "`, 1)
	requireRejected(t, applyRecorded(t, db, json.RawMessage(out)), "needs its complete contrast")
}

func TestB18_NewSectionWithoutTitleIsRejected(t *testing.T) {
	db := seedStepProject(t)
	out := strings.Replace(string(stepFile(t, "two-edits")), `"title": "Conclusiones"`, `"title": ""`, 1)
	requireRejected(t, applyRecorded(t, db, json.RawMessage(out)), "a new section needs its title")
}

// P-32: the agent never turns a confirmed role back to proposed; the
// proposal stays in step_ended as not applied (like B-17).
func TestP32_AgentProposalDoesNotChangeAConfirmedRole(t *testing.T) {
	db := seedStepProject(t)
	applied := applyRecorded(t, db, stepFile(t, "role-on-confirmed"))
	if !applied.Applied {
		t.Fatalf("not applied: %v", applied.Errors)
	}
	sec, b := object(t, db, "sec-1")
	role := b["role"].(map[string]any)
	if role["value"] != "other" || role["state"] != "confirmed" || sec.CurrentVersion != 1 {
		t.Fatalf("sec-1 role: %v (version %d), want other confirmed, unchanged", role, sec.CurrentVersion)
	}
	if n := len(eventsOfType(t, db, store.EventRoleChanged)); n != 0 {
		t.Fatalf("role_changed events: %d, want 0", n)
	}
	ended := eventsOfType(t, db, store.EventStepEnded)
	notApplied, _ := eventData(t, ended[len(ended)-1])["not_applied"].(map[string]any)
	roles, _ := notApplied["proposed_roles"].([]any)
	if len(roles) != 1 || roles[0].(map[string]any)["section_id"] != "sec-1" || roles[0].(map[string]any)["role"] != "conclusion" {
		t.Fatalf("not_applied: %v", notApplied)
	}
}

// lockedStep writes in sec-1 (with a figure in that text) and in a new
// section, with a new decision used there (B-14).
const lockedStep = `{
  "triage_line": "Construir: dos secciones.", "closing_risk": "none", "note": "Paso de prueba.",
  "new_text": [
    {"section_id": "sec-1", "base_version": 1, "anchor": "Cierre.", "operation": "replace", "text": "Cierre con 1,5 veces."},
    {"section_id": "new-1", "base_version": 0, "anchor": "", "operation": "insert_after", "text": "Texto de la otra sección."}
  ],
  "proposed_roles": [{"section_id": "new-1", "role": "other", "title": "Otra"}],
  "new_decisions": [{"id": "new-2", "statement": "Contar por veces.", "instead_of": "Contar crudo.",
    "would_choose_if": "Todos midieran igual.", "assumption": false, "used_in_sections": ["new-1"]}],
  "figure_links": [{"figure_anchor": "1,5 veces", "decision_id": "new-2"}]
}`

func requireOnlyLockedTextRejected(t *testing.T, db *store.DB, applied cycle.ApplyResult, version int) {
	t.Helper()
	if !applied.Applied {
		t.Fatalf("the rest of the step must apply: %v", applied.Errors)
	}
	if len(applied.Rejected) != 1 || !strings.Contains(applied.Rejected[0], "sec-1") {
		t.Fatalf("rejected: %v, want one message for the agent naming sec-1", applied.Rejected)
	}
	sec, b := object(t, db, "sec-1")
	if sec.CurrentVersion != version || strings.Contains(b["text"].(string), "1,5") {
		t.Fatalf("sec-1 changed: version %d, %q", sec.CurrentVersion, b["text"])
	}
	other, ob := object(t, db, applied.Labels["new-1"])
	if other.Type != store.ObjectSection || ob["text"] != "Texto de la otra sección." {
		t.Fatalf("the other section: %+v %v", other, ob)
	}
	if _, err := db.GetObject(context.Background(), "p1", applied.Labels["new-2"]); err != nil {
		t.Fatalf("the new decision must apply: %v", err)
	}
	if figures, _ := db.ListObjects(context.Background(), "p1", store.ObjectFigure); len(figures) != 0 {
		t.Fatalf("the figure of the rejected text must not apply: %d", len(figures))
	}
	ended := eventsOfType(t, db, store.EventStepEnded)
	notApplied, _ := eventData(t, ended[len(ended)-1])["not_applied"].(map[string]any)
	if texts, _ := notApplied["new_text"].([]any); len(texts) != 1 {
		t.Fatalf("not_applied: %v, want the rejected new_text", notApplied)
	}
}

// B-14: an agent step that touches a section in signoff: only that
// new_text is rejected, with a message for the agent; the rest applies.
func TestB14_StepOnASectionInSignoffRejectsOnlyThatText(t *testing.T) {
	db := seedStepProject(t)
	if open, err := cycle.NewGate(db).RequestSignoff(context.Background(), "p1", "sec-1"); err != nil || !open.Opened {
		t.Fatalf("RequestSignoff: %+v, %v", open, err)
	}
	requireOnlyLockedTextRejected(t, db, applyRecorded(t, db, json.RawMessage(lockedStep)), 1)
}

// B-14: the same for a signed section (until slice 4).
func TestB14_StepOnASignedSectionRejectsOnlyThatText(t *testing.T) {
	db := seedStepProject(t)
	err := db.Write(context.Background(), "p1", func(tx *store.Tx) error {
		sec, err := tx.GetObject(context.Background(), "p1", "sec-1")
		if err != nil {
			return err
		}
		var b map[string]any
		_ = json.Unmarshal(sec.Body, &b)
		_, err = tx.UpdateObject(sec.ID, sec.CurrentVersion, "signed", b, store.EventInput{Type: store.EventStateChanged, Actor: "human"})
		return err
	})
	if err != nil {
		t.Fatalf("sign sec-1: %v", err)
	}
	// The section is at version 2 now; the step reads it there.
	requireOnlyLockedTextRejected(t, db,
		applyRecorded(t, db, json.RawMessage(strings.Replace(lockedStep, `"base_version": 1`, `"base_version": 2`, 1))), 2)
}

// The previous behavior holds: without a signoff the same step writes
// sec-1 and its figure.
func TestB14_StepOnASectionNotInSignoffStillApplies(t *testing.T) {
	db := seedStepProject(t)
	applied := applyRecorded(t, db, json.RawMessage(lockedStep))
	if !applied.Applied || len(applied.Rejected) != 0 {
		t.Fatalf("got %+v, want applied with nothing rejected", applied)
	}
	if _, b := object(t, db, "sec-1"); !strings.Contains(b["text"].(string), "1,5 veces") {
		t.Fatalf("sec-1: %q", b["text"])
	}
	if figures, _ := db.ListObjects(context.Background(), "p1", store.ObjectFigure); len(figures) != 1 {
		t.Fatalf("figures: %d, want 1", len(figures))
	}
}

// P-40: a section in changed_since_signoff rejects the agent's text like
// a signed one (estados §1: until the new signature, steps arrive as
// proposals, slice 4).
func TestP40_StepOnAChangedSinceSignoffSectionRejectsOnlyThatText(t *testing.T) {
	db := seedStepProject(t)
	err := db.Write(context.Background(), "p1", func(tx *store.Tx) error {
		sec, err := tx.GetObject(context.Background(), "p1", "sec-1")
		if err != nil {
			return err
		}
		var b map[string]any
		_ = json.Unmarshal(sec.Body, &b)
		_, err = tx.UpdateObject(sec.ID, sec.CurrentVersion, "changed_since_signoff", b, store.EventInput{Type: store.EventStateChanged, Actor: "human"})
		return err
	})
	if err != nil {
		t.Fatalf("change sec-1: %v", err)
	}
	requireOnlyLockedTextRejected(t, db,
		applyRecorded(t, db, json.RawMessage(strings.Replace(lockedStep, `"base_version": 1`, `"base_version": 2`, 1))), 2)
}

// P-46: a text inserted with insert_after is its own paragraph: it is
// separated from the text before and after it by a blank line, never
// glued («…equipos.Se consideran…»).
func TestP46_InsertedTextIsASeparateParagraph(t *testing.T) {
	db := seedStepProject(t)
	applied := applyRecorded(t, db, json.RawMessage(`{"triage_line":"Escribo.","closing_risk":"none","note":"Dos párrafos.",
		"new_text":[
		 {"section_id":"sec-1","base_version":1,"anchor":"","operation":"insert_after","text":"Primero el contexto."},
		 {"section_id":"sec-1","base_version":1,"anchor":"Los estudios usan filtros.","operation":"insert_after","text":"Se consideran dos filtros."}]}`))
	if !applied.Applied {
		t.Fatalf("ApplyStep: %+v", applied)
	}
	_, body := object(t, db, "sec-1")
	want := "Primero el contexto.\n\nLa síntesis compara estudios. Los estudios usan filtros.\n\nSe consideran dos filtros.\n\nCierre."
	if body["text"] != want {
		t.Fatalf("text:\n%q\nwant\n%q", body["text"], want)
	}
}
