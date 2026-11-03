package server_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/LaurenteEber/tupaqnin/internal/checks"
	"github.com/LaurenteEber/tupaqnin/internal/graph"
	"github.com/LaurenteEber/tupaqnin/internal/server"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// seedClash leaves a «por aclarar» of the given origin on «Diagnóstico» and a
// decision used in «Recomendación». It returns the used_in edge.
func seedClash(t *testing.T, db *store.DB, projectID, origin string) string {
	t.Helper()
	var usedIn string
	err := db.Write(context.Background(), projectID, func(tx *store.Tx) error {
		agent := store.EventInput{Type: store.EventStateChanged, Actor: "agent"}
		diagnosis, err := tx.CreateObject(store.ObjectSection, "draft", map[string]any{"title": "Diagnóstico", "text": "Renovar primero."}, agent)
		if err != nil {
			return err
		}
		reco, err := tx.CreateObject(store.ObjectSection, "draft", map[string]any{"title": "Recomendación", "text": "Sectorizar."}, agent)
		if err != nil {
			return err
		}
		dec, err := tx.CreateObject(store.ObjectDecision, "agent", map[string]any{"statement": "Sectorizar antes de renovar.", "origin": "agent"}, agent)
		if err != nil {
			return err
		}
		edge, err := tx.AddEdge(store.EdgeUsedIn, dec.ID, reco.ID, "", nil, store.EventInput{Type: store.EventEdge, Actor: "agent"})
		if err != nil {
			return err
		}
		usedIn = edge.ID
		_, err = checks.Open(context.Background(), tx, checks.OpenQuestion{
			Question: "¿Renovar o sectorizar primero?", DependsOn: []string{diagnosis.ID, dec.ID},
			Options: []map[string]any{{"option": "Renovar", "contrast": "c", "consequence": "c", "risk": "r"}, {"option": "Sectorizar", "contrast": "c", "consequence": "c", "risk": "r"}},
			Extra:   map[string]any{"origin": origin, "kind": graph.CandidateContradiction},
			Event:   agent,
		})
		return err
	})
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	return usedIn
}

func todoSections(t *testing.T, h http.Handler, projectID string) []string {
	t.Helper()
	todo := decode[server.TodoView](t, do(t, h, http.MethodGet, "/api/projects/"+projectID+"/todo", nil))
	if len(todo.OpenQuestions) != 1 {
		t.Fatalf("todo: %+v, want one open question", todo)
	}
	titles := []string{}
	for _, s := range todo.OpenQuestions[0].Sections {
		titles = append(titles, s.Title)
	}
	return titles
}

func TestB108_TheCardNamesBothSections(t *testing.T) {
	h, db, _ := newAPI(t)
	projectID := newProject(t, h, nil)
	usedIn := seedClash(t, db, projectID, checks.OriginChecker)
	got := todoSections(t, h, projectID)
	if len(got) != 2 || got[0] != "Diagnóstico" || got[1] != "Recomendación" {
		t.Fatalf("sections: %v", got)
	}
	// The name is read live: a section that stopped using the decision is free
	// and no longer named.
	err := db.Write(context.Background(), projectID, func(tx *store.Tx) error {
		return tx.EndEdge(usedIn, store.EventInput{Type: store.EventEdge, Actor: "agent"})
	})
	if err != nil {
		t.Fatalf("EndEdge: %v", err)
	}
	if got := todoSections(t, h, projectID); len(got) != 1 || got[0] != "Diagnóstico" {
		t.Fatalf("sections after the edge ended: %v", got)
	}
}

// Only the checker's clash names them: another «por aclarar» reads as it did.
func TestB108_AnotherQuestionKeepsItsSections(t *testing.T) {
	h, db, _ := newAPI(t)
	projectID := newProject(t, h, nil)
	seedClash(t, db, projectID, checks.OriginObjection)
	if got := todoSections(t, h, projectID); len(got) != 1 || got[0] != "Diagnóstico" {
		t.Fatalf("sections: %v", got)
	}
}
