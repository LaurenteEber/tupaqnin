package cycle_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/LaurenteEber/tupaqnin/internal/cycle"
	"github.com/LaurenteEber/tupaqnin/internal/graph"
	"github.com/LaurenteEber/tupaqnin/internal/provider"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// P-72: the probe of a new premise (B-46, estados §6, fichas §3) is
// queued instead of skipped while a turn of the project runs. The queue
// lives in the history (a probe event with kind queued, closed by the
// probe's own events); while it is pending, the affected sections are
// «sin comprobar» (B-50) and their signoff is blocked. It runs when the
// turn ends or at the next B-36 trigger, over the current text.

const newPremise = "Cambiar la premisa: ambas son igual de seguras."

// contradictedPremise leaves oq-1 open on the contradicted premise
// (the batch of contradictingEdit, judged by the first recording).
func contradictedPremise(t *testing.T, db *store.DB, runner *cycle.Runner) {
	t.Helper()
	if _, err := runner.RequestSignoff(context.Background(), "p1", "sec-1"); err != nil {
		t.Fatalf("RequestSignoff: %v", err)
	}
	if prem, _ := object(t, db, "prem-1"); prem.State != "contradicted" {
		t.Fatalf("prem-1: %s, want contradicted", prem.State)
	}
}

func uncheckedNow(t *testing.T, db *store.DB, label string) bool {
	t.Helper()
	_, pendings, err := graph.SectionStatus(context.Background(), db, "p1", label)
	if err != nil {
		t.Fatalf("SectionStatus: %v", err)
	}
	return hasPending(pendings, graph.PendingUnchecked, "")
}

func queuedEvents(t *testing.T, db *store.DB) []map[string]any {
	t.Helper()
	events, err := db.EventsSince(context.Background(), "p1", 0)
	if err != nil {
		t.Fatalf("EventsSince: %v", err)
	}
	var out []map[string]any
	for _, ev := range events {
		var data map[string]any
		_ = json.Unmarshal(ev.Data, &data)
		if ev.Type == store.EventProbe && data["kind"] == graph.ProbeKindQueued {
			out = append(out, data)
		}
	}
	return out
}

func TestP72_NewPremiseDuringATurnIsProbedWhenTheTurnEnds(t *testing.T) {
	db := seedCheckerProject(t)
	yourEdit(t, db, "sec-1", contradictingEdit)
	fake := provider.NewFake(loadAppFixture(t, "04-principal-no-work.json"),
		checkerFixture(t, "contradicts-premise"), checkerFixture(t, "no-findings"))
	fake.Recorder = db
	ctx := context.Background()
	var runner *cycle.Runner
	var cap *captureProvider
	fired := false
	inner := &fireDuringPrincipal{inner: fake, fire: func() {
		fired = true
		answer, err := runner.AnswerOpenQuestion(ctx, "p1", "oq-1", intp(2), "Porque los muestreos nuevos no muestran diferencia.")
		if err != nil || !answer.PremiseChanged {
			t.Errorf("answer during the turn: %+v, %v", answer, err)
			return
		}
		if n := len(cap.checker); n != 1 {
			t.Errorf("checker calls during the turn: %d, want 1 (the probe is queued, not sent)", n)
		}
		if q := queuedEvents(t, db); len(q) != 1 || q[0]["protected"] != "prem-1" ||
			!strings.Contains(mustJSON(t, q[0]["queued_sections"]), "sec-1") {
			t.Errorf("queued probe events: %v, want one for prem-1 over sec-1", q)
		}
		if !uncheckedNow(t, db, "sec-1") {
			t.Errorf("sec-1 while the probe is queued: want unchecked (B-50)")
		}
		open, err := cycle.NewGate(db).RequestSignoff(ctx, "p1", "sec-1")
		if err != nil || open.Opened || !hasPending(open.Pendings, graph.PendingUnchecked, "") {
			t.Errorf("signoff while the probe is queued: %+v, %v; want blocked by unchecked", open, err)
		}
	}}
	cap = &captureProvider{inner: inner}
	count := func(ctx context.Context, input []json.RawMessage) (int, error) {
		return fake.CountTokens(ctx, provider.PrincipalModel, input)
	}
	runner = cycle.NewRunner(db, cap, count, &fakePublisher{})
	contradictedPremise(t, db, runner)

	if err := runner.RunTurn(ctx, "p1", "turn-1", "Sigue."); err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	if !fired {
		t.Fatal("the principal was never called")
	}
	if n := len(cap.checker); n != 2 {
		t.Fatalf("checker calls: %d, want 2 (the batch, then the queued probe once when the turn ends)", n)
	}
	brief := briefOf(t, cap.checker[1])
	for _, s := range []string{"prem-1 (premise): " + newPremise, "New passage of section sec-1", contradictingEdit} {
		if !strings.Contains(brief, s) {
			t.Fatalf("probe brief lacks %q:\n%s", s, brief)
		}
	}
	if uncheckedNow(t, db, "sec-1") {
		t.Fatal("sec-1 after the probe ran: still unchecked")
	}
	// It ran once: the next trigger sends nothing more.
	if _, err := runner.RequestSignoff(ctx, "p1", "sec-1"); err != nil {
		t.Fatalf("RequestSignoff: %v", err)
	}
	if n := len(cap.checker); n != 2 {
		t.Fatalf("checker calls after asking to sign: %d, want 2 (the probe already ran)", n)
	}
}

