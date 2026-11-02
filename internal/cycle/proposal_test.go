package cycle_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/LaurenteEber/tupaqnin/internal/cycle"
	"github.com/LaurenteEber/tupaqnin/internal/memory"
	"github.com/LaurenteEber/tupaqnin/internal/provider"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// «el principal recibe en la misma conversación que su propuesta quedó
// abierta» (backlog-r4 4.1): in «lo que cambió», with its state, and no
// internal id reaches it.
func TestB73_ThePrincipalHearsInTheSameConversationThatItsProposalIsOpen(t *testing.T) {
	onSigning := stepFixture("on-signing", `{"stretch": "build", "triage_line": "Construir.", "closing_risk": "none", "note": "n",
	  "new_text": [{"section_id": "sec-1", "base_version": 1, "anchor": "Así la comparación es justa.", "claim_origin": "material", "operation": "replace", "text": "Así es justa."},
	    {"section_id": "new-1", "base_version": 0, "anchor": "", "claim_origin": "material", "operation": "insert_after", "text": "Revisamos 12 estudios."}],
	  "proposed_roles": [{"section_id": "new-1", "role": "other", "title": "Muestra"}]}`)
	db, runner, cap, _ := newCaptureRunner(t, onSigning, loadAppFixture(t, "04-principal-no-work.json"))
	seedSignable(t, db, true)
	openSignoff(t, cycle.NewGate(db))
	if err := runner.RunTurn(context.Background(), "p1", "turn-1", "Mejora la sección."); err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	if len(cap.sent) != 2 {
		t.Fatalf("provider calls: %d, want 2", len(cap.sent))
	}
	raw, _ := json.Marshal(cap.sent[1])
	sent := string(raw)
	if !strings.Contains(sent, "prop-1") || !strings.Contains(sent, "open") || !strings.Contains(sent, "Así es justa.") {
		t.Fatalf("the second step must say prop-1 is open: %s", sent)
	}
	props, _ := db.ListObjects(context.Background(), "p1", store.ObjectProposal)
	if len(props) != 1 {
		t.Fatalf("proposals: %d", len(props))
	}
	sec, _ := db.GetObject(context.Background(), "p1", "sec-1")
	if strings.Contains(sent, sec.ID) {
		t.Fatalf("an internal id reached the model: %s", sent)
	}
}

// B-75: «pedir un paso nuevo»: with no turn running the app opens one, and
// its step carries the notice. Accepting itself calls no model.
func TestB75_TheNewStepOpensATurnThatCarriesTheNotice(t *testing.T) {
	db, runner, cap, _ := newCaptureRunner(t, stepFixture("on-signing", onSigned), loadAppFixture(t, "04-principal-no-work.json"))
	seedSignable(t, db, false)
	label := proposeThenRepeatAnchor(t, db, runner)
	calls := len(cap.sent)
	out, err := cycle.NewGate(db).AcceptProposal(context.Background(), "p1", label)
	if err != nil || !out.NewStep || out.State != "discarded" {
		t.Fatalf("AcceptProposal: %+v, %v", out, err)
	}
	if len(cap.sent) != calls {
		t.Fatalf("accepting called a model: %d calls", len(cap.sent)-calls)
	}
	turn, err := runner.StartStepAfterProposal("p1")
	if err != nil || turn == "" {
		t.Fatalf("StartStepAfterProposal: %q, %v", turn, err)
	}
	cycle.WaitAfterAnswersForTest(runner)
	if len(cap.sent) != calls+1 {
		t.Fatalf("provider calls: %d, want one more than %d", len(cap.sent), calls)
	}
	if n := noticesIn(t, cap.sent[calls]); n != 1 {
		t.Fatalf("the new step carries the notice %d times, want 1: %s", n, cap.sent[calls])
	}
}

