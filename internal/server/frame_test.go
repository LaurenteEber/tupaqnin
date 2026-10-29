package server_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/LaurenteEber/tupaqnin/internal/cycle"
	"github.com/LaurenteEber/tupaqnin/internal/demo/case5"
	"github.com/LaurenteEber/tupaqnin/internal/graph"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

const (
	synthesisText   = "Los estudios de campo coinciden en que el agua embotellada trae más partículas que la de grifo. La diferencia depende del filtro.\n\nCon el mismo filtro, la diferencia es de 1,5 veces."
	conclusionsText = "Con el mismo filtro, la diferencia es de 1,5 veces, menor de lo que sugieren los conteos crudos. Se mantiene la idea."
)

// seedCase5Frame takes the case-5 project past its request: one case
// that counts and one that does not for each criterion, «Síntesis» and
// «Conclusiones» (both conclusions) and the agent's critical decision
// used in both, so its card opens (T-5).
func seedCase5Frame(t *testing.T, h http.Handler, db *store.DB) string {
	t.Helper()
	projectID := newProject(t, h, map[string]any{"template": "case-5"})
	if rec := do(t, h, http.MethodPost, "/api/projects/"+projectID+"/request/submit", nil); rec.Code != http.StatusAccepted {
		t.Fatalf("submit: %d (%s)", rec.Code, rec.Body.String())
	}
	ctx := context.Background()
	agent := store.EventInput{Type: store.EventStateChanged, Actor: "agent"}
	err := db.Write(ctx, projectID, func(tx *store.Tx) error {
		criteria, err := tx.ListObjects(ctx, projectID, store.ObjectCriterion)
		if err != nil {
			return err
		}
		for _, c := range criteria {
			for _, counts := range []bool{true, false} {
				if _, err := tx.CreateObject(store.ObjectUnderstandingCase, "unconfirmed",
					map[string]any{"text": "Caso de " + c.Label, "counts": counts, "criterion_id": c.ID}, agent); err != nil {
					return err
				}
			}
		}
		return cycle.TrackCriticality(ctx, tx, "", func() error {
			var sections []store.Object
			for _, s := range []struct{ title, text string }{{"Síntesis", synthesisText}, {"Conclusiones", conclusionsText}} {
				sec, err := tx.CreateObject(store.ObjectSection, "draft", map[string]any{"title": s.title, "text": s.text,
					"role": map[string]any{"value": "conclusion", "state": "proposed"}, "open_changes": []any{}}, agent)
				if err != nil {
					return err
				}
				sections = append(sections, sec)
			}
			dec, err := tx.CreateObject(store.ObjectDecision, "agent", map[string]any{
				"statement": "Comparar solo con el mismo filtro.", "instead_of": "Comparar los conteos crudos.",
				"would_choose_if": "Todos usaran el mismo filtro.", "origin": "agent"}, agent)
			if err != nil {
				return err
			}
			for _, sec := range sections {
				if _, err := tx.AddEdge(store.EdgeUsedIn, dec.ID, sec.ID, "", store.EventInput{Type: store.EventEdge, Actor: "agent"}); err != nil {
					return err
				}
			}
			return nil
		})
	})
	if err != nil {
		t.Fatalf("seed case 5: %v", err)
	}
	return projectID
}

func getFrame(t *testing.T, h http.Handler, projectID string) graph.Frame {
	t.Helper()
	rec := do(t, h, http.MethodGet, "/api/projects/"+projectID+"/frame", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("frame: %d (%s)", rec.Code, rec.Body.String())
	}
	return decode[graph.Frame](t, rec)
}

func TestM1_FrameOfCase5BringsLiteralRequestCriteriaSectionsAndSharedCriticalDecision(t *testing.T) {
	h, db, _ := newAPI(t)
	projectID := seedCase5Frame(t, h, db)
	tpl, _, err := case5.Load()
	if err != nil {
		t.Fatal(err)
	}
	f := getFrame(t, h, projectID)

	if f.Purpose.RequestText != tpl.Text || f.Purpose.Audience != tpl.Audience || f.Purpose.PremiseText != tpl.Premise {
		t.Fatalf("purpose not literal: %+v", f.Purpose)
	}
	if len(f.Criteria) != len(tpl.Criteria) {
		t.Fatalf("criteria: %+v", f.Criteria)
	}
	for i, c := range f.Criteria {
		if c.Text != tpl.Criteria[i] || len(c.Cases) != 2 || !c.Cases[0].Counts || c.Cases[1].Counts {
			t.Fatalf("criterion %d: %+v", i, c)
		}
	}
	if len(f.Sections) != 2 || f.Sections[0].Title != "Síntesis" || f.Sections[1].Title != "Conclusiones" {
		t.Fatalf("sections: %+v", f.Sections)
	}
	if want := "Los estudios de campo coinciden en que el agua embotellada trae más partículas que la de grifo."; f.Sections[0].FirstLine != want {
		t.Fatalf("first line (M-4): %q, want %q", f.Sections[0].FirstLine, want)
	}
	var labels []string
	for _, s := range f.Sections {
		if len(s.Decisions) != 1 {
			t.Fatalf("section %s decisions: %+v", s.Title, s.Decisions)
		}
		d := s.Decisions[0]
		if d.Icon != graph.ReasonConclusionSection || d.Card == "" || len(d.Sections) != 2 || d.Seen {
			t.Fatalf("decision under %s: %+v", s.Title, d)
		}
		labels = append(labels, d.Label)
	}
	if labels[0] != labels[1] || labels[0] != "dec-1" {
		t.Fatalf("the shared decision must carry the same label in both sections: %v", labels)
	}
	if len(f.Todo.Cards) != 1 || f.Todo.Cards[0].Decision != "dec-1" || f.Todo.UnconfirmedCases != 4 {
		t.Fatalf("todo: %+v", f.Todo)
	}
	// Never an internal id (rule 7): only labels in the whole frame.
	raw, _ := json.Marshal(f)
	dec, _ := db.GetObject(context.Background(), projectID, "dec-1")
	if strings.Contains(string(raw), dec.ID) {
		t.Fatalf("frame shows an internal id: %s", raw)
	}
}

