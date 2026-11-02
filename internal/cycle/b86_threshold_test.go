package cycle_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/LaurenteEber/tupaqnin/internal/cycle"
	"github.com/LaurenteEber/tupaqnin/internal/provider"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// b86Window is the small window of these tests (backlog-r4 4.5): 60 % is
// 6000 tokens and the startup's cap 1500.
const b86Window = 10000

// failingSteps answers a chosen principal step call, counted from 1, with an
// error of its own instead of reaching the fake.
type failingSteps struct {
	provider.Provider
	calls int
	fail  map[int]error
}

func (f *failingSteps) Respond(ctx context.Context, req provider.Request, onDelta func(provider.Delta)) (provider.Response, error) {
	if req.TurnKind == provider.TurnStep {
		f.calls++
		if err := f.fail[f.calls]; err != nil {
			return provider.Response{}, err
		}
	}
	return f.Provider.Respond(ctx, req, onDelta)
}

func b86Runner(t *testing.T, db *store.DB, fail map[int]error, fixtures ...provider.Fixture) (*cycle.Runner, *captureProvider, *fakePublisher) {
	t.Helper()
	fake := provider.NewFake(fixtures...)
	fake.Recorder = db
	cap := &captureProvider{inner: &failingSteps{Provider: fake, fail: fail}}
	count := func(ctx context.Context, input []json.RawMessage) (int, error) {
		return fake.CountTokens(ctx, provider.PrincipalModel, input)
	}
	pub := &fakePublisher{}
	runner := cycle.NewRunner(db, cap, count, pub)
	cycle.SetContextWindowForTest(runner, b86Window)
	return runner, cap, pub
}

// using is a recording that reports these input tokens.
func using(fx provider.Fixture, tokens int) provider.Fixture {
	fx.Usage = provider.Usage{InputTokens: tokens}
	return fx
}

func b86NoWork(t *testing.T, tokens int) provider.Fixture {
	t.Helper()
	return using(loadAppFixture(t, "04-principal-no-work.json"), tokens)
}

func startupReasons(t *testing.T, db *store.DB) []string {
	t.Helper()
	var out []string
	for _, ev := range eventsOfType(t, db, store.EventStartup) {
		var data struct {
			Reason string `json:"reason"`
		}
		if err := json.Unmarshal(ev.Data, &data); err != nil {
			t.Fatal(err)
		}
		out = append(out, data.Reason)
	}
	return out
}

func wantReasons(t *testing.T, db *store.DB, want ...string) {
	t.Helper()
	if got := strings.Join(startupReasons(t, db), ","); got != strings.Join(want, ",") {
		t.Fatalf("startups: %s, want %s", got, strings.Join(want, ","))
	}
}

// isStartup: nothing the model answered is in the input, so nothing of the old
// conversation came along.
func isStartup(t *testing.T, input []json.RawMessage) {
	t.Helper()
	for _, item := range input {
		var m struct {
			Role string `json:"role"`
		}
		_ = json.Unmarshal(item, &m)
		if m.Role != "user" {
			t.Fatalf("not a startup, it carries an answer: %s", item)
		}
	}
	if !strings.HasPrefix(firstText(t, input), "Core\n") {
		t.Fatalf("not a startup: %s", input)
	}
}

func firstText(t *testing.T, input []json.RawMessage) string {
	t.Helper()
	var m struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(input[0], &m); err != nil || len(m.Content) == 0 {
		t.Fatalf("first item: %s", input[0])
	}
	return m.Content[0].Text
}

