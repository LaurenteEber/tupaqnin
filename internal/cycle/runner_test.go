package cycle_test

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

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

// captureProvider records what was actually sent to the provider
// and what it answered, so tests compare against the real calls.
type captureProvider struct {
	inner provider.Provider
	mu    sync.Mutex
	sent  [][]json.RawMessage
	got   []provider.Response
}

func (c *captureProvider) Respond(ctx context.Context, req provider.Request, onDelta func(provider.Delta)) (provider.Response, error) {
	c.mu.Lock()
	c.sent = append(c.sent, append([]json.RawMessage{}, req.Input...))
	c.mu.Unlock()
	resp, err := c.inner.Respond(ctx, req, onDelta)
	if err == nil {
		c.mu.Lock()
		c.got = append(c.got, resp)
		c.mu.Unlock()
	}
	return resp, err
}

func userItemForTest(text string) json.RawMessage {
	raw, _ := json.Marshal(map[string]any{
		"type": "message", "role": "user",
		"content": []map[string]string{{"type": "input_text", "text": text}},
	})
	return raw
}

func TestB7_ConversationRebuildsFromLastCallOnlyWithThreeMessages(t *testing.T) {
	db, err := store.Open(context.Background(), t.TempDir())
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := db.Writer.ExecContext(context.Background(),
		`INSERT INTO project (id, created_at) VALUES ('p1', '2026-10-28T00:00:00.000Z')`); err != nil {
		t.Fatalf("seed project: %v", err)
	}
	fake := provider.NewFake(loadAppFixture(t, "01-principal-valid.json"))
	fake.Recorder = db
	cap := &captureProvider{inner: fake}
	count := func(ctx context.Context, input []json.RawMessage) (int, error) {
		return fake.CountTokens(ctx, provider.PrincipalModel, input)
	}
	runner := cycle.NewRunner(db, cap, count, &fakePublisher{})

	for i, msg := range []string{"first message", "second message", "third message"} {
		turn := "turn-" + string(rune('1'+i))
		if err := runner.RunTurn(context.Background(), "p1", turn, msg); err != nil {
			t.Fatalf("RunTurn %d: %v", i+1, err)
		}
	}
	if len(cap.sent) != 3 {
		t.Fatalf("provider calls: got %d, want 3", len(cap.sent))
	}
	// What the third call must send: exactly what the second call
	// sent, plus what it answered, plus the new message. Nothing
	// from the first call may appear twice.
	want := append([]json.RawMessage{}, cap.sent[1]...)
	want = append(want, cap.got[1].RawItems...)
	want = append(want, userItemForTest("third message"))
	got := cap.sent[2]
	if len(got) != len(want) {
		t.Fatalf("third call items: got %d, want %d\ngot:  %s\nwant: %s",
			len(got), len(want), cap.sent[2], want)
	}
	for i := range want {
		if string(got[i]) != string(want[i]) {
			t.Fatalf("third call item %d:\ngot:  %s\nwant: %s", i, got[i], want[i])
		}
	}
}

// blockingProvider holds the first call until released, then
// delegates to the fake.
type blockingProvider struct {
	inner   provider.Provider
	entered chan struct{}
	release chan struct{}
	once    sync.Once
}

func (b *blockingProvider) Respond(ctx context.Context, req provider.Request, onDelta func(provider.Delta)) (provider.Response, error) {
	b.once.Do(func() {
		close(b.entered)
		<-b.release
	})
	return b.inner.Respond(ctx, req, onDelta)
}

func TestB7_SecondTurnWhileOneRunsIsRejectedPerProject(t *testing.T) {
	db, err := store.Open(context.Background(), t.TempDir())
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := db.Writer.ExecContext(context.Background(),
		`INSERT INTO project (id, created_at) VALUES ('p1', '2026-10-28T00:00:00.000Z')`); err != nil {
		t.Fatalf("seed project: %v", err)
	}
	fake := provider.NewFake(loadAppFixture(t, "01-principal-valid.json"))
	fake.Recorder = db
	blocking := &blockingProvider{inner: fake, entered: make(chan struct{}), release: make(chan struct{})}
	count := func(ctx context.Context, input []json.RawMessage) (int, error) {
		return fake.CountTokens(ctx, provider.PrincipalModel, input)
	}
	runner := cycle.NewRunner(db, blocking, count, &fakePublisher{})

	if _, err := runner.StartTurn("p1", "first"); err != nil {
		t.Fatalf("StartTurn 1: %v", err)
	}
	select {
	case <-blocking.entered:
	case <-time.After(5 * time.Second):
		t.Fatal("first turn never reached the provider")
	}
	// [EL DISEÑO NO LO DICE] one turn per project: the second is
	// rejected instead of running at the same time.
	if _, err := runner.StartTurn("p1", "second"); !errors.Is(err, cycle.ErrTurnRunning) {
		t.Fatalf("StartTurn 2: got %v, want ErrTurnRunning", err)
	}
	if err := runner.RunTurn(context.Background(), "p1", "turn-x", "second"); !errors.Is(err, cycle.ErrTurnRunning) {
		t.Fatalf("RunTurn while busy: got %v, want ErrTurnRunning", err)
	}
	close(blocking.release)
	// After the first turn ends, the lock is free again. Poll the
	// synchronous path until it accepts a turn.
	deadline := time.Now().Add(5 * time.Second)
	for {
		err := runner.RunTurn(context.Background(), "p1", "turn-3", "third")
		if err == nil {
			break
		}
		if !errors.Is(err, cycle.ErrTurnRunning) || time.Now().After(deadline) {
			t.Fatalf("RunTurn after release: %v", err)
		}
		time.Sleep(10 * time.Millisecond)
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
