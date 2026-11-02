package server_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/LaurenteEber/tupaqnin/internal/cycle"
	"github.com/LaurenteEber/tupaqnin/internal/graph"
	"github.com/LaurenteEber/tupaqnin/internal/provider"
	"github.com/LaurenteEber/tupaqnin/internal/server"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// Backlog-r3 3.6: «Auditar sección», «No aplica» and the judge round's
// card through the API.

const judgedSection = "# Conteos\n\nLos conteos subieron 60 veces."

func judgeOutputFixture(t *testing.T, out map[string]any) provider.Fixture {
	t.Helper()
	text, err := json.Marshal(out)
	if err != nil {
		t.Fatal(err)
	}
	item, _ := json.Marshal(map[string]any{"type": "message", "role": "assistant",
		"content": []map[string]string{{"type": "output_text", "text": string(text)}}})
	return provider.Fixture{TurnKind: provider.TurnJudgeVerify, Items: []json.RawMessage{item}}
}

// heldStep holds the principal's step until release, so the batch stays
// where the test wants it.
type heldStep struct {
	inner   provider.Provider
	release chan struct{}
	entered chan struct{}
}

func (h *heldStep) Respond(ctx context.Context, req provider.Request, onDelta func(provider.Delta)) (provider.Response, error) {
	if req.TurnKind == provider.TurnStep {
		close(h.entered)
		h.entered = make(chan struct{})
		<-h.release
	}
	return h.inner.Respond(ctx, req, onDelta)
}

