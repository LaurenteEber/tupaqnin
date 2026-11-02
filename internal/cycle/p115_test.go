package cycle_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/LaurenteEber/tupaqnin/internal/checks"
	"github.com/LaurenteEber/tupaqnin/internal/cycle"
	"github.com/LaurenteEber/tupaqnin/internal/graph"
	"github.com/LaurenteEber/tupaqnin/internal/provider"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// P-115: what the Chrome walkthrough with Meta found, and Eber's three answers.

// Item 2: an agent's write is a reason to refuse only over an edit of yours.
func TestB79_WithNoEditOfYoursUndoSaysSoEvenAfterTheAgentWrote(t *testing.T) {
	db := seedCheckerProject(t)
	ctx := context.Background()
	reason := func() string {
		t.Helper()
		sec, err := db.GetObject(ctx, "p1", "sec-1")
		if err != nil {
			t.Fatal(err)
		}
		undo, err := graph.SectionUndo(ctx, db, "p1", sec.ID)
		if err != nil {
			t.Fatal(err)
		}
		return undo.Reason
	}
	agentWrites := func(anchor, text string) {
		t.Helper()
		sec, _ := db.GetObject(ctx, "p1", "sec-1")
		step, _ := json.Marshal(map[string]any{"stretch": "build", "triage_line": "C.", "closing_risk": "none", "note": "n",
			"new_text": []map[string]any{{"section_id": "sec-1", "base_version": sec.CurrentVersion, "anchor": anchor,
				"claim_origin": "material", "operation": "replace", "text": text}}})
		if applied, err := cycle.NewGate(db).ApplyStep(ctx, "p1", "turn-x", step); err != nil || !applied.Applied {
			t.Fatalf("step: %+v, %v", applied, err)
		}
	}
	agentWrites("Cierre.", "Cierre uno.")
	if got := reason(); got != graph.UndoReasonNoEdit {
		t.Fatalf("no edit of yours, the agent wrote: %q", got)
	}
	// Your edit, the agent over it: now it is true, and it stays true.
	yourEdit(t, db, "sec-1", "Texto tuyo. Cierre uno.")
	agentWrites("Cierre uno.", "Cierre dos.")
	if got := reason(); got != graph.UndoReasonAgentWrote {
		t.Fatalf("the agent wrote over your edit: %q", got)
	}
	agentWrites("Cierre dos.", "Cierre tres.")
	if got := reason(); got != graph.UndoReasonAgentWrote {
		t.Fatalf("after a second write: %q", got)
	}
}

// Item 5: the question is the decision, literal; your reason goes apart.
func TestE1_AnObjectionsQuestionIsTheDecisionWithYourReasonApart(t *testing.T) {
	db := seedStepProject(t)
	ctx := context.Background()
	dec, err := db.GetObject(ctx, "p1", "dec-2")
	if err != nil {
		t.Fatal(err)
	}
	got, err := cycle.NewGate(db).ObjectDecision(ctx, "p1", "dec-2", objectReason)
	if err != nil {
		t.Fatal(err)
	}
	oq, err := db.GetObject(ctx, "p1", got.OpenQuestion)
	if err != nil {
		t.Fatal(err)
	}
	var body, decBody map[string]any
	_ = json.Unmarshal(oq.Body, &body)
	_ = json.Unmarshal(dec.Body, &decBody)
	if body["question"] != decBody["statement"] || body["objection_reason"] != objectReason {
		t.Fatalf("question %v, reason %v", body["question"], body["objection_reason"])
	}
}

