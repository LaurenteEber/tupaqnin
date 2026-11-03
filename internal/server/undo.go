package server

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/LaurenteEber/tupaqnin/internal/cycle"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// UndoRequest names the tab that acts: only one edits (specification §2).
// EditVersion, when set, is the edit you mean, from a card's «Fue un error» or
// an alert's «Deshacer»: under a later one it is refused (B-79).
type UndoRequest struct {
	TabID       string `json:"tab_id"`
	EditVersion int    `json:"edit_version,omitempty"`
}

// UndoResponse is what the undo left: the new version, the version whose text
// it restored, and the section's state.
type UndoResponse struct {
	Label           string `json:"label"`
	Version         int    `json:"version"`
	Status          string `json:"status"`
	RestoredVersion int    `json:"restored_version"`
}

// UndoRefusal is «Deshacer» off, with its reason (B-79).
type UndoRefusal struct {
	Error  string `json:"error"`
	Reason string `json:"reason"`
}

// POST /api/projects/{projectID}/sections/{label}/undo (backlog-r4 4.2).
func (s *Server) undoSection(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectID")
	if !s.requireProject(w, r, projectID) {
		return
	}
	var req UndoRequest
	// The body is optional, as for a proposal's acts (actOnProposal).
	_ = json.NewDecoder(io.LimitReader(r.Body, 1<<12)).Decode(&req)
	if !s.editors.claim(projectID, req.TabID) {
		writeError(w, http.StatusConflict, "read_only_tab")
		return
	}
	out, err := s.signoffGate().UndoEdit(r.Context(), projectID, r.PathValue("label"), req.EditVersion)
	var refused cycle.UndoRefused
	switch {
	case errors.As(err, &refused):
		writeJSON(w, http.StatusConflict, UndoRefusal{Error: "undo_unavailable", Reason: refused.Reason})
		return
	case errors.Is(err, cycle.ErrNotASection):
		writeError(w, http.StatusNotFound, "section_not_found")
		return
	case errors.Is(err, store.ErrVersionConflict):
		writeError(w, http.StatusConflict, "version_conflict")
		return
	case err != nil:
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}
	// A restored version enters the batch for the checker, like a save (B-36).
	if n, ok := s.starter.(EditNotifier); ok {
		n.EditSaved(projectID)
	}
	if out.SignoffCancelled {
		s.signoffClosed(r.Context(), projectID)
	}
	writeJSON(w, http.StatusOK, UndoResponse{Label: out.Label, Version: out.Version, Status: out.Status, RestoredVersion: out.RestoredVersion})
}

// POST /api/projects/{projectID}/alerts/{n}/answer: «Fue a propósito». n is the
// seq of the alert's event; anything else is not found, and writes nothing.
func (s *Server) answerAlert(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectID")
	if !s.requireProject(w, r, projectID) {
		return
	}
	var req UndoRequest
	_ = json.NewDecoder(io.LimitReader(r.Body, 1<<12)).Decode(&req)
	if !s.editors.claim(projectID, req.TabID) {
		writeError(w, http.StatusConflict, "read_only_tab")
		return
	}
	seq, err := strconv.Atoi(r.PathValue("n"))
	if err != nil {
		writeError(w, http.StatusNotFound, "alert_not_found")
		return
	}
	switch err := cycle.NewGate(s.db).AnswerAlert(r.Context(), projectID, seq); {
	case errors.Is(err, cycle.ErrAlertNotFound):
		writeError(w, http.StatusNotFound, "alert_not_found")
	case errors.Is(err, cycle.ErrAlertAnswered):
		writeError(w, http.StatusConflict, "alert_answered")
	case err != nil:
		writeError(w, http.StatusInternalServerError, "internal")
	default:
		w.WriteHeader(http.StatusOK)
	}
}

// editGroupWindow is the gap under which two saves are one edition (C.20,
// [inicial]). It runs from the previous save, so a chain of close saves is one.
const editGroupWindow = 5 * time.Second

// lastSaves is each project's last save of yours, by the server's clock: what
// tells a save that joins the edition before it (C.20). [EL DISEÑO NO LO DICE]:
// in memory, like the editor lease: the event's created_at has no clock a test can
// move, and a normal edit's event carries no data. A restart inside the window
// only makes the next save an edition of its own.
type lastSaves struct {
	mu   sync.Mutex
	last map[string]lastSave
}

type lastSave struct {
	sectionID string
	at        time.Time
}

func newLastSaves() *lastSaves { return &lastSaves{last: map[string]lastSave{}} }

// joins says the save comes under the window after the project's last save, and
// that save was of this section: another section's save ended the edition.
func (l *lastSaves) joins(projectID, sectionID string, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	prev, ok := l.last[projectID]
	return ok && prev.sectionID == sectionID && now.Sub(prev.at) < editGroupWindow
}

func (l *lastSaves) record(projectID, sectionID string, now time.Time) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.last[projectID] = lastSave{sectionID: sectionID, at: now}
}
