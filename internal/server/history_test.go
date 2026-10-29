package server_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/LaurenteEber/tupaqnin/internal/server"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// P5, M-1: opening a block's history records decision_seen (history)
// for each decision it shows.
func TestP5_OpeningTheHistoryRecordsDecisionSeen(t *testing.T) {
	h, db, _ := newAPI(t)
	projectID := newProject(t, h, nil)
	seedUnderstanding(t, db, projectID)
	ctx := context.Background()
	err := db.Write(ctx, projectID, func(tx *store.Tx) error {
		sec, _ := tx.GetObject(ctx, projectID, "sec-1")
		var b map[string]any
		_ = json.Unmarshal(sec.Body, &b)
		b["text"] = "La comparación da 1,5 veces."
		if _, err := tx.UpdateObject(sec.ID, sec.CurrentVersion, sec.State, b, store.EventInput{Type: store.EventEdit, Actor: "human"}); err != nil {
			return err
		}
		dec, err := tx.CreateObject(store.ObjectDecision, "agent", map[string]any{"statement": "Comparar por veces.", "origin": "agent"},
			store.EventInput{Type: store.EventStateChanged, Actor: "agent"})
		if err != nil {
			return err
		}
		fig, err := tx.CreateObject(store.ObjectFigure, "", map[string]any{"section_id": sec.ID, "anchor": "1,5 veces", "value": "1,5"},
			store.EventInput{Type: store.EventStateChanged, Actor: "agent"})
		if err != nil {
			return err
		}
		_, err = tx.AddEdge(store.EdgeCovers, fig.ID, dec.ID, "1,5 veces", store.EventInput{Type: store.EventEdge, Actor: "agent"})
		return err
	})
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	rec := do(t, h, http.MethodGet, "/api/projects/"+projectID+"/sections/sec-1/history?block=1", nil)
	view := decode[server.BlockHistoryView](t, rec)
	if rec.Code != http.StatusOK || len(view.Entries) == 0 {
		t.Fatalf("history: %d %+v", rec.Code, view)
	}
	events, _ := db.EventsSince(ctx, projectID, 0)
	seen := 0
	for _, ev := range events {
		var data map[string]any
		_ = json.Unmarshal(ev.Data, &data)
		if ev.Type == store.EventDecisionSeen && data["seen_via"] == "history" && data["label"] == "dec-1" {
			seen++
		}
	}
	if seen != 1 {
		t.Fatalf("decision_seen history for dec-1: %d, want 1", seen)
	}
	if rec := do(t, h, http.MethodGet, "/api/projects/"+projectID+"/sections/sec-1/history?block=7", nil); rec.Code != http.StatusNotFound {
		t.Fatalf("block 7: %d, want 404", rec.Code)
	}
}
