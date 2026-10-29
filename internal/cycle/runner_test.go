package cycle_test

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/LaurenteEber/tupaqnin/internal/cycle"
	"github.com/LaurenteEber/tupaqnin/internal/provider"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

type fakePublisher struct {
	mu     sync.Mutex
	deltas []string
	steps  []json.RawMessage
	errs   []string
}

func (p *fakePublisher) BeginTurn(string, string) {}
func (p *fakePublisher) PublishDelta(_, _, text string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.deltas = append(p.deltas, text)
}
func (p *fakePublisher) PublishStep(_, _ string, step json.RawMessage) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.steps = append(p.steps, step)
}
func (p *fakePublisher) PublishError(_, _, code string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.errs = append(p.errs, code)
}
func (p *fakePublisher) step(t *testing.T) map[string]any {
	t.Helper()
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.steps) != 1 {
		t.Fatalf("published steps: got %d, want 1 (errors %v)", len(p.steps), p.errs)
	}
	var out map[string]any
	if err := json.Unmarshal(p.steps[0], &out); err != nil {
		t.Fatalf("step payload: %v", err)
	}
	return out
}

func loadAppFixture(t *testing.T, name string) provider.Fixture {
	t.Helper()
	fx, err := provider.LoadFixture(filepath.Join("..", "provider", "testdata", "app", name))
	if err != nil {
		t.Fatalf("LoadFixture: %v", err)
	}
	return fx
}

func newTestRunner(t *testing.T, fixtures ...provider.Fixture) (*store.DB, *cycle.Runner, *fakePublisher) {
	t.Helper()
	db, err := store.Open(context.Background(), t.TempDir())
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := db.Writer.ExecContext(context.Background(),
		`INSERT INTO project (id, created_at) VALUES ('p1', '2026-10-28T00:00:00.000Z')`); err != nil {
		t.Fatalf("seed project: %v", err)
	}
	fake := provider.NewFake(fixtures...)
	fake.Recorder = db
	pub := &fakePublisher{}
	// The counter is bound to the principal's model (B-5, backlog 0.9).
	count := func(ctx context.Context, input []json.RawMessage) (int, error) {
		return fake.CountTokens(ctx, provider.PrincipalModel, input)
	}
	runner := cycle.NewRunner(db, fake, count, pub)
	return db, runner, pub
}

func eventTypes(t *testing.T, db *store.DB) []store.EventType {
	t.Helper()
	events, err := db.EventsSince(context.Background(), "p1", 0)
	if err != nil {
		t.Fatalf("EventsSince: %v", err)
	}
	var out []store.EventType
	for _, ev := range events {
		out = append(out, ev.Type)
	}
	return out
}

func TestB7_TurnEndToEndWithTheFakeLeavesTheHistoryInOrder(t *testing.T) {
	db, runner, pub := newTestRunner(t, loadAppFixture(t, "01-principal-valid.json"))

	if err := runner.RunTurn(context.Background(), "p1", "turn-1", "Escribe la primera sección."); err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	got := eventTypes(t, db)
	want := []store.EventType{store.EventHumanMessage, store.EventStartup, store.EventStepEnded}
	if len(got) != len(want) {
		t.Fatalf("history: got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("history: got %v, want %v", got, want)
		}
	}
	if len(pub.deltas) == 0 {
		t.Fatal("no deltas published")
	}
	step := pub.step(t)
	if step["status"] != "accepted" || step["triage_line"] == "" || step["note"] == "" {
		t.Fatalf("step payload: got %v", step)
	}

	// A second message inside the 2 h window (B-7): no new startup,
	// and the conversation is resent byte-for-byte.
	if err := runner.RunTurn(context.Background(), "p1", "turn-2", "Sigue con la segunda."); err != nil {
		t.Fatalf("RunTurn 2: %v", err)
	}
	var startups int
	for _, typ := range eventTypes(t, db) {
		if typ == store.EventStartup {
			startups++
		}
	}
	if startups != 1 {
		t.Fatalf("startup events after second turn: got %d, want 1", startups)
	}
	events, err := db.EventsSince(context.Background(), "p1", 0)
	if err != nil {
		t.Fatalf("EventsSince: %v", err)
	}
	var instance string
	for _, ev := range events {
		if ev.Type == store.EventStartup {
			var data map[string]any
			_ = json.Unmarshal(ev.Data, &data)
			instance, _ = data["instance_id"].(string)
		}
	}
	calls, err := db.ModelCalls(context.Background(), "p1", instance)
	if err != nil {
		t.Fatalf("ModelCalls: %v", err)
	}
	if len(calls) != 2 {
		t.Fatalf("model calls of the instance: got %d, want 2", len(calls))
	}
	if !strings.Contains(string(calls[1].InputItems), "fixture-encrypted-content") {
		t.Fatal("second call did not resend the first call's raw items byte-for-byte")
	}
}

func TestB2_InvalidRecordedOutputRetriesOnceThenAgentPending(t *testing.T) {
	invalid := loadAppFixture(t, "02-principal-invalid.json")
	db, runner, pub := newTestRunner(t, invalid, invalid)

	if err := runner.RunTurn(context.Background(), "p1", "turn-1", "Escribe algo."); err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	step := pub.step(t)
	if step["status"] != "agent_pending" {
		t.Fatalf("step payload: got %v, want agent_pending", step)
	}
	var retries, ended int
	for _, typ := range eventTypes(t, db) {
		switch typ {
		case store.EventOutputRetry:
			retries++
		case store.EventStepEnded:
			ended++
		}
	}
	if retries != 2 || ended != 0 {
		t.Fatalf("output_retry=%d step_ended=%d, want 2 and 0", retries, ended)
	}
}

func TestB2_InvalidThenValidOutputIsAcceptedAfterRetry(t *testing.T) {
	invalid := loadAppFixture(t, "02-principal-invalid.json")
	valid := loadAppFixture(t, "01-principal-valid.json")
	db, runner, pub := newTestRunner(t, invalid, valid)

	if err := runner.RunTurn(context.Background(), "p1", "turn-1", "Escribe algo."); err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	if step := pub.step(t); step["status"] != "accepted" {
		t.Fatalf("step payload: got %v, want accepted after retry", step)
	}
	var retries, ended int
	for _, typ := range eventTypes(t, db) {
		switch typ {
		case store.EventOutputRetry:
			retries++
		case store.EventStepEnded:
			ended++
		}
	}
	if retries != 1 || ended != 1 {
		t.Fatalf("output_retry=%d step_ended=%d, want 1 and 1", retries, ended)
	}
}
