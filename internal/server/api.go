package server

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/LaurenteEber/tupaqnin/internal/cycle"
	"github.com/LaurenteEber/tupaqnin/internal/fingerprint"
	"github.com/LaurenteEber/tupaqnin/internal/graph"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// TurnStarter starts a turn in the background and returns its id at
// once (B-1). *cycle.Runner satisfies it.
type TurnStarter interface {
	StartTurn(projectID, text string) (string, error)
	StartUnderstandingTurn(projectID string) (string, error)
}

// API request and response shapes (backlog 0.10). TypeScript types
// are generated from these with tygo (web/src/api/types.ts); the API
// only transports what Go computes (rule 5 of AGENTS.md).
type CreateProjectResponse struct {
	ProjectID string `json:"project_id"`
}

type SectionResponse struct {
	Label   string `json:"label"`
	Version int    `json:"version"`
}

type SectionView struct {
	Label   string `json:"label"`
	Title   string `json:"title"`
	Focus   string `json:"focus"`
	Version int    `json:"version"`
	// State is the section's status from graph.SectionStatus, the one
	// function every view uses (ready included, computed).
	State    string `json:"state"`
	Pendings int    `json:"pendings"`
	Text     string `json:"text"`
}

type PutSectionRequest struct {
	TabID       string `json:"tab_id"`
	BaseVersion int    `json:"base_version"`
	Text        string `json:"text"`
}

type PostMessageRequest struct {
	Text string `json:"text"`
}

