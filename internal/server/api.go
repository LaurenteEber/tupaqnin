package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// TurnStarter starts a turn in the background and returns its id at
// once (B-1). *cycle.Runner satisfies it.
type TurnStarter interface {
	StartTurn(projectID, text string) (string, error)
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
	Version int    `json:"version"`
	State   string `json:"state"`
	Text    string `json:"text"`
}

type PutSectionRequest struct {
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

// POST /api/projects → {project_id}
func (s *Server) createProject(w http.ResponseWriter, r *http.Request) {
	id, err := s.db.CreateProject(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}
	writeJSON(w, http.StatusCreated, CreateProjectResponse{ProjectID: id})
}

// POST /api/projects/{projectID}/sections → {label, version}:
// a new empty section in draft.
func (s *Server) createSection(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectID")
	if !s.requireProject(w, r, projectID) {
		return
	}
	var obj store.Object
	err := s.db.Write(r.Context(), projectID, func(tx *store.Tx) error {
		var err error
		obj, err = tx.CreateObject(store.ObjectSection, "draft",
			map[string]any{
				"text":         "",
				"role":         map[string]any{"value": "other", "state": "proposed"},
				"open_changes": []any{},
			},
			store.EventInput{Type: store.EventStateChanged, Actor: "human"})
		return err
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}
	writeJSON(w, http.StatusCreated, SectionResponse{Label: obj.Label, Version: obj.CurrentVersion})
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
	objects, err := s.db.ListObjects(r.Context(), projectID, store.ObjectSection)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}
	views := make([]SectionView, 0, len(objects))
	for _, obj := range objects {
		views = append(views, SectionView{Label: obj.Label, Version: obj.CurrentVersion,
			State: obj.State, Text: sectionText(obj)})
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
	var body map[string]any
	_ = json.Unmarshal(current.Body, &body)
	if body == nil {
		body = make(map[string]any)
	}
	body["text"] = req.Text

	var updated store.Object
	err = s.db.Write(r.Context(), projectID, func(tx *store.Tx) error {
		var err error
		updated, err = tx.UpdateObject(current.ID, req.BaseVersion, current.State, body,
			store.EventInput{Type: store.EventEdit, Actor: "human"})
		return err
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
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}
	writeJSON(w, http.StatusAccepted, PostMessageResponse{TurnID: turnID})
}
