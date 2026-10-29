package server

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/LaurenteEber/tupaqnin/internal/graph"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// BlockHistoryView is P5: the history of one block of a section.
type BlockHistoryView struct {
	Section string        `json:"section"`
	Block   int           `json:"block"`
	Entries []graph.Entry `json:"entries"`
}

// GET /api/projects/{projectID}/sections/{label}/history?block={n}.
// Opening it records decision_seen with history for each decision shown
// (M-1).
func (s *Server) getBlockHistory(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectID")
	if !s.requireProject(w, r, projectID) {
		return
	}
	label := r.PathValue("label")
	block, err := strconv.Atoi(r.URL.Query().Get("block"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_block")
		return
	}
	entries, err := graph.BlockHistory(r.Context(), s.db, projectID, label, block)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "block_not_found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}
	err = s.db.Write(r.Context(), projectID, func(tx *store.Tx) error {
		for _, e := range entries {
			if e.Kind != graph.EntryDecision {
				continue
			}
			dec, err := tx.GetObject(r.Context(), projectID, e.Decision.Label)
			if err != nil {
				return err
			}
			if _, err := tx.AppendEvent(store.EventInput{Type: store.EventDecisionSeen, Actor: "human",
				ObjectID: dec.ID, Data: map[string]any{"label": dec.Label, "seen_via": "history"}}); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}
	writeJSON(w, http.StatusOK, BlockHistoryView{Section: label, Block: block, Entries: entries})
}