type PostMessageResponse struct {
	TurnID string `json:"turn_id"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, code string) {
	writeJSON(w, status, map[string]string{"error": code})
}

func (s *Server) requireProject(w http.ResponseWriter, r *http.Request, projectID string) bool {
	exists, err := s.db.ProjectExists(r.Context(), projectID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal")
		return false
	}
	if !exists {
		writeError(w, http.StatusNotFound, "project_not_found")
		return false
	}
	return true
}

// CreateSectionRequest is «+ Sección» (K-17): a title (required), the
// «aquí se concluye» mark (role "conclusion", or empty: the principal
// proposes it) and «qué debe abordar».
type CreateSectionRequest struct {
	TabID string `json:"tab_id"`
	Title string `json:"title"`
	Role  string `json:"role"`
	Focus string `json:"focus"`
}

// POST /api/projects/{projectID}/sections with {title, role, focus} →
// {label, version}: a new empty section in draft, last (K-17). It
// replaces the 0.10 route.
func (s *Server) createSection(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectID")
	if !s.requireProject(w, r, projectID) {
		return
	}
	var req CreateSectionRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<16)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body")
		return
	}
	if !s.editors.claim(projectID, req.TabID) {
		writeError(w, http.StatusConflict, "read_only_tab")
		return
	}
	obj, err := cycle.CreateSection(r.Context(), s.db, projectID, req.Title, req.Role, req.Focus)
	if err != nil {
		writeSectionError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, SectionResponse{Label: obj.Label, Version: obj.CurrentVersion})
}

// SectionDetailsRequest changes the title or «qué debe abordar» of a
// section (P-49); a missing field stays as it is.
type SectionDetailsRequest struct {
	TabID string  `json:"tab_id"`
	Title *string `json:"title"`
	Focus *string `json:"focus"`
}

// PUT /api/projects/{projectID}/sections/{label}/details with
// {title?, focus?} → 204. The state does not change (K-17).
func (s *Server) putSectionDetails(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectID")
	if !s.requireProject(w, r, projectID) {
		return
	}
	var req SectionDetailsRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<16)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body")
		return
	}
	if !s.editors.claim(projectID, req.TabID) {
		writeError(w, http.StatusConflict, "read_only_tab")
		return
	}
	if err := cycle.ChangeSectionDetails(r.Context(), s.db, projectID, r.PathValue("label"), req.Title, req.Focus); err != nil {
		writeSectionError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// SectionOrderRequest names every section once, in the new order.
type SectionOrderRequest struct {
	TabID  string   `json:"tab_id"`
	Labels []string `json:"labels"`
}

// PUT /api/projects/{projectID}/sections/order with {labels[]} → 204
// (P-49: «subir» and «bajar» in the frame skeleton).
func (s *Server) putSectionOrder(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectID")
	if !s.requireProject(w, r, projectID) {
		return
	}
	var req SectionOrderRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<16)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body")
		return
	}
	if !s.editors.claim(projectID, req.TabID) {
		writeError(w, http.StatusConflict, "read_only_tab")
		return
	}
	if err := cycle.ReorderSections(r.Context(), s.db, projectID, req.Labels); err != nil {
		writeSectionError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func writeSectionError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, cycle.ErrTitleRequired), errors.Is(err, cycle.ErrFocusTooLong),
		errors.Is(err, cycle.ErrInvalidRole), errors.Is(err, cycle.ErrInvalidOrder):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, cycle.ErrNotASection):
		writeError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, store.ErrVersionConflict):
		writeError(w, http.StatusConflict, "version_conflict")
	default:
		writeError(w, http.StatusInternalServerError, "internal")
	}
}

func sectionText(obj store.Object) string {
	var b map[string]any
	_ = json.Unmarshal(obj.Body, &b)
	text, _ := b["text"].(string)
	return text
}

// GET /api/projects/{projectID}/sections
func (s *Server) listSections(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectID")
	if !s.requireProject(w, r, projectID) {
		return
	}
	objects, err := graph.OrderedSections(r.Context(), s.db, projectID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}
	views := make([]SectionView, 0, len(objects))
	for _, obj := range objects {
		status, pendings, err := graph.SectionStatus(r.Context(), s.db, projectID, obj.Label)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal")
			return
		}
		views = append(views, SectionView{Label: obj.Label, Title: str(bodyMap(obj), "title"), Focus: str(bodyMap(obj), "focus"),
			Version: obj.CurrentVersion, State: string(status), Pendings: len(pendings), Text: sectionText(obj)})
	}
	writeJSON(w, http.StatusOK, views)
}

// PUT /api/projects/{projectID}/sections/{label} with
// {base_version, text} → 200 (new version), 204 (no change, B-3) or
// 409 (stale base version, specification §2).
func (s *Server) putSection(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectID")
	label := r.PathValue("label")
	if !s.requireProject(w, r, projectID) {
		return
	}
	var req PutSectionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body")
		return
	}
	current, err := s.db.GetObject(r.Context(), projectID, label)
	if err != nil || current.Type != store.ObjectSection {
		writeError(w, http.StatusNotFound, "section_not_found")
		return
	}
	// Only one tab edits (specification §2): a save from another tab is
	// refused.
	if !s.editors.claim(projectID, req.TabID) {
		writeError(w, http.StatusConflict, "read_only_tab")
		return
	}
	var body map[string]any
	_ = json.Unmarshal(current.Body, &body)
	if body == nil {
		body = make(map[string]any)
	}
	// The section fingerprint replaces B-3's comparison (specification
	// §4): the same content, whatever its formatting, is no change.
	if req.BaseVersion == current.CurrentVersion &&
		fingerprint.Section(req.Text) == fingerprint.Section(str(body, "text")) {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	body["text"] = req.Text
	// Your edit on a signed section moves it to changed_since_signoff
	// (estados §1: your edit rules).
	state := current.State
	if state == "signed" {
		state = "changed_since_signoff"
	}
	// Your edit of a section in signoff cancels the signoff in progress
	// (K-16, B-14: signoff_cancelled, edited), in the same transaction.
	signing, inSignoff, err := graph.ActiveSignoff(r.Context(), s.db, projectID, current.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}

	var updated store.Object
	err = s.db.Write(r.Context(), projectID, func(tx *store.Tx) error {
		var err error
		updated, err = tx.UpdateObject(current.ID, req.BaseVersion, state, body,
			store.EventInput{Type: store.EventEdit, Actor: "human"})
		if err != nil || !inSignoff {
			return err
		}
		return cycle.CancelSignoff(tx, signing, graph.CancelEdited, "human", map[string]any{"section": current.Label})
	})
	switch {
	case errors.Is(err, store.ErrVersionConflict):
		writeError(w, http.StatusConflict, "version_conflict")
	case errors.Is(err, store.ErrNoChange):
		w.WriteHeader(http.StatusNoContent)
	case err != nil:
		writeError(w, http.StatusInternalServerError, "internal")
	default:
		writeJSON(w, http.StatusOK, SectionResponse{Label: updated.Label, Version: updated.CurrentVersion})
	}
}

// POST /api/projects/{projectID}/messages with {text} → {turn_id}
// at once; the turn runs in the background (B-1).
func (s *Server) postMessage(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectID")
	if !s.requireProject(w, r, projectID) {
		return
	}
	var req PostMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body")
		return
	}
	if strings.TrimSpace(req.Text) == "" {
		writeError(w, http.StatusBadRequest, "empty_message")
		return
	}
	turnID, err := s.starter.StartTurn(projectID, req.Text)
	// [EL DISEÑO NO LO DICE] One turn per project: a message sent
	// while a turn runs is refused with 409 turn_running.
	if errors.Is(err, cycle.ErrTurnRunning) {
		writeError(w, http.StatusConflict, "turn_running")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}
	writeJSON(w, http.StatusAccepted, PostMessageResponse{TurnID: turnID})
}
