package server_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/LaurenteEber/tupaqnin/internal/cycle"
	"github.com/LaurenteEber/tupaqnin/internal/graph"
	"github.com/LaurenteEber/tupaqnin/internal/provider"
	"github.com/LaurenteEber/tupaqnin/internal/server"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// heldSteps holds the first principal step until the test lets it go.
type heldSteps struct {
	provider.Provider
	entered, release chan struct{}
	seen             bool
	// failing: every step after the held one fails, as a provider that went down.
	failing atomic.Bool
}

func (h *heldSteps) Respond(ctx context.Context, req provider.Request, d func(provider.Delta)) (provider.Response, error) {
	if req.TurnKind == provider.TurnStep && h.seen && h.failing.Load() {
		return provider.Response{}, errors.New("provider down")
	}
	if req.TurnKind == provider.TurnStep && !h.seen {
		h.seen = true
		close(h.entered)
		select {
		case <-h.release:
		case <-ctx.Done():
			return provider.Response{}, ctx.Err()
		}
	}
	return h.Provider.Respond(ctx, req, d)
}

// modeAPI is the API over a runner whose first step waits for the test.
func modeAPI(t *testing.T, later *time.Duration) (http.Handler, *store.DB, string, *heldSteps, *cycle.Runner) {
	t.Helper()
	db, err := store.Open(context.Background(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	noWork, err := provider.LoadFixture(filepath.Join("..", "provider", "testdata", "app", "04-principal-no-work.json"))
	if err != nil {
		t.Fatal(err)
	}
	fake := provider.NewFake(noWork)
	fake.Recorder = db
	hold := &heldSteps{Provider: fake, entered: make(chan struct{}), release: make(chan struct{})}
	hub := server.NewHub()
	count := func(ctx context.Context, input []json.RawMessage) (int, error) {
		return fake.CountTokens(ctx, provider.PrincipalModel, input)
	}
	runner := cycle.NewRunner(db, hold, count, hub)
	t.Cleanup(runner.Close)
	opts := []server.Option{}
	if later != nil {
		opts = append(opts, server.WithClock(func() time.Time { return time.Now().Add(*later) }))
	}
	h := withVisitor(t, db, server.NewWithAPI(nil, hub, db, runner, opts...))
	return h, db, newProject(t, h, nil), hold, runner
}

func summaryOf(t *testing.T, h http.Handler, base string) server.SummaryView {
	t.Helper()
	rec := do(t, h, http.MethodGet, base+"/summary", nil)
	var v server.SummaryView
	if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
		t.Fatal(err)
	}
	return v
}

func waitNormal(t *testing.T, h http.Handler, base string) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for summaryOf(t, h, base).Mode != graph.ModeNormal {
		if time.Now().After(deadline) {
			t.Fatal("the mode did not end")
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// PUT /mode: your message in literal, the turn it started, the bar's mode; a
// mode that does not exist is refused and writes nothing.
func TestB83_PutModeSetsItAndAnswersItsTurn(t *testing.T) {
	h, db, projectID, hold, _ := modeAPI(t, nil)
	base := "/api/projects/" + projectID
	if got := summaryOf(t, h, base); got.Mode != graph.ModeNormal || got.RunningTurn != "" {
		t.Fatalf("summary before: %+v", got)
	}
	if rec := do(t, h, http.MethodPut, base+"/mode", map[string]string{"mode": "turbo"}); rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid mode: %d", rec.Code)
	}
	const message = "Si surge algo, tú decides."
	rec := do(t, h, http.MethodPut, base+"/mode", server.ModeRequest{Mode: graph.ModeRunToCompletion, Message: message})
	var res server.ModeResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil || rec.Code != http.StatusOK ||
		res.Mode != graph.ModeRunToCompletion || res.TurnID == "" {
		t.Fatalf("PUT /mode: %d %s", rec.Code, rec.Body.String())
	}
	<-hold.entered
	if got := summaryOf(t, h, base); got.Mode != graph.ModeRunToCompletion || got.RunningTurn != res.TurnID {
		t.Fatalf("summary during: %+v, turn %s", got, res.TurnID)
	}
	events, err := db.EventsSince(context.Background(), projectID, 0)
	if err != nil {
		t.Fatal(err)
	}
	var sets int
	for _, ev := range events {
		if ev.Type == store.EventModeSet {
			sets++
			var data map[string]any
			_ = json.Unmarshal(ev.Data, &data)
			if ev.Actor != "human" || data["message"] != message {
				t.Fatalf("mode_set: %s %v", ev.Actor, data)
			}
		}
	}
	if sets != 1 {
		t.Fatalf("mode_set: %d", sets)
	}
	close(hold.release)
	waitNormal(t, h, base)
}

// A message and asking to sign during the mode get 202 queued, never 409.
func TestB83_DuringTheModeAMessageAndAskingToSignAnswerQueued(t *testing.T) {
	h, db, projectID, hold, _ := modeAPI(t, nil)
	base := "/api/projects/" + projectID
	seedSignSection(t, db, projectID, "Comparamos solo estudios.", false)
	if rec := do(t, h, http.MethodPut, base+"/mode", server.ModeRequest{Mode: graph.ModeRunToCompletion}); rec.Code != http.StatusOK {
		t.Fatalf("PUT /mode: %d", rec.Code)
	}
	<-hold.entered
	rec := do(t, h, http.MethodPost, base+"/messages", map[string]string{"text": "Una cosa."})
	var msg server.PostMessageResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &msg); err != nil || rec.Code != http.StatusAccepted || !msg.Queued || msg.TurnID != "" {
		t.Fatalf("message: %d %s", rec.Code, rec.Body.String())
	}
	rec = do(t, h, http.MethodPost, base+"/sections/sec-1/signoff", nil)
	var queued server.SignoffQueuedResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &queued); err != nil || rec.Code != http.StatusAccepted || !queued.Queued || queued.Section != "sec-1" {
		t.Fatalf("signoff: %d %s", rec.Code, rec.Body.String())
	}
	close(hold.release)
	waitNormal(t, h, base)
}

