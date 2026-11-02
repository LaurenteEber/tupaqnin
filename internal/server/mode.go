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

// ModeSetter sets the mode (backlog-r4 4.4, B-83). *cycle.Runner satisfies it;
// the route exists only when the starter does.
type ModeSetter interface {
	SetMode(ctx context.Context, projectID, mode, message string) (string, error)
}

// ChainFollower says which turn of the mode's chain runs now, for a page that did
// not start it: after a reload, or a chain the app opened.
type ChainFollower interface {
	RunningTurn(projectID string) string
}

// SignoffCloser is told every time a signoff closes (B-86): if the mode goes on
// and the chain waited for it, the chain reopens.
type SignoffCloser interface {
	SignoffClosed(ctx context.Context, projectID string)
}

// ModeRequest is PUT /mode: the mode and, when activating, your message.
type ModeRequest struct {
	Mode    string `json:"mode"`
	Message string `json:"message"`
}

// ModeResponse is the mode now and the turn the activation started, if it did.
type ModeResponse struct {
	Mode   string `json:"mode"`
	TurnID string `json:"turn_id,omitempty"`
}

// SignoffQueuedResponse answers asking to sign during the mode (B-83, 202): it
// opens when the running step ends; asking again reads what happened.
// [EL DISEÑO NO LO DICE]: how you learn of it: the page asks again.
type SignoffQueuedResponse struct {
	Queued  bool   `json:"queued"`
	Section string `json:"section"`
}

// PUT /api/projects/{projectID}/mode {mode, message} → 200 {mode, turn_id} (B-83):
// 400 invalid_body or invalid_mode. The mode and your message go in literal to
// mode_set; activating with the agent stopped starts a turn.
func (s *Server) putMode(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectID")
	if !s.requireProject(w, r, projectID) {
		return
	}
	var req ModeRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, maxSignoffBody)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body")
		return
	}
	turnID, err := s.modes.SetMode(r.Context(), projectID, req.Mode, req.Message)
	switch {
	case errors.Is(err, cycle.ErrInvalidMode):
		writeError(w, http.StatusBadRequest, "invalid_mode")
		return
	case err != nil:
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}
	mode, err := graph.CurrentMode(r.Context(), s.db, projectID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}
	writeJSON(w, http.StatusOK, ModeResponse{Mode: mode.Mode, TurnID: turnID})
}

// signoffClosed tells the runner a signoff closed, from any handler.
func (s *Server) signoffClosed(ctx context.Context, projectID string) {
	if c, ok := s.starter.(SignoffCloser); ok {
		c.SignoffClosed(ctx, projectID)
	}
}

// writeTurnStarted answers a message: its turn, or «queued» with none when the
// mode's running chain takes it (B-83).
func writeTurnStarted(w http.ResponseWriter, turnID string) {
	writeJSON(w, http.StatusAccepted, PostMessageResponse{TurnID: turnID, Queued: turnID == ""})
}