func callRows(t *testing.T, db *store.DB) int {
	t.Helper()
	var n int
	if err := db.Reader.QueryRow(`SELECT COUNT(*) FROM model_call WHERE role = 'principal'`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// «con una ventana de prueba chica, el paso que cruza el 60 % termina y el
// siguiente arranca con startup threshold y su arranque (≤ 15 %)».
func TestB86_TheStepThatCrosses60PercentEndsAndTheNextStartsWithThreshold(t *testing.T) {
	db := openTemp(t)
	runner, cap, _ := b86Runner(t, db, nil, using(textFixture(t, pendingFigureStep), 6000), b86NoWork(t, 100))
	if err := runner.RunTurn(context.Background(), "p1", "turn-1", "Escribe."); err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	if len(cap.sent) != 2 {
		t.Fatalf("provider calls: %d, want 2", len(cap.sent))
	}
	wantReasons(t, db, "new_session", "threshold")
	if n := len(stepEndedData(t, db)); n != 2 {
		t.Fatalf("step_ended: %d, want 2: the crossing step ends and applies", n)
	}
	if sections, _ := db.ListObjects(context.Background(), "p1", store.ObjectSection); len(sections) != 1 {
		t.Fatalf("sections: %d, want the crossing step's", len(sections))
	}
	next := cap.sent[1]
	isStartup(t, next)
	if !strings.HasSuffix(lastUserText(t, next), cycle.ContinueMessage) {
		t.Fatalf("the chained message: %q", lastUserText(t, next))
	}
	tokens := 0
	for _, item := range next[:len(next)-1] {
		tokens += len(item) / 4
	}
	if tokens > b86Window*15/100 {
		t.Fatalf("startup: %d tokens, over 15 %% of %d", tokens, b86Window)
	}
}

// A normal step below the threshold keeps its conversation: same instance, the
// answer resent, no second startup.
func TestB86_BelowTheThresholdTheConversationGoesOn(t *testing.T) {
	db := openTemp(t)
	runner, cap, _ := b86Runner(t, db, nil, using(textFixture(t, pendingFigureStep), 5999), b86NoWork(t, 100))
	if err := runner.RunTurn(context.Background(), "p1", "turn-1", "Escribe."); err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	wantReasons(t, db, "new_session")
	raw, _ := json.Marshal(cap.sent[1])
	if !strings.Contains(string(raw), "Escrib\\u00ed la muestra") && !strings.Contains(string(raw), "Escribí la muestra") {
		t.Fatalf("the conversation did not go on: %s", raw)
	}
}

// A conversation over two hours old starts a new session, over the threshold
// or not: there is nothing to clean.
func TestB7_ADeadConversationOverTheThresholdIsANewSession(t *testing.T) {
	db := openTemp(t)
	runner, _, _ := b86Runner(t, db, nil, b86NoWork(t, 9000), b86NoWork(t, 100))
	if err := runner.RunTurn(context.Background(), "p1", "turn-1", "Hola."); err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	if !runner.NeedsReset(context.Background(), "p1") {
		t.Fatal("NeedsReset: false on a live conversation at 90 %")
	}
	runner.Now = func() time.Time { return time.Now().Add(3 * time.Hour) }
	if runner.NeedsReset(context.Background(), "p1") {
		t.Fatal("NeedsReset: true on a dead conversation")
	}
	if err := runner.RunTurn(context.Background(), "p1", "turn-2", "Otro día."); err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	wantReasons(t, db, "new_session", "new_session")
}

// «con una firma en curso en el proyecto, la limpieza espera a que termine»: the
// turn ends and releases the slot, no step runs, nothing cleans; when the
// signoff closes the next step cleans, with your message.
func TestB86_ASignoffInProgressDelaysTheCleanup(t *testing.T) {
	db := openTemp(t)
	seedSignable(t, db, false)
	gate := cycle.NewGate(db)
	sign := openSignoff(t, gate)
	runner, cap, pub := b86Runner(t, db, nil, b86NoWork(t, 7000), b86NoWork(t, 100))
	ctx := context.Background()
	if err := runner.RunTurn(ctx, "p1", "turn-1", "Hola."); err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	if err := runner.RunTurn(ctx, "p1", "turn-2", "Mensaje en la firma."); err != nil {
		t.Fatalf("RunTurn during the signoff: %v", err)
	}
	if len(cap.sent) != 1 {
		t.Fatalf("provider calls: %d, want 1: no step during the signoff over the threshold", len(cap.sent))
	}
	wantReasons(t, db, "new_session")
	if len(cap.reports) != 0 {
		t.Fatalf("reports: %d, the held turn is not a stop", len(cap.reports))
	}
	if step := pub.stepAt(t, 1, 2); step["status"] != "accepted" || step["stop_reason"] != nil {
		t.Fatalf("the held turn's step: %v", step)
	}
	if !runner.NeedsReset(ctx, "p1") {
		t.Fatal("NeedsReset: false while the cleanup waits")
	}
	if err := gate.LeaveSignoff(ctx, "p1", sign); err != nil {
		t.Fatalf("LeaveSignoff: %v", err)
	}
	if err := runner.RunTurn(ctx, "p1", "turn-3", "Ya salí."); err != nil {
		t.Fatalf("RunTurn after the signoff: %v", err)
	}
	wantReasons(t, db, "new_session", "threshold")
	if len(cap.sent) != 2 {
		t.Fatalf("provider calls: %d, want 2", len(cap.sent))
	}
	isStartup(t, cap.sent[1])
	raw, _ := json.Marshal(cap.sent[1])
	if !strings.Contains(string(raw), "Mensaje en la firma.") || !strings.Contains(string(raw), "Ya sal") {
		t.Fatalf("the messages must reach the new principal: %s", raw)
	}
}

// A chain that crosses during a signoff launches no new step and does not write
// its report yet: the next chain's report covers the work.
func TestB86_AChainOverTheThresholdDuringASignoffEndsTheTurn(t *testing.T) {
	db := openTemp(t)
	seedSignable(t, db, false)
	sign := openSignoff(t, cycle.NewGate(db))
	runner, cap, pub := b86Runner(t, db, nil, using(textFixture(t, pendingFigureStep), 7000), b86NoWork(t, 100))
	ctx := context.Background()
	if err := runner.RunTurn(ctx, "p1", "turn-1", "Escribe."); err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	if len(cap.sent) != 1 || len(cap.reports) != 0 {
		t.Fatalf("calls: %d steps, %d reports; want 1 and 0", len(cap.sent), len(cap.reports))
	}
	wantReasons(t, db, "new_session")
	if step := pub.step(t); step["triage_line"] == "" || step["stop_reason"] != nil {
		t.Fatalf("the turn ends with its last step: %v", step)
	}
	if err := cycle.NewGate(db).LeaveSignoff(ctx, "p1", sign); err != nil {
		t.Fatal(err)
	}
	if err := runner.RunTurn(ctx, "p1", "turn-2", "Sigue."); err != nil {
		t.Fatal(err)
	}
	wantReasons(t, db, "new_session", "threshold")
	if len(cap.reports) != 1 {
		t.Fatalf("reports: %d, want the work's one", len(cap.reports))
	}
}

// «tras una edición tuya con el contexto ya sobre el 60 %, el siguiente paso
// arranca con startup threshold», which carries the edit.
func TestB86_AfterYourEditOverTheThresholdTheNextStepStartsWithThreshold(t *testing.T) {
	db := openTemp(t)
	seedSignable(t, db, false)
	runner, cap, _ := b86Runner(t, db, nil, b86NoWork(t, 6500), b86NoWork(t, 100))
	ctx := context.Background()
	if err := runner.RunTurn(ctx, "p1", "turn-1", "Hola."); err != nil {
		t.Fatal(err)
	}
	wantReasons(t, db, "new_session")
	editSectionOne(t, db, " Línea tuya.")
	if err := runner.RunTurn(ctx, "p1", "turn-2", "Mira mi edición."); err != nil {
		t.Fatal(err)
	}
	wantReasons(t, db, "new_session", "threshold")
	isStartup(t, cap.sent[1])
	raw, _ := json.Marshal(cap.sent[1])
	if !strings.Contains(string(raw), "What changed") || !strings.Contains(string(raw), "- sec-1 (section)") {
		t.Fatalf("the startup must carry the edit: %s", raw)
	}
}

func editSectionOne(t *testing.T, db *store.DB, suffix string) {
	t.Helper()
	sec, err := db.GetObject(context.Background(), "p1", "sec-1")
	if err != nil {
		t.Fatal(err)
	}
	var b map[string]any
	_ = json.Unmarshal(sec.Body, &b)
	b["text"] = b["text"].(string) + suffix
	err = db.Write(context.Background(), "p1", func(tx *store.Tx) error {
		_, err := tx.UpdateObject(sec.ID, sec.CurrentVersion, sec.State, b, store.EventInput{Type: store.EventEdit, Actor: "human"})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
}

// «un paso que devuelve error de largo se descarta, se limpia y se reintenta una
// vez»: what the discarded step delivered reaches the retry's startup.
func TestB86_AStepThatFillsTheWindowIsDiscardedCleanedAndRetriedOnce(t *testing.T) {
	db := openTemp(t)
	seedSignable(t, db, false)
	runner, cap, _ := b86Runner(t, db, map[int]error{2: provider.ErrContextFull}, b86NoWork(t, 100))
	ctx := context.Background()
	if err := runner.RunTurn(ctx, "p1", "turn-1", "Hola."); err != nil {
		t.Fatal(err)
	}
	editSectionOne(t, db, " Línea tuya.")
	if err := runner.RunTurn(ctx, "p1", "turn-2", "Sigue."); err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	if len(cap.sent) != 3 {
		t.Fatalf("provider calls: %d, want 3 (step, discarded, retry)", len(cap.sent))
	}
	raw, _ := json.Marshal(cap.sent[1])
	if !strings.Contains(string(raw), "Changed since it was delivered to you (sec-1") {
		t.Fatalf("setup: the discarded step must have delivered the edit: %s", raw)
	}
	wantReasons(t, db, "new_session", "threshold")
	if callRows(t, db) != 2 {
		t.Fatalf("model_call rows: %d, the discarded step leaves none", callRows(t, db))
	}
	if n := len(eventsOfType(t, db, store.EventOutputRetry)); n != 0 {
		t.Fatalf("output_retry: %d, a retry that passed leaves no pending", n)
	}
	retry := cap.sent[2]
	isStartup(t, retry)
	raw, _ = json.Marshal(retry)
	if !strings.Contains(string(raw), "- sec-1 (section)") {
		t.Fatalf("what the discarded step delivered must be in the startup: %s", raw)
	}
	if !strings.Contains(string(raw), "Sigue.") {
		t.Fatalf("your message must be in the startup: %s", raw)
	}
}

// «al segundo fallo queda pendiente del agente», which the next step's startup
// carries; the error is the app's text, never the provider's.
func TestB86_ASecondFullWindowLeavesAnAgentPending(t *testing.T) {
	db := openTemp(t)
	runner, cap, pub := b86Runner(t, db, map[int]error{1: provider.ErrContextFull, 2: provider.ErrContextFull}, b86NoWork(t, 100))
	ctx := context.Background()
	if err := runner.RunTurn(ctx, "p1", "turn-1", "Hola."); err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	if len(cap.sent) != 2 {
		t.Fatalf("provider calls: %d, want 2", len(cap.sent))
	}
	wantReasons(t, db, "new_session", "threshold")
	retries := eventsOfType(t, db, store.EventOutputRetry)
	if len(retries) != 1 {
		t.Fatalf("output_retry: %d", len(retries))
	}
	data := eventData(t, retries[0])
	if data["result"] != "agent_pending" || data["turn_kind"] != "step" {
		t.Fatalf("output_retry: %v", data)
	}
	if step := pub.step(t); step["status"] != "agent_pending" || step["stop_reason"] != "agent_pending" {
		t.Fatalf("published step: %v", step)
	}
	// The instance that never answered has no conversation to go on with.
	if err := runner.RunTurn(ctx, "p1", "turn-2", "Otra vez."); err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	isStartup(t, cap.sent[2])
	raw, _ := json.Marshal(cap.sent[2])
	if !strings.Contains(string(raw), "Agent pending: "+cycle.ContextFullPending) {
		t.Fatalf("the next startup must carry the pending: %s", raw)
	}
}

// During a signoff the full step cannot be cleaned: it stays pending (bloque-2
// §4: never during a signoff).
func TestB86_AFullWindowDuringASignoffStaysPendingWithoutCleaning(t *testing.T) {
	db := openTemp(t)
	seedSignable(t, db, false)
	openSignoff(t, cycle.NewGate(db))
	runner, cap, _ := b86Runner(t, db, map[int]error{1: provider.ErrContextFull}, b86NoWork(t, 100))
	if err := runner.RunTurn(context.Background(), "p1", "turn-1", "Hola."); err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	if len(cap.sent) != 1 {
		t.Fatalf("provider calls: %d, want 1", len(cap.sent))
	}
	wantReasons(t, db, "new_session")
	if retries := eventsOfType(t, db, store.EventOutputRetry); len(retries) != 1 || eventData(t, retries[0])["result"] != "agent_pending" {
		t.Fatalf("output_retry: %v", retries)
	}
}

// B-89: a cap refusal is not a full window: nothing cleans, no retry, the turn
// stops at the cap.
func TestB89_ACapRefusalIsNotRetriedAsAFullWindow(t *testing.T) {
	db := openTemp(t)
	refused := &provider.CapError{Reason: provider.CapProjectStop}
	runner, cap, pub := b86Runner(t, db, map[int]error{1: refused}, b86NoWork(t, 100))
	if err := runner.RunTurn(context.Background(), "p1", "turn-1", "Hola."); err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	if len(cap.sent) != 1 {
		t.Fatalf("provider calls: %d, want 1", len(cap.sent))
	}
	wantReasons(t, db, "new_session")
	if step := pub.step(t); step["stop_reason"] != provider.CapProjectStop {
		t.Fatalf("published step: %v", step)
	}
	if errors.Is(refused, provider.ErrContextFull) {
		t.Fatal("a cap reads as a full window")
	}
}

// afterCrossing is the first principal call after the crossing step: the next
// step or, when that step stopped the chain, its stop report.
func afterCrossing(t *testing.T, cap *captureProvider) []json.RawMessage {
	t.Helper()
	if len(cap.sent) > 1 {
		return cap.sent[1]
	}
	if len(cap.reports) == 0 {
		t.Fatal("no call after the crossing step")
	}
	return cap.reports[0].Input
}

// «tras la limpieza, el principal tiene en su arranque las tarjetas y avisos que
// abrió el último paso»: bloque-2 §4's order, and delivered once.
func TestB86_TheStartupAfterTheCleanupHasTheCardsAndNoticesOfTheLastStep(t *testing.T) {
	db := seedStepProject(t)
	figure := using(textFixture(t, string(stepFile(t, "figure-datum-loose"))), 7000)
	runner, cap, _ := b86Runner(t, db, nil, figure, b86NoWork(t, 100), b86NoWork(t, 100))
	ctx := context.Background()
	if err := runner.RunTurn(ctx, "p1", "turn-1", "Escribe."); err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	if cardsOf(t, db)["dec-3"] != "open" {
		t.Fatalf("setup: cards %v", cardsOf(t, db))
	}
	wantReasons(t, db, "new_session", "threshold")
	input := afterCrossing(t, cap)
	isStartup(t, input)
	raw, _ := json.Marshal(input)
	if !strings.Contains(string(raw), "Open cards: card-1") || !strings.Contains(string(raw), "- card-1 (card)") {
		t.Fatalf("the startup must carry the step's card: %s", raw)
	}
	if err := runner.RunTurn(ctx, "p1", "turn-2", "Sigue."); err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	last, _ := json.Marshal(cap.sent[len(cap.sent)-1])
	if strings.Contains(string(last), "Changed since it was delivered to you (card-1") {
		t.Fatalf("the card was delivered again: %s", last)
	}
}

// B-73 and the same order: an open proposal the crossing step left reaches the
// new principal once, in the startup, and never again in «lo que cambió».
func TestB86_AProposalOfTheLastStepIsNeitherLostNorRepeatedAfterTheCleanup(t *testing.T) {
	db := openTemp(t)
	seedSignable(t, db, false)
	runner, cap, _ := b86Runner(t, db, nil, using(stepFixture("on-signed", onSigned), 7000), b86NoWork(t, 100), b86NoWork(t, 100))
	label := proposeThenRepeatAnchor(t, db, runner)
	wantReasons(t, db, "new_session", "threshold")
	input := afterCrossing(t, cap)
	isStartup(t, input)
	raw, _ := json.Marshal(input)
	if !strings.Contains(string(raw), "Open proposals: "+label) || !strings.Contains(string(raw), "- "+label+" (proposal)") {
		t.Fatalf("the startup must carry %s: %s", label, raw)
	}
	if err := runner.RunTurn(context.Background(), "p1", "turn-2", "Sigue."); err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	last, _ := json.Marshal(cap.sent[len(cap.sent)-1])
	if strings.Contains(string(last), "Changed since it was delivered to you ("+label) {
		t.Fatalf("the proposal was delivered again: %s", last)
	}
}