// B-75: with a turn running no second one opens: the notice enters the next
// step.
func TestB75_WithATurnRunningTheNoticeWaitsForTheNextStep(t *testing.T) {
	db, runner, cap, _ := newCaptureRunner(t, stepFixture("on-signing", onSigned), loadAppFixture(t, "04-principal-no-work.json"))
	seedSignable(t, db, false)
	label := proposeThenRepeatAnchor(t, db, runner)
	if _, err := cycle.NewGate(db).AcceptProposal(context.Background(), "p1", label); err != nil {
		t.Fatalf("AcceptProposal: %v", err)
	}
	end := cycle.HoldTurnForTest(runner, "p1")
	turn, err := runner.StartStepAfterProposal("p1")
	end()
	if err != nil || turn != "" {
		t.Fatalf("StartStepAfterProposal with a turn running: %q, %v", turn, err)
	}
	if err := runner.RunTurn(context.Background(), "p1", "turn-2", "Sigue"); err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	if n := noticesIn(t, cap.sent[len(cap.sent)-1]); n != 1 {
		t.Fatalf("the next step carries the notice %d times, want 1", n)
	}
}

// onSigned writes on seedSignable's sec-1 after its text was edited by you.
const onSigned = `{"stretch": "build", "triage_line": "Construir.", "closing_risk": "none", "note": "n",
  "new_text": [{"section_id": "sec-1", "base_version": 1, "anchor": "Así la comparación es justa.", "claim_origin": "material", "operation": "replace", "text": "Así es justa."}]}`

