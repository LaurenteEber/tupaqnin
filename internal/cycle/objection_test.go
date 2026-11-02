package cycle_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/LaurenteEber/tupaqnin/internal/cycle"
	"github.com/LaurenteEber/tupaqnin/internal/graph"
	"github.com/LaurenteEber/tupaqnin/internal/memory"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// Piece 4.3 (backlog-r4): objecting (E-1, B-81) and revoking (B-82). In
// seedStepProject, dec-1 is yours and used nowhere; dec-2 is the agent's and is
// used in sec-1.

const objectReason = "Porque el promedio esconde los valores extremos."

func allEvents(t *testing.T, db *store.DB) []store.Event {
	t.Helper()
	events, err := db.EventsSince(context.Background(), "p1", 0)
	if err != nil {
		t.Fatal(err)
	}
	return events
}

func setSectionSigned(t *testing.T, db *store.DB, label string) {
	t.Helper()
	ctx := context.Background()
	err := db.Write(ctx, "p1", func(tx *store.Tx) error {
		sec, err := tx.GetObject(ctx, "p1", label)
		if err != nil {
			return err
		}
		var b map[string]any
		_ = json.Unmarshal(sec.Body, &b)
		_, err = tx.UpdateObject(sec.ID, sec.CurrentVersion, "signed", b,
			store.EventInput{Type: store.EventStateChanged, Actor: "human"})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
}

// useDecision adds the used_in edge from a decision to a section.
func useDecision(t *testing.T, db *store.DB, decision, section string) {
	t.Helper()
	ctx := context.Background()
	err := db.Write(ctx, "p1", func(tx *store.Tx) error {
		dec, err := tx.GetObject(ctx, "p1", decision)
		if err != nil {
			return err
		}
		sec, err := tx.GetObject(ctx, "p1", section)
		if err != nil {
			return err
		}
		_, err = tx.AddEdge(store.EdgeUsedIn, dec.ID, sec.ID, "", graph.UsageOf(dec),
			store.EventInput{Type: store.EventEdge, Actor: "human"})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
}

func optionsOf(body map[string]any) []map[string]any {
	raw, _ := body["options"].([]any)
	out := make([]map[string]any, 0, len(raw))
	for _, o := range raw {
		m, _ := o.(map[string]any)
		out = append(out, m)
	}
	return out
}

// E-1, B-81, estados §2: objecting an agent's decision opens the «por aclarar»
// with the app's two options and leaves it in_open_question; «la otra» with a
// «porque» makes a decision of yours that replaces it, and the sections that
// used it get their notice.
func TestE1_ObjectingAnAgentDecisionOpensTheTwoOptionsAndTheOtherMakesItYours(t *testing.T) {
	db := seedStepProject(t)
	ctx := context.Background()
	gate := cycle.NewGate(db)
	got, err := gate.ObjectDecision(ctx, "p1", "dec-2", objectReason)
	if err != nil || got.OpenQuestion != "oq-1" {
		t.Fatalf("ObjectDecision: %+v, %v", got, err)
	}
	if dec, _ := object(t, db, "dec-2"); dec.State != "in_open_question" {
		t.Fatalf("dec-2: %s", dec.State)
	}
	oq, body := object(t, db, "oq-1")
	if oq.State != "open" || body["question"] != objectReason || body["origin"] != "objection" || body["previous_state"] != "agent" {
		t.Fatalf("oq-1: %s %v", oq.State, body)
	}
	opts := optionsOf(body)
	if len(opts) != 2 {
		t.Fatalf("options: %v", opts)
	}
	if opts[0]["keep"] != true || opts[0]["option"] != "Comparar por promedio." || opts[0]["contrast"] != "Hubiera valores extremos." ||
		opts[0]["consequence"] != "Síntesis" {
		t.Fatalf("keep: %v", opts[0])
	}
	if opts[1]["other"] != true || opts[1]["option"] != "Comparar por mediana." || opts[1]["contrast"] != "Hubiera valores extremos." ||
		opts[1]["consequence"] != "Síntesis" {
		t.Fatalf("other: %v", opts[1])
	}
	if blocks := currentBlocks(t, db, "oq-1"); strings.Join(blocks, ",") != "dec-2,sec-1" {
		t.Fatalf("blocks: %v", blocks)
	}
	st, pendings := status(t, db, "sec-1")
	if st != graph.StatusDraft || !hasPending(pendings, graph.PendingOpenQuestion, "oq-1") {
		t.Fatalf("sec-1: %s %v", st, pendings)
	}
	var moved, born bool
	for _, ev := range allEvents(t, db) {
		var data map[string]any
		_ = json.Unmarshal(ev.Data, &data)
		if ev.Type == store.EventStateChanged && data["label"] == "dec-2" && data["to"] == "in_open_question" &&
			data["from"] == "agent" && ev.Actor == "human" && ev.Reason == objectReason {
			moved = true
		}
		if ev.Type == store.EventStateChanged && ev.ObjectID == oq.ID && data["via"] == "objection" && ev.Actor == "human" {
			born = true
		}
	}
	if !moved || !born {
		t.Fatalf("events: moved %v, born %v", moved, born)
	}

	reason := "Porque hay valores extremos en dos estudios."
	answer, err := gate.AnswerOpenQuestion(ctx, "p1", "oq-1", intp(1), reason)
	if err != nil || answer.NewDecision != "dec-3" {
		t.Fatalf("answer: %+v, %v", answer, err)
	}
	if dec, _ := object(t, db, "dec-2"); dec.State != "superseded" {
		t.Fatalf("dec-2: %s, want superseded", dec.State)
	}
	dec, b3 := object(t, db, "dec-3")
	if dec.State != "human" || b3["statement"] != "Comparar por mediana." || b3["instead_of"] != "Comparar por promedio." ||
		b3["reason"] != reason || b3["origin"] != "human" {
		t.Fatalf("dec-3: %s %v", dec.State, b3)
	}
	if got := edgeTypes(t, db, dec.ID)[store.EdgeSupersedes]; len(got) != 1 {
		t.Fatalf("supersedes: %v", got)
	}
	if oq, _ := object(t, db, "oq-1"); oq.State != "resolved" {
		t.Fatalf("oq-1: %s", oq.State)
	}
	if blocks := currentBlocks(t, db, "oq-1"); len(blocks) != 0 {
		t.Fatalf("blocks after resolving: %v", blocks)
	}
	sec, _ := object(t, db, "sec-1")
	if sec.State != "stale" || len(graph.OpenChanges(sec)) != 1 || graph.OpenChanges(sec)[0].About != "dec-2" {
		t.Fatalf("sec-1: %s %v, want stale with the notice of dec-2", sec.State, graph.OpenChanges(sec))
	}
}

// B-81, estados §2 «PorAclarar → Tuya»: «mantener» with a «porque» leaves it
// human, with no new decision and no notice: its content did not change.
func TestB81_KeepingWithAReasonLeavesItHuman(t *testing.T) {
	db := seedStepProject(t)
	ctx := context.Background()
	gate := cycle.NewGate(db)
	if _, err := gate.ObjectDecision(ctx, "p1", "dec-2", objectReason); err != nil {
		t.Fatal(err)
	}
	reason := "Porque los estudios no tienen valores extremos."
	answer, err := gate.AnswerOpenQuestion(ctx, "p1", "oq-1", intp(0), reason)
	if err != nil || answer.NewDecision != "" {
		t.Fatalf("answer: %+v, %v", answer, err)
	}
	dec, body := object(t, db, "dec-2")
	if dec.State != "human" || body["reason"] != reason || body["statement"] != "Comparar por promedio." {
		t.Fatalf("dec-2: %s %v", dec.State, body)
	}
	if oq, _ := object(t, db, "oq-1"); oq.State != "resolved" {
		t.Fatalf("oq-1: %s", oq.State)
	}
	sec, _ := object(t, db, "sec-1")
	if st, _ := status(t, db, "sec-1"); st != graph.StatusReady || len(graph.OpenChanges(sec)) != 0 {
		t.Fatalf("sec-1: %s %v", st, graph.OpenChanges(sec))
	}
	if _, err := db.GetObject(ctx, "p1", "dec-3"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("dec-3: %v, want none", err)
	}
	var moved bool
	for _, ev := range allEvents(t, db) {
		var data map[string]any
		_ = json.Unmarshal(ev.Data, &data)
		if data["label"] == "dec-2" && data["from"] == "in_open_question" && data["to"] == "human" && ev.Reason == reason && ev.Actor == "human" {
			moved = true
		}
	}
	if !moved {
		t.Fatal("no state_changed with your «porque»")
	}
}

// B-81, estados §1: objecting a decision of a signed section moves it to
// «cambió», and resolving the question does not sign it back.
func TestE1_ObjectingADecisionOfASignedSectionMovesItToChanged(t *testing.T) {
	db := seedStepProject(t)
	ctx := context.Background()
	setSectionSigned(t, db, "sec-1")
	gate := cycle.NewGate(db)
	if _, err := gate.ObjectDecision(ctx, "p1", "dec-2", objectReason); err != nil {
		t.Fatal(err)
	}
	if st := sectionState(t, db, "sec-1"); st != "changed_since_signoff" {
		t.Fatalf("sec-1: %s", st)
	}
	var moves int
	for _, ev := range allEvents(t, db) {
		var data map[string]any
		_ = json.Unmarshal(ev.Data, &data)
		if data["label"] == "sec-1" && data["to"] == "changed_since_signoff" && data["why"] == "open_question" && data["open_question"] == "oq-1" {
			moves++
		}
	}
	if moves != 1 {
		t.Fatalf("moves of sec-1: %d", moves)
	}
	if _, err := gate.AnswerOpenQuestion(ctx, "p1", "oq-1", intp(0), "Porque sigue siendo la mejor regla."); err != nil {
		t.Fatal(err)
	}
	if st := sectionState(t, db, "sec-1"); st != "changed_since_signoff" {
		t.Fatalf("sec-1 after resolving: %s", st)
	}
}

// Adopted 1-nov (6B): a judge's «por aclarar» (B-67) on a signed section moves it
// to changed_since_signoff.
func TestB67_AFlawQuestionOnASignedSectionMovesItToChanged(t *testing.T) {
	db, runner, _, _ := newCaptureRunner(t, verifyFlaws(t, oneFlaw("sec-1", flawOne, "figures_and_calculations")),
		judgeVerifyFixture(t, map[string]any{"flaws": []any{}, "recheck": []any{flawStillOpen(1, flawOne)}}))
	seedJudged(t, db)
	ctx := context.Background()
	label, err := runner.RunJudgeVerify(ctx, "p1", "sec-1", cycle.JudgeReasonRisk, nil)
	if err != nil {
		t.Fatal(err)
	}
	setSectionSigned(t, db, "sec-1")
	if err := runner.RunJudgeRecheck(ctx, "p1", label); err != nil {
		t.Fatal(err)
	}
	if oqs := flawOQs(t, db); len(oqs) != 1 || oqs[0].State != "open" {
		t.Fatalf("questions: %+v", oqs)
	}
	st, pendings := status(t, db, "sec-1")
	if st != graph.StatusChangedSinceSignoff || !hasPending(pendings, graph.PendingOpenQuestion, flawOQs(t, db)[0].Label) {
		t.Fatalf("sec-1: %s %v", st, pendings)
	}
}

// B-82, estados §2: an agent's decision is not revoked: nothing changes.
func TestB82_RevokingAnAgentDecisionIsRefused(t *testing.T) {
	db := seedStepProject(t)
	before := len(allEvents(t, db))
	err := cycle.NewGate(db).RevokeDecision(context.Background(), "p1", "dec-2", objectReason)
	if !errors.Is(err, cycle.ErrDecisionNotYours) {
		t.Fatalf("revoke: %v", err)
	}
	if dec, _ := object(t, db, "dec-2"); dec.State != "agent" {
		t.Fatalf("dec-2: %s", dec.State)
	}
	if n := len(allEvents(t, db)); n != before {
		t.Fatalf("events: %d, want %d", n, before)
	}
}

// B-82, estados §2: yours stays revoked, without a successor, and the section
// that uses it gets its notice: stale when unsigned, «cambió» when signed.
func TestB82_RevokingYoursLeavesItRevokedAndTheSectionsGetTheirNotice(t *testing.T) {
	for _, tc := range []struct {
		name, want string
		signed     bool
	}{{"unsigned", "stale", false}, {"signed", "changed_since_signoff", true}} {
		t.Run(tc.name, func(t *testing.T) {
			db := seedStepProject(t)
			ctx := context.Background()
			useDecision(t, db, "dec-1", "sec-1")
			if tc.signed {
				setSectionSigned(t, db, "sec-1")
			}
			if err := cycle.NewGate(db).RevokeDecision(ctx, "p1", "dec-1", objectReason); err != nil {
				t.Fatal(err)
			}
			dec, _ := object(t, db, "dec-1")
			if dec.State != "revoked" {
				t.Fatalf("dec-1: %s", dec.State)
			}
			edges, err := db.EdgesTo(ctx, "p1", dec.ID)
			if err != nil {
				t.Fatal(err)
			}
			for _, e := range edges {
				if e.Type == store.EdgeSupersedes {
					t.Fatalf("a successor: %+v", e)
				}
			}
			sec, _ := object(t, db, "sec-1")
			if sec.State != tc.want || len(graph.OpenChanges(sec)) != 1 || graph.OpenChanges(sec)[0].About != "dec-1" {
				t.Fatalf("sec-1: %s %v", sec.State, graph.OpenChanges(sec))
			}
			var moved, entered bool
			for _, ev := range allEvents(t, db) {
				var data map[string]any
				_ = json.Unmarshal(ev.Data, &data)
				if data["label"] == "dec-1" && data["to"] == "revoked" && ev.Reason == objectReason && ev.Actor == "human" {
					moved = true
				}
				if ev.Type == store.EventUsageChange && data["about"] == "dec-1" && data["kind"] == "entered" {
					entered = true
				}
			}
			if !moved || !entered {
				t.Fatalf("events: moved %v, entered %v", moved, entered)
			}
		})
	}
}

// B-82, T-1: revoking without a valid «porque» does nothing at all.
func TestB82_RevokingWithoutAValidReasonDoesNothing(t *testing.T) {
	db := seedStepProject(t)
	useDecision(t, db, "dec-1", "sec-1")
	before := len(allEvents(t, db))
	for _, reason := range []string{"", "   ", "ok", "sí"} {
		err := cycle.NewGate(db).RevokeDecision(context.Background(), "p1", "dec-1", reason)
		if !errors.Is(err, cycle.ErrInvalidReason) {
			t.Fatalf("revoke %q: %v", reason, err)
		}
	}
	if dec, _ := object(t, db, "dec-1"); dec.State != "human" {
		t.Fatalf("dec-1: %s", dec.State)
	}
	sec, _ := object(t, db, "sec-1")
	if len(graph.OpenChanges(sec)) != 0 {
		t.Fatalf("notice: %v", graph.OpenChanges(sec))
	}
	if n := len(allEvents(t, db)); n != before {
		t.Fatalf("events: %d, want %d", n, before)
	}
}

// E-1: an objection without a valid motive, or on a decision that is not
// current or already under one, changes nothing.
func TestE1_AnObjectionThatDoesNotApplyChangesNothing(t *testing.T) {
	db := seedStepProject(t)
	ctx := context.Background()
	gate := cycle.NewGate(db)
	before := len(allEvents(t, db))
	if _, err := gate.ObjectDecision(ctx, "p1", "dec-2", "ok"); !errors.Is(err, cycle.ErrInvalidReason) {
		t.Fatalf("assent: %v", err)
	}
	if _, err := gate.ObjectDecision(ctx, "p1", "sec-1", objectReason); !errors.Is(err, cycle.ErrDecisionNotFound) {
		t.Fatalf("a section: %v", err)
	}
	if n := len(allEvents(t, db)); n != before {
		t.Fatalf("events: %d, want %d", n, before)
	}
	if _, err := gate.ObjectDecision(ctx, "p1", "dec-2", objectReason); err != nil {
		t.Fatal(err)
	}
	before = len(allEvents(t, db))
	if _, err := gate.ObjectDecision(ctx, "p1", "dec-2", objectReason); !errors.Is(err, cycle.ErrDecisionNotCurrent) {
		t.Fatalf("objected twice: %v", err)
	}
	// Estados §2: revoking starts only from yours or a confirmed one.
	if err := gate.RevokeDecision(ctx, "p1", "dec-2", objectReason); !errors.Is(err, cycle.ErrDecisionNotYours) {
		t.Fatalf("revoking while objected: %v", err)
	}
	if n := len(allEvents(t, db)); n != before {
		t.Fatalf("events: %d, want %d", n, before)
	}
}

// Estados §3: objecting a decision with its card open closes the card as
// obsolete, and the «por aclarar» is what is left to answer.
func TestE3_ObjectingADecisionWithItsCardOpenLeavesTheCardObsolete(t *testing.T) {
	db := seedStepProject(t)
	ctx := context.Background()
	if err := cycle.MarkCritical(ctx, db, "p1", "dec-2", true); err != nil {
		t.Fatal(err)
	}
	if card, _ := object(t, db, "card-1"); card.State != "open" {
		t.Fatalf("card-1: %s", card.State)
	}
	if _, err := cycle.NewGate(db).ObjectDecision(ctx, "p1", "dec-2", objectReason); err != nil {
		t.Fatal(err)
	}
	if card, _ := object(t, db, "card-1"); card.State != "obsolete" {
		t.Fatalf("card-1: %s, want obsolete", card.State)
	}
	_, body := object(t, db, "oq-1")
	opts := optionsOf(body)
	risks, _ := opts[0]["risk_reasons"].([]any)
	if len(risks) == 0 {
		t.Fatalf("keep without its risk: %v", opts[0])
	}
	if others, _ := opts[1]["risk_reasons"].([]any); len(others) != len(risks) {
		t.Fatalf("other without its risk: %v", opts[1])
	}
}

// P-113: the objection's question is a pending of yours, so it blocks a signoff
// at a cap as well.
func TestP113_AnObjectionBlocksTheSignoffAtACap(t *testing.T) {
	db := seedStepProject(t)
	if _, err := cycle.NewGate(db).ObjectDecision(context.Background(), "p1", "dec-2", objectReason); err != nil {
		t.Fatal(err)
	}
	_, pendings := status(t, db, "sec-1")
	blocking, _ := graph.PendingsAtCap(pendings, "project_cap")
	if !hasPending(blocking, graph.PendingOpenQuestion, "oq-1") {
		t.Fatalf("at a cap: %v", blocking)
	}
}

// B-14 lists what cancels a signoff in progress, and an objection or a revocation
// is not there: the signoff stays open and signing is refused until it is settled.
func TestB14_AnObjectionWithASignoffInProgressLeavesItOpenAndBlocksSigning(t *testing.T) {
	db := seedStepProject(t)
	ctx := context.Background()
	gate := cycle.NewGate(db)
	open, err := gate.RequestSignoff(ctx, "p1", "sec-1")
	if err != nil || !open.Opened {
		t.Fatalf("RequestSignoff: %+v, %v", open, err)
	}
	if err := gate.RecordExplanation(ctx, "p1", open.Label, "Lo explico yo."); err != nil {
		t.Fatal(err)
	}
	if _, err := gate.ObjectDecision(ctx, "p1", "dec-2", objectReason); err != nil {
		t.Fatal(err)
	}
	sec, _ := object(t, db, "sec-1")
	if _, active, err := graph.ActiveSignoff(ctx, db, "p1", sec.ID); err != nil || !active {
		t.Fatalf("active: %v, %v", active, err)
	}
	var blocked *cycle.PendingsError
	if err := gate.Sign(ctx, "p1", open.Label); !errors.As(err, &blocked) || !hasPending(blocked.Pendings, graph.PendingOpenQuestion, "oq-1") {
		t.Fatalf("Sign: %v", err)
	}
	if st := sectionState(t, db, "sec-1"); st == "signed" {
		t.Fatal("signed with the question open")
	}
	if _, err := gate.AnswerOpenQuestion(ctx, "p1", "oq-1", intp(0), "Porque sigue siendo la mejor regla."); err != nil {
		t.Fatal(err)
	}
	if err := gate.Sign(ctx, "p1", open.Label); err != nil {
		t.Fatalf("Sign after settling it: %v", err)
	}
}

// B-85, 4.4: the mode does not answer for you: with only an objection left it
// stops with human_only, and a turn leaves the question open.
func TestB85_AnObjectionDuringTheModeStaysOpenAndStopsWithHumanOnly(t *testing.T) {
	db := openTemp(t)
	seedSignable(t, db, true)
	ctx := context.Background()
	if _, err := cycle.NewGate(db).ObjectDecision(ctx, "p1", "dec-1", objectReason); err != nil {
		t.Fatal(err)
	}
	runner, _, _ := modeRunnerOn(t, db, nil, loadAppFixture(t, "04-principal-no-work.json"))
	modeOn(t, db, "")
	if reason, stop, err := cycle.NewGate(db).ShouldStop(ctx, "p1", nil); err != nil || !stop || reason != cycle.StopHumanOnly {
		t.Fatalf("ShouldStop: %s %v %v", reason, stop, err)
	}
	if err := runner.RunTurn(ctx, "p1", "turn-1", "Sigue."); err != nil {
		t.Fatal(err)
	}
	if oq, _ := object(t, db, "oq-1"); oq.State != "open" {
		t.Fatalf("oq-1: %s", oq.State)
	}
	if dec, _ := object(t, db, "dec-1"); dec.State != "in_open_question" {
		t.Fatalf("dec-1: %s", dec.State)
	}
}

// Rule 6 of the gate (estados §2): the agent does not replace a decision under
// objection. No output field reaches objecting or revoking.
func TestE1_TheAgentCannotSupersedeAnObjectedDecision(t *testing.T) {
	db := seedStepProject(t)
	if _, err := cycle.NewGate(db).ObjectDecision(context.Background(), "p1", "dec-2", objectReason); err != nil {
		t.Fatal(err)
	}
	gate := cycle.NewGate(db)
	output := stepFile(t, "supersedes-agent")
	res, err := gate.EndStep(context.Background(), "p1", "turn-1", 1, output)
	if err != nil || res.Status != cycle.StatusAccepted {
		t.Fatalf("EndStep: %+v, %v", res, err)
	}
	applied, err := gate.ApplyStep(context.Background(), "p1", "turn-1", output)
	if err != nil {
		t.Fatal(err)
	}
	if applied.Applied {
		t.Fatal("the step superseded an objected decision")
	}
	if dec, _ := object(t, db, "dec-2"); dec.State != "in_open_question" {
		t.Fatalf("dec-2: %s", dec.State)
	}
}

// Rule 7: the principal knows a decision is objected and that one is revoked,
// and reading the question gives it no internal id.
func TestE1_TheMemoryShowsTheObjectionAndDropsWhatIsRevoked(t *testing.T) {
	db := seedStepProject(t)
	ctx := context.Background()
	gate := cycle.NewGate(db)
	if _, err := gate.ObjectDecision(ctx, "p1", "dec-2", objectReason); err != nil {
		t.Fatal(err)
	}
	if err := gate.RevokeDecision(ctx, "p1", "dec-1", objectReason); err != nil {
		t.Fatal(err)
	}
	mem := memory.New(db)
	st, err := mem.WorkingState(ctx, "p1")
	if err != nil {
		t.Fatal(err)
	}
	var seen2, seen1 bool
	for _, d := range st.Essentials.Decisions {
		seen2 = seen2 || d.Label == "dec-2" && d.State == "in_open_question"
		seen1 = seen1 || d.Label == "dec-1"
	}
	if !seen2 || seen1 {
		t.Fatalf("essentials: %+v", st.Essentials.Decisions)
	}
	item, err := mem.Read(ctx, "p1", "inst-1", "oq-1")
	if err != nil {
		t.Fatal(err)
	}
	dec, _ := object(t, db, "dec-2")
	if strings.Contains(item.Text, dec.ID) || !strings.Contains(item.Text, "dec-2") || !strings.Contains(item.Text, objectReason) {
		t.Fatalf("read: %q", item.Text)
	}
}

// Existing behaviour shared with applyKept: a checker's «por aclarar» kept leaves
// your decision as it was, with no new decision and no reason on it.
func TestE2_KeepingYourDecisionOnACheckerQuestionLeavesItAlone(t *testing.T) {
	db := seedCheckerProject(t)
	yourEdit(t, db, "sec-1", "La síntesis compara estudios. Usamos solo datos de tesis, no el informe del comité.")
	runner, _ := checkerRunner(t, db, checkerFixture(t, "contradicts-decision"))
	ctx := context.Background()
	if _, err := runner.RequestSignoff(ctx, "p1", "sec-1"); err != nil {
		t.Fatal(err)
	}
	before, beforeBody := object(t, db, "dec-1")
	answer, err := runner.AnswerOpenQuestion(ctx, "p1", "oq-1", intp(0), "Porque el comité sí publicó datos.")
	if err != nil || answer.NewDecision != "" {
		t.Fatalf("answer: %+v, %v", answer, err)
	}
	after, afterBody := object(t, db, "dec-1")
	if after.State != "human" || after.CurrentVersion != before.CurrentVersion || afterBody["reason"] != beforeBody["reason"] {
		t.Fatalf("dec-1: %s v%d %v", after.State, after.CurrentVersion, afterBody)
	}
}

// Existing behaviour: a judge's «por aclarar» keeps nothing: either option
// resolves it and no decision is born.
func TestB67_AnsweringAFlawQuestionCreatesNoDecision(t *testing.T) {
	for _, option := range []int{0, 1} {
		db, runner, _, _ := newCaptureRunner(t, verifyFlaws(t, oneFlaw("sec-1", flawOne, "figures_and_calculations")),
			judgeVerifyFixture(t, map[string]any{"flaws": []any{}, "recheck": []any{flawStillOpen(1, flawOne)}}))
		seedJudged(t, db)
		ctx := context.Background()
		label, err := runner.RunJudgeVerify(ctx, "p1", "sec-1", cycle.JudgeReasonRisk, nil)
		if err != nil {
			t.Fatal(err)
		}
		if err := runner.RunJudgeRecheck(ctx, "p1", label); err != nil {
			t.Fatal(err)
		}
		oq := flawOQs(t, db)[0]
		answer, err := runner.AnswerOpenQuestion(ctx, "p1", oq.Label, intp(option), "Porque rehago ese pasaje con el dato medido.")
		if err != nil || answer.NewDecision != "" {
			t.Fatalf("option %d: %+v, %v", option, answer, err)
		}
		if got, _ := object(t, db, oq.Label); got.State != "resolved" {
			t.Fatalf("option %d: %s", option, got.State)
		}
	}
}

// figureStep writes a figure in sec-1, linked to the decision.
func figureStep(version int, decision string) json.RawMessage {
	return json.RawMessage(fmt.Sprintf(`{"stretch":"build","triage_line":"Construir.","closing_risk":"none","note":"Paso.",`+
		`"new_text":[{"section_id":"sec-1","base_version":%d,"anchor":"Cierre.","claim_origin":"material","operation":"replace","text":"Cierre con 7 casos."}],`+
		`"figure_links":[{"figure_anchor":"7 casos","decision_id":%q}]}`, version, decision))
}

// citingStep writes in sec-1 and cites the decision.
func citingStep(version int, decision string) json.RawMessage {
	return json.RawMessage(fmt.Sprintf(`{"stretch":"build","triage_line":"Construir.","closing_risk":"none","note":"Paso.",`+
		`"new_text":[{"section_id":"sec-1","base_version":%d,"anchor":"Cierre.","claim_origin":"material","operation":"replace","text":"Cierre nuevo."}],`+
		`"used_decisions":[%q]}`, version, decision))
}

// B-82, estados §1: a figure covered only by a revoked decision is a figure
// without its decision: closing the notice does not free the section, the edge
// stays in the history, and rewriting the text does.
func TestB82_AFigureOfARevokedDecisionIsAFigureWithoutDecision(t *testing.T) {
	db := seedStepProject(t)
	ctx := context.Background()
	gate := cycle.NewGate(db)
	sec, _ := object(t, db, "sec-1")
	if applied := applyRecorded(t, db, figureStep(sec.CurrentVersion, "dec-1")); !applied.Applied {
		t.Fatalf("step: %v", applied.Errors)
	}
	if st, pendings := status(t, db, "sec-1"); hasPending(pendings, graph.PendingFigureWithoutDecision, "7") {
		t.Fatalf("before: %s %v", st, pendings)
	}
	if err := gate.RevokeDecision(ctx, "p1", "dec-1", objectReason); err != nil {
		t.Fatal(err)
	}
	sec, _ = object(t, db, "sec-1")
	if err := gate.CloseChange(ctx, "p1", graph.OpenChanges(sec)[0].Label, "reviewed_unchanged", "Porque la cifra sigue siendo la misma."); err != nil {
		t.Fatal(err)
	}
	st, pendings := status(t, db, "sec-1")
	if st != graph.StatusDraft || !hasPending(pendings, graph.PendingFigureWithoutDecision, "7") {
		t.Fatalf("after closing the notice: %s %v", st, pendings)
	}
	open, err := gate.RequestSignoff(ctx, "p1", "sec-1")
	if err != nil || open.Opened || !hasPending(open.Pendings, graph.PendingFigureWithoutDecision, "7") {
		t.Fatalf("RequestSignoff: %+v, %v", open, err)
	}
	fig, _ := object(t, db, "fig-1")
	if got := edgeTypes(t, db, fig.ID)[store.EdgeCovers]; len(got) != 1 {
		t.Fatalf("the edge stays: %v", got)
	}
	yourEdit(t, db, "sec-1", "La síntesis compara estudios. Los estudios usan filtros. Cierre.")
	if _, pendings := status(t, db, "sec-1"); hasPending(pendings, graph.PendingFigureWithoutDecision, "7") {
		t.Fatalf("after rewriting: %v", pendings)
	}
}

// B-82, rule 3 of the step: citing a revoked decision rejects the step with its
// reason, and nothing is written.
func TestB82_AStepCitingARevokedDecisionIsRejected(t *testing.T) {
	for name, build := range map[string]func(int, string) json.RawMessage{"used_decisions": citingStep, "figure_links": figureStep} {
		t.Run(name, func(t *testing.T) {
			db := seedStepProject(t)
			gate := cycle.NewGate(db)
			if err := gate.RevokeDecision(context.Background(), "p1", "dec-1", objectReason); err != nil {
				t.Fatal(err)
			}
			before, _ := object(t, db, "sec-1")
			applied := applyRecorded(t, db, build(before.CurrentVersion, "dec-1"))
			requireRejected(t, applied, "dec-1 was revoked")
			after, body := object(t, db, "sec-1")
			if after.CurrentVersion != before.CurrentVersion || strings.Contains(body["text"].(string), "Cierre nuevo") {
				t.Fatalf("sec-1 changed: v%d %v", after.CurrentVersion, body)
			}
			dec, _ := object(t, db, "dec-1")
			if got := edgeTypes(t, db, dec.ID)[store.EdgeUsedIn]; len(got) != 0 {
				t.Fatalf("used_in: %v", got)
			}
		})
	}
}

// Baseline kept: a step citing a superseded decision is still accepted.
func TestB82_AStepCitingASupersededDecisionIsStillAccepted(t *testing.T) {
	db := seedStepProject(t)
	ctx := context.Background()
	gate := cycle.NewGate(db)
	if _, err := gate.ObjectDecision(ctx, "p1", "dec-2", objectReason); err != nil {
		t.Fatal(err)
	}
	if _, err := gate.AnswerOpenQuestion(ctx, "p1", "oq-1", intp(1), "Porque hay valores extremos."); err != nil {
		t.Fatal(err)
	}
	sec, _ := object(t, db, "sec-1")
	if applied := applyRecorded(t, db, citingStep(sec.CurrentVersion, "dec-2")); !applied.Applied {
		t.Fatalf("step: %v", applied.Errors)
	}
}

// B-82: accepting a proposal that cites a decision revoked after it was born
// asks for a new step, as a repeated anchor does, and writes nothing. One that
// cites an objected decision applies, and the question blocks the section.
func TestB82_AcceptingAProposalCitingARevokedDecisionAsksForANewStep(t *testing.T) {
	for name, build := range map[string]func(int, string) json.RawMessage{"used_decisions": citingStep, "figure_links": figureStep} {
		t.Run(name, func(t *testing.T) {
			db := seedStepProject(t)
			ctx := context.Background()
			gate := cycle.NewGate(db)
			setSectionSigned(t, db, "sec-1")
			sec, _ := object(t, db, "sec-1")
			applied := applyRecorded(t, db, build(sec.CurrentVersion, "dec-1"))
			if len(applied.Proposals) != 1 {
				t.Fatalf("proposals: %+v", applied)
			}
			if err := gate.RevokeDecision(ctx, "p1", "dec-1", objectReason); err != nil {
				t.Fatal(err)
			}
			before, _ := object(t, db, "sec-1")
			out, err := gate.AcceptProposal(ctx, "p1", applied.Proposals[0])
			if err != nil || !out.NewStep || out.State != "discarded" || out.Why != graph.ProposalDecisionRevoked {
				t.Fatalf("accept: %+v, %v", out, err)
			}
			after, _ := object(t, db, "sec-1")
			if after.CurrentVersion != before.CurrentVersion {
				t.Fatalf("sec-1 written: v%d, want v%d", after.CurrentVersion, before.CurrentVersion)
			}
		})
	}
	db := seedStepProject(t)
	ctx := context.Background()
	gate := cycle.NewGate(db)
	setSectionSigned(t, db, "sec-1")
	sec, _ := object(t, db, "sec-1")
	applied := applyRecorded(t, db, citingStep(sec.CurrentVersion, "dec-1"))
	if _, err := gate.ObjectDecision(ctx, "p1", "dec-1", objectReason); err != nil {
		t.Fatal(err)
	}
	out, err := gate.AcceptProposal(ctx, "p1", applied.Proposals[0])
	if err != nil || out.State != "accepted" {
		t.Fatalf("accept objected: %+v, %v", out, err)
	}
	if _, pendings := status(t, db, "sec-1"); !hasPending(pendings, graph.PendingOpenQuestion, "oq-1") {
		t.Fatalf("pendings: %v", pendings)
	}
}

// E-1: the principal reads the decision's own «porque» as its reason, and your
// motive as what it is, an objection.
func TestE1_TheMemoryReadsTheObjectionMotiveApartFromTheReason(t *testing.T) {
	db := seedStepProject(t)
	ctx := context.Background()
	gate := cycle.NewGate(db)
	if err := cycle.MarkCritical(ctx, db, "p1", "dec-2", true); err != nil {
		t.Fatal(err)
	}
	cardReason := "Porque ya lo revisé con el comité."
	if _, err := gate.AnswerCard(ctx, "p1", "card-1", cycle.OptionKeep, cardReason); err != nil {
		t.Fatal(err)
	}
	if _, err := gate.ObjectDecision(ctx, "p1", "dec-2", objectReason); err != nil {
		t.Fatal(err)
	}
	st, err := memory.New(db).WorkingState(ctx, "p1")
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range st.Essentials.Decisions {
		if d.Label != "dec-2" {
			continue
		}
		if d.Reason != cardReason || d.Objection != objectReason {
			t.Fatalf("dec-2: reason %q, objection %q", d.Reason, d.Objection)
		}
		kept := "Porque sigue siendo la mejor regla."
		if _, err := gate.AnswerOpenQuestion(ctx, "p1", "oq-1", intp(0), kept); err != nil {
			t.Fatal(err)
		}
		st, err := memory.New(db).WorkingState(ctx, "p1")
		if err != nil {
			t.Fatal(err)
		}
		for _, d := range st.Essentials.Decisions {
			if d.Label == "dec-2" && (d.Reason != kept || d.Objection != "") {
				t.Fatalf("after keeping: reason %q, objection %q", d.Reason, d.Objection)
			}
		}
		return
	}
	t.Fatal("dec-2 not in the essentials")
}
