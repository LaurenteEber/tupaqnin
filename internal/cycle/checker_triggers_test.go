package cycle_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/LaurenteEber/tupaqnin/internal/cycle"
	"github.com/LaurenteEber/tupaqnin/internal/graph"
	"github.com/LaurenteEber/tupaqnin/internal/provider"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// P-67 and P-68: the two other moments of B-36 (asking to sign, and
// 60 s after the last save), a provider error counted toward «sin
// comprobar» (B-50), and the principal's proposal on a protected
// decision (flujos §1).

const contradictingEdit = "La síntesis compara estudios. El agua del grifo es igual de segura que la embotellada."

// checkerRunner is a runner over a fake with the given recordings; the
// principal's turns get «no work».
func checkerRunner(t *testing.T, db *store.DB, fixtures ...provider.Fixture) (*cycle.Runner, *captureProvider) {
	t.Helper()
	fixtures = append(fixtures, loadAppFixture(t, "04-principal-no-work.json"))
	fake := provider.NewFake(fixtures...)
	fake.Recorder = db
	cap := &captureProvider{inner: fake}
	count := func(ctx context.Context, input []json.RawMessage) (int, error) {
		return fake.CountTokens(ctx, provider.PrincipalModel, input)
	}
	return cycle.NewRunner(db, cap, count, &fakePublisher{}), cap
}

// B-36: asking to sign runs the pending batch of your edits before the
// signoff opens; the checker's «por aclarar» is open by then (2.4 will
// count it as the signoff's pending). Nothing pending: no call.
func TestB36_AskingToSignRunsThePendingBatchFirst(t *testing.T) {
	db := seedCheckerProject(t)
	yourEdit(t, db, "sec-1", contradictingEdit)
	runner, cap := checkerRunner(t, db, checkerFixture(t, "contradicts-premise"))

	open, err := runner.RequestSignoff(context.Background(), "p1", "sec-1")
	if err != nil {
		t.Fatalf("RequestSignoff: %v", err)
	}
	if len(cap.checker) != 1 || !strings.Contains(briefOf(t, cap.checker[0]), contradictingEdit) {
		t.Fatalf("checker calls: %d, want the batch of your edit before the signoff", len(cap.checker))
	}
	oq, body := object(t, db, "oq-1")
	if oq.State != "open" || body["undeclared_by_agent"] != false {
		t.Fatalf("oq-1: %s %v, want open (your edit: no agent to declare it)", oq.State, body)
	}
	// The batch ran first: its probe events come before the signoff.
	var lastProbe, opened int
	for _, ev := range eventsOfType(t, db, store.EventProbe) {
		lastProbe = ev.Seq
	}
	for _, ev := range eventsOfType(t, db, store.EventStateChanged) {
		if len(ev.Data) > 0 && eventData(t, ev)["signoff_step"] == graph.StepRequested {
			opened = ev.Seq
		}
	}
	if !open.Opened || opened == 0 || lastProbe == 0 || lastProbe > opened {
		t.Fatalf("signoff opened %v at %d, last probe at %d: want the batch before the signoff", open.Opened, opened, lastProbe)
	}

	// The batch already judged: asking again calls nothing.
	if _, err := runner.RequestSignoff(context.Background(), "p1", "sec-1"); err != nil {
		t.Fatalf("RequestSignoff again: %v", err)
	}
	if len(cap.checker) != 1 {
		t.Fatalf("checker calls: %d, want 1 (nothing pending)", len(cap.checker))
	}
}

// B-50 with B-36: when the batch asked for by the signoff fails twice,
// the section is «sin comprobar» and the signoff does not open.
func TestB36_AskingToSignWithAFailedBatchLeavesItUnchecked(t *testing.T) {
	db := seedCheckerProject(t)
	yourEdit(t, db, "sec-1", contradictingEdit)
	runner, _ := checkerRunner(t, db, checkerFixture(t, "malformed"), checkerFixture(t, "malformed"))
	open, err := runner.RequestSignoff(context.Background(), "p1", "sec-1")
	if err != nil || open.Opened || !hasPending(open.Pendings, graph.PendingUnchecked, "") {
		t.Fatalf("RequestSignoff: %+v, %v; want blocked by unchecked", open, err)
	}
}

// fakeTimers stands in for time.AfterFunc: it records each timer's
// duration and lets the test fire it.
type fakeTimers struct {
	mu     sync.Mutex
	timers []*fakeTimer
}

type fakeTimer struct {
	d       time.Duration
	f       func()
	stopped bool
}

func (ft *fakeTimers) AfterFunc(d time.Duration, f func()) func() bool {
	ft.mu.Lock()
	defer ft.mu.Unlock()
	tm := &fakeTimer{d: d, f: f}
	ft.timers = append(ft.timers, tm)
	return func() bool {
		ft.mu.Lock()
		defer ft.mu.Unlock()
		was := !tm.stopped
		tm.stopped = true
		return was
	}
}

func (ft *fakeTimers) last(t *testing.T) *fakeTimer {
	t.Helper()
	ft.mu.Lock()
	defer ft.mu.Unlock()
	if len(ft.timers) == 0 {
		t.Fatal("no timer was started")
	}
	return ft.timers[len(ft.timers)-1]
}

