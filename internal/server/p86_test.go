package server_test

import (
	"context"
	"encoding/json"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"github.com/LaurenteEber/tupaqnin/internal/cycle"
	"github.com/LaurenteEber/tupaqnin/internal/provider"
	"github.com/LaurenteEber/tupaqnin/internal/server"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// P-86 (Eber 31-oct, fichas §8.2): your answer to the last card of a
// concluding section prepares its defense; its audience questions show
// in the conversation as open questions, with no stop report, and they
// block nothing: the signoff of the section opens.
func TestP86_TheAudienceQuestionsShowInTheConversationAndBlockNothing(t *testing.T) {
	db, err := store.Open(context.Background(), t.TempDir())
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	fx, err := provider.LoadFixture(filepath.Join("..", "provider", "testdata", "judge", "conclusion-section.json"))
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

	ctx := context.Background()
	err = db.Write(ctx, projectID, func(tx *store.Tx) error {
		human := store.EventInput{Type: store.EventStateChanged, Actor: "human"}
		agent := store.EventInput{Type: store.EventStateChanged, Actor: "agent", AgentRole: "principal"}
		sec, err := tx.CreateObject(store.ObjectSection, "draft", map[string]any{
			"title": "Regla de comparación",
			"text":  "Comparamos solo estudios con el mismo tamaño de partícula.\n\nAsí la comparación es justa y repetible.",
			"role":  map[string]any{"value": "conclusion", "state": "confirmed"}, "open_changes": []any{},
		}, human)
		if err != nil {
			return err
		}
		mine, err := tx.CreateObject(store.ObjectDecision, "human", map[string]any{
			"statement": "Comparar solo estudios con el mismo tamaño de partícula.", "origin": "human",
			"instead_of": "Comparar los conteos tal cual."}, human)
		if err != nil {
			return err
		}
		theirs, err := tx.CreateObject(store.ObjectDecision, "agent", map[string]any{
			"statement": "Excluir los estudios sin filtro.", "origin": "agent",
			"instead_of": "Incluir todos los estudios."}, agent)
		if err != nil {
			return err
		}
		for _, d := range []store.Object{mine, theirs} {
			if _, err := tx.AddEdge(store.EdgeUsedIn, d.ID, sec.ID, "", nil,
				store.EventInput{Type: store.EventEdge, Actor: "agent", AgentRole: "principal"}); err != nil {
				return err
			}
		}
		_, err = tx.CreateObject(store.ObjectCard, "open", map[string]any{"decision_id": theirs.ID}, agent)
		return err
	})
	if err != nil {
		t.Fatalf("seed: %v", err)
	}

	rec := do(t, h, http.MethodPost, base+"/cards/card-1/answer",
		map[string]any{"option": "keep", "reason": "Porque compara estudios con el mismo filtro."})
	if rec.Code != http.StatusOK {
		t.Fatalf("answer card: %d %s", rec.Code, rec.Body.String())
	}
	// P-92: the answer says a defense is being prepared.
	if got := decode[server.AnswerCardResponse](t, rec); !got.DefensePreparing {
		t.Fatalf("answer: %+v, want defense_preparing", got)
	}
	// P-90: the defense runs in the background; its questions show when
	// the conversation updates.
	var got, questions []server.ConversationEntry
	for deadline := time.Now().Add(5 * time.Second); ; time.Sleep(10 * time.Millisecond) {
		got = decode[[]server.ConversationEntry](t, do(t, h, http.MethodGet, base+"/conversation", nil))
		questions = questions[:0]
		for _, e := range got {
			if e.Kind == "audience_question" {
				questions = append(questions, e)
			}
		}
		if len(questions) >= 4 || time.Now().After(deadline) {
			break
		}
	}
	// P-92: once they show, the summary says the preparation ended.
	runner.Close()
	if sum := decode[server.SummaryView](t, do(t, h, http.MethodGet, base+"/summary", nil)); sum.DefensePreparing {
		t.Fatalf("summary after the defense: %+v, want defense_preparing false", sum)
	}
	if len(questions) != 4 {
		t.Fatalf("conversation: %+v, want the judge's 3 questions and 1 «no cubre»", got)
	}
	for _, q := range questions {
		if q.Label == "" || q.State != "open" || q.Text == "" || q.Section != "Regla de comparación" {
			t.Fatalf("audience question entry: %+v, want open, with its text, label and section title", q)
		}
	}
	if !questions[3].NotCovered || questions[0].NotCovered {
		t.Fatalf("entries: %+v, want only the last one marked not covered", questions)
	}
	// They block nothing: asking to sign opens the signoff.
	rec = do(t, h, http.MethodPost, base+"/sections/sec-1/signoff", nil)
	if rec.Code != http.StatusOK && rec.Code != http.StatusCreated {
		t.Fatalf("request signoff: %d %s", rec.Code, rec.Body.String())
	}
	var open struct {
		Phase    string `json:"phase"`
		Pendings []any  `json:"pendings"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &open)
	if open.Phase != "in_progress" || len(open.Pendings) != 0 {
		t.Fatalf("request signoff: %s, want it opened", rec.Body.String())
	}
	// Answered, it keeps showing in the conversation with its new state.
	if rec := do(t, h, http.MethodPost, base+"/audience-questions/"+questions[0].Label+"/answer",
		map[string]any{"answer": "no"}); rec.Code != http.StatusOK {
		t.Fatalf("answer audience question: %d %s", rec.Code, rec.Body.String())
	}
	got = decode[[]server.ConversationEntry](t, do(t, h, http.MethodGet, base+"/conversation", nil))
	for _, e := range got {
		if e.Kind == "audience_question" && e.Label == questions[0].Label && e.State != "declined" {
			t.Fatalf("answered entry: %+v, want declined", e)
		}
	}
}