// «tras el fin del modo, Frame trae la línea de vuelta aunque no hayan pasado 3
// días»; the report waits a click away, and your next event ends it.
func TestB85_AfterTheModeEndsTheFrameBringsTheReturnLine(t *testing.T) {
	var later time.Duration
	h, _, projectID, hold, runner := modeAPI(t, &later)
	base := "/api/projects/" + projectID
	if f := getFrame(t, h, projectID); f.ReturnLine != nil {
		t.Fatalf("line before: %+v", f.ReturnLine)
	}
	if rec := do(t, h, http.MethodPut, base+"/mode", server.ModeRequest{Mode: graph.ModeRunToCompletion}); rec.Code != http.StatusOK {
		t.Fatalf("PUT /mode: %d", rec.Code)
	}
	<-hold.entered
	if f := getFrame(t, h, projectID); f.ReturnLine != nil {
		t.Fatalf("line during the mode: %+v", f.ReturnLine)
	}
	close(hold.release)
	waitNormal(t, h, base)
	f := getFrame(t, h, projectID)
	if f.ReturnLine == nil || !f.ReturnLine.ModeFinished || f.ReturnLine.Days != 0 || f.ReturnLine.Act != "mode" {
		t.Fatalf("line after the mode: %+v", f.ReturnLine)
	}
	if !f.Todo.StopReportUnseen {
		t.Fatalf("the report must be in «lo que te toca»: %+v", f.Todo)
	}
	var report server.StopReportView
	rec := do(t, h, http.MethodGet, base+"/stop-report", nil)
	if err := json.Unmarshal(rec.Body.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	if report.OpensConversation || report.StopReason != "mode_finished" || report.Cause != "no_work" {
		t.Fatalf("report: %+v", report)
	}
	_ = runner
	if rec := do(t, h, http.MethodPost, base+"/stop-report/seen", nil); rec.Code != http.StatusNoContent {
		t.Fatalf("seen: %d", rec.Code)
	}
	if f := getFrame(t, h, projectID); f.ReturnLine != nil {
		t.Fatalf("line after your next event: %+v", f.ReturnLine)
	}
}

// closeSpy counts the close sites the server tells.
type closeSpy struct {
	*stubStarter
	closed int
}

func (c *closeSpy) SignoffClosed(context.Context, string) { c.closed++ }

// B-86: every handler that closes a signoff tells the runner, once; a handler
// that closes none does not.
func TestB86_EveryHandlerThatClosesASignoffSaysSo(t *testing.T) {
	db, err := store.Open(context.Background(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	spy := &closeSpy{stubStarter: &stubStarter{}}
	h := withVisitor(t, db, server.NewWithAPI(nil, server.NewHub(), db, spy))
	projectID := newProject(t, h, nil)
	base := "/api/projects/" + projectID
	seedSignSection(t, db, projectID, "Comparamos solo estudios.", false)
	open := func() string {
		t.Helper()
		rec := do(t, h, http.MethodPost, base+"/sections/sec-1/signoff", nil)
		var v graph.SignoffView
		if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil || rec.Code != http.StatusCreated {
			t.Fatalf("signoff: %d %s", rec.Code, rec.Body.String())
		}
		return v.Label
	}
	sign := open()
	if spy.closed != 0 {
		t.Fatalf("opening told it: %d", spy.closed)
	}
	if rec := do(t, h, http.MethodPost, base+"/signoffs/"+sign+"/leave", nil); rec.Code != http.StatusOK {
		t.Fatalf("leave: %d", rec.Code)
	}
	if spy.closed != 1 {
		t.Fatalf("after leaving: %d", spy.closed)
	}
	editSection(t, h, db, projectID, "sec-1", "Otro texto sin firma en curso.")
	if spy.closed != 1 {
		t.Fatalf("an edit with no signoff told it: %d", spy.closed)
	}
	open()
	editSection(t, h, db, projectID, "sec-1", "Texto que cancela la firma.")
	if spy.closed != 2 {
		t.Fatalf("after your edit: %d", spy.closed)
	}
	open()
	if applied := stepOn(t, db, projectID, replaceStep(versionOf(t, db, projectID, "sec-1"), "Texto que cancela la firma.", "Texto del agente.")); !applied.Applied {
		t.Fatalf("step on the section in signoff: %v", applied.Errors)
	}
	act(t, h, base, "prop-1", "accept")
	if spy.closed != 3 {
		t.Fatalf("after accepting a proposal on it: %d", spy.closed)
	}
	// P-114: the accepted text waits for the checker's pass and its decision.
	if err := db.Write(context.Background(), projectID, func(tx *store.Tx) error {
		if _, err := tx.AppendEvent(store.EventInput{Type: store.EventProbe, Actor: "app",
			Data: map[string]any{"sections": []string{"sec-1"}, "saw_versions": map[string]int{"sec-1": versionOf(t, db, projectID, "sec-1")}}}); err != nil {
			return err
		}
		_, err := tx.AppendEvent(store.EventInput{Type: store.EventStepEnded, Actor: "agent", AgentRole: "principal",
			Data: map[string]any{"turn_kind": "step", "decided_sections": []string{"sec-1"}}})
		return err
	}); err != nil {
		t.Fatal(err)
	}
	sign = open()
	if err := cycle.NewGate(db).RecordExplanation(context.Background(), projectID, sign, "Porque así la comparación es justa."); err != nil {
		t.Fatal(err)
	}
	if rec := do(t, h, http.MethodPost, base+"/signoffs/"+sign+"/sign", nil); rec.Code != http.StatusOK {
		t.Fatalf("sign: %d %s", rec.Code, rec.Body.String())
	}
	if spy.closed != 4 {
		t.Fatalf("after signing: %d", spy.closed)
	}
}

// B-101: in the normal mode too, with a turn running, a message queues.
func TestB101_TheNormalModeQueuesAMessageDuringATurn(t *testing.T) {
	h, _, projectID, hold, _ := modeAPI(t, nil)
	base := "/api/projects/" + projectID
	rec := do(t, h, http.MethodPost, base+"/messages", map[string]string{"text": "Hola."})
	if rec.Code != http.StatusAccepted {
		t.Fatalf("first: %d", rec.Code)
	}
	<-hold.entered
	rec = do(t, h, http.MethodPost, base+"/messages", map[string]string{"text": "Otra."})
	if rec.Code != http.StatusAccepted || !decode[server.PostMessageResponse](t, rec).Queued {
		t.Fatalf("second: %d %s, want 202 queued", rec.Code, rec.Body)
	}
	entries := decode[[]server.ConversationEntry](t, do(t, h, http.MethodGet, base+"/conversation", nil))
	var mark *server.ConversationEntry
	for i := range entries {
		if entries[i].Kind == "message" && entries[i].Text == "Otra." {
			mark = &entries[i]
		}
	}
	if mark == nil || !mark.Queued || mark.N == 0 {
		t.Fatalf("the queued message in the conversation: %+v", mark)
	}
	path := fmt.Sprintf("%s/messages/%d", base, mark.N)
	if rec := do(t, h, http.MethodPut, path, map[string]string{"text": " "}); rec.Code != http.StatusBadRequest {
		t.Fatalf("empty edit: %d, want 400", rec.Code)
	}
	if rec := do(t, h, http.MethodDelete, path, nil); rec.Code != http.StatusNoContent {
		t.Fatalf("remove: %d, want 204", rec.Code)
	}
	if rec := do(t, h, http.MethodDelete, path, nil); rec.Code != http.StatusConflict {
		t.Fatalf("remove twice: %d, want 409", rec.Code)
	}
	entries = decode[[]server.ConversationEntry](t, do(t, h, http.MethodGet, base+"/conversation", nil))
	for _, e := range entries {
		if e.Text == "Otra." {
			t.Fatal("a removed message still shows")
		}
	}
	close(hold.release)
}

func versionOf(t *testing.T, db *store.DB, projectID, label string) int {
	t.Helper()
	sec, err := db.GetObject(context.Background(), projectID, label)
	if err != nil {
		t.Fatal(err)
	}
	return sec.CurrentVersion
}

// editSpy counts the saves the runner's checker batch hears of (B-36).
type editSpy struct {
	*stubStarter
	saved int
}

func (e *editSpy) EditSaved(string) { e.saved++ }

// P-114: an accepted text goes to the checker's next batch, like your edit; a
// discarded proposal does not.
func TestB50_AcceptingAProposalArmsTheCheckersBatchAndDiscardingDoesNot(t *testing.T) {
	db, err := store.Open(context.Background(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	spy := &editSpy{stubStarter: &stubStarter{}}
	h := withVisitor(t, db, server.NewWithAPI(nil, server.NewHub(), db, spy))
	projectID := newProject(t, h, nil)
	base := "/api/projects/" + projectID
	seedSignSection(t, db, projectID, "Comparamos solo estudios.", false)
	rec := do(t, h, http.MethodPost, base+"/sections/sec-1/signoff", nil)
	if rec.Code != http.StatusCreated {
		t.Fatalf("signoff: %d %s", rec.Code, rec.Body.String())
	}
	for _, label := range []string{"prop-1", "prop-2"} {
		step := replaceStep(versionOf(t, db, projectID, "sec-1"), "Comparamos solo estudios.", "Comparamos estudios.")
		if applied := stepOn(t, db, projectID, step); !applied.Applied || len(applied.Proposals) != 1 || applied.Proposals[0] != label {
			t.Fatalf("step: %+v", applied)
		}
	}
	act(t, h, base, "prop-1", "discard")
	if spy.saved != 0 {
		t.Fatalf("discarding armed the batch: %d", spy.saved)
	}
	act(t, h, base, "prop-2", "accept")
	if spy.saved != 1 {
		t.Fatalf("accepting armed it %d times", spy.saved)
	}
}

// P-114: the conversation's line under your message lasts while its signoff does.
func TestB86_TheConversationSaysTheAgentAnswersWhenTheSignoffEnds(t *testing.T) {
	db, err := store.Open(context.Background(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	h := withVisitor(t, db, server.NewWithAPI(nil, server.NewHub(), db, &stubStarter{}))
	projectID := newProject(t, h, nil)
	base := "/api/projects/" + projectID
	seedSignSection(t, db, projectID, "Comparamos solo estudios.", false)
	rec := do(t, h, http.MethodPost, base+"/sections/sec-1/signoff", nil)
	var v graph.SignoffView
	if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil || rec.Code != http.StatusCreated {
		t.Fatalf("signoff: %d %s", rec.Code, rec.Body.String())
	}
	if err := db.Write(context.Background(), projectID, func(tx *store.Tx) error {
		_, err := tx.AppendEvent(store.EventInput{Type: store.EventHumanMessage, Actor: "human",
			Data: map[string]any{"text": "¿Y esto?", "queued": true, "waits_signoff": true}})
		return err
	}); err != nil {
		t.Fatal(err)
	}
	waits := func() bool {
		t.Helper()
		got := decode[[]server.ConversationEntry](t, do(t, h, http.MethodGet, base+"/conversation", nil))
		for _, e := range got {
			if e.Kind == "message" && e.Text == "¿Y esto?" {
				return e.WaitsSignoff
			}
		}
		t.Fatalf("no message: %+v", got)
		return false
	}
	if !waits() {
		t.Fatal("the line is missing during the signoff")
	}
	if rec := do(t, h, http.MethodPost, base+"/signoffs/"+v.Label+"/leave", nil); rec.Code != http.StatusOK {
		t.Fatalf("leave: %d", rec.Code)
	}
	if waits() {
		t.Fatal("the line outlived the signoff")
	}
	// Answered, a later signoff does not bring the line back.
	if err := db.Write(context.Background(), projectID, func(tx *store.Tx) error {
		_, err := tx.AppendEvent(store.EventInput{Type: store.EventStepEnded, Actor: "agent", AgentRole: "principal",
			Data: map[string]any{"turn_kind": "step"}})
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if rec := do(t, h, http.MethodPost, base+"/sections/sec-1/signoff", nil); rec.Code != http.StatusCreated {
		t.Fatalf("signoff: %d %s", rec.Code, rec.Body.String())
	}
	if waits() {
		t.Fatal("the line came back on an answered message")
	}
}

// P-115 item 3: a refused «ok» is no answer: one answer, one line.
func TestH1_OneAnswerToACardIsOneLineAfterARefusedOk(t *testing.T) {
	db, err := store.Open(context.Background(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	h := withVisitor(t, db, server.NewWithAPI(nil, server.NewHub(), db, &stubStarter{}))
	projectID := newProject(t, h, nil)
	base := "/api/projects/" + projectID
	seedSignSection(t, db, projectID, "Comparamos solo estudios.", true)
	if err := db.Write(context.Background(), projectID, func(tx *store.Tx) error {
		dec, err := tx.GetObject(context.Background(), projectID, "dec-1")
		if err != nil {
			return err
		}
		_, err = tx.CreateObject(store.ObjectCard, "open", map[string]any{"decision_id": dec.ID, "chosen_option": "", "reason": ""},
			store.EventInput{Type: store.EventStateChanged, Actor: "agent"})
		return err
	}); err != nil {
		t.Fatal(err)
	}
	card := func(reason string) int {
		return do(t, h, http.MethodPost, base+"/cards/card-1/answer", map[string]any{"option": "keep", "reason": reason}).Code
	}
	lines := func() int {
		n := 0
		for _, e := range decode[[]server.ConversationEntry](t, do(t, h, http.MethodGet, base+"/conversation", nil)) {
			if e.Kind == "card_answered" {
				n++
			}
		}
		return n
	}
	if code := card("ok"); code != http.StatusUnprocessableEntity {
		t.Fatalf("«ok»: %d", code)
	}
	if n := lines(); n != 0 {
		t.Fatalf("a refused «ok» shows %d lines", n)
	}
	if code := card("Porque así la comparación es justa."); code != http.StatusOK {
		t.Fatalf("answer: %d", code)
	}
	if n := lines(); n != 1 {
		t.Fatalf("one answer shows %d lines", n)
	}
}

// P-115 item 5: the view gives your reason apart from the question.
func TestE1_TheOpenQuestionViewGivesYourReasonApart(t *testing.T) {
	db, err := store.Open(context.Background(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	h := withVisitor(t, db, server.NewWithAPI(nil, server.NewHub(), db, &stubStarter{}))
	projectID := newProject(t, h, nil)
	base := "/api/projects/" + projectID
	seedSignSection(t, db, projectID, "Comparamos solo estudios.", true)
	const reason = "Porque el promedio esconde los extremos."
	rec := do(t, h, http.MethodPost, base+"/decisions/dec-1/object", map[string]any{"reason": reason})
	if rec.Code != http.StatusOK {
		t.Fatalf("object: %d %s", rec.Code, rec.Body.String())
	}
	body := do(t, h, http.MethodGet, base+"/todo", nil).Body.String()
	var todo struct {
		OpenQuestions []server.OpenQuestionView `json:"open_questions"`
	}
	if err := json.Unmarshal([]byte(body), &todo); err != nil || len(todo.OpenQuestions) != 1 {
		t.Fatalf("todo: %s, %v", body, err)
	}
	if q := todo.OpenQuestions[0]; q.ObjectionReason != reason || q.Question == reason {
		t.Fatalf("question %q, reason %q", q.Question, q.ObjectionReason)
	}
}

// The conversation says a queued message was not sent after a failure, and the
// retry route sends it: 202 with the turn to follow, 409 once it no longer waits.
func TestB101_ARetryFromTheConversationSendsAMessageThatWasNotSent(t *testing.T) {
	h, _, projectID, hold, _ := modeAPI(t, nil)
	base := "/api/projects/" + projectID
	hold.failing.Store(true)
	if rec := do(t, h, http.MethodPost, base+"/messages", map[string]string{"text": "Hola."}); rec.Code != http.StatusAccepted {
		t.Fatalf("first: %d", rec.Code)
	}
	<-hold.entered
	if rec := do(t, h, http.MethodPost, base+"/messages", map[string]string{"text": "Otra."}); rec.Code != http.StatusAccepted {
		t.Fatalf("second: %d", rec.Code)
	}
	close(hold.release)
	entry := func() *server.ConversationEntry {
		for _, e := range decode[[]server.ConversationEntry](t, do(t, h, http.MethodGet, base+"/conversation", nil)) {
			if e.Kind == "message" && e.Text == "Otra." {
				return &e
			}
		}
		return nil
	}
	var mark *server.ConversationEntry
	for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		if mark = entry(); mark != nil && mark.NotSent {
			break
		}
	}
	if mark == nil || !mark.Queued || !mark.NotSent || mark.N == 0 {
		t.Fatalf("after the failure: %+v, want queued and not sent", mark)
	}
	path := fmt.Sprintf("%s/messages/%d/retry", base, mark.N)
	hold.failing.Store(false)
	rec := do(t, h, http.MethodPost, path, nil)
	if rec.Code != http.StatusAccepted || decode[server.PostMessageResponse](t, rec).TurnID == "" {
		t.Fatalf("retry: %d %s, want 202 with a turn", rec.Code, rec.Body)
	}
	for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		if mark = entry(); mark != nil && !mark.Queued {
			break
		}
	}
	if mark == nil || mark.Queued || mark.NotSent {
		t.Fatalf("after the retry: %+v, want it delivered", mark)
	}
	if rec := do(t, h, http.MethodPost, path, nil); rec.Code != http.StatusConflict {
		t.Fatalf("retry twice: %d, want 409", rec.Code)
	}
}