// fireDuringPrincipal edits sec-1 and fires the timer while a turn is
// running: the batch must wait for the next trigger.
type fireDuringPrincipal struct {
	inner provider.Provider
	fire  func()
	once  sync.Once
}

func (f *fireDuringPrincipal) Respond(ctx context.Context, req provider.Request, onDelta func(provider.Delta)) (provider.Response, error) {
	if req.TurnKind != provider.TurnChecker {
		f.once.Do(f.fire)
	}
	return f.inner.Respond(ctx, req, onDelta)
}

// B-36: 60 s [inicial] after the last save of your edit, the batch goes
// to the checker; each save restarts the wait; a batch already judged
// does not go again; while a turn of the project runs, the batch waits
// for the next trigger; Close stops the pending wait.
func TestB36_SixtySecondsAfterTheLastSaveRunTheBatch(t *testing.T) {
	db := seedCheckerProject(t)
	timers := &fakeTimers{}
	fake := provider.NewFake(loadAppFixture(t, "04-principal-no-work.json"))
	fake.Recorder = db
	fire := func() {}
	cap := &captureProvider{inner: &fireDuringPrincipal{inner: fake, fire: func() { fire() }}}
	count := func(ctx context.Context, input []json.RawMessage) (int, error) {
		return fake.CountTokens(ctx, provider.PrincipalModel, input)
	}
	runner := cycle.NewRunner(db, cap, count, &fakePublisher{})
	runner.AfterFunc = timers.AfterFunc

	yourEdit(t, db, "sec-1", "La síntesis compara estudios. Primer guardado.")
	runner.EditSaved("p1")
	yourEdit(t, db, "sec-1", contradictingEdit)
	runner.EditSaved("p1")
	if len(timers.timers) != 2 || !timers.timers[0].stopped || timers.timers[1].stopped {
		t.Fatalf("timers: %d, want the second save to restart the first", len(timers.timers))
	}
	for _, tm := range timers.timers {
		if tm.d != cycle.EditBatchDelay || cycle.EditBatchDelay != 60*time.Second {
			t.Fatalf("timer: %v, want 60 s", tm.d)
		}
	}
	timers.last(t).f()
	if len(cap.checker) != 1 || !strings.Contains(briefOf(t, cap.checker[0]), contradictingEdit) {
		t.Fatalf("checker calls: %d, want the batch with the last save", len(cap.checker))
	}
	timers.last(t).f()
	if len(cap.checker) != 1 {
		t.Fatalf("checker calls: %d, want 1 (the batch was already judged)", len(cap.checker))
	}

	// A save during a turn: its timer fires while the turn runs, and the
	// batch waits; the next trigger (here, the timer again) runs it.
	const duringTurn = "La síntesis compara estudios. Editado durante el turno."
	fire = func() {
		yourEdit(t, db, "sec-1", duringTurn)
		runner.EditSaved("p1")
		timers.last(t).f()
	}
	if err := runner.RunTurn(context.Background(), "p1", "turn-1", "Sigue."); err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	if len(cap.checker) != 1 {
		t.Fatalf("checker calls: %d, want 1: the batch never runs while a turn runs", len(cap.checker))
	}
	timers.last(t).f()
	if len(cap.checker) != 2 || !strings.Contains(briefOf(t, cap.checker[1]), duringTurn) {
		t.Fatalf("checker calls: %d, want the waiting batch on the next trigger", len(cap.checker))
	}

	yourEdit(t, db, "sec-1", "La síntesis compara estudios. Último.")
	runner.EditSaved("p1")
	runner.Close()
	if !timers.last(t).stopped {
		t.Fatal("Close left the pending timer running")
	}
	runner.EditSaved("p1") // after Close: no new timer
	if n := len(timers.timers); n != 4 {
		t.Fatalf("timers: %d, want 4, one per save before Close", n)
	}
}

// B-36: asking to sign while a turn of the project runs, with edits the
// checker has not seen, is refused (turn_running) instead of opening a
// signoff on unchecked text.
func TestB36_AskingToSignDuringATurnWithEditsPendingIsRefused(t *testing.T) {
	db := seedCheckerProject(t)
	fake := provider.NewFake(loadAppFixture(t, "04-principal-no-work.json"))
	fake.Recorder = db
	var runner *cycle.Runner
	var signErr error
	inner := &fireDuringPrincipal{inner: fake, fire: func() {
		yourEdit(t, db, "sec-1", contradictingEdit)
		_, signErr = runner.RequestSignoff(context.Background(), "p1", "sec-1")
	}}
	count := func(ctx context.Context, input []json.RawMessage) (int, error) {
		return fake.CountTokens(ctx, provider.PrincipalModel, input)
	}
	runner = cycle.NewRunner(db, &captureProvider{inner: inner}, count, &fakePublisher{})
	if err := runner.RunTurn(context.Background(), "p1", "turn-1", "Sigue."); err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	if !errors.Is(signErr, cycle.ErrTurnRunning) {
		t.Fatalf("RequestSignoff during a turn: %v, want ErrTurnRunning", signErr)
	}
}
