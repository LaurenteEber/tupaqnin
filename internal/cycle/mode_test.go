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

// Piece 4.4 (backlog-r4): «continúa hasta terminar», B-83, B-84, B-85.

// holdProvider holds a chosen call of a turn kind, counted from 1, until the
// test lets it go, honouring its context (Close cancels it).
type holdProvider struct {
	provider.Provider
	kind    provider.TurnKind
	at      int
	mu      sync.Mutex
	calls   int
	entered chan struct{}
	release chan struct{}
	fail    map[int]error
}

func (h *holdProvider) Respond(ctx context.Context, req provider.Request, onDelta func(provider.Delta)) (provider.Response, error) {
	if req.TurnKind == h.kind {
		h.mu.Lock()
		h.calls++
		n := h.calls
		h.mu.Unlock()
		if err := h.fail[n]; err != nil {
			return provider.Response{}, err
		}
		if n == h.at {
			close(h.entered)
			select {
			case <-h.release:
			case <-ctx.Done():
				return provider.Response{}, ctx.Err()
			}
		}
	}
	return h.Provider.Respond(ctx, req, onDelta)
}

func newModeRunner(t *testing.T, hold *holdProvider, fixtures ...provider.Fixture) (*store.DB, *cycle.Runner, *captureProvider, *fakePublisher) {
	t.Helper()
	db := openTemp(t)
	runner, cap, pub := modeRunnerOn(t, db, hold, fixtures...)
	return db, runner, cap, pub
}

func modeRunnerOn(t *testing.T, db *store.DB, hold *holdProvider, fixtures ...provider.Fixture) (*cycle.Runner, *captureProvider, *fakePublisher) {
	t.Helper()
	fake := provider.NewFake(fixtures...)
	fake.Recorder = db
	var inner provider.Provider = fake
	if hold != nil {
		hold.Provider = fake
		if hold.entered == nil {
			hold.entered, hold.release = make(chan struct{}), make(chan struct{})
		}
		inner = hold
	}
	cap := &captureProvider{inner: inner}
	count := func(ctx context.Context, input []json.RawMessage) (int, error) {
		return fake.CountTokens(ctx, provider.PrincipalModel, input)
	}
	pub := &fakePublisher{}
	runner := cycle.NewRunner(db, cap, count, pub)
	t.Cleanup(runner.Close)
	return runner, cap, pub
}