// Item 6: the checker's brief lists the open «por aclarar» with the rule, and the
// recorded pass that respects it opens no other. With none open the brief is as it was.
func TestB46_TheCheckerGetsTheOpenQuestionsAndARepeatOpensNoOther(t *testing.T) {
	db := seedCheckerProject(t)
	ctx := context.Background()
	const asked = "¿El piloto incluye a los colegios rurales?"
	yourEdit(t, db, "sec-1", "El piloto cubre solo colegios urbanos.")
	before, err := checks.YourEditsToCheck(ctx, db, "p1")
	if err != nil {
		t.Fatal(err)
	}
	brief, err := checks.Brief(ctx, db, "p1", before, nil)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(brief, "Open questions already asked") {
		t.Fatalf("with none open the brief changed:\n%s", brief)
	}
	err = db.Write(ctx, "p1", func(tx *store.Tx) error {
		sec, err := tx.GetObject(ctx, "p1", "sec-1")
		if err != nil {
			return err
		}
		_, err = checks.Open(ctx, tx, checks.OpenQuestion{Question: asked, DependsOn: []string{sec.ID},
			Event: store.EventInput{Type: store.EventStateChanged, Actor: "agent"}})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	runner, cap := checkerRunner(t, db, checkerFixture(t, "no-findings"))
	t.Cleanup(runner.Close)
	if _, err := runner.RequestSignoff(ctx, "p1", "sec-1"); err != nil {
		t.Fatal(err)
	}
	if len(cap.checker) != 1 {
		t.Fatalf("checker calls: %d", len(cap.checker))
	}
	if text := briefOf(t, cap.checker[0]); !strings.Contains(text, "Open questions already asked") || !strings.Contains(text, asked) {
		t.Fatalf("the brief lacks the open question:\n%s", text)
	}
	if oqs, _ := db.ListObjects(ctx, "p1", store.ObjectOpenQuestion); len(oqs) != 1 {
		t.Fatalf("«por aclarar»: %d, want the one already open", len(oqs))
	}
}

// Answer a: at a cap the signoff opens, the text the checker did not see is listed
// with the agent's pendings at the cap, and the signature records it.
func TestB3_20_AtACapAnAcceptedTextIsListedUncheckedAndSigns(t *testing.T) {
	db := openTemp(t)
	seedSignable(t, db, false)
	ctx := context.Background()
	g := cycle.NewGate(db)
	label := proposeOnSigned(t, db)
	if _, err := g.AcceptProposal(ctx, "p1", label); err != nil {
		t.Fatal(err)
	}
	stopped := func(context.Context, string) (string, error) { return "project_cap", nil }
	g.StoppedBy = stopped
	open, err := g.RequestSignoff(ctx, "p1", "sec-1")
	if err != nil || !open.Opened {
		t.Fatalf("RequestSignoff: %+v, %v", open, err)
	}
	view, err := graph.SignoffDetailAtCap(ctx, db, "p1", open.Label, "project_cap")
	if err != nil {
		t.Fatal(err)
	}
	if !hasKind(view.PendingsAtCap, graph.PendingUnchecked) || hasKind(view.Pendings, graph.PendingUnchecked) {
		t.Fatalf("at cap %v, blocking %v", view.PendingsAtCap, view.Pendings)
	}
}

// Answer b: a proposal identical to one open or discarded is no work: three of them
// after your message's step end the mode, also with new_text in them.
func TestB83_AProposalIdenticalToAnOpenOneIsNoWorkForTheMode(t *testing.T) {
	db := seedStepProject(t)
	ctx := context.Background()
	setSectionSigned(t, db, "sec-1")
	addJudgedDraft(t, db, "Muestra", "Revisamos 12 estudios.") // keeps the agent a pending
	same := `{"stretch":"build","triage_line":"C.","closing_risk":"none","note":"n",` +
		`"new_text":[{"section_id":"sec-1","base_version":1,"anchor":"Cierre.","claim_origin":"material","operation":"replace","text":"Otro cierre."}]}`
	runner, cap, pub := modeRunnerOn(t, db, nil, textFixture(t, same))
	modeOn(t, db, "")
	if err := runner.RunTurn(ctx, "p1", "turn-1", "Sigue."); err != nil {
		t.Fatal(err)
	}
	if len(cap.sent) != 1+cycle.ModeMaxIdleSteps {
		t.Fatalf("steps: %d, want %d", len(cap.sent), 1+cycle.ModeMaxIdleSteps)
	}
	wantModeFinished(t, db, pub, "no_work", "")
}

// Answer b: what it touches: a proposal with another text is work.
func TestB83_AProposalWithAnotherTextIsWorkForTheMode(t *testing.T) {
	db := seedStepProject(t)
	ctx := context.Background()
	setSectionSigned(t, db, "sec-1")
	addJudgedDraft(t, db, "Muestra", "Revisamos 12 estudios.") // keeps the agent a pending
	step := func(text string) provider.Fixture {
		return textFixture(t, `{"stretch":"build","triage_line":"C.","closing_risk":"none","note":"n",`+
			`"new_text":[{"section_id":"sec-1","base_version":1,"anchor":"Cierre.","claim_origin":"material","operation":"replace","text":"`+text+`"}]}`)
	}
	same := step("Repetido.")
	fixtures := []provider.Fixture{step("Uno."), step("Dos."), step("Tres."), step("Cuatro."), same, same, same, same}
	runner, cap, _ := modeRunnerOn(t, db, nil, fixtures...)
	modeOn(t, db, "")
	if err := runner.RunTurn(ctx, "p1", "turn-1", "Sigue."); err != nil {
		t.Fatal(err)
	}
	// Four new, the first «Repetido» new too, then three repeats.
	if len(cap.sent) != 8 {
		t.Fatalf("steps: %d, want 8", len(cap.sent))
	}
}

// Answer c: the bar's fact: on with nothing running, over the threshold, during a
// signoff; not once the signoff closes, and not below the threshold.
func TestB86_TheModeSaysItWaitsForYourSignoff(t *testing.T) {
	db := openTemp(t)
	seedSignable(t, db, false)
	gate := cycle.NewGate(db)
	sign := openSignoff(t, gate)
	runner, _, _ := b86Runner(t, db, nil, using(textFixture(t, pendingFigureStep), 7000), b86NoWork(t, 100))
	t.Cleanup(runner.Close)
	ctx := context.Background()
	if waits, err := runner.ModeWaitsForSignoff(ctx, "p1"); err != nil || waits {
		t.Fatalf("before the mode: %v, %v", waits, err)
	}
	modeOn(t, db, "")
	if waits, err := runner.ModeWaitsForSignoff(ctx, "p1"); err != nil || waits {
		t.Fatalf("below the threshold: %v, %v", waits, err)
	}
	if err := runner.RunTurn(ctx, "p1", "turn-1", "Escribe."); err != nil {
		t.Fatal(err)
	}
	if waits, err := runner.ModeWaitsForSignoff(ctx, "p1"); err != nil || !waits {
		t.Fatalf("during the wait: %v, %v", waits, err)
	}
	if err := gate.LeaveSignoff(ctx, "p1", sign); err != nil {
		t.Fatal(err)
	}
	if waits, err := runner.ModeWaitsForSignoff(ctx, "p1"); err != nil || waits {
		t.Fatalf("after the signoff: %v, %v", waits, err)
	}
}
