package cycle_test

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/LaurenteEber/tupaqnin/internal/cycle"
	"github.com/LaurenteEber/tupaqnin/internal/graph"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

const (
	ownStatement = "Usar los conteos de 2024, no la mediana de los cinco años."
	ownReason    = "Porque el comité solo aceptó la serie de 2024."
)

// objectedProject has dec-3, the agent's decision behind sec-1's figure, objected.
func objectedProject(t *testing.T) (*store.DB, string) {
	t.Helper()
	db := cleanCardProject(t)
	closeStretch(t, db)
	objected, err := cycle.NewGate(db).ObjectDecision(context.Background(), "p1", "dec-3", "Porque quiero revisar esta decisión.")
	if err != nil {
		t.Fatal(err)
	}
	return db, objected.OpenQuestion
}

func decisionCount(t *testing.T, db *store.DB) int {
	t.Helper()
	list, err := db.ListObjects(context.Background(), "p1", store.ObjectDecision)
	if err != nil {
		t.Fatal(err)
	}
	return len(list)
}

// B-107, Listo cuando: your own option creates a decision of yours, versioned, that
// replaces the objected one, with your text literal.
func TestB107_YourOptionCreatesYourDecisionReplacingTheObjectedOne(t *testing.T) {
	db, question := objectedProject(t)
	ctx := context.Background()
	old, oldBody := object(t, db, "dec-3")
	answer, err := cycle.NewGate(db).AnswerObjectionWithYours(ctx, "p1", question, ownStatement, ownReason)
	if err != nil || answer.NewDecision == "" {
		t.Fatalf("answer: %+v %v", answer, err)
	}
	born, body := object(t, db, answer.NewDecision)
	if born.State != "human" || body["statement"] != ownStatement || body["reason"] != ownReason ||
		body["instead_of"] != oldBody["statement"] || body["origin"] != "human" {
		t.Fatalf("born: %s %v", born.State, body)
	}
	if replaced, _ := object(t, db, "dec-3"); replaced.State != "superseded" || replaced.CurrentVersion <= old.CurrentVersion {
		t.Fatalf("the objected one: %s v%d", replaced.State, replaced.CurrentVersion)
	}
	edges, err := db.EdgesFrom(ctx, "p1", born.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.ContainsFunc(edges, func(e store.Edge) bool { return e.Type == store.EdgeSupersedes && e.ToID == old.ID }) {
		t.Fatalf("no supersedes edge: %+v", edges)
	}
	q, qBody := object(t, db, question)
	options, _ := qBody["options"].([]any)
	if q.State != "resolved" || len(options) != 3 || qBody["chosen_option"] != float64(2) {
		t.Fatalf("the question: %s %v", q.State, qBody)
	}
	if chosen, _ := options[2].(map[string]any); chosen["option"] != ownStatement {
		t.Fatalf("the chosen option: %v", chosen)
	}
}

// The invariants of a replacement: the figure it covered moves to yours, and the
// section it reached gets its open change and waits for B-100's step.
func TestB107_TheReplacementKeepsItsInvariants(t *testing.T) {
	db, question := objectedProject(t)
	ctx := context.Background()
	answer, err := cycle.NewGate(db).AnswerObjectionWithYours(ctx, "p1", question, ownStatement, ownReason)
	if err != nil {
		t.Fatal(err)
	}
	born, _ := object(t, db, answer.NewDecision)
	in, err := db.EdgesTo(ctx, "p1", born.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.ContainsFunc(in, func(e store.Edge) bool { return e.Type == store.EdgeCovers }) {
		t.Fatalf("the figure did not move: %+v", in)
	}
	sec, _ := object(t, db, "sec-1")
	if len(graph.OpenChanges(sec)) == 0 {
		t.Fatal("no open change on the section")
	}
	left, err := graph.UnappliedDecisions(ctx, db, "p1")
	if err != nil || len(left) != 1 || left[0].Text != ownStatement || left[0].Reason != ownReason || left[0].Decision != answer.NewDecision {
		t.Fatalf("unapplied: %+v %v", left, err)
	}
}

// An empty or assent «porque» is refused, as today, and nothing changes.
func TestB107_AnEmptyOrOkReasonIsRefusedAndNothingChanges(t *testing.T) {
	db, question := objectedProject(t)
	before := decisionCount(t, db)
	for _, reason := range []string{"", "  ", "ok", "Ok, dale."} {
		if _, err := cycle.NewGate(db).AnswerObjectionWithYours(context.Background(), "p1", question, ownStatement, reason); !errors.Is(err, cycle.ErrInvalidReason) {
			t.Fatalf("%q: %v", reason, err)
		}
	}
	if len(eventsOfType(t, db, store.EventAnswerWithoutReason)) != 4 {
		t.Fatal("answer_without_reason is not recorded, as today")
	}
	q, body := object(t, db, question)
	dec, _ := object(t, db, "dec-3")
	if q.State != "open" || dec.State != "in_open_question" || decisionCount(t, db) != before || len(body["options"].([]any)) != 2 {
		t.Fatalf("something changed: %s %s", q.State, dec.State)
	}
}

func TestB107_AnEmptyStatementIsRefusedAndNothingChanges(t *testing.T) {
	db, question := objectedProject(t)
	before := decisionCount(t, db)
	for _, statement := range []string{"", " \n "} {
		if _, err := cycle.NewGate(db).AnswerObjectionWithYours(context.Background(), "p1", question, statement, ownReason); !errors.Is(err, cycle.ErrEmptyStatement) {
			t.Fatalf("%q: %v", statement, err)
		}
	}
	q, _ := object(t, db, question)
	dec, _ := object(t, db, "dec-3")
	if q.State != "open" || dec.State != "in_open_question" || decisionCount(t, db) != before {
		t.Fatalf("something changed: %s %s", q.State, dec.State)
	}
}

// Only an objection's question offers it.
func TestB107_AnOrdinaryQuestionRefusesIt(t *testing.T) {
	db := cleanCardProject(t)
	ctx := context.Background()
	dec, _ := object(t, db, "dec-2")
	var label string
	if err := db.Write(ctx, "p1", func(tx *store.Tx) error {
		q, err := tx.CreateObject(store.ObjectOpenQuestion, "open", map[string]any{"question": "¿Sigue valiendo?",
			"options": []any{map[string]any{"option": "Mantener", "keep": true}, map[string]any{"option": "Cambiarla"}},
			"keeps":   map[string]any{"kind": "decision", "id": dec.ID}},
			store.EventInput{Type: store.EventStateChanged, Actor: "agent"})
		label = q.Label
		return err
	}); err != nil {
		t.Fatal(err)
	}
	seen, err := db.EventsSince(ctx, "p1", 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := cycle.NewGate(db).AnswerObjectionWithYours(ctx, "p1", label, ownStatement, ownReason); !errors.Is(err, cycle.ErrOwnOptionNotOffered) {
		t.Fatalf("answer: %v", err)
	}
	after, err := db.EventsSince(ctx, "p1", 0)
	if err != nil {
		t.Fatal(err)
	}
	q, _ := object(t, db, label)
	if q.State != "open" || len(after) != len(seen) {
		t.Fatalf("%s, events %d → %d", q.State, len(seen), len(after))
	}
}

// The section's text gets B-100's one step, with your words literal in its task, and
// the next report's «decided» lists it (C.9).
func TestB107_TheSectionGetsItsStepAndTheReportListsYourDecision(t *testing.T) {
	db, question := objectedProject(t)
	sec, _ := object(t, db, "sec-1")
	step := strings.Replace(editSection(t, db, "Aplico tu decisión.", "Cierre.", "Cierre con tu decisión."),
		`"base_version":`+jsonInt(sec.CurrentVersion), `"base_version":`+jsonInt(sec.CurrentVersion+1), 1)
	step = strings.Replace(step, `"note":"Paso."`, `"note":"Paso.","handled_changes":[{"change_id":"chg-1","result":"updated"}]`, 1)
	runner, cap, _ := modeRunnerOn(t, db, nil, textFixture(t, step), loadAppFixture(t, "04-principal-no-work.json"))
	answer, err := runner.AnswerObjectionWithYours(context.Background(), "p1", question, ownStatement, ownReason)
	if err != nil || answer.TurnID == "" {
		t.Fatalf("answer: %+v %v, want the one step", answer, err)
	}
	cycle.WaitTurnsForTest(runner)
	if len(cap.sent) != 1 {
		t.Fatalf("steps: %d, want one", len(cap.sent))
	}
	task := lastUserText(t, cap.sent[0])
	if !strings.Contains(task, ownStatement) || !strings.Contains(task, ownReason) || !strings.Contains(task, answer.NewDecision) {
		t.Fatalf("the task: %s", task)
	}
	if left, _ := graph.UnappliedDecisions(context.Background(), db, "p1"); len(left) != 0 {
		t.Fatalf("unapplied: %+v", left)
	}
	report := lastReport(t, db)
	if report.StopReason != cycle.StopDecisionApplied {
		t.Fatalf("reason: %s", report.StopReason)
	}
	if !slices.ContainsFunc(report.Decided, func(d graph.ReportDecided) bool {
		return d.Statement == ownStatement && d.Reason == ownReason && d.Who == "human" && len(d.Sections) > 0
	}) {
		t.Fatalf("decided: %+v", report.Decided)
	}
}

// «mantener» stays as it was: yours, with your reason, no decision born, no third option.
func TestB107_KeepIsUnchanged(t *testing.T) {
	db, question := objectedProject(t)
	before := decisionCount(t, db)
	keep := 0
	answer, err := cycle.NewGate(db).AnswerOpenQuestion(context.Background(), "p1", question, &keep, ownReason)
	if err != nil || answer.NewDecision != "" {
		t.Fatalf("answer: %+v %v", answer, err)
	}
	dec, body := object(t, db, "dec-3")
	_, q := object(t, db, question)
	if dec.State != "human" || body["reason"] != ownReason || decisionCount(t, db) != before ||
		len(q["options"].([]any)) != 2 || q["chosen_option"] != float64(0) {
		t.Fatalf("%s %v %v", dec.State, body, q)
	}
}

// «la otra» stays as it was: its statement is the previous «en vez de».
func TestB107_TheOtherIsUnchanged(t *testing.T) {
	db, question := objectedProject(t)
	_, old := object(t, db, "dec-3")
	other := 1
	answer, err := cycle.NewGate(db).AnswerOpenQuestion(context.Background(), "p1", question, &other, ownReason)
	if err != nil || answer.NewDecision == "" {
		t.Fatalf("answer: %+v %v", answer, err)
	}
	_, born := object(t, db, answer.NewDecision)
	_, q := object(t, db, question)
	if born["statement"] != old["instead_of"] || born["instead_of"] != old["statement"] ||
		len(q["options"].([]any)) != 2 || q["chosen_option"] != float64(1) {
		t.Fatalf("%v %v", born, q)
	}
}

// A signed section's replacement goes as elsewhere: the step leaves a proposal, and
// the text stays.
func TestB107_OnASignedSectionTheStepLeavesAProposal(t *testing.T) {
	db := cleanCardProject(t)
	setSectionSigned(t, db, "sec-1")
	ctx := context.Background()
	objected, err := cycle.NewGate(db).ObjectDecision(ctx, "p1", "dec-3", "Porque quiero revisar esta decisión.")
	if err != nil {
		t.Fatal(err)
	}
	sec, body := object(t, db, "sec-1")
	raw, err := json.Marshal(map[string]any{"stretch": "build", "triage_line": "Aplico tu decisión.", "closing_risk": "none", "note": "Propongo.",
		"new_text": []any{map[string]any{"section_id": "sec-1", "base_version": sec.CurrentVersion + 1, "anchor": body["text"], "operation": "replace",
			"claim_origin": "model_knowledge", "text": "El texto con tu decisión."}}})
	if err != nil {
		t.Fatal(err)
	}
	runner, _, _ := modeRunnerOn(t, db, nil, textFixture(t, string(raw)), loadAppFixture(t, "04-principal-no-work.json"))
	if _, err := runner.AnswerObjectionWithYours(ctx, "p1", objected.OpenQuestion, ownStatement, ownReason); err != nil {
		t.Fatal(err)
	}
	cycle.WaitTurnsForTest(runner)
	if _, after := object(t, db, "sec-1"); after["text"] != body["text"] {
		t.Fatalf("a signed section's text changed: %v", after["text"])
	}
	proposals := 0
	for _, ev := range eventsOfType(t, db, store.EventStateChanged) {
		var d map[string]any
		_ = json.Unmarshal(ev.Data, &d)
		if ev.Actor == "agent" && d["section"] == "sec-1" && d["to"] == "open" {
			proposals++
		}
	}
	if proposals != 1 {
		t.Fatalf("proposals: %d", proposals)
	}
}