// modeOn activates the mode in the history, for a test that drives the turn
// itself.
func modeOn(t *testing.T, db *store.DB, message string) {
	t.Helper()
	err := db.Write(context.Background(), "p1", func(tx *store.Tx) error {
		_, err := tx.AppendEvent(store.EventInput{Type: store.EventModeSet, Actor: "human",
			Data: map[string]any{"mode": graph.ModeRunToCompletion, "message": message}})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
}

func currentMode(t *testing.T, db *store.DB) string {
	t.Helper()
	mode, err := graph.CurrentMode(context.Background(), db, "p1")
	if err != nil {
		t.Fatal(err)
	}
	return mode.Mode
}

// wantModeFinished asserts the end of the mode everywhere it shows: the one
// report, the mode_set of the app, and the stream's last step.
func wantModeFinished(t *testing.T, db *store.DB, pub *fakePublisher, cause, capName string) cycle.StopReport {
	t.Helper()
	report := oneReport(t, db)
	if report.StopReason != cycle.StopModeFinished || report.Cause != cause || report.Cap != capName {
		t.Fatalf("report: reason %s cause %s cap %s; want mode_finished %s %s", report.StopReason, report.Cause, report.Cap, cause, capName)
	}
	if got := currentMode(t, db); got != graph.ModeNormal {
		t.Fatalf("mode after the end: %s", got)
	}
	sets := eventsOfType(t, db, store.EventModeSet)
	last := sets[len(sets)-1]
	var data map[string]any
	_ = json.Unmarshal(last.Data, &data)
	if last.Actor != "app" || data["mode"] != graph.ModeNormal || data["cause"] != cause {
		t.Fatalf("last mode_set: %s %v", last.Actor, data)
	}
	if capName != "" && data["cap"] != capName {
		t.Fatalf("mode_set cap: %v", data)
	}
	pub.mu.Lock()
	raw := pub.steps[len(pub.steps)-1]
	pub.mu.Unlock()
	var step map[string]any
	if err := json.Unmarshal(raw, &step); err != nil {
		t.Fatal(err)
	}
	if step["stop_reason"] != "mode_finished" || step["cause"] != cause {
		t.Fatalf("stream's last step: %v", step)
	}
	if capName != "" && step["cap"] != capName {
		t.Fatalf("stream's cap: %v", step)
	}
	return report
}

func noStepStops(t *testing.T, db *store.DB) {
	t.Helper()
	for _, d := range stepEndedData(t, db) {
		if d["stop_reason"] != nil {
			t.Fatalf("a step_ended of stop in the mode: %v", d)
		}
	}
}

func fiveDecisionsStep(t *testing.T) string {
	t.Helper()
	var decisions []map[string]any
	for i := 2; i <= 6; i++ {
		decisions = append(decisions, map[string]any{"id": "new-" + string(rune('0'+i)),
			"statement": "Regla " + string(rune('0'+i)) + " del alcance.", "instead_of": "Otra regla.",
			"would_choose_if": "Cambiara el pedido.", "assumption": false, "used_in_sections": []string{"new-1"}})
	}
	raw, err := json.Marshal(map[string]any{"stretch": "build", "triage_line": "Construir: fijo el alcance.",
		"closing_risk": "none", "note": "Registré cinco reglas.",
		"new_text": []map[string]any{{"section_id": "new-1", "base_version": 0, "anchor": "", "claim_origin": "material",
			"operation": "insert_after", "text": "# Alcance\n\nEl alcance sigue cinco reglas."}},
		"proposed_roles": []map[string]any{{"section_id": "new-1", "role": "other", "title": "Alcance"}},
		"new_decisions":  decisions})
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

// «con el modo activo, una sección que queda lista no para la cadena y 5
// decisiones sin ver tampoco», with recorded outputs.
func TestB83_ASectionReadyAndFiveUnseenDecisionsDoNotStopTheChain(t *testing.T) {
	db, runner, cap, pub := newModeRunner(t, nil,
		textFixture(t, pendingFigureStep),              // keeps a pending, so «only yours» never holds
		loadAppFixture(t, "01-principal-valid.json"),   // a recorded section with no pendings
		textFixture(t, fiveDecisionsStep(t)),           // five unseen decisions
		loadAppFixture(t, "04-principal-no-work.json")) // the end
	modeOn(t, db, "Sigue hasta terminar.")
	if err := runner.RunTurn(context.Background(), "p1", "turn-1", "Escribe."); err != nil {
		t.Fatal(err)
	}
	if len(cap.sent) != 4 {
		t.Fatalf("step calls: %d, want 4: neither a ready section nor 5 decisions stop it", len(cap.sent))
	}
	noStepStops(t, db)
	report := wantModeFinished(t, db, pub, "no_work", "")
	if len(report.Decisions) < 5 {
		t.Fatalf("the report still shows what you did not see: %d decisions", len(report.Decisions))
	}
}

// «la cadena para cuando solo quedan tarjetas y «por aclarar», con mode_finished
// y la defensa de cada sección que la necesita».
func TestB85_TheChainStopsWhenOnlyYoursIsLeftWithTheDefenseOfEachSectionThatNeedsIt(t *testing.T) {
	db := openTemp(t)
	seedSignable(t, db, false) // sec-1 is a conclusion: it needs its defense
	// sec-2 has no conclusion and no reader: it needs none. The step's sec-3 is a
	// conclusion with its card open: it needs one too.
	err := db.Write(context.Background(), "p1", func(tx *store.Tx) error {
		_, err := tx.CreateObject(store.ObjectSection, "draft", map[string]any{"title": "Contexto", "text": "Un contexto.",
			"role": map[string]any{"value": "other", "state": "confirmed"}, "open_changes": []any{}},
			store.EventInput{Type: store.EventStateChanged, Actor: "agent"})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	runner, cap, pub := modeRunnerOn(t, db, nil, loadAppFixture(t, "01b-principal-critical-decision.json"))
	modeOn(t, db, "")
	if err := runner.RunTurn(context.Background(), "p1", "turn-1", "Escribe la regla."); err != nil {
		t.Fatal(err)
	}
	if len(cap.sent) != 1 || len(cap.reports) != 1 {
		t.Fatalf("steps %d, reports %d: want 1 and 1", len(cap.sent), len(cap.reports))
	}
	report := wantModeFinished(t, db, pub, "human_only", "")
	if len(report.Open) == 0 {
		t.Fatalf("the open card must be in the report: %+v", report.Open)
	}
	var defended []string
	for _, j := range cap.judges {
		defended = append(defended, j.DefenseSection)
	}
	if strings.Join(defended, ",") != "sec-1,sec-3" {
		t.Fatalf("defense calls: %v, want sec-1 and sec-3", defended)
	}
	questions, err := db.ListObjects(context.Background(), "p1", store.ObjectAudienceQuestion)
	if err != nil || len(questions) == 0 {
		t.Fatalf("audience questions: %d %v", len(questions), err)
	}
}

// «a los 12 pasos se abre otro turno solo; a los 60 para».
func TestB83_AtTwelveStepsAnotherTurnOpensOnItsOwnAndAtSixtyItStops(t *testing.T) {
	db, runner, cap, pub := newModeRunner(t, nil, textFixture(t, pendingFigureStep))
	modeOn(t, db, "")
	if err := runner.RunTurn(context.Background(), "p1", "turn-1", "Escribe."); err != nil {
		t.Fatal(err)
	}
	if cycle.ModeMaxSteps != 60 || len(cap.sent) != 60 {
		t.Fatalf("ModeMaxSteps %d, step calls %d: want 60", cycle.ModeMaxSteps, len(cap.sent))
	}
	noStepStops(t, db)
	byTurn := map[string]int{}
	for _, ev := range eventsOfType(t, db, store.EventStepEnded) {
		byTurn[ev.TurnID]++
	}
	if len(byTurn) != 5 {
		t.Fatalf("turns: %v, want 5 of 12", byTurn)
	}
	for id, n := range byTurn {
		if n != 12 {
			t.Fatalf("turn %s: %d steps", id, n)
		}
	}
	if pub.begun != 4 {
		t.Fatalf("turns the app opened: %d, want 4", pub.begun)
	}
	for i := 0; i < 4; i++ {
		step := pub.stepAt(t, i, 5)
		if step["next_turn_id"] == nil || step["stop_reason"] != nil {
			t.Fatalf("self-opened cut %d: %v", i, step)
		}
	}
	if step := pub.stepAt(t, 4, 5); step["next_turn_id"] != nil {
		t.Fatalf("the last step opens nothing: %v", step)
	}
	wantModeFinished(t, db, pub, "mode_cap", "")
}

// «un mensaje tuyo durante el modo se encola (no 409), entra al siguiente paso y
// el modo sigue», also when that step only answers it.
func TestB83_YourMessageDuringTheModeQueuesEntersTheNextStepOnceAndTheModeGoesOn(t *testing.T) {
	hold := &holdProvider{kind: provider.TurnStep, at: 1}
	const answerOnly = `{"stretch":"build","triage_line":"Respondo.","closing_risk":"none","note":"Solo respondí.","reply":"Sí."}`
	db, runner, cap, pub := newModeRunner(t, hold,
		textFixture(t, pendingFigureStep), textFixture(t, answerOnly), textFixture(t, pendingFigureStep),
		loadAppFixture(t, "04-principal-no-work.json"))
	if _, err := runner.SetMode(context.Background(), "p1", graph.ModeRunToCompletion, ""); err != nil {
		t.Fatal(err)
	}
	<-hold.entered
	const message = "Usa la serie de 2024, por favor."
	turn, err := runner.StartTurn("p1", message)
	if err != nil || turn != "" {
		t.Fatalf("StartTurn during the mode: %q %v, want queued", turn, err)
	}
	var data map[string]any
	msgs := eventsOfType(t, db, store.EventHumanMessage)
	if len(msgs) != 1 {
		t.Fatalf("messages in the history: %d", len(msgs))
	}
	_ = json.Unmarshal(msgs[0].Data, &data)
	if data["text"] != message {
		t.Fatalf("literal message: %v", data)
	}
	close(hold.release)
	cycle.WaitTurnsForTest(runner)
	if len(cap.sent) != 4 {
		t.Fatalf("step calls: %d, want 4: answering it does not end the mode", len(cap.sent))
	}
	if strings.Contains(asJSON(t, cap.sent[0]), message) {
		t.Fatal("the message reached the step already running")
	}
	if got := lastUserText(t, cap.sent[1]); got != message {
		t.Fatalf("next step's message: %q", got)
	}
	if got := strings.Count(asJSON(t, cap.sent[1]), message); got != 1 {
		t.Fatalf("message in the next step: %d times", got)
	}
	if got := lastUserText(t, cap.sent[2]); strings.Contains(got, message) || !strings.HasSuffix(got, cycle.ContinueMessage) {
		t.Fatalf("the step after it: %q", got)
	}
	wantModeFinished(t, db, pub, "no_work", "")
}

func asJSON(t *testing.T, v any) string {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

// A message that arrives while the final report is being written is not lost:
// the turn after the stop answers it, and the page is told which one it is.
func TestB83_AMessageRacingTheEndOfTheModeOpensTheNextTurn(t *testing.T) {
	hold := &holdProvider{kind: provider.TurnStopReport, at: 1}
	db, runner, cap, pub := newModeRunner(t, hold, loadAppFixture(t, "04-principal-no-work.json"))
	if _, err := runner.SetMode(context.Background(), "p1", graph.ModeRunToCompletion, ""); err != nil {
		t.Fatal(err)
	}
	<-hold.entered
	const message = "Una cosa más."
	if turn, err := runner.StartTurn("p1", message); err != nil || turn != "" {
		t.Fatalf("StartTurn: %q %v", turn, err)
	}
	close(hold.release)
	cycle.WaitTurnsForTest(runner)
	if len(cap.sent) != 2 {
		t.Fatalf("step calls: %d, want the mode's and the message's", len(cap.sent))
	}
	if got := lastUserText(t, cap.sent[1]); got != message {
		t.Fatalf("message's step: %q", got)
	}
	first := pub.stepAt(t, 0, 2)
	if first["stop_reason"] != "mode_finished" || first["next_turn_id"] == nil {
		t.Fatalf("the mode's last step: %v", first)
	}
	if got := currentMode(t, db); got != graph.ModeNormal {
		t.Fatalf("mode: %s", got)
	}
}

// «activar el modo con el agente parado arranca un turno», with your message in
// literal in mode_set and in the principal's layer 3.
func TestB83_ActivatingWithTheAgentStoppedStartsATurn(t *testing.T) {
	db, runner, cap, pub := newModeRunner(t, nil, loadAppFixture(t, "04-principal-no-work.json"))
	const message = "Continúa hasta terminar. Si surge algo, tú decides."
	turn, err := runner.SetMode(context.Background(), "p1", graph.ModeRunToCompletion, message)
	if err != nil || turn == "" {
		t.Fatalf("SetMode: %q %v, want a turn", turn, err)
	}
	cycle.WaitTurnsForTest(runner)
	sets := eventsOfType(t, db, store.EventModeSet)
	var data map[string]any
	_ = json.Unmarshal(sets[0].Data, &data)
	if sets[0].Actor != "human" || data["mode"] != graph.ModeRunToCompletion || data["message"] != message {
		t.Fatalf("mode_set: %s %v", sets[0].Actor, data)
	}
	raw := asJSON(t, cap.sent[0])
	if !strings.Contains(raw, "Mode: run_to_completion") || !strings.Contains(raw, message) {
		t.Fatalf("the principal's layer 3: %s", raw)
	}
	wantModeFinished(t, db, pub, "no_work", "")
}

// A live conversation has no startup: the principal still receives the mode and
// your message, in the next step only.
func TestB83_TheModeReachesALiveConversationOnce(t *testing.T) {
	db, runner, cap, _ := newModeRunner(t, nil, textFixture(t, pendingFigureStep), textFixture(t, pendingFigureStep),
		loadAppFixture(t, "04-principal-no-work.json"), textFixture(t, pendingFigureStep),
		loadAppFixture(t, "04-principal-no-work.json"))
	if err := runner.RunTurn(context.Background(), "p1", "turn-1", "Escribe."); err != nil {
		t.Fatal(err)
	}
	normal := len(cap.sent)
	const message = "Si surge algo, tú decides."
	modeOn(t, db, message)
	if err := runner.RunTurn(context.Background(), "p1", "turn-2", "Sigue."); err != nil {
		t.Fatal(err)
	}
	if got := strings.Count(asJSON(t, cap.sent[normal]), message); got != 1 {
		t.Fatalf("mode message in the next step: %d", got)
	}
	if !strings.Contains(asJSON(t, cap.sent[normal]), "Mode: run_to_completion") || lastUserText(t, cap.sent[normal]) != "Sigue." {
		t.Fatalf("next step: %s", asJSON(t, cap.sent[normal]))
	}
	if len(cap.sent) > normal+1 {
		if got := strings.Count(asJSON(t, cap.sent[normal+1]), "Mode: run_to_completion"); got != 1 {
			t.Fatalf("the second step adds the mode again: %d", got)
		}
	}
}

// Activating with a turn running opens no second chain: the running one takes
// the mode, or the end of a turn that is no chain opens it.
func TestB83_ActivatingWithATurnRunningRunsOneChain(t *testing.T) {
	db, runner, cap, pub := newModeRunner(t, nil, loadAppFixture(t, "04-principal-no-work.json"))
	end := cycle.HoldTurnForTest(runner, "p1")
	turn, err := runner.SetMode(context.Background(), "p1", graph.ModeRunToCompletion, "")
	if err != nil || turn != "" {
		t.Fatalf("SetMode with the slot taken: %q %v", turn, err)
	}
	if len(cap.sent) != 0 {
		t.Fatal("a chain ran while the slot was taken")
	}
	end()
	cycle.WaitTurnsForTest(runner)
	if len(cap.sent) != 1 {
		t.Fatalf("step calls: %d, want the one chain", len(cap.sent))
	}
	wantModeFinished(t, db, pub, "no_work", "")
}

// Turning it off in the bar: the chain goes back to the stops of the normal
// mode at the next step, and no mode_finished is written.
func TestB83_TurningItOffInTheBarGoesBackToTheNormalStops(t *testing.T) {
	hold := &holdProvider{kind: provider.TurnStep, at: 1}
	db, runner, _, pub := newModeRunner(t, hold, loadAppFixture(t, "01-principal-valid.json"), textFixture(t, pendingFigureStep))
	if _, err := runner.SetMode(context.Background(), "p1", graph.ModeRunToCompletion, ""); err != nil {
		t.Fatal(err)
	}
	<-hold.entered
	if _, err := runner.SetMode(context.Background(), "p1", graph.ModeNormal, ""); err != nil {
		t.Fatal(err)
	}
	close(hold.release)
	cycle.WaitTurnsForTest(runner)
	if r := oneReport(t, db); r.StopReason != cycle.StopSectionReady || r.Cause != "" {
		t.Fatalf("report: %+v", r)
	}
	if step := pub.step(t); step["stop_reason"] != "section_ready" {
		t.Fatalf("step: %v", step)
	}
}

// «con el modo activo, una firma en curso y el contexto sobre el umbral, la
// cadena no lanza pasos hasta que la firma termina: el turno termina y suelta el
// turno del proyecto, y al cerrar la firma se limpia y la cadena se reabre».
func TestB86_WithTheModeAtTheThresholdDuringASignoffTheChainReopensWhenItCloses(t *testing.T) {
	db := openTemp(t)
	seedSignable(t, db, false)
	gate := cycle.NewGate(db)
	sign := openSignoff(t, gate)
	runner, cap, pub := b86Runner(t, db, nil, using(textFixture(t, pendingFigureStep), 7000), b86NoWork(t, 100))
	t.Cleanup(runner.Close)
	ctx := context.Background()
	modeOn(t, db, "")
	if err := runner.RunTurn(ctx, "p1", "turn-1", "Escribe."); err != nil {
		t.Fatal(err)
	}
	if len(cap.sent) != 1 || len(cap.reports) != 0 {
		t.Fatalf("steps %d, reports %d: no new step and no stop", len(cap.sent), len(cap.reports))
	}
	if got := currentMode(t, db); got != graph.ModeRunToCompletion {
		t.Fatalf("mode while it waits: %s", got)
	}
	if err := runner.RunTurn(ctx, "p1", "turn-probe", "x"); err != nil {
		t.Fatalf("the turn must be free: %v", err)
	}
	if len(cap.sent) != 1 {
		t.Fatalf("a step ran during the signoff: %d", len(cap.sent))
	}
	runner.SignoffClosed(ctx, "p1") // still in progress: nothing opens
	cycle.WaitTurnsForTest(runner)
	if len(cap.sent) != 1 {
		t.Fatalf("a chain opened with the signoff in progress: %d", len(cap.sent))
	}
	if err := gate.LeaveSignoff(ctx, "p1", sign); err != nil {
		t.Fatal(err)
	}
	runner.SignoffClosed(ctx, "p1")
	cycle.WaitTurnsForTest(runner)
	if len(cap.sent) != 2 {
		t.Fatalf("steps after the signoff closed: %d", len(cap.sent))
	}
	wantReasons(t, db, "new_session", "threshold")
	isStartup(t, cap.sent[1])
	wantModeFinished(t, db, pub, "no_work", "")
}

// «pedir firmar con el modo activo se encola (no 409)»: it opens when the running
// step ends, and the chain goes on.
func TestB83_AskingToSignDuringTheModeQueuesAndOpensWhenTheStepEnds(t *testing.T) {
	db := openTemp(t)
	seedSignable(t, db, false)
	hold := &holdProvider{kind: provider.TurnStep, at: 1}
	runner, cap, pub := modeRunnerOn(t, db, hold, textFixture(t, pendingFigureStep), loadAppFixture(t, "04-principal-no-work.json"))
	ctx := context.Background()
	if _, err := runner.SetMode(ctx, "p1", graph.ModeRunToCompletion, ""); err != nil {
		t.Fatal(err)
	}
	<-hold.entered
	_, err := runner.RequestSignoff(ctx, "p1", "sec-1")
	if !errors.Is(err, cycle.ErrSignoffQueued) {
		t.Fatalf("RequestSignoff during the mode: %v, want queued", err)
	}
	if signs, _ := db.ListObjects(ctx, "p1", store.ObjectSignoff); len(signs) != 0 {
		t.Fatalf("signoffs before the step ends: %d", len(signs))
	}
	close(hold.release)
	cycle.WaitTurnsForTest(runner)
	if signs, _ := db.ListObjects(ctx, "p1", store.ObjectSignoff); len(signs) != 1 {
		t.Fatalf("signoffs after the step: %d, want 1", len(signs))
	}
	if len(cap.sent) != 2 {
		t.Fatalf("step calls: %d: the chain goes on after it opens", len(cap.sent))
	}
	open, err := runner.RequestSignoff(ctx, "p1", "sec-1")
	if err != nil || !open.Opened {
		t.Fatalf("asking again: %+v %v, want the one in progress", open, err)
	}
	wantModeFinished(t, db, pub, "no_work", "")
}

// P-113: a queued request keeps the signoff's rules: what blocks it comes back
// to you, and nothing opens.
func TestB83_AQueuedSignoffOnASectionWithPendingsListsThem(t *testing.T) {
	db := openTemp(t)
	seedSignable(t, db, false)
	hold := &holdProvider{kind: provider.TurnStep, at: 1}
	runner, _, _ := modeRunnerOn(t, db, hold, textFixture(t, proposalOpenQuestionStep), loadAppFixture(t, "04-principal-no-work.json"))
	ctx := context.Background()
	if _, err := runner.SetMode(ctx, "p1", graph.ModeRunToCompletion, ""); err != nil {
		t.Fatal(err)
	}
	<-hold.entered
	if _, err := runner.RequestSignoff(ctx, "p1", "sec-1"); !errors.Is(err, cycle.ErrSignoffQueued) {
		t.Fatalf("RequestSignoff: %v", err)
	}
	close(hold.release)
	cycle.WaitTurnsForTest(runner)
	if signs, _ := db.ListObjects(ctx, "p1", store.ObjectSignoff); len(signs) != 0 {
		t.Fatalf("signoffs: %d, the open question blocks it", len(signs))
	}
	open, err := runner.RequestSignoff(ctx, "p1", "sec-1")
	if err != nil || open.Opened || len(open.Pendings) == 0 {
		t.Fatalf("what blocks it: %+v %v", open, err)
	}
}

const proposalOpenQuestionStep = `{"stretch":"build","triage_line":"Construir.","closing_risk":"none","note":"n",` + proposedQuestion + `}`

// «un provider_error con el modo activo termina el modo con mode_finished y
// data.cause: provider_error»; the report keeps the app's part, with no call to
// a provider that just failed.
func TestB83_AProviderErrorEndsTheModeWithItsCause(t *testing.T) {
	hold := &holdProvider{kind: provider.TurnStep, fail: map[int]error{2: errors.New("meta down")}}
	db, runner, cap, pub := newModeRunner(t, hold, textFixture(t, pendingFigureStep))
	modeOn(t, db, "")
	if err := runner.RunTurn(context.Background(), "p1", "turn-1", "Escribe."); err != nil {
		t.Fatal(err)
	}
	if len(cap.reports) != 0 || len(cap.judges) != 0 {
		t.Fatalf("calls after the failure: %d reports, %d judges", len(cap.reports), len(cap.judges))
	}
	report := wantModeFinished(t, db, pub, "provider_error", "")
	if !report.PrincipalFailed {
		t.Fatalf("report: %+v", report)
	}
	pub.mu.Lock()
	defer pub.mu.Unlock()
	if len(pub.errs) != 0 {
		t.Fatalf("stream errors: %v", pub.errs)
	}
}

// An agent step pending after its retry also ends the mode.
func TestB83_AnAgentPendingEndsTheModeWithItsCause(t *testing.T) {
	db, runner, _, pub := newModeRunner(t, nil, loadAppFixture(t, "02-principal-invalid.json"))
	modeOn(t, db, "")
	if err := runner.RunTurn(context.Background(), "p1", "turn-1", "Escribe."); err != nil {
		t.Fatal(err)
	}
	wantModeFinished(t, db, pub, "agent_pending", "")
}

// B-83, adopted 1-nov: the mode never gets past a cap, and ends with its cause.
func TestB89_AtTheProjectCapTheModeEnds(t *testing.T) {
	db, runner, cap, pub := newModeRunner(t, nil, textFixture(t, pendingFigureStep))
	runner.ProjectCapUSD = 1
	spend(t, db, "p1", 1)
	modeOn(t, db, "")
	if err := runner.RunTurn(context.Background(), "p1", "turn-1", "Escribe."); err != nil {
		t.Fatal(err)
	}
	if len(cap.sent) != 0 {
		t.Fatalf("calls past the cap: %d", len(cap.sent))
	}
	report := wantModeFinished(t, db, pub, "project_cap", "")
	if !report.PrincipalFailed {
		t.Fatalf("report: %+v", report)
	}
}

func TestB89_AtALinkCapTheModeEndsWithDataCap(t *testing.T) {
	db, runner, cap, pub := newModeRunner(t, nil, textFixture(t, pendingFigureStep))
	newLinkFor(t, db, "p1", "demo", 0.01)
	modeOn(t, db, "")
	if err := runner.RunTurn(context.Background(), "p1", "turn-1", "Escribe."); err != nil {
		t.Fatal(err)
	}
	if len(cap.sent) != 0 {
		t.Fatalf("calls past the cap: %d", len(cap.sent))
	}
	wantModeFinished(t, db, pub, "demo_cap", "link")
}

// «un alcance nuevo (caso 7: la mina cierra antes que la planta) queda «por
// aclarar» con dos variantes y el modo sigue».
func TestB84_ANewScopeStaysOpenWithTwoVariantsAndTheModeGoesOn(t *testing.T) {
	const scope = `{"stretch":"build","triage_line":"Construir: el horizonte.","closing_risk":"none","note":"La mina cierra en 2037.",` +
		`"new_text":[{"section_id":"new-2","base_version":0,"anchor":"","claim_origin":"material","operation":"insert_after","text":"# Valor\n\nCon dos variantes."}],` +
		`"proposed_roles":[{"section_id":"new-2","role":"other","title":"Valor"}],` +
		`"proposed_open_questions":[{"id":"new-1","question":"¿Qué pasa con la planta después del cierre de la mina en 2037?",` +
		`"options":[{"option":"12 años y vender equipos","contrast":"Termina con la mina.","consequence":"Sin red.","risk":"Se pierde valor."},` +
		`{"option":"25 años y vender a la red","contrast":"Sigue sin la mina.","consequence":"Con red.","risk":"Depende de la red."}],` +
		`"depends_on":["sec-1"],"variants":["12 años","25 años"]}]}`
	db, runner, cap, pub := newModeRunner(t, nil, textFixture(t, pendingFigureStep), textFixture(t, scope),
		textFixture(t, pendingFigureStep), loadAppFixture(t, "04-principal-no-work.json"))
	modeOn(t, db, "Si surge algo, tú decides.")
	if err := runner.RunTurn(context.Background(), "p1", "turn-1", "Evalúa la planta."); err != nil {
		t.Fatal(err)
	}
	oq, body := object(t, db, "oq-1")
	options, _ := body["options"].([]any)
	if oq.State != "open" || len(options) != 2 {
		t.Fatalf("oq-1: %s %v", oq.State, body)
	}
	if len(cap.sent) != 4 {
		t.Fatalf("step calls: %d: the chain goes on after it", len(cap.sent))
	}
	noStepStops(t, db)
	report := wantModeFinished(t, db, pub, "no_work", "")
	found := false
	for _, item := range report.Open {
		found = found || item.Kind == "open_question"
	}
	if !found {
		t.Fatalf("report's open items: %+v", report.Open)
	}
}

// Close during a turn the mode opened: the call is cancelled, Close waits, the
// mode stays on in the history and nothing says it finished.
func TestB83_CloseDuringASelfOpenedTurnWaitsAndLeavesTheModeOn(t *testing.T) {
	hold := &holdProvider{kind: provider.TurnStep, at: cycle.MaxChainSteps + 1}
	db, runner, cap, pub := newModeRunner(t, hold, textFixture(t, pendingFigureStep))
	if _, err := runner.SetMode(context.Background(), "p1", graph.ModeRunToCompletion, ""); err != nil {
		t.Fatal(err)
	}
	<-hold.entered
	done := make(chan struct{})
	go func() { runner.Close(); close(done) }()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("Close did not end")
	}
	if len(cap.sent) != cycle.MaxChainSteps+1 {
		t.Fatalf("step calls: %d", len(cap.sent))
	}
	if got := currentMode(t, db); got != graph.ModeRunToCompletion {
		t.Fatalf("mode after Close: %s", got)
	}
	if reports := stopReports(t, db); len(reports) != 0 {
		t.Fatalf("reports after Close: %+v", reports)
	}
	pub.mu.Lock()
	defer pub.mu.Unlock()
	for _, raw := range pub.steps {
		if strings.Contains(string(raw), "mode_finished") {
			t.Fatalf("mode_finished after Close: %s", raw)
		}
	}
}

// What shares ShouldStop: in the normal mode, after the mode ended, the stops of
// the r2 hold.
func TestB83_TheNormalModeStillStopsAtAReadySection(t *testing.T) {
	db, runner, cap, _ := newModeRunner(t, nil, loadAppFixture(t, "01-principal-valid.json"))
	modeOn(t, db, "")
	err := db.Write(context.Background(), "p1", func(tx *store.Tx) error {
		_, err := tx.AppendEvent(store.EventInput{Type: store.EventModeSet, Actor: "human", Data: map[string]any{"mode": graph.ModeNormal}})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := runner.RunTurn(context.Background(), "p1", "turn-1", "Escribe."); err != nil {
		t.Fatal(err)
	}
	if r := oneReport(t, db); r.StopReason != cycle.StopSectionReady || len(cap.sent) != 1 {
		t.Fatalf("report: %+v, %d steps", r, len(cap.sent))
	}
}

// What shares the turn limit: in the normal mode, 12 steps stop with step_limit.
func TestB83_TheNormalModeStillStopsAtTwelveSteps(t *testing.T) {
	db, runner, cap, pub := newModeRunner(t, nil, textFixture(t, pendingFigureStep))
	if err := runner.RunTurn(context.Background(), "p1", "turn-1", "Escribe."); err != nil {
		t.Fatal(err)
	}
	if len(cap.sent) != cycle.MaxChainSteps {
		t.Fatalf("steps: %d", len(cap.sent))
	}
	steps := stepEndedData(t, db)
	if steps[len(steps)-1]["stop_reason"] != "step_limit" || pub.step(t)["stop_reason"] != "step_limit" {
		t.Fatalf("last step: %v", steps[len(steps)-1])
	}
	if r := oneReport(t, db); r.StopReason != cycle.StopStepLimit || r.Cause != "" {
		t.Fatalf("report: %+v", r)
	}
}

// What shares RequestSignoff: in the normal mode, with edits unchecked and a turn
// running, it still answers 409 and queues nothing.
func TestB83_TheNormalModeStillRefusesToSignDuringATurn(t *testing.T) {
	db := openTemp(t)
	seedSignable(t, db, false)
	runner, _, _ := modeRunnerOn(t, db, nil)
	editSectionOne(t, db, " Línea tuya.")
	end := cycle.HoldTurnForTest(runner, "p1")
	defer end()
	if _, err := runner.RequestSignoff(context.Background(), "p1", "sec-1"); !errors.Is(err, cycle.ErrTurnRunning) {
		t.Fatalf("RequestSignoff: %v, want turn_running", err)
	}
}

// B-84: a step that only leaves what is yours «por aclarar» does not end the mode.
func TestB84_AStepThatOnlyAsksDoesNotEndTheMode(t *testing.T) {
	db := openTemp(t)
	seedSignable(t, db, false)
	runner, cap, pub := modeRunnerOn(t, db, nil, textFixture(t, proposalOpenQuestionStep), textFixture(t, pendingFigureStep),
		loadAppFixture(t, "04-principal-no-work.json"))
	modeOn(t, db, "")
	if err := runner.RunTurn(context.Background(), "p1", "turn-1", "Sigue."); err != nil {
		t.Fatal(err)
	}
	if len(cap.sent) != 3 {
		t.Fatalf("steps: %d, want 3", len(cap.sent))
	}
	wantModeFinished(t, db, pub, "no_work", "")
}

// A round of the signoff that cancels it (missing_in_text) is a close site too.
func TestB86_ARoundThatCancelsTheSignoffReopensTheModesChain(t *testing.T) {
	db := openTemp(t)
	seedSignable(t, db, true)
	gate := cycle.NewGate(db)
	sign := openSignoff(t, gate)
	first, second, wonder := b86NoWork(t, 7000), b86NoWork(t, 100), signoffFixture(t, "missing-in-text")
	first.TurnKind, second.TurnKind, wonder.TurnKind = provider.TurnStep, provider.TurnStep, provider.TurnSignoff
	runner, cap, _ := b86Runner(t, db, nil, first, second, wonder)
	t.Cleanup(runner.Close)
	ctx := context.Background()
	modeOn(t, db, "")
	if err := runner.RunTurn(ctx, "p1", "turn-1", "Hola."); err != nil {
		t.Fatal(err)
	}
	if len(cap.sent) != 1 {
		t.Fatalf("steps during the signoff: %d", len(cap.sent))
	}
	if err := gate.RecordExplanation(ctx, "p1", sign, "Comparo los estudios entre sí."); err != nil {
		t.Fatal(err)
	}
	if err := runner.RunSignoffRound(ctx, "p1", "turn-w", sign); err != nil {
		t.Fatal(err)
	}
	if n := len(eventsOfType(t, db, store.EventSignoffCancelled)); n != 1 {
		t.Fatalf("signoff_cancelled: %d", n)
	}
	cycle.WaitTurnsForTest(runner)
	// The capture keeps Wonder's call with the steps: step, Wonder, the new step.
	if len(cap.sent) != 3 {
		t.Fatalf("calls after the round cancelled it: %d", len(cap.sent))
	}
	wantReasons(t, db, "new_session", "threshold")
	isStartup(t, cap.sent[2])
}

// Activating during a step that only answers: that step was the normal mode's, so
// its lack of work does not end the mode just activated (B-83).
func TestB83_ActivatingDuringAStepThatOnlyAnswersKeepsTheChainGoing(t *testing.T) {
	hold := &holdProvider{kind: provider.TurnStep, at: 1}
	const answerOnly = `{"stretch":"build","triage_line":"Respondo.","closing_risk":"none","note":"Solo respondí.","reply":"Sí."}`
	db, runner, cap, pub := newModeRunner(t, hold, textFixture(t, answerOnly), textFixture(t, pendingFigureStep),
		loadAppFixture(t, "04-principal-no-work.json"))
	if _, err := runner.StartTurn("p1", "¿Qué falta?"); err != nil {
		t.Fatal(err)
	}
	<-hold.entered
	if turn, err := runner.SetMode(context.Background(), "p1", graph.ModeRunToCompletion, "Sigue tú"); err != nil || turn != "" {
		t.Fatalf("SetMode: %q %v", turn, err)
	}
	close(hold.release)
	cycle.WaitTurnsForTest(runner)
	if len(cap.sent) != 3 {
		t.Fatalf("step calls: %d, want 3: the chain takes the mode", len(cap.sent))
	}
	wantModeFinished(t, db, pub, "no_work", "")
}

// queuedThenLater runs a chain where msg queues during step 1 and the next step
// cannot take it, then sends another message: both reach that later step, once.
func wantBothOnce(t *testing.T, cap *captureProvider, from int, msg, second string) {
	t.Helper()
	if len(cap.sent) <= from {
		t.Fatalf("no later step: %d calls", len(cap.sent))
	}
	raw := asJSON(t, cap.sent[from])
	if got := strings.Count(raw, msg); got != 1 {
		t.Fatalf("queued message in the later step: %d", got)
	}
	if got := strings.Count(raw, second); got != 1 {
		t.Fatalf("new message in the later step: %d", got)
	}
	if last := lastUserText(t, cap.sent[from]); last != second {
		t.Fatalf("the later step's message: %q", last)
	}
	for i := from + 1; i < len(cap.sent); i++ {
		if got := strings.Count(asJSON(t, cap.sent[i]), msg); got != 1 {
			t.Fatalf("step %d carries the message %d times: it enters once", i, got)
		}
	}
}

// An accepted message reaches a step, once, also when the step that took it failed.
func TestB83_AMessageAFailedStepTookReachesTheNextStep(t *testing.T) {
	hold := &holdProvider{kind: provider.TurnStep, at: 1, fail: map[int]error{2: errors.New("meta down")}}
	db, runner, cap, pub := newModeRunner(t, hold, textFixture(t, pendingFigureStep), loadAppFixture(t, "04-principal-no-work.json"))
	if _, err := runner.SetMode(context.Background(), "p1", graph.ModeRunToCompletion, ""); err != nil {
		t.Fatal(err)
	}
	<-hold.entered
	const msg = "Mensaje que el paso fallido tomó."
	if _, err := runner.StartTurn("p1", msg); err != nil {
		t.Fatal(err)
	}
	close(hold.release)
	cycle.WaitTurnsForTest(runner)
	wantModeFinished(t, db, pub, "provider_error", "")
	before := len(cap.sent)
	if _, err := runner.StartTurn("p1", "SECOND"); err != nil {
		t.Fatal(err)
	}
	cycle.WaitTurnsForTest(runner)
	wantBothOnce(t, cap, before, msg, "SECOND")
}

// The same when the cap refused that step.
func TestB89_AMessageTheCapKeptReachesTheNextStep(t *testing.T) {
	hold := &holdProvider{kind: provider.TurnStep, at: 1}
	db, runner, cap, pub := newModeRunner(t, hold, textFixture(t, pendingFigureStep), loadAppFixture(t, "04-principal-no-work.json"))
	runner.ProjectCapUSD = 1
	if _, err := runner.SetMode(context.Background(), "p1", graph.ModeRunToCompletion, ""); err != nil {
		t.Fatal(err)
	}
	<-hold.entered
	const msg = "Mensaje que el tope retuvo."
	if _, err := runner.StartTurn("p1", msg); err != nil {
		t.Fatal(err)
	}
	spend(t, db, "p1", 1)
	close(hold.release)
	cycle.WaitTurnsForTest(runner)
	wantModeFinished(t, db, pub, "project_cap", "")
	before := len(cap.sent)
	runner.ProjectCapUSD = 100
	if _, err := runner.StartTurn("p1", "SECOND"); err != nil {
		t.Fatal(err)
	}
	cycle.WaitTurnsForTest(runner)
	wantBothOnce(t, cap, before, msg, "SECOND")
}

// A restart loses nothing: the message is in the history, and a live principal
// that never received it receives it in the next step.
func TestB83_AQueuedMessageSurvivesARestart(t *testing.T) {
	hold := &holdProvider{kind: provider.TurnStep, at: 2}
	db := openTemp(t)
	runner, _, _ := modeRunnerOn(t, db, hold, textFixture(t, pendingFigureStep))
	if _, err := runner.SetMode(context.Background(), "p1", graph.ModeRunToCompletion, ""); err != nil {
		t.Fatal(err)
	}
	<-hold.entered
	const msg = "Mensaje antes de reiniciar."
	if _, err := runner.StartTurn("p1", msg); err != nil {
		t.Fatal(err)
	}
	runner.Close()
	runner2, cap2, _ := modeRunnerOn(t, db, nil, textFixture(t, pendingFigureStep), loadAppFixture(t, "04-principal-no-work.json"))
	if _, err := runner2.StartTurn("p1", "SECOND"); err != nil {
		t.Fatal(err)
	}
	cycle.WaitTurnsForTest(runner2)
	wantBothOnce(t, cap2, 0, msg, "SECOND")
}

// The same text twice is two messages: the one delivered does not stand for the
// one that was not, and a startup's lines count as delivered.
func TestB83_AQueuedMessageEqualToADeliveredOneStillEnters(t *testing.T) {
	hold := &holdProvider{kind: provider.TurnStep, at: 1}
	_, runner, cap, _ := newModeRunner(t, hold, textFixture(t, pendingFigureStep), textFixture(t, pendingFigureStep),
		loadAppFixture(t, "04-principal-no-work.json"))
	const msg = "Sí."
	if _, err := runner.StartTurn("p1", msg); err != nil {
		t.Fatal(err)
	}
	<-hold.entered
	if _, err := runner.SetMode(context.Background(), "p1", graph.ModeRunToCompletion, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := runner.StartTurn("p1", msg); err != nil {
		t.Fatal(err)
	}
	close(hold.release)
	cycle.WaitTurnsForTest(runner)
	if got := lastUserText(t, cap.sent[1]); got != msg {
		t.Fatalf("next step's message: %q", got)
	}
	if got := lastUserText(t, cap.sent[2]); got == msg {
		t.Fatalf("the message entered twice: %q", got)
	}
}

// An outcome nobody read does not answer a request made after its signoff ended.
func TestB14_AQueuedSignoffThatEndedDoesNotAnswerALaterRequest(t *testing.T) {
	db := openTemp(t)
	seedSignable(t, db, false)
	hold := &holdProvider{kind: provider.TurnStep, at: 1}
	runner, _, _ := modeRunnerOn(t, db, hold, textFixture(t, pendingFigureStep), loadAppFixture(t, "04-principal-no-work.json"))
	ctx := context.Background()
	if _, err := runner.SetMode(ctx, "p1", graph.ModeRunToCompletion, ""); err != nil {
		t.Fatal(err)
	}
	<-hold.entered
	if _, err := runner.RequestSignoff(ctx, "p1", "sec-1"); !errors.Is(err, cycle.ErrSignoffQueued) {
		t.Fatal(err)
	}
	close(hold.release)
	cycle.WaitTurnsForTest(runner)
	if err := cycle.NewGate(db).LeaveSignoff(ctx, "p1", "sign-1"); err != nil {
		t.Fatal(err)
	}
	open, err := runner.RequestSignoff(ctx, "p1", "sec-1")
	if err != nil || !open.Opened || open.Label != "sign-2" {
		t.Fatalf("the request after it ended: %+v %v, want sign-2", open, err)
	}
	if d, err := graph.SignoffDetail(ctx, db, "p1", open.Label); err != nil || d.Phase == graph.SignoffCancelled {
		t.Fatalf("its phase: %v %v", d.Phase, err)
	}
}
