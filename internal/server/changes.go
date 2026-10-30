package server

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/LaurenteEber/tupaqnin/internal/cycle"
	"github.com/LaurenteEber/tupaqnin/internal/graph"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// ChangesView is a section's list of open changes (A-1, backlog-r2
// 2.2): each with its label, the kind of what changed and its label.
type ChangesView struct {
	Section string             `json:"section"`
	State   string             `json:"state"`
	Changes []graph.OpenChange `json:"changes"`
}

// CloseChangeRequest closes a change: updated (after editing that
// section) or reviewed_unchanged with a «porque» (fichas §3).
type CloseChangeRequest struct {
	Result string `json:"result"`
	Reason string `json:"reason"`
}

// GET /api/projects/{projectID}/sections/{label}/changes
func (s *Server) getSectionChanges(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectID")
	if !s.requireProject(w, r, projectID) {
		return
	}
	sec, err := s.db.GetObject(r.Context(), projectID, r.PathValue("label"))
	if err != nil || sec.Type != store.ObjectSection {
		writeError(w, http.StatusNotFound, "section_not_found")
		return
	}
	status, _, err := graph.SectionStatus(r.Context(), s.db, projectID, sec.Label)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}
	writeJSON(w, http.StatusOK, ChangesView{Section: sec.Label, State: string(status), Changes: graph.OpenChanges(sec)})
}

// POST /api/projects/{projectID}/changes/{label}/close with {result,
// reason} → 204; usage_change in the history.
// [EL DISEÑO NO LO DICE]: the status codes; 404 for a label in no
// section's list, 400 for another result, 422 for a reason that does
// not count (as the card), 409 for updated without an edit of yours.
func (s *Server) closeChange(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectID")
	if !s.requireProject(w, r, projectID) {
		return
	}
	var req CloseChangeRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body")
		return
	}
	err := cycle.NewGate(s.db).CloseChange(r.Context(), projectID, r.PathValue("label"), req.Result, req.Reason)
	switch {
	case errors.Is(err, cycle.ErrChangeNotFound):
		writeError(w, http.StatusNotFound, "change_not_found")
	case errors.Is(err, cycle.ErrInvalidResult):
		writeError(w, http.StatusBadRequest, "invalid_result")
	case errors.Is(err, cycle.ErrInvalidReason):
		writeError(w, http.StatusUnprocessableEntity, "invalid_reason")
	case errors.Is(err, cycle.ErrSectionNotEdited):
		writeError(w, http.StatusConflict, "section_not_edited")
	case err != nil:
		writeError(w, http.StatusInternalServerError, "internal")
	default:
		w.WriteHeader(http.StatusNoContent)
	}
}
