package server_test

import (
	"context"
	"encoding/json"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"github.com/LaurenteEber/tupaqnin/internal/cycle"
	"github.com/LaurenteEber/tupaqnin/internal/graph"
	"github.com/LaurenteEber/tupaqnin/internal/provider"
	"github.com/LaurenteEber/tupaqnin/internal/server"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

func eventsOf(t *testing.T, db *store.DB, projectID string, typ store.EventType) []store.Event {
	t.Helper()
	events, err := db.EventsSince(context.Background(), projectID, 0)
	if err != nil {
		t.Fatalf("EventsSince: %v", err)
	}
	var out []store.Event
	for _, ev := range events {
		if ev.Type == typ {
			out = append(out, ev)
		}
	}
	return out
}

// B-40 through the routes: the report is unseen in «Lo que te toca»
// until it is opened; opening it records stop_report_seen and
// decision_seen (report) for each decision it shows.
func TestB40_OpeningTheReportRecordsDecisionSeenForEachDecisionItShows(t *testing.T) {
	h, db, _ := newAPI(t)
	projectID := newProject(t, h, nil)
	base := "/api/projects/" + projectID
	if rec := do(t, h, http.MethodGet, base+"/stop-report", nil); rec.Code != http.StatusNotFound {
		t.Fatalf("GET without a report: %d", rec.Code)
	}
	seedSignSection(t, db, projectID, "Comparamos solo estudios con el mismo filtro.", true)
	report := cycle.StopReport{StopReason: cycle.StopUnseenDecisions, Section: "sec-1",
		Answer: "Paré: hay decisiones que no viste.", Done: []string{"Fijé la regla."}, Next: "Seguir con la síntesis.",
		Unseen: []cycle.ReportUnseen{{Decision: "dec-1", Gist: "Mismo filtro, no tal cual.", Sections: []string{"sec-1"}}}}
	report.Decisions = []graph.UnseenDecision{{Label: "dec-1",
		Statement: "Comparar solo estudios con el mismo tamaño mínimo.", Sections: []string{"sec-1"}}}
	if err := db.Write(context.Background(), projectID, func(tx *store.Tx) error {
		_, err := tx.AppendEvent(store.EventInput{Type: store.EventStopReport, Actor: "agent", AgentRole: "principal", Data: report})
		return err
	}); err != nil {
		t.Fatalf("seed report: %v", err)
	}

	frame := decode[map[string]any](t, do(t, h, http.MethodGet, base+"/frame", nil))
	if todo, _ := frame["todo"].(map[string]any); todo["stop_report_unseen"] != true {
		t.Fatalf("frame todo before opening: %v", frame["todo"])
	}
	rec := do(t, h, http.MethodGet, base+"/stop-report", nil)
	view := decode[server.StopReportView](t, rec)
	if rec.Code != http.StatusOK || view.Seen || view.StopReason != "unseen_decisions" || view.Section == nil ||
		view.Section.Title != "Regla de comparación" || len(view.Decisions) != 1 ||
		view.Decisions[0].Gist != "Mismo filtro, no tal cual." || view.Answer != report.Answer {
		t.Fatalf("GET before opening: %d %+v", rec.Code, view)
	}
	if len(eventsOf(t, db, projectID, store.EventDecisionSeen)) != 0 {
		t.Fatal("reading the report folded recorded decision_seen")
	}
	if rec := do(t, h, http.MethodPost, base+"/stop-report/seen", nil); rec.Code != http.StatusNoContent {
		t.Fatalf("POST seen: %d %s", rec.Code, rec.Body.String())
	}
	seen := eventsOf(t, db, projectID, store.EventDecisionSeen)
	var data map[string]any
	if len(seen) == 1 {
		_ = json.Unmarshal(seen[0].Data, &data)
	}
	if len(seen) != 1 || data["seen_via"] != "report" || data["label"] != "dec-1" || seen[0].Actor != "human" {
		t.Fatalf("decision_seen: %d %v", len(seen), data)
	}
	if n := len(eventsOf(t, db, projectID, store.EventStopReportSeen)); n != 1 {
		t.Fatalf("stop_report_seen: %d", n)
	}
	if view := decode[server.StopReportView](t, do(t, h, http.MethodGet, base+"/stop-report", nil)); !view.Seen {
		t.Fatal("the report is still unseen after opening it")
	}
	frame = decode[map[string]any](t, do(t, h, http.MethodGet, base+"/frame", nil))
	if todo, _ := frame["todo"].(map[string]any); todo["stop_report_unseen"] != false {
		t.Fatalf("frame todo after opening: %v", frame["todo"])
	}
}

// Backlog-r2 2.5: with the fake provider, a turn ends in P6 (the report
// in the conversation and at its route) and «Sigue» opens another turn,
// as a message of yours with the button's text.
func TestB37_WithTheFakeTheTurnShowsP6AndSigueOpensAnotherTurn(t *testing.T) {
	db, err := store.Open(context.Background(), t.TempDir())
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	fx, err := provider.LoadFixture(filepath.Join("..", "provider", "testdata", "app", "01-principal-valid.json"))
	if err != nil {
		t.Fatalf("LoadFixture: %v", err)
	}
	fake := provider.NewFake(fx)
	fake.Recorder = db
	count := func(ctx context.Context, input []json.RawMessage) (int, error) {
		return fake.CountTokens(ctx, provider.PrincipalModel, input)
	}
	hub := server.NewHub()
	runner := cycle.NewRunner(db, fake, count, hub)
	t.Cleanup(runner.Close)
	h := withVisitor(t, db, server.NewWithAPI(nil, hub, db, runner))
	projectID := newProject(t, h, nil)
	base := "/api/projects/" + projectID
	if err := runner.RunTurn(context.Background(), projectID, "turn-1", "Escribe el alcance."); err != nil {
		t.Fatalf("RunTurn: %v", err)
	}

	view := decode[server.StopReportView](t, do(t, h, http.MethodGet, base+"/stop-report", nil))
	if view.StopReason != "section_ready" || !view.ProposeSignoff || view.Section == nil || view.Section.Title != "Alcance" ||
		view.Answer == "" || view.Seen {
		t.Fatalf("stop report: %+v", view)
	}
	entries := decode[[]server.ConversationEntry](t, do(t, h, http.MethodGet, base+"/conversation", nil))
	if last := entries[len(entries)-1]; last.Kind != "stop_report" || last.Text != view.Answer || last.StopReason != "section_ready" {
		t.Fatalf("last conversation entry: %+v", last)
	}

	if rec := do(t, h, http.MethodPost, base+"/stop-report/continue", map[string]any{"text": "  "}); rec.Code != http.StatusBadRequest {
		t.Fatalf("continue without text: %d", rec.Code)
	}
	rec := do(t, h, http.MethodPost, base+"/stop-report/continue", map[string]any{"text": "Sigue"})
	if rec.Code != http.StatusAccepted || decode[server.PostMessageResponse](t, rec).TurnID == "" {
		t.Fatalf("continue: %d %s", rec.Code, rec.Body.String())
	}
	deadline := time.Now().Add(5 * time.Second)
	for len(eventsOf(t, db, projectID, store.EventStopReport)) < 2 {
		if time.Now().After(deadline) {
			t.Fatal("«Sigue» did not run another turn to its stop")
		}
		time.Sleep(10 * time.Millisecond)
	}
	var texts []string
	for _, ev := range eventsOf(t, db, projectID, store.EventHumanMessage) {
		var data map[string]any
		_ = json.Unmarshal(ev.Data, &data)
		texts = append(texts, data["text"].(string))
	}
	if len(texts) != 2 || texts[1] != "Sigue" {
		t.Fatalf("human messages: %q", texts)
	}
}

// P-75 (c): a report whose principal failed twice is exposed as
// incomplete by Go, in GET /stop-report (principal_failed) and in the
// conversation (incomplete); a well-formed one is not.
func TestP75_AFailedReportIsIncompleteInTheRoutesAndAWellFormedOneIsNot(t *testing.T) {
	h, db, _ := newAPI(t)
	projectID := newProject(t, h, nil)
	base := "/api/projects/" + projectID
	for _, report := range []cycle.StopReport{
		{StopReason: cycle.StopNoWork, Answer: "Paré.", Done: []string{}, Unseen: []cycle.ReportUnseen{}},
		{StopReason: cycle.StopNoWork, Done: []string{}, Unseen: []cycle.ReportUnseen{}, PrincipalFailed: true},
	} {
		if err := db.Write(context.Background(), projectID, func(tx *store.Tx) error {
			_, err := tx.AppendEvent(store.EventInput{Type: store.EventStopReport, Actor: "agent", AgentRole: "principal", Data: report})
			return err
		}); err != nil {
			t.Fatalf("seed report: %v", err)
		}
	}
	view := decode[server.StopReportView](t, do(t, h, http.MethodGet, base+"/stop-report", nil))
	if !view.PrincipalFailed {
		t.Fatalf("GET /stop-report: %+v, want principal_failed", view)
	}
	var reports []server.ConversationEntry
	for _, e := range decode[[]server.ConversationEntry](t, do(t, h, http.MethodGet, base+"/conversation", nil)) {
		if e.Kind == "stop_report" {
			reports = append(reports, e)
		}
	}
	if len(reports) != 2 || reports[0].Incomplete || !reports[1].Incomplete {
		t.Fatalf("conversation stop reports: %+v, want the second incomplete only", reports)
	}
}
