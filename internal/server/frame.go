package server

import (
	"errors"
	"net/http"

	"github.com/LaurenteEber/tupaqnin/internal/graph"
	"github.com/LaurenteEber/tupaqnin/internal/memory"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// GET /api/projects/{projectID}/frame: the frame (M-1, specification
// §9). Read only: it writes nothing and no model writes it.
func (s *Server) getFrame(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectID")
	if !s.requireProject(w, r, projectID) {
		return
	}
	// The frame reuses the function that computes the working state's
	// layers 1 and 2 (§9): a change there changes the frame.
	st, err := memory.New(s.db).WorkingState(r.Context(), projectID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}
	frame, err := graph.ComputeFrame(r.Context(), s.db, projectID, frameLayers(st))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}
	writeJSON(w, http.StatusOK, frame)
}

func frameLayers(st memory.State) graph.FrameLayers {
	e := st.Essentials
	layers := graph.FrameLayers{
		Purpose: graph.FramePurpose{RequestText: e.RequestText, Audience: e.Audience,
			PremiseText: e.PremiseText, PremiseExploratory: e.PremiseExploratory},
		Criteria:         []graph.FrameCriterion{},
		UnconfirmedCases: e.UnconfirmedCases,
		StopReportUnseen: e.StopReportUnseen,
	}
	for _, c := range e.Criteria {
		fc := graph.FrameCriterion{Label: c.Label, Text: c.Text, Cases: []graph.FrameCase{}}
		for _, cs := range c.Cases {
			fc.Cases = append(fc.Cases, graph.FrameCase{Label: cs.Label, Text: cs.Text, Counts: cs.Counts, State: cs.State})
		}
		layers.Criteria = append(layers.Criteria, fc)
	}
	return layers
}

// POST /api/projects/{projectID}/frame/decisions/{label}/seen: you
// expanded a decision in the frame; it records decision_seen with
// seen_via frame (§9). Scrolling the frame records nothing.
// [EL DISEÑO NO LO DICE]: the route and its 204; the design names the
// event, not the call.
func (s *Server) postFrameDecisionSeen(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectID")
	if !s.requireProject(w, r, projectID) {
		return
	}
	label := r.PathValue("label")
	err := s.db.Write(r.Context(), projectID, func(tx *store.Tx) error {
		dec, err := tx.GetObject(r.Context(), projectID, label)
		if err != nil {
			return err
		}
		if dec.Type != store.ObjectDecision {
			return store.ErrNotFound
		}
		_, err = tx.AppendEvent(store.EventInput{Type: store.EventDecisionSeen, Actor: "human",
			ObjectID: dec.ID, Data: map[string]any{"label": dec.Label, "seen_via": "frame"}})
		return err
	})
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "decision_not_found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
