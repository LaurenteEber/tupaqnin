package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/LaurenteEber/tupaqnin/internal/cycle"
	"github.com/LaurenteEber/tupaqnin/internal/graph"
)

// ProposalStepper opens the new step a proposal asks for (B-75).
// *cycle.Runner satisfies it; without it, the notice waits for the next turn.
type ProposalStepper interface {
	StartStepAfterProposal(projectID string) (string, error)
}

// ProposalActRequest names the tab that acts: only one edits (specification §2).
type ProposalActRequest struct {
	TabID string `json:"tab_id"`
}

// ProposalActResponse is what accepting or discarding left: why is set when
// discarded, and turn_id when the app opened a turn for the new step (B-75).
type ProposalActResponse struct {
	Label   string `json:"label"`
	State   string `json:"state"`
	Why     string `json:"why,omitempty"`
	Section string `json:"section,omitempty"`
	TurnID  string `json:"turn_id,omitempty"`
}

// GET /api/projects/{projectID}/proposals?section={label}: every proposal,
// or those of a section (backlog-r4 4.1); a closed one stays in the list, for
// the history (B-76).
func (s *Server) listProposals(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectID")
	if !s.requireProject(w, r, projectID) {
		return
	}
	views, err := graph.Proposals(r.Context(), s.db, projectID, r.URL.Query().Get("section"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}
	writeJSON(w, http.StatusOK, views)
}

// POST /api/projects/{projectID}/proposals/{label}/accept
func (s *Server) acceptProposal(w http.ResponseWriter, r *http.Request) {
	s.actOnProposal(w, r, cycle.NewGate(s.db).AcceptProposal)
}

// POST /api/projects/{projectID}/proposals/{label}/discard
func (s *Server) discardProposal(w http.ResponseWriter, r *http.Request) {
	s.actOnProposal(w, r, cycle.NewGate(s.db).DiscardProposal)
}

func (s *Server) actOnProposal(w http.ResponseWriter, r *http.Request,
	act func(ctx context.Context, projectID, label string) (cycle.ProposalOutcome, error)) {
	projectID := r.PathValue("projectID")
	if !s.requireProject(w, r, projectID) {
		return
	}
	var req ProposalActRequest
	// The body is optional: without a tab the id is empty, which claim refuses
	// against a holder and takes the lease when free, as for any tab.
	_ = json.NewDecoder(io.LimitReader(r.Body, 1<<12)).Decode(&req)
	if !s.editors.claim(projectID, req.TabID) {
		writeError(w, http.StatusConflict, "read_only_tab")
		return
	}
	out, err := act(r.Context(), projectID, r.PathValue("label"))
	switch {
	case errors.Is(err, cycle.ErrProposalNotFound):
		writeError(w, http.StatusNotFound, "proposal_not_found")
		return
	case errors.Is(err, cycle.ErrProposalNotOpen):
		writeError(w, http.StatusConflict, "proposal_not_open")
		return
	case err != nil:
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}
	// Accepting on a section in signoff cancels it (K-16).
	if out.State == "accepted" {
		s.signoffClosed(r.Context(), projectID)
	}
	res := ProposalActResponse{Label: out.Label, State: out.State, Why: out.Why, Section: out.Section}
	if stepper, ok := s.starter.(ProposalStepper); ok && out.NewStep {
		turn, err := stepper.StartStepAfterProposal(projectID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal")
			return
		}
		res.TurnID = turn
	}
	writeJSON(w, http.StatusOK, res)
}
