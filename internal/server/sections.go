package server

import (
	"encoding/json"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/LaurenteEber/tupaqnin/internal/graph"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// Only one tab edits (specification §2, backlog-r1 1.5).
// [EL DISEÑO NO LO DICE]: how the editing tab is recognized; the
// smallest choice is an editor lease per project, in memory: the first
// tab that claims it (or saves) holds it while it renews it, and it
// lapses after editorLease without renewal. Another tab opens read-only
// and its saves are refused.
const editorLease = 45 * time.Second // [inicial]

type editors struct {
	mu      sync.Mutex
	holders map[string]editorHolder
	now     func() time.Time
}

type editorHolder struct {
	tab  string
	seen time.Time
}

func newEditors() *editors {
	return &editors{holders: map[string]editorHolder{}, now: time.Now}
}

// claim gives the lease to tab when nobody holds it, it lapsed or tab
// holds it already, and reports whether tab is the editor.
func (e *editors) claim(projectID, tab string) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	now := e.now()
	h, ok := e.holders[projectID]
	if ok && h.tab != tab && now.Sub(h.seen) < editorLease {
		return false
	}
	e.holders[projectID] = editorHolder{tab: tab, seen: now}
	return true
}

type EditorRequest struct {
	TabID string `json:"tab_id"`
}

type EditorResponse struct {
	Editor bool `json:"editor"`
}

// POST /api/projects/{projectID}/editor with {tab_id}: claims or renews
// the editor lease; a second tab gets editor false and stays read-only.
func (s *Server) claimEditor(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectID")
	if !s.requireProject(w, r, projectID) {
		return
	}
	var req EditorRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<16)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body")
		return
	}
	writeJSON(w, http.StatusOK, EditorResponse{Editor: s.editors.claim(projectID, req.TabID)})
}

// SectionDetail is P3's editor: the text, the state and pendings from
// the one status function, and the linked figures and citations.
type SectionDetail struct {
	Label     string          `json:"label"`
	Title     string          `json:"title"`
	Focus     string          `json:"focus"`
	Version   int             `json:"version"`
	Status    string          `json:"status"`
	Text      string          `json:"text"`
	Role      string          `json:"role"`
	RoleState string          `json:"role_state"`
	Pendings  []graph.Pending `json:"pendings"`
	Figures   []LinkedAnchor  `json:"figures"`
	Citations []LinkedAnchor  `json:"citations"`
}

// LinkedAnchor is a figure (to its decision) or a citation (to its
// source); Linked is false for a citation its source does not support.
type LinkedAnchor struct {
	Label  string `json:"label"`
	Anchor string `json:"anchor"`
	Target string `json:"target"`
	Linked bool   `json:"linked"`
}

// GET /api/projects/{projectID}/sections/{label}
func (s *Server) getSection(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectID")
	if !s.requireProject(w, r, projectID) {
		return
	}
	ctx := r.Context()
	sec, err := s.db.GetObject(ctx, projectID, r.PathValue("label"))
	if err != nil || sec.Type != store.ObjectSection {
		writeError(w, http.StatusNotFound, "section_not_found")
		return
	}
	status, pendings, err := graph.SectionStatus(ctx, s.db, projectID, sec.Label)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}
	b := bodyMap(sec)
	role, _ := b["role"].(map[string]any)
	detail := SectionDetail{Label: sec.Label, Title: str(b, "title"), Focus: str(b, "focus"), Version: sec.CurrentVersion,
		Status: string(status), Text: str(b, "text"), Role: str(role, "value"), RoleState: str(role, "state"),
		Pendings: pendings, Figures: []LinkedAnchor{}, Citations: []LinkedAnchor{}}
	for _, kind := range []struct {
		t    store.ObjectType
		edge store.EdgeType
		out  *[]LinkedAnchor
	}{{store.ObjectFigure, store.EdgeCovers, &detail.Figures}, {store.ObjectCitation, store.EdgeComesFrom, &detail.Citations}} {
		objs, err := s.db.ListObjects(ctx, projectID, kind.t)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal")
			return
		}
		for _, obj := range objs {
			ob := bodyMap(obj)
			if str(ob, "section_id") != sec.ID {
				continue
			}
			a := LinkedAnchor{Label: obj.Label, Anchor: str(ob, "anchor")}
			edges, _ := s.db.EdgesFrom(ctx, projectID, obj.ID)
			for _, e := range edges {
				if e.Type == kind.edge {
					if target, err := s.db.GetObject(ctx, projectID, e.ToID); err == nil {
						a.Target, a.Linked = target.Label, true
					}
				}
			}
			*kind.out = append(*kind.out, a)
		}
	}
	writeJSON(w, http.StatusOK, detail)
}

// SummaryView is P3's bar: what was spent against the project's cap
// (B3-20) and the last triage line.
type SummaryView struct {
	SpentUSD   float64 `json:"spent_usd"`
	CapUSD     float64 `json:"cap_usd"`
	LastTriage string  `json:"last_triage"`
	// Model data in view (U-2): provider (meta · fake), the
	// principal's model and effort, and the context its last call used.
	Provider          string `json:"provider"`
	Model             string `json:"model"`
	Effort            string `json:"effort"`
	ContextUsedTokens int    `json:"context_used_tokens"`
	ContextWindow     int    `json:"context_window"`
}

// GET /api/projects/{projectID}/summary
func (s *Server) getSummary(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectID")
	if !s.requireProject(w, r, projectID) {
		return
	}
	spent, err := s.db.ProjectSpend(r.Context(), projectID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}
	events, err := s.db.EventsSince(r.Context(), projectID, 0)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}
	view := SummaryView{SpentUSD: spent, CapUSD: s.projectCap}
	if err := s.modelInfo(r, projectID, &view); err != nil {
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}
	for _, ev := range events {
		if ev.Type != store.EventStepEnded {
			continue
		}
		var data map[string]any
		_ = json.Unmarshal(ev.Data, &data)
		if t, _ := data["triage_line"].(string); t != "" {
			view.LastTriage = t
		}
	}
	writeJSON(w, http.StatusOK, view)
}
