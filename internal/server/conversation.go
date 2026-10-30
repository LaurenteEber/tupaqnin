package server

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/LaurenteEber/tupaqnin/internal/provider"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// ConversationEntry is one item of the conversation as the person reads
// it (U-5): a message of theirs, the end of an agent step with its
// triage line, reply (R-5), note and stop reason (B-24), the end of the
// understanding, or a card they answered. Never an internal id.
type ConversationEntry struct {
	// Kind is message · step · understanding · card_answered.
	Kind       string `json:"kind"`
	Text       string `json:"text,omitempty"`
	TriageLine string `json:"triage_line,omitempty"`
	Reply      string `json:"reply,omitempty"`
	Note       string `json:"note,omitempty"`
	// Status is accepted or agent_pending for a step or understanding.
	Status     string `json:"status,omitempty"`
	StopReason string `json:"stop_reason,omitempty"`
	CreatedAt  string `json:"created_at"`
}

// GET /api/projects/{projectID}/conversation: the whole conversation,
// oldest first, read from the history (U-5), so a reload or a return
// days later shows every message, reply, note and why the agent stopped.
func (s *Server) getConversation(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectID")
	if !s.requireProject(w, r, projectID) {
		return
	}
	events, err := s.db.EventsSince(r.Context(), projectID, 0)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}
	out := []ConversationEntry{}
	for _, ev := range events {
		var data map[string]any
		_ = json.Unmarshal(ev.Data, &data)
		entry := ConversationEntry{CreatedAt: ev.CreatedAt}
		switch {
		case ev.Type == store.EventHumanMessage:
			entry.Kind, entry.Text = "message", str(data, "text")
		case ev.Type == store.EventStepEnded && str(data, "turn_kind") == "understanding":
			entry.Kind, entry.Status, entry.Text = "understanding", "accepted", str(data, "understood")
		case ev.Type == store.EventStepEnded && str(data, "turn_kind") == "step":
			entry.Kind, entry.Status = "step", "accepted"
			entry.TriageLine, entry.Reply = str(data, "triage_line"), str(data, "reply")
			entry.Note, entry.StopReason = str(data, "note"), str(data, "stop_reason")
		case ev.Type == store.EventOutputRetry && str(data, "result") == "agent_pending":
			// [EL DISEÑO NO LO DICE]: a step left pending for the agent
			// writes no step_ended (B-24 names step_ended); its stop reason
			// is read from the output_retry that ended it.
			entry.Kind, entry.Status = str(data, "turn_kind"), "agent_pending"
			if entry.Kind == "step" {
				entry.StopReason = "agent_pending"
			}
			if entry.Kind != "step" && entry.Kind != "understanding" {
				continue
			}
		case ev.Type == store.EventActTimed && ev.Actor == "human":
			obj, err := s.db.GetObject(r.Context(), projectID, ev.ObjectID)
			if err != nil || obj.Type != store.ObjectCard {
				continue
			}
			entry.Kind = "card_answered"
			if dec, err := s.db.GetObject(r.Context(), projectID, str(bodyMap(obj), "decision_id")); err == nil {
				entry.Text = str(bodyMap(dec), "statement")
			}
		default:
			continue
		}
		out = append(out, entry)
	}
	writeJSON(w, http.StatusOK, out)
}

type RenameProjectRequest struct {
	Name string `json:"name"`
}

// PUT /api/projects/{projectID}/name: the person edits the title the
// understanding proposed (U-1).
func (s *Server) putProjectName(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectID")
	if !s.requireProject(w, r, projectID) {
		return
	}
	var req RenameProjectRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<10)).Decode(&req); err != nil ||
		strings.TrimSpace(req.Name) == "" {
		writeError(w, http.StatusBadRequest, "invalid_name")
		return
	}
	if err := s.db.RenameProject(r.Context(), projectID, strings.TrimSpace(req.Name)); err != nil {
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// modelInfo fills the model data of the summary (U-2): the provider,
// the principal's model and the effort and context of its last call.
func (s *Server) modelInfo(r *http.Request, projectID string, view *SummaryView) error {
	view.Provider = s.providerName
	view.Model = provider.PrincipalModel
	view.Effort = provider.PrincipalEffort
	view.ContextWindow = provider.ContextWindow(provider.PrincipalModel)
	last, ok, err := s.db.LastModelCall(r.Context(), projectID, string(provider.RolePrincipal))
	if err != nil || !ok {
		return err
	}
	if model, effort, found := strings.Cut(last.ModelAndEffort, " "); found {
		view.Model, view.Effort = model, effort
	}
	view.ContextUsedTokens = last.InputTokens
	if last.ContextWindow > 0 {
		view.ContextWindow = last.ContextWindow
	}
	return nil
}