// A probe queued in the history (here the gate's answer, with no runner
// to send it) runs at the next B-36 trigger: asking to sign. Its
// contradiction opens its «por aclarar», which the signoff lists.
func TestP72_AQueuedProbeRunsAtTheNextTrigger(t *testing.T) {
	db := seedCheckerProject(t)
	yourEdit(t, db, "sec-1", contradictingEdit)
	runner, cap := checkerRunner(t, db, checkerFixture(t, "contradicts-premise"))
	contradictedPremise(t, db, runner)
	ctx := context.Background()

	answer, err := cycle.NewGate(db).AnswerOpenQuestion(ctx, "p1", "oq-1", intp(2), "Porque los muestreos nuevos no muestran diferencia.")
	if err != nil || !answer.PremiseChanged {
		t.Fatalf("answer: %+v, %v", answer, err)
	}
	if len(queuedEvents(t, db)) != 1 || !uncheckedNow(t, db, "sec-1") {
		t.Fatal("after the answer: want one queued probe and sec-1 unchecked")
	}
	open, err := runner.RequestSignoff(ctx, "p1", "sec-1")
	if err != nil {
		t.Fatalf("RequestSignoff: %v", err)
	}
	if n := len(cap.checker); n != 2 || !strings.Contains(briefOf(t, cap.checker[1]), "prem-1 (premise): "+newPremise) {
		t.Fatalf("checker calls: %d, want the queued probe on asking to sign", n)
	}
	if uncheckedNow(t, db, "sec-1") {
		t.Fatal("sec-1 after the probe ran: still unchecked")
	}
	if open.Opened || !hasPending(open.Pendings, graph.PendingOpenQuestion, "oq-2") {
		t.Fatalf("RequestSignoff: %+v; want blocked by the probe's oq-2", open)
	}
	if prem, _ := object(t, db, "prem-1"); prem.State != "contradicted" {
		t.Fatalf("prem-1: %s, want contradicted again by the probe", prem.State)
	}
}

// Without a turn running, the probe still runs at once (2.4 kept), and
// nothing stays queued.
func TestB46_WithoutARunningTurnTheNewPremiseIsProbedAtOnce(t *testing.T) {
	db := seedCheckerProject(t)
	yourEdit(t, db, "sec-1", contradictingEdit)
	runner, cap := checkerRunner(t, db, checkerFixture(t, "contradicts-premise"), checkerFixture(t, "no-findings"))
	contradictedPremise(t, db, runner)
	ctx := context.Background()
	if _, err := runner.AnswerOpenQuestion(ctx, "p1", "oq-1", intp(2), "Porque los muestreos nuevos no muestran diferencia."); err != nil {
		t.Fatalf("answer: %v", err)
	}
	if n := len(cap.checker); n != 2 {
		t.Fatalf("checker calls: %d, want 2 (the batch, then the probe at once)", n)
	}
	if uncheckedNow(t, db, "sec-1") {
		t.Fatal("sec-1 after the probe: unchecked")
	}
}

// A queued probe whose checker fails twice ran: its output_retry closes
// it (it is not sent again), and its sections stay «sin comprobar» by
// B-50 until the next valid judgment over them.
func TestB50_AQueuedProbeThatFailsTwiceRunsOnceAndLeavesItsSectionsUnchecked(t *testing.T) {
	db := seedCheckerProject(t)
	yourEdit(t, db, "sec-1", contradictingEdit)
	runner, cap := checkerRunner(t, db, checkerFixture(t, "contradicts-premise"),
		checkerFixture(t, "malformed"), checkerFixture(t, "malformed"))
	contradictedPremise(t, db, runner)
	ctx := context.Background()
	if _, err := cycle.NewGate(db).AnswerOpenQuestion(ctx, "p1", "oq-1", intp(2), "Porque los muestreos nuevos no muestran diferencia."); err != nil {
		t.Fatalf("answer: %v", err)
	}
	for i := 0; i < 2; i++ {
		open, err := runner.RequestSignoff(ctx, "p1", "sec-1")
		if err != nil || open.Opened || !hasPending(open.Pendings, graph.PendingUnchecked, "") {
			t.Fatalf("RequestSignoff %d: %+v, %v; want blocked by unchecked", i+1, open, err)
		}
	}
	if n := len(cap.checker); n != 3 {
		t.Fatalf("checker calls: %d, want 3 (the batch, then the queued probe and its retry, once)", n)
	}
}
