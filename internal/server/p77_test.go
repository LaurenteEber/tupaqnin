package server_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/LaurenteEber/tupaqnin/internal/cycle"
	"github.com/LaurenteEber/tupaqnin/internal/graph"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// frameDecision is the decision with the label under any section of the
// frame, and whether it is in the filter «solo lo que no viste».
func frameDecision(t *testing.T, f graph.Frame, label string) (graph.FrameDecision, bool) {
	t.Helper()
	var dec *graph.FrameDecision
	for _, s := range f.Sections {
		for i := range s.Decisions {
			if s.Decisions[i].Label == label {
				dec = &s.Decisions[i]
			}
		}
	}
	if dec == nil {
		t.Fatalf("%s is not in the frame", label)
	}
	inFilter := false
	for _, u := range f.Unseen {
		inFilter = inFilter || u.Decision.Label == label
	}
	return *dec, inFilter
}

// P-77 (B-39 «tus decisiones las viste al tomarlas»): the decision born
// from your «la otra opción» on a card counts as seen: no dot, and not
// in the filter.
func TestP77_DecisionBornFromYourCardAnswerCountsAsSeen(t *testing.T) {
	h, db, _ := newAPI(t)
	projectID := seedCase5Frame(t, h, db)
	base := "/api/projects/" + projectID
	if rec := do(t, h, http.MethodPost, base+"/cards/card-1/answer",
		map[string]any{"option": "other", "reason": "Porque el comité pidió conteos crudos."}); rec.Code != http.StatusOK {
		t.Fatalf("answer: %d (%s)", rec.Code, rec.Body.String())
	}
	if n := len(eventsOf(t, db, projectID, store.EventDecisionSeen)); n != 1 {
		t.Fatalf("decision_seen events: %d, want only the replaced one's", n)
	}
	dec, inFilter := frameDecision(t, getFrame(t, h, projectID), "dec-2")
	if !dec.Seen || inFilter {
		t.Fatalf("dec-2 (born from your answer): seen %v, in filter %v; want seen and out of the filter", dec.Seen, inFilter)
	}
}

// P-77: a critical decision of your own (origin human) counts as seen
// without any decision_seen; one of the agent keeps its dot until seen.
func TestB39_YourOwnDecisionCountsAsSeenAndTheAgentsKeepsItsDot(t *testing.T) {
	h, db, _ := newAPI(t)
	projectID := seedCase5Frame(t, h, db) // dec-1, of the agent
	ctx := context.Background()
	human := store.EventInput{Type: store.EventStateChanged, Actor: "human"}
	err := db.Write(ctx, projectID, func(tx *store.Tx) error {
		return cycle.TrackCriticality(ctx, tx, "", func() error {
			dec, err := tx.CreateObject(store.ObjectDecision, "human", map[string]any{
				"statement": "Contar solo partículas de más de 1 µm.", "instead_of": "Contar todas.", "origin": "human"}, human)
			if err != nil {
				return err
			}
			sec, err := tx.GetObject(ctx, projectID, "sec-2")
			if err != nil {
				return err
			}
			_, err = tx.AddEdge(store.EdgeUsedIn, dec.ID, sec.ID, "", nil, store.EventInput{Type: store.EventEdge, Actor: "human"})
			return err
		})
	})
	if err != nil {
		t.Fatalf("add your decision: %v", err)
	}
	if n := len(eventsOf(t, db, projectID, store.EventDecisionSeen)); n != 0 {
		t.Fatalf("decision_seen events: %d, want none", n)
	}
	f := getFrame(t, h, projectID)
	yours, inFilter := frameDecision(t, f, "dec-2")
	if !yours.Seen || inFilter {
		t.Fatalf("dec-2 (yours): seen %v, in filter %v; want seen and out of the filter", yours.Seen, inFilter)
	}
	agents, inFilter := frameDecision(t, f, "dec-1")
	if agents.Seen || !inFilter {
		t.Fatalf("dec-1 (the agent's, never seen): seen %v, in filter %v; want its dot and in the filter", agents.Seen, inFilter)
	}
}
