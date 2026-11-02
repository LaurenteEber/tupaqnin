package server_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/LaurenteEber/tupaqnin/internal/cycle"
	"github.com/LaurenteEber/tupaqnin/internal/evidence"
	"github.com/LaurenteEber/tupaqnin/internal/provider"
	"github.com/LaurenteEber/tupaqnin/internal/server"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// Backlog-r3 3.4 through the API: the round card carries the digest
// (first the answer, «Descartados (n)», «Sin revisar»), «Repetir con
// otra pregunta» with its cost when the doubt was not reduced, and
// «dejarlo por aclarar»; nothing repeats without your click.

func messageFixture(kind provider.TurnKind, text string) provider.Fixture {
	item, _ := json.Marshal(map[string]any{"type": "message", "role": "assistant",
		"content": []map[string]string{{"type": "output_text", "text": text}}})
	return provider.Fixture{TurnKind: kind, Items: []json.RawMessage{item}}
}

// waitCard reads the round card until its round closed.
func waitCard(t *testing.T, h http.Handler, base, label string) server.RoundCard {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		card := decode[server.RoundCard](t, do(t, h, http.MethodGet, base+"/rounds/"+label, nil))
		if card.State == "closed" && !card.Running {
			return card
		}
		if time.Now().After(deadline) {
			t.Fatalf("round %s did not close: %+v", label, card)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestK5_RoundCardCarriesTheDigestAndRepeatAndLeaveOpenAreYourClicks(t *testing.T) {
	db, err := store.Open(context.Background(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	reader := `{"findings":[` +
		`{"kind":"fact","passage":"Los filtros finos retienen más partículas.","source_id":"src-1","hit":null,"reference":null,"note":null},` +
		`{"kind":"fact","passage":"Una frase que no está.","source_id":"src-1","hit":null,"reference":null,"note":null}],` +
		`"covered":[],"sources_read":["src-1"]}`
	digest := `{"answer":"Los filtros finos retienen más.","outline":["Filtros."],"findings":[{"finding":1,"outline_line":1}],` +
		`"expectation_contrast":null,"reduced_doubt":"no","contradictions":[]}`
	fake := provider.NewFake(messageFixture(provider.TurnReader, reader), messageFixture(provider.TurnDigest, digest))
	fake.Recorder = db
	hub := server.NewHub()
	count := func(ctx context.Context, input []json.RawMessage) (int, error) {
		return fake.CountTokens(ctx, provider.PrincipalModel, input)
	}
	runner := cycle.NewRunner(db, fake, count, hub)
	runner.Evidence = &noEvidence{}
	t.Cleanup(runner.Close)
	h := withVisitor(t, db, server.NewWithAPI(nil, hub, db, runner))
	projectID := newProject(t, h, nil)
	human := store.EventInput{Type: store.EventStateChanged, Actor: "human"}
	if err := db.Write(context.Background(), projectID, func(tx *store.Tx) error {
		if _, err := tx.CreateObject(store.ObjectRequest, "", map[string]any{"text": "Busca evidencia."}, human); err != nil {
			return err
		}
		_, err := tx.CreateObject(store.ObjectSource, "", map[string]any{"kind": "file", "filename": "a.md",
			"text": "Los filtros finos retienen más partículas."}, human)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	base := "/api/projects/" + projectID
	if rec := do(t, h, http.MethodPost, base+"/rounds", nil); rec.Code != http.StatusAccepted {
		t.Fatalf("POST /rounds: %d %s", rec.Code, rec.Body.String())
	}
	card := waitCard(t, h, base, "rnd-1")
	if card.Digest == nil || card.Digest.Answer != "Los filtros finos retienen más." || card.ReducedDoubt != "no" ||
		len(card.Digest.Outline) != 1 || len(card.Digest.Outline[0].Findings) != 1 ||
		card.Digest.Outline[0].Findings[0].Source != "src-1" || len(card.Discarded) == 0 ||
		card.Discarded[0].Reason != cycle.ReasonPassageNotFound || card.Repeat == nil || card.Repeat.EstimatedCostUSD <= 0 ||
		len(card.Coverage) == 0 || card.Unused == nil || card.ReaderContradictions == nil {
		t.Fatalf("card: %+v", card)
	}
	// Nothing repeats without your click.
	if rec := do(t, h, http.MethodGet, base+"/rounds/rnd-2", nil); rec.Code != http.StatusNotFound {
		t.Fatalf("rnd-2 before your click: %d", rec.Code)
	}

	if rec := do(t, h, http.MethodPost, base+"/rounds/rnd-1/repeat", map[string]string{"question": ""}); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("repeat without a question: %d", rec.Code)
	}
	rec := do(t, h, http.MethodPost, base+"/rounds/rnd-1/repeat", map[string]string{"question": "¿Qué filtros?"})
	if rec.Code != http.StatusAccepted {
		t.Fatalf("repeat: %d %s", rec.Code, rec.Body.String())
	}
	got := decode[server.RepeatRoundResponse](t, rec)
	if got.Label != "rnd-2" || got.TurnID == "" || got.EstimatedCostUSD != card.Repeat.EstimatedCostUSD {
		t.Fatalf("repeat response: %+v", got)
	}
	waitCard(t, h, base, "rnd-2")

	rec = do(t, h, http.MethodPost, base+"/rounds/rnd-1/leave-open", nil)
	if rec.Code != http.StatusCreated || decode[server.LeaveRoundOpenResponse](t, rec).OpenQuestion == "" {
		t.Fatalf("leave-open: %d %s", rec.Code, rec.Body.String())
	}
	if rec := do(t, h, http.MethodPost, base+"/rounds/rnd-1/leave-open", nil); rec.Code != http.StatusConflict {
		t.Fatalf("leave-open twice: %d", rec.Code)
	}
	// «Lo que te toca» lists it, its options as codes the page translates.
	todo := decode[server.TodoView](t, do(t, h, http.MethodGet, base+"/todo", nil))
	if len(todo.OpenQuestions) != 1 || len(todo.OpenQuestions[0].Options) != 2 ||
		todo.OpenQuestions[0].Options[0].Code != cycle.ScopeOptionCompleteReview ||
		todo.OpenQuestions[0].Options[1].Code != cycle.ScopeOptionDeclareLimit {
		t.Fatalf("todo: %+v", todo.OpenQuestions)
	}
	if card := decode[server.RoundCard](t, do(t, h, http.MethodGet, base+"/rounds/rnd-1", nil)); card.ScopeQuestion != todo.OpenQuestions[0].Label {
		t.Fatalf("card scope question: %q", card.ScopeQuestion)
	}
}

// noEvidence is evidence that never answers (no network).
type noEvidence struct{}

func (noEvidence) SearchLiterature(context.Context, evidence.Query) ([]evidence.Hit, error) {
	return nil, nil
}
func (noEvidence) FetchText(context.Context, string) (evidence.Text, error) {
	return evidence.Text{}, evidence.ErrUnavailable
}
func (noEvidence) Lookup(context.Context, evidence.Reference) (evidence.Record, error) {
	return evidence.Record{}, evidence.ErrNotFound
}
