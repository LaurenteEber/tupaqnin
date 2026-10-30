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

// Shapes for the right half of P2 (backlog 1.2). Cases and roles are
// proposed by the principal and confirmed or corrected by the human;
// nothing here blocks the work (fichas §2).
type CaseView struct {
	Label          string `json:"label"`
	Text           string `json:"text"`
	Counts         *bool  `json:"counts,omitempty"`
	CriterionLabel string `json:"criterion_label,omitempty"`
	State          string `json:"state"`
}

type UnderstandingView struct {
	// Status is how the last understanding turn ended (P-43): "none"
	// (none ended yet), "accepted", or "agent_pending" (it failed twice
	// and stays pending for the agent), read from the events so a
	// reload still shows it.
	Status     string            `json:"status"`
	Understood string            `json:"understood"`
	Cases      []CaseView        `json:"cases"`
	Sections   []SectionRoleView `json:"sections"`
}

type SectionRoleView struct {
	Label     string `json:"label"`
	Title     string `json:"title"`
	Role      string `json:"role"`
	RoleState string `json:"role_state"`
}

type CorrectCaseRequest struct {
	Text string `json:"text"`
}

type PutRoleRequest struct {
	Role string `json:"role"`
}

// GET /api/projects/{projectID}/understanding
func (s *Server) getUnderstanding(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectID")
	if !s.requireProject(w, r, projectID) {
		return
	}
	view := UnderstandingView{Status: "none", Cases: []CaseView{}, Sections: []SectionRoleView{}}

	cases, _ := s.db.ListObjects(r.Context(), projectID, store.ObjectUnderstandingCase)
	for _, c := range cases {
		b := bodyMap(c)
		cv := CaseView{Label: c.Label, Text: str(b, "text"), State: c.State}
		if v, ok := b["counts"].(bool); ok {
			cv.Counts = &v
		}
		if id := str(b, "criterion_id"); id != "" {
			if crit, err := s.db.GetObject(r.Context(), projectID, id); err == nil {
				cv.CriterionLabel = crit.Label
			}
		}
		view.Cases = append(view.Cases, cv)
	}

	sections, _ := graph.OrderedSections(r.Context(), s.db, projectID)
	for _, sec := range sections {
		b := bodyMap(sec)
		role, _ := b["role"].(map[string]any)
		view.Sections = append(view.Sections, SectionRoleView{
			Label: sec.Label, Title: str(b, "title"),
			Role: str(role, "value"), RoleState: str(role, "state"),
		})
	}

	events, _ := s.db.EventsSince(r.Context(), projectID, 0)
	for _, ev := range events {
		if ev.Type != store.EventStepEnded && ev.Type != store.EventOutputRetry {
			continue
		}
		var data map[string]any
		_ = json.Unmarshal(ev.Data, &data)
		if str(data, "turn_kind") != "understanding" {
			continue
		}
		switch {
		case ev.Type == store.EventStepEnded:
			view.Status = "accepted"
			view.Understood = str(data, "understood")
		case str(data, "result") == "agent_pending":
			view.Status = "agent_pending"
		}
	}
	writeJSON(w, http.StatusOK, view)
}

func (s *Server) updateCase(w http.ResponseWriter, r *http.Request, state, text string, keepText bool) {
	projectID := r.PathValue("projectID")
	label := r.PathValue("label")
	if !s.requireProject(w, r, projectID) {
		return
	}
	obj, err := s.db.GetObject(r.Context(), projectID, label)
	if err != nil || obj.Type != store.ObjectUnderstandingCase {
		writeError(w, http.StatusNotFound, "case_not_found")
		return
	}
	body := bodyMap(obj)
	if !keepText {
		body["text"] = text
	}
	var updated store.Object
	err = s.db.Write(r.Context(), projectID, func(tx *store.Tx) error {
		var err error
		updated, err = tx.UpdateObject(obj.ID, obj.CurrentVersion, state, body,
			store.EventInput{Type: store.EventStateChanged, Actor: "human", ObjectID: obj.ID})
		return err
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}
	b := bodyMap(updated)
	cv := CaseView{Label: updated.Label, Text: str(b, "text"), State: updated.State}
	if v, ok := b["counts"].(bool); ok {
		cv.Counts = &v
	}
	writeJSON(w, http.StatusOK, cv)
}

// POST /api/projects/{projectID}/cases/{label}/confirm
func (s *Server) confirmCase(w http.ResponseWriter, r *http.Request) {
	s.updateCase(w, r, "confirmed", "", true)
}

// POST /api/projects/{projectID}/cases/{label}/correct with {text}:
// the human's text is kept literal.
func (s *Server) correctCase(w http.ResponseWriter, r *http.Request) {
	var req CorrectCaseRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil || req.Text == "" {
		writeError(w, http.StatusBadRequest, "invalid_body")
		return
	}
	s.updateCase(w, r, "corrected", req.Text, false)
}

// PUT /api/projects/{projectID}/sections/{label}/role with {role}:
// confirming or changing the role leaves it confirmed, with its
// role_changed event (estados §6).
func (s *Server) putSectionRole(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectID")
	label := r.PathValue("label")
	if !s.requireProject(w, r, projectID) {
		return
	}
	var req PutRoleRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body")
		return
	}
	switch req.Role {
	case "conclusion", "recommendation", "other":
	default:
		writeError(w, http.StatusBadRequest, "invalid_role")
		return
	}
	obj, err := s.db.GetObject(r.Context(), projectID, label)
	if err != nil || obj.Type != store.ObjectSection {
		writeError(w, http.StatusNotFound, "section_not_found")
		return
	}
	body := bodyMap(obj)
	oldRole, _ := body["role"].(map[string]any)
	body["role"] = map[string]any{"value": req.Role, "state": "confirmed"}
	var updated store.Object
	err = s.db.Write(r.Context(), projectID, func(tx *store.Tx) error {
		// Changing the role can make a decision critical (T-5, K-13).
		return cycle.TrackCriticality(r.Context(), tx, "", func() error {
			var err error
			updated, err = tx.UpdateObject(obj.ID, obj.CurrentVersion, obj.State, body,
				store.EventInput{Type: store.EventRoleChanged, Actor: "human", ObjectID: obj.ID,
					Data: map[string]any{"label": label, "from": str(oldRole, "value"), "to": req.Role, "state": "confirmed"}})
			if errors.Is(err, store.ErrNoChange) {
				err = nil
				updated = obj
			}
			return err
		})
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}
	b := bodyMap(updated)
	role, _ := b["role"].(map[string]any)
	writeJSON(w, http.StatusOK, SectionRoleView{Label: updated.Label, Title: str(b, "title"),
		Role: str(role, "value"), RoleState: str(role, "state")})
}
