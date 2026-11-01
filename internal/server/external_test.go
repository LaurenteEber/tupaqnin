package server_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/LaurenteEber/tupaqnin/internal/cycle"
	"github.com/LaurenteEber/tupaqnin/internal/evidence"
	"github.com/LaurenteEber/tupaqnin/internal/graph"
	"github.com/LaurenteEber/tupaqnin/internal/provider"
	"github.com/LaurenteEber/tupaqnin/internal/server"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// Backlog-r3 3.5 through the API.

func readerFixture() provider.Fixture {
	item, _ := json.Marshal(map[string]any{"type": "message", "role": "assistant", "content": []map[string]string{{
		"type": "output_text", "text": `{"findings":[],"covered":[],"sources_read":[]}`}}})
	return provider.Fixture{TurnKind: provider.TurnReader, Items: []json.RawMessage{item}}
}

func externalServer(t *testing.T) (*store.DB, http.Handler, string) {
	t.Helper()
	db, err := store.Open(context.Background(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	fake := provider.NewFake(readerFixture(), readerFixture())
	fake.Recorder = db
	hub := server.NewHub()
	runner := cycle.NewRunner(db, fake, nil, hub)
	runner.Evidence = evidence.NewReplay()
	t.Cleanup(runner.Close)
	h := server.NewWithAPI(nil, hub, db, runner)
	projectID := newProject(t, h, nil)
	if err := db.Write(context.Background(), projectID, func(tx *store.Tx) error {
		_, err := tx.CreateObject(store.ObjectRequest, "", map[string]any{"text": "Un estado del arte sobre pobreza.",
			"evidence_required": map[string]any{"value": true, "why": "Resume estudios.", "state": "proposed"}},
			store.EventInput{Type: store.EventStateChanged, Actor: "human"})
		return err
	}); err != nil {
		t.Fatal(err)
	}
	return db, h, projectID
}

// readersEnded waits for a round's readers: the fake has no digest to give.
func readersEnded(t *testing.T, h http.Handler, base, label string) server.RoundCard {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		card := decode[server.RoundCard](t, do(t, h, http.MethodGet, base+"/rounds/"+label, nil))
		if !card.Running && len(card.Parts) > 0 && card.Parts[len(card.Parts)-1].Status == cycle.ReaderReturned {
			return card
		}
		if time.Now().After(deadline) {
			t.Fatalf("round %s did not end: %+v", label, card)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// R-4, B-72: without material and with evidence_required the card says so
// and «buscar evidencia» creates the round although the principal asked
// for none: your act, reason and parts the app's; the first material
// uploaded ends the offer of uploading.
func TestR4_WithoutMaterialSearchingCreatesTheRoundThoughThePrincipalAskedForNone(t *testing.T) {
	db, h, projectID := externalServer(t)
	base := "/api/projects/" + projectID
	view := decode[server.UnderstandingView](t, do(t, h, http.MethodGet, base+"/understanding", nil))
	if !view.NoMaterial {
		t.Fatalf("understanding: %+v, want no_material", view)
	}
	if rounds, _ := db.ListObjects(context.Background(), projectID, store.ObjectRound); len(rounds) != 0 {
		t.Fatalf("rounds before your act: %d", len(rounds))
	}
	rec := do(t, h, http.MethodPost, base+"/rounds", nil)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("POST /rounds: %d %s", rec.Code, rec.Body.String())
	}
	started := decode[server.StartRoundResponse](t, rec)
	card := readersEnded(t, h, base, started.Label)
	if card.Question != "Un estado del arte sobre pobreza." || card.DelegationReason != "too_much_to_read" ||
		len(card.Parts) != 1 || len(card.Parts[0].Where) != 1 || card.Parts[0].Where[0] != "openalex" {
		t.Fatalf("card: %+v", card)
	}
	events, _ := db.EventsSince(context.Background(), projectID, 0)
	for _, ev := range events {
		if ev.Type == store.EventStepEnded {
			t.Fatalf("the principal took no step, but: %s", ev.Data)
		}
	}
	if err := db.Write(context.Background(), projectID, func(tx *store.Tx) error {
		_, err := tx.CreateObject(store.ObjectSource, "", map[string]any{"kind": "file", "filename": "a.md", "text": "Texto."},
			store.EventInput{Type: store.EventStateChanged, Actor: "human"})
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if view := decode[server.UnderstandingView](t, do(t, h, http.MethodGet, base+"/understanding", nil)); view.NoMaterial {
		t.Fatalf("understanding with material: %+v", view)
	}
}

// R-2, B-69: the frame and the paragraph's history show the candidate with
// its number, and POST /rounds {references: [n]} is «Buscarla», on the same
// route; a number that is no candidate is 422 and creates nothing.
func TestB69_FrameAndHistoryShowTheCandidateAndPostRoundsSearchesIt(t *testing.T) {
	db, h, projectID := externalServer(t)
	base := "/api/projects/" + projectID
	const para = "Según Valdivia y Ortiz (2022) los conteos suben."
	if err := db.Write(context.Background(), projectID, func(tx *store.Tx) error {
		_, err := tx.CreateObject(store.ObjectSection, "draft", map[string]any{"title": "Síntesis", "text": para,
			"role": map[string]any{"value": "other", "state": "confirmed"}, "open_changes": []any{}},
			store.EventInput{Type: store.EventStateChanged, Actor: "agent"})
		if err != nil {
			return err
		}
		_, err = tx.AppendEvent(store.EventInput{Type: store.EventStepEnded, Actor: "agent", AgentRole: "principal",
			Data: map[string]any{"turn_kind": "step", "named_references": []map[string]any{
				{"text": "Valdivia y Ortiz (2022)", "title": "Conteos en Lima", "authors": nil, "year": 2022, "doi": nil}}}})
		return err
	}); err != nil {
		t.Fatal(err)
	}
	frame := decode[graph.Frame](t, do(t, h, http.MethodGet, base+"/frame", nil))
	if len(frame.Candidates) != 1 || frame.Candidates[0].N != 1 || frame.Candidates[0].Text != "Valdivia y Ortiz (2022)" {
		t.Fatalf("frame candidates: %+v", frame.Candidates)
	}
	hist := decode[server.BlockHistoryView](t, do(t, h, http.MethodGet, base+"/sections/sec-1/history?block=1", nil))
	if len(hist.Provenance.Candidates) != 1 || hist.Provenance.Verified || hist.Provenance.Origin != graph.OriginUnverified {
		t.Fatalf("history: %+v", hist.Provenance)
	}
	if frame.Sections[0].Unverified != 1 {
		t.Fatalf("frame section: %+v, a candidate never counts as a source", frame.Sections[0])
	}

	if rec := do(t, h, http.MethodPost, base+"/rounds", map[string]any{"references": []int{9}}); rec.Code != http.StatusUnprocessableEntity ||
		!strings.Contains(rec.Body.String(), "candidate_not_found") {
		t.Fatalf("POST /rounds with candidate 9: %d %s", rec.Code, rec.Body.String())
	}
	if rounds, _ := db.ListObjects(context.Background(), projectID, store.ObjectRound); len(rounds) != 0 {
		t.Fatalf("rounds after a refusal: %d", len(rounds))
	}
	rec := do(t, h, http.MethodPost, base+"/rounds", map[string]any{"references": []int{1}})
	if rec.Code != http.StatusAccepted {
		t.Fatalf("POST /rounds with candidate 1: %d %s", rec.Code, rec.Body.String())
	}
	card := readersEnded(t, h, base, decode[server.StartRoundResponse](t, rec).Label)
	if card.Question != "Valdivia y Ortiz (2022)" || len(card.Parts) != 1 || card.Parts[0].Where[0] != "openalex" {
		t.Fatalf("card: %+v", card)
	}
	// B-72 keeps working with a body that names no candidate.
	deadline := time.Now().Add(10 * time.Second)
	for {
		rec := do(t, h, http.MethodPost, base+"/rounds", map[string]any{})
		if rec.Code == http.StatusAccepted {
			if got := decode[server.StartRoundResponse](t, rec); got.Label != "rnd-2" {
				t.Fatalf("POST /rounds with an empty object: %+v", got)
			}
			break
		}
		if rec.Code != http.StatusConflict || time.Now().After(deadline) {
			t.Fatalf("POST /rounds with an empty object: %d %s", rec.Code, rec.Body.String())
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// Backlog-r3 3.5: the section list says beside each paragraph where it
// comes from, and the frame's decisions carry the searched sources of
// their section with title, year and link.
func TestR1_TheSectionListAndTheFrameShowTheOrigin(t *testing.T) {
	db, h, projectID := externalServer(t)
	base := "/api/projects/" + projectID
	const passage = "Micro-FTIR detects particles down to 20 micrometres."
	const para = "Micro-FTIR llega hasta 20 micrómetros."
	human := store.EventInput{Type: store.EventStateChanged, Actor: "human"}
	var section, source, decision store.Object
	if err := db.Write(context.Background(), projectID, func(tx *store.Tx) error {
		var err error
		if section, err = tx.CreateObject(store.ObjectSection, "draft", map[string]any{"title": "Síntesis", "text": para + "\n\nOtro párrafo.",
			"role": map[string]any{"value": "conclusion", "state": "confirmed"}, "open_changes": []any{}}, human); err != nil {
			return err
		}
		if source, err = tx.CreateObject(store.ObjectSource, "", map[string]any{"kind": graph.SourceKindWeb, "url": "https://openalex.example/W1",
			"text": passage, "title": "Detection limits", "year": 2021, "access": "abstract"}, human); err != nil {
			return err
		}
		cit, err := tx.CreateObject(store.ObjectCitation, "", map[string]any{"section_id": section.ID, "anchor": para, "passage": passage}, human)
		if err != nil {
			return err
		}
		if _, err := tx.AddEdge(store.EdgeComesFrom, cit.ID, source.ID, "", nil, store.EventInput{Type: store.EventEdge, Actor: "app"}); err != nil {
			return err
		}
		if decision, err = tx.CreateObject(store.ObjectDecision, "human", map[string]any{"statement": "Usar Micro-FTIR.", "instead_of": "otra",
			"would_choose_if": "x", "origin": "human"}, human); err != nil {
			return err
		}
		_, err = tx.AddEdge(store.EdgeUsedIn, decision.ID, section.ID, "", nil, store.EventInput{Type: store.EventEdge, Actor: "app"})
		return err
	}); err != nil {
		t.Fatal(err)
	}
	sections := decode[[]server.SectionView](t, do(t, h, http.MethodGet, base+"/sections", nil))
	if len(sections) != 1 || len(sections[0].Origins) != 2 ||
		sections[0].Origins[0] != (server.ParagraphOrigin{Block: 1, Origin: graph.OriginExternal}) ||
		sections[0].Origins[1] != (server.ParagraphOrigin{Block: 2, Origin: graph.OriginUnverified}) {
		t.Fatalf("sections: %+v", sections)
	}
	frame := decode[graph.Frame](t, do(t, h, http.MethodGet, base+"/frame", nil))
	if len(frame.Sections) != 1 || len(frame.Sections[0].Decisions) != 1 {
		t.Fatalf("frame: %+v", frame.Sections)
	}
	got := frame.Sections[0].Decisions[0].Sources
	if len(got) != 1 || got[0] != (graph.SearchedSource{Label: source.Label, Title: "Detection limits", Year: 2021, URL: "https://openalex.example/W1"}) {
		t.Fatalf("decision's sources: %+v", got)
	}
}