// proposeThenRepeatAnchor leaves a proposal on sec-1 whose anchor you then
// repeat, and returns its label.
func proposeThenRepeatAnchor(t *testing.T, db *store.DB, runner *cycle.Runner) string {
	t.Helper()
	sec, err := db.GetObject(context.Background(), "p1", "sec-1")
	if err != nil {
		t.Fatal(err)
	}
	var b map[string]any
	_ = json.Unmarshal(sec.Body, &b)
	text, _ := b["text"].(string)
	b["text"] = text + " Así la comparación es justa."
	err = db.Write(context.Background(), "p1", func(tx *store.Tx) error {
		_, err := tx.UpdateObject(sec.ID, sec.CurrentVersion, sec.State, b, store.EventInput{Type: store.EventEdit, Actor: "human"})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := runner.RunTurn(context.Background(), "p1", "turn-1", "Mejora la sección."); err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	props, _ := db.ListObjects(context.Background(), "p1", store.ObjectProposal)
	if len(props) != 1 {
		t.Fatalf("proposals: %d, want 1", len(props))
	}
	return props[0].Label
}

// noticesIn counts B-75's notice in one call's input.
func noticesIn(t *testing.T, input []json.RawMessage) int {
	t.Helper()
	n := 0
	for _, item := range input {
		n += strings.Count(string(item), "Write a new step for sec-1")
	}
	return n
}

// beforeStep runs a function before each principal call, while the step that
// call belongs to is running.
type beforeStep struct {
	provider.Provider
	before func()
}

func (b *beforeStep) Respond(ctx context.Context, req provider.Request, onDelta func(provider.Delta)) (provider.Response, error) {
	if req.TurnKind == provider.TurnStep && b.before != nil {
		b.before()
	}
	return b.Provider.Respond(ctx, req, onDelta)
}

// B-75: accepting what cannot apply while a step is at the model: that step
// ends after the discard, without having read it, and no turn opens; the next
// step still carries the notice, once, and the one after it does not.
func TestB75_ANoticeLeftDuringARunningStepReachesTheNextStep(t *testing.T) {
	work := stepFixture("work", `{"stretch": "build", "triage_line": "C.", "closing_risk": "none", "note": "n",
	  "deferred": [], "new_text": [{"section_id": "sec-1", "base_version": 2, "anchor": "", "claim_origin": "material", "operation": "insert_after", "text": "Otra cosa."}]}`)
	noWork := loadAppFixture(t, "04-principal-no-work.json")
	db, runner, cap, _ := newCaptureRunner(t, stepFixture("on-signing", onSigned), work, noWork, noWork)
	seedSignable(t, db, false)
	label := proposeThenRepeatAnchor(t, db, runner)
	calls := len(cap.sent)
	inner, accepted := cap.inner, false
	cap.inner = &beforeStep{Provider: inner, before: func() {
		if accepted {
			return
		}
		accepted = true
		out, err := cycle.NewGate(db).AcceptProposal(context.Background(), "p1", label)
		if err != nil || !out.NewStep {
			t.Errorf("AcceptProposal: %+v, %v", out, err)
		}
		if turn, err := runner.StartStepAfterProposal("p1"); err != nil || turn != "" {
			t.Errorf("no second turn opens while one runs: %q, %v", turn, err)
		}
	}}
	if err := runner.RunTurn(context.Background(), "p1", "turn-2", "Sigue"); err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	if len(cap.sent) < calls+2 {
		t.Fatalf("provider calls: %d, want at least two more than %d", len(cap.sent), calls)
	}
	if n := noticesIn(t, cap.sent[calls]); n != 0 {
		t.Fatalf("the running step was sent before the discard: %d notices", n)
	}
	if n := noticesIn(t, cap.sent[calls+1]); n != 1 {
		t.Fatalf("the next step carries the notice %d times, want 1", n)
	}
	if err := runner.RunTurn(context.Background(), "p1", "turn-3", "Otra"); err != nil {
		t.Fatalf("RunTurn 3: %v", err)
	}
	last := cap.sent[len(cap.sent)-1]
	if got := lastUserText(t, last); strings.Contains(got, "Write a new step for sec-1") {
		t.Fatalf("the notice repeats in the next turn: %q", got)
	}
}

// B-75: after a new startup, which marks everything delivered, the notice
// still reaches the first step, once.
func TestB75_TheNoticeReachesANewStartupOnce(t *testing.T) {
	noWork := loadAppFixture(t, "04-principal-no-work.json")
	db, runner, cap, _ := newCaptureRunner(t, stepFixture("on-signing", onSigned), noWork, noWork)
	seedSignable(t, db, false)
	label := proposeThenRepeatAnchor(t, db, runner)
	if _, err := cycle.NewGate(db).AcceptProposal(context.Background(), "p1", label); err != nil {
		t.Fatalf("AcceptProposal: %v", err)
	}
	later := time.Now().Add(3 * time.Hour)
	runner.Now = func() time.Time { return later }
	calls := len(cap.sent)
	if err := runner.RunTurn(context.Background(), "p1", "turn-2", "Sigue"); err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	if n := noticesIn(t, cap.sent[calls]); n != 1 {
		t.Fatalf("the startup's step carries the notice %d times, want 1", n)
	}
}

// B-76, with Wonder's rounds: a full signoff discards the open proposal too.
func TestB76_AFullSignoffDiscardsTheOpenProposal(t *testing.T) {
	db := openTemp(t)
	seedSignable(t, db, true)
	g := cycle.NewGate(db)
	ctx := context.Background()
	sign := openSignoff(t, g)
	applied, err := g.ApplyStep(ctx, "p1", "turn-x", json.RawMessage(onSigned))
	if err != nil || len(applied.Proposals) != 1 {
		t.Fatalf("ApplyStep: %+v, %v", applied, err)
	}
	if err := g.RecordExplanation(ctx, "p1", sign, "Comparo solo estudios con el mismo tamaño porque si no la diferencia es del filtro."); err != nil {
		t.Fatal(err)
	}
	round(t, g, sign, "round1-all-match")
	if err := g.RecordAnswers(ctx, "p1", sign, "Supongo que todos cuentan desde el mismo tamaño mínimo.", ""); err != nil {
		t.Fatal(err)
	}
	round(t, g, sign, "round2-all-match-answers")
	if err := g.Sign(ctx, "p1", sign); err != nil {
		t.Fatalf("Sign: %v", err)
	}
	prop, err := db.GetObject(ctx, "p1", applied.Proposals[0])
	if err != nil {
		t.Fatal(err)
	}
	if prop.State != "discarded" || !strings.Contains(string(prop.Body), "signed_without") {
		t.Fatalf("proposal: %s %s", prop.State, prop.Body)
	}
}

// Backlog-r4 4.1: what the startup lists of a proposal is a whole line; its
// text comes only when the object is read.
func TestB74_TheChangesLineOfAProposalDoesNotDangle(t *testing.T) {
	db := seedStepProject(t)
	if _, err := cycle.ConfirmPlan(context.Background(), db, "p1"); err != nil {
		t.Fatal(err)
	}
	applied := applyRecorded(t, db, json.RawMessage(lockedStep))
	changes, err := memory.New(db).ChangesSinceDelivery(context.Background(), "p1")
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, c := range changes {
		if c.Label == applied.Proposals[0] {
			found = true
			if strings.Contains(c.FirstLine, "Its text") {
				t.Fatalf("line: %q", c.FirstLine)
			}
		}
	}
	if !found {
		t.Fatalf("changes: %+v", changes)
	}
}