func TestM1_AnsweredCardBringsItsReasonAndSupersededDecisionIsNotInTheFrame(t *testing.T) {
	h, db, _ := newAPI(t)
	projectID := seedCase5Frame(t, h, db)
	base := "/api/projects/" + projectID
	reason := "Si no, comparo filtros y no aguas."
	if rec := do(t, h, http.MethodPost, base+"/cards/card-1/answer", map[string]any{"option": "other", "reason": reason}); rec.Code != http.StatusOK {
		t.Fatalf("answer: %d (%s)", rec.Code, rec.Body.String())
	}
	// The agent's superseded decision keeps an edge to «Síntesis», as a
	// step that replaced it could leave; the frame still excludes it.
	ctx := context.Background()
	err := db.Write(ctx, projectID, func(tx *store.Tx) error {
		old, err := tx.GetObject(ctx, projectID, "dec-1")
		if err != nil {
			return err
		}
		sec, err := tx.GetObject(ctx, projectID, "sec-1")
		if err != nil {
			return err
		}
		_, err = tx.AddEdge(store.EdgeUsedIn, old.ID, sec.ID, "", store.EventInput{Type: store.EventEdge, Actor: "agent"})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	f := getFrame(t, h, projectID)
	for _, s := range f.Sections {
		for _, d := range s.Decisions {
			if d.Label == "dec-1" {
				t.Fatalf("superseded dec-1 appears under %s", s.Title)
			}
		}
		if len(s.Decisions) != 1 || s.Decisions[0].Label != "dec-2" || s.Decisions[0].Reason != reason || s.Decisions[0].Card != "" {
			t.Fatalf("section %s: %+v, want dec-2 with its reason", s.Title, s.Decisions)
		}
	}
	if len(f.Todo.Cards) != 0 {
		t.Fatalf("todo after answer: %+v", f.Todo)
	}
}

func TestM1_ExpandingADecisionRecordsDecisionSeenWithFrame(t *testing.T) {
	h, db, _ := newAPI(t)
	projectID := seedCase5Frame(t, h, db)
	base := "/api/projects/" + projectID
	ctx := context.Background()
	before, _ := db.EventsSince(ctx, projectID, 0)

	if rec := do(t, h, http.MethodPost, base+"/frame/decisions/dec-1/seen", nil); rec.Code != http.StatusNoContent {
		t.Fatalf("seen: %d (%s)", rec.Code, rec.Body.String())
	}
	after, _ := db.EventsSince(ctx, projectID, 0)
	if len(after) != len(before)+1 {
		t.Fatalf("events: %d → %d, want one more", len(before), len(after))
	}
	ev := after[len(after)-1]
	dec, _ := db.GetObject(ctx, projectID, "dec-1")
	var data map[string]any
	_ = json.Unmarshal(ev.Data, &data)
	if ev.Type != store.EventDecisionSeen || ev.ObjectID != dec.ID || data["seen_via"] != "frame" || data["label"] != "dec-1" || ev.Actor != "human" {
		t.Fatalf("event: %+v %v", ev, data)
	}
	if f := getFrame(t, h, projectID); !f.Sections[0].Decisions[0].Seen {
		t.Fatalf("decision not seen after expanding: %+v", f.Sections[0].Decisions[0])
	}
	// Reading the frame records nothing (§9: scrolling records nothing).
	reads, _ := db.EventsSince(ctx, projectID, 0)
	if len(reads) != len(after) {
		t.Fatalf("reading the frame wrote %d events", len(reads)-len(after))
	}
	for _, label := range []string{"sec-1", "dec-9"} {
		if rec := do(t, h, http.MethodPost, base+"/frame/decisions/"+label+"/seen", nil); rec.Code != http.StatusNotFound {
			t.Fatalf("seen %s: %d, want 404", label, rec.Code)
		}
	}
}

func TestM1_FrameOfCase5Within300ms(t *testing.T) {
	h, db, _ := newAPI(t)
	projectID := seedCase5Frame(t, h, db)
	getFrame(t, h, projectID) // warm the connection
	start := time.Now()
	getFrame(t, h, projectID)
	if took := time.Since(start); took > 300*time.Millisecond {
		t.Fatalf("frame took %s, want ≤ 300 ms (§9)", took)
	}
}