func judgeServer(t *testing.T, prov provider.Provider, db *store.DB) (http.Handler, string, *cycle.Runner) {
	t.Helper()
	hub := server.NewHub()
	runner := cycle.NewRunner(db, prov, func(_ context.Context, input []json.RawMessage) (int, error) { return len(input), nil }, hub)
	t.Cleanup(runner.Close)
	h := withVisitor(t, db, server.NewWithAPI(nil, hub, db, runner))
	projectID := newProject(t, h, nil)
	human := store.EventInput{Type: store.EventStateChanged, Actor: "human"}
	if err := db.Write(context.Background(), projectID, func(tx *store.Tx) error {
		if _, err := tx.CreateObject(store.ObjectRequest, "", map[string]any{"text": "Escribe los conteos."}, human); err != nil {
			return err
		}
		_, err := tx.CreateObject(store.ObjectSection, "draft", map[string]any{"title": "Conteos", "text": judgedSection,
			"role": map[string]any{"value": "other", "state": "confirmed"}, "open_changes": []any{}}, human)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	return h, projectID, runner
}

func waitJudgeCard(t *testing.T, h http.Handler, base, label string, done func(server.RoundCard) bool) server.RoundCard {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		card := decode[server.RoundCard](t, do(t, h, http.MethodGet, base+"/rounds/"+label, nil))
		if done(card) {
			return card
		}
		if time.Now().After(deadline) {
			t.Fatalf("card: %+v", card)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// B-67 (b): POST /sections/{label}/audit calls the judge always and shows
// the round's card in the conversation; a label that is not a section is
// 404; a section without text is 422.
func TestB67_PostAuditCallsTheJudgeAndTheCardShowsInTheConversation(t *testing.T) {
	db, err := store.Open(context.Background(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	fake := provider.NewFake()
	fake.Recorder = db
	h, projectID, _ := judgeServer(t, fake, db)
	base := "/api/projects/" + projectID

	if rec := do(t, h, http.MethodPost, base+"/sections/req-1/audit", nil); rec.Code != http.StatusNotFound {
		t.Fatalf("audit of a request: %d", rec.Code)
	}
	rec := do(t, h, http.MethodPost, base+"/sections/sec-1/audit", nil)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("audit: %d %s", rec.Code, rec.Body.String())
	}
	started := decode[server.StartRoundResponse](t, rec)
	if started.Label == "" || started.TurnID == "" {
		t.Fatalf("started: %+v", started)
	}
	card := waitJudgeCard(t, h, base, started.Label, func(c server.RoundCard) bool { return c.State == "closed" && !c.Running })
	if card.Kind != "judge" || card.Section == nil || card.Section.Label != "sec-1" || card.Section.Title != "Conteos" ||
		card.Reason != cycle.JudgeReasonAudit || card.JudgeStatus != cycle.JudgeDone {
		t.Fatalf("card: %+v", card)
	}
	found := false
	for _, e := range decode[[]server.ConversationEntry](t, do(t, h, http.MethodGet, base+"/conversation", nil)) {
		found = found || (e.Kind == "round" && e.Label == started.Label)
	}
	if !found {
		t.Fatal("the conversation does not list the judge round")
	}
}

// B-67: POST /rounds/{label}/flaws/{n}/discard writes flaw_discarded, only
// before the batch starts; the flaws show in «Lo que te toca» and as a
// warning in the text.
func TestB67_DiscardWritesFlawDiscardedOnlyBeforeTheBatchStarts(t *testing.T) {
	db, err := store.Open(context.Background(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	passage := "Los conteos subieron 60 veces."
	flaw := func(risk string) map[string]any {
		return map[string]any{"section": "sec-1", "passage": passage, "risk": risk, "scenario": "1. Piden la fuente."}
	}
	fake := provider.NewFake(judgeOutputFixture(t, map[string]any{"flaws": []any{flaw("unverifiable"), flaw("figures_and_calculations")}, "recheck": []any{}}))
	fake.Recorder = db
	h, projectID, runner := judgeServer(t, fake, db)
	base := "/api/projects/" + projectID
	label, err := runner.RunJudgeVerify(context.Background(), projectID, "sec-1", cycle.JudgeReasonRisk, []string{"unverifiable"})
	if err != nil {
		t.Fatal(err)
	}
	card := decode[server.RoundCard](t, do(t, h, http.MethodGet, base+"/rounds/"+label, nil))
	if len(card.Flaws) != 2 || card.Flaws[0].N != 1 || card.Flaws[1].Passage != passage || card.BatchStarted || len(card.Risks) != 1 {
		t.Fatalf("card: %+v", card)
	}
	frame := decode[graph.Frame](t, do(t, h, http.MethodGet, base+"/frame", nil))
	if len(frame.Todo.FlawsToReview) != 1 || frame.Todo.FlawsToReview[0].Round != label || frame.Todo.FlawsToReview[0].Flaws != 2 {
		t.Fatalf("todo: %+v", frame.Todo)
	}

	if rec := do(t, h, http.MethodPost, base+"/rounds/"+label+"/flaws/9/discard", nil); rec.Code != http.StatusNotFound {
		t.Fatalf("unknown flaw: %d", rec.Code)
	}
	if rec := do(t, h, http.MethodPost, base+"/rounds/"+label+"/flaws/x/discard", nil); rec.Code != http.StatusNotFound {
		t.Fatalf("a flaw that is no number: %d", rec.Code)
	}
	if rec := do(t, h, http.MethodPost, base+"/rounds/"+label+"/flaws/1/discard", nil); rec.Code != http.StatusNoContent {
		t.Fatalf("discard: %d %s", rec.Code, rec.Body.String())
	}
	if evs := eventsOf(t, db, projectID, store.EventFlawDiscarded); len(evs) != 1 {
		t.Fatalf("flaw_discarded: %d", len(evs))
	}
	if rec := do(t, h, http.MethodPost, base+"/rounds/"+label+"/flaws/1/discard", nil); rec.Code != http.StatusConflict {
		t.Fatalf("second discard: %d", rec.Code)
	}
	card = decode[server.RoundCard](t, do(t, h, http.MethodGet, base+"/rounds/"+label, nil))
	if !card.Flaws[0].Discarded || card.Flaws[1].Discarded {
		t.Fatalf("flaws: %+v", card.Flaws)
	}

	// The batch starts with the next step: «No aplica» answers 409 then.
	stepText := `{"stretch":"direct","triage_line":"x","closing_risk":"none","note":"n"}`
	item, _ := json.Marshal(map[string]any{"type": "message", "role": "assistant",
		"content": []map[string]string{{"type": "output_text", "text": stepText}}})
	fake2 := provider.NewFake(provider.Fixture{TurnKind: provider.TurnStep, Items: []json.RawMessage{item}})
	fake2.Recorder = db
	held := &heldStep{inner: fake2, release: make(chan struct{}), entered: make(chan struct{})}
	entered := held.entered
	hub := server.NewHub()
	runner2 := cycle.NewRunner(db, held, func(_ context.Context, input []json.RawMessage) (int, error) { return len(input), nil }, hub)
	t.Cleanup(runner2.Close)
	h2 := withVisitor(t, db, server.NewWithAPI(nil, hub, db, runner2))
	if rec := do(t, h2, http.MethodPost, base+"/messages", map[string]string{"text": "Sigue."}); rec.Code != http.StatusAccepted {
		t.Fatalf("message: %d %s", rec.Code, rec.Body.String())
	}
	<-entered
	if rec := do(t, h2, http.MethodPost, base+"/rounds/"+label+"/flaws/2/discard", nil); rec.Code != http.StatusConflict {
		t.Fatalf("discard after the batch started: %d %s", rec.Code, rec.Body.String())
	}
	if rec := do(t, h2, http.MethodPost, base+"/sections/sec-1/audit", nil); rec.Code != http.StatusConflict {
		t.Fatalf("audit during a turn: %d", rec.Code)
	}
	close(held.release)
}

// «No aplica» on a round that is not a judge's is 404.
func TestB67_DiscardOnAResearchRoundIsNotFound(t *testing.T) {
	db, err := store.Open(context.Background(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	fake := provider.NewFake()
	fake.Recorder = db
	h, projectID, _ := judgeServer(t, fake, db)
	human := store.EventInput{Type: store.EventStateChanged, Actor: "human"}
	if err := db.Write(context.Background(), projectID, func(tx *store.Tx) error {
		_, err := tx.CreateObject(store.ObjectRound, "closed", map[string]any{"kind": "research", "question": "q"}, human)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if rec := do(t, h, http.MethodPost, "/api/projects/"+projectID+"/rounds/rnd-1/flaws/1/discard", nil); rec.Code != http.StatusNotFound {
		t.Fatalf("discard: %d", rec.Code)
	}
}

// The report says the judge could not verify, naming the section.
func TestB67_TheStopReportViewSaysTheJudgeCouldNotVerify(t *testing.T) {
	db, err := store.Open(context.Background(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	step := `{"stretch":"build","triage_line":"x","closing_risk":"figures_and_calculations","note":"n","new_text":[{"section_id":"new-1","base_version":0,"anchor":"","claim_origin":"model_knowledge","operation":"insert_after","text":"# Otra\n\nOtra parte."}],"proposed_roles":[{"section_id":"new-1","role":"other","title":"Otra"}]}`
	item, _ := json.Marshal(map[string]any{"type": "message", "role": "assistant",
		"content": []map[string]string{{"type": "output_text", "text": step}}})
	bad := judgeOutputFixture(t, map[string]any{"flaws": "no"})
	fake := provider.NewFake(provider.Fixture{TurnKind: provider.TurnStep, Items: []json.RawMessage{item}}, bad)
	fake.Recorder = db
	h, projectID, _ := judgeServer(t, fake, db)
	base := "/api/projects/" + projectID
	if rec := do(t, h, http.MethodPost, base+"/messages", map[string]string{"text": "Escribe."}); rec.Code != http.StatusAccepted {
		t.Fatalf("message: %d", rec.Code)
	}
	deadline := time.Now().Add(10 * time.Second)
	var rec *httptest.ResponseRecorder
	for {
		rec = do(t, h, http.MethodGet, base+"/stop-report", nil)
		if rec.Code == http.StatusOK {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("no report: %d", rec.Code)
		}
		time.Sleep(10 * time.Millisecond)
	}
	view := decode[server.StopReportView](t, rec)
	if len(view.JudgeNotVerified) != 1 || view.JudgeNotVerified[0].Title != "Otra" {
		t.Fatalf("report: %+v", view)
	}
	if !view.ProposeSignoff {
		t.Fatal("a judge that failed does not block the signoff")
	}
}

// A chain without work that verifies after the report writes no other
// report (P-75 b): the report's view stops saying it could not verify.
func TestB67_TheReportViewDropsASectionVerifiedAfterIt(t *testing.T) {
	db, err := store.Open(context.Background(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	stepItem := func(text string) provider.Fixture {
		item, _ := json.Marshal(map[string]any{"type": "message", "role": "assistant",
			"content": []map[string]string{{"type": "output_text", "text": text}}})
		return provider.Fixture{TurnKind: provider.TurnStep, Items: []json.RawMessage{item}}
	}
	risk := `{"stretch":"build","triage_line":"x","closing_risk":"figures_and_calculations","note":"n","new_text":[{"section_id":"new-1","base_version":0,"anchor":"","claim_origin":"model_knowledge","operation":"insert_after","text":"# Otra\n\nOtra parte."}],"proposed_roles":[{"section_id":"new-1","role":"other","title":"Otra"}]}`
	noWork := `{"stretch":"direct","triage_line":"x","closing_risk":"none","note":"n"}`
	bad := judgeOutputFixture(t, map[string]any{"flaws": "no"})
	good := judgeOutputFixture(t, map[string]any{"flaws": []any{}, "recheck": []any{}})
	fake := provider.NewFake(stepItem(risk), stepItem(noWork), bad, bad, good)
	fake.Recorder = db
	h, projectID, _ := judgeServer(t, fake, db)
	base := "/api/projects/" + projectID
	send := func(text string) {
		t.Helper()
		deadline := time.Now().Add(10 * time.Second)
		for {
			rec := do(t, h, http.MethodPost, base+"/messages", map[string]string{"text": text})
			if rec.Code == http.StatusAccepted {
				return
			}
			if time.Now().After(deadline) {
				t.Fatalf("message: %d", rec.Code)
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
	report := func(want int) {
		t.Helper()
		deadline := time.Now().Add(10 * time.Second)
		for {
			rec := do(t, h, http.MethodGet, base+"/stop-report", nil)
			if rec.Code == http.StatusOK && len(decode[server.StopReportView](t, rec).JudgeNotVerified) == want {
				return
			}
			if time.Now().After(deadline) {
				t.Fatalf("report: %d %s", rec.Code, rec.Body.String())
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
	send("Escribe.")
	report(1)
	send("¿Cómo vas?")
	report(0)
}
