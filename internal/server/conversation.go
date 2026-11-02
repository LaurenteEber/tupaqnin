package server

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/LaurenteEber/tupaqnin/internal/graph"
	"github.com/LaurenteEber/tupaqnin/internal/provider"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// ConversationEntry is one item of the conversation as the person reads
// it (U-5): a message of theirs, the end of an agent step with its
// triage line, reply (R-5), note and stop reason (B-24), the end of the
// understanding, a card they answered, a stop report (P6, backlog-r2
// 2.5: its first line and why the chain stopped; GET /stop-report has
// the last one complete), or an audience question of a defense
// prepared without a stop (P-86). Never an internal id.
type ConversationEntry struct {
	// Kind is message · step · understanding · card_answered ·
	// stop_report · audience_question · round (backlog-r3 3.2: Label is
	// the round; GET /rounds/{label} has its card).
	Kind string `json:"kind"`
	Text string `json:"text,omitempty"`
	// WaitsSignoff: on a message, the agent answers it when the signoff in
	// progress ends (P-114).
	WaitsSignoff bool   `json:"waits_signoff,omitempty"`
	TriageLine   string `json:"triage_line,omitempty"`
	Reply        string `json:"reply,omitempty"`
	Note         string `json:"note,omitempty"`
	// Status is accepted or agent_pending for a step or understanding.
	Status     string `json:"status,omitempty"`
	StopReason string `json:"stop_reason,omitempty"`
	// Cause says why the mode finished, with stop_reason mode_finished (B-83).
	Cause string `json:"cause,omitempty"`
	// StoppedWithoutReport marks the step that ended a chain with no
	// stop report (P-75 (b)): it shows «El agente se detuvo: …» with
	// its stop reason (P-85, B-24).
	StoppedWithoutReport bool `json:"stopped_without_report,omitempty"`
	// Stretch is the step's stretch as the app settled it (B-43);
	// Reclassified says why rule 0 moved a direct step to build, shown
	// with the triage line.
	Stretch      string   `json:"stretch,omitempty"`
	Reclassified []string `json:"reclassified,omitempty"`
	// Incomplete marks a stop report whose principal failed twice: it
	// keeps the app's part only (P-75 (c), B-41).
	Incomplete bool `json:"incomplete,omitempty"`
	// An audience question (P-86): Text is the judge's literal question
	// or what the work does not cover (NotCovered), Label answers it
	// (POST /audience-questions/{label}/answer), State is its current
	// state (open, addressed, declined) and Section its section's title.
	Label      string `json:"label,omitempty"`
	State      string `json:"state,omitempty"`
	NotCovered bool   `json:"not_covered,omitempty"`
	Section    string `json:"section,omitempty"`
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
	// P-85: the turns whose chain ended in a stop report; a step that
	// ended any other chain shows why it stopped itself.
	reported := map[string]bool{}
	// The last step: a message before it was answered (P-114).
	lastStep := 0
	for _, ev := range events {
		if ev.Type == store.EventStopReport {
			reported[ev.TurnID] = true
		}
		if ev.Type == store.EventStepEnded {
			lastStep = ev.Seq
		}
	}
	signing, err := graph.SectionsInSignoff(r.Context(), s.db, projectID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}
	inSignoff := len(signing) > 0
	out := []ConversationEntry{}
	// The act of an answer the app refused (a card's «ok») is timed too, but you
	// did not answer: it follows its answer_without_reason (P-115).
	refused := map[string]bool{}
	for _, ev := range events {
		var data map[string]any
		_ = json.Unmarshal(ev.Data, &data)
		if ev.Type == store.EventAnswerWithoutReason {
			refused[ev.ObjectID] = true
			continue
		}
		if ev.Type == store.EventActTimed && refused[ev.ObjectID] {
			delete(refused, ev.ObjectID)
			continue
		}
		entry := ConversationEntry{CreatedAt: ev.CreatedAt}
		switch {
		case ev.Type == store.EventHumanMessage:
			entry.Kind, entry.Text = "message", str(data, "text")
			// P-114: the line under your message, while its signoff lasts.
			if waits, _ := data["waits_signoff"].(bool); waits && inSignoff && ev.Seq > lastStep {
				entry.WaitsSignoff = true
			}
		case ev.Type == store.EventStepEnded && str(data, "turn_kind") == "understanding":
			entry.Kind, entry.Status, entry.Text = "understanding", "accepted", str(data, "understood")
		case ev.Type == store.EventStepEnded && str(data, "turn_kind") == "step":
			entry.Kind, entry.Status = "step", "accepted"
			entry.TriageLine, entry.Reply = str(data, "triage_line"), str(data, "reply")
			entry.Note, entry.StopReason = str(data, "note"), str(data, "stop_reason")
			entry.Stretch = str(data, "stretch")
			if reasons, ok := data["reclassified"].([]any); ok {
				for _, reason := range reasons {
					if s, ok := reason.(string); ok {
						entry.Reclassified = append(entry.Reclassified, s)
					}
				}
			}
		case ev.Type == store.EventOutputRetry && str(data, "result") == "agent_pending":
			// [EL DISEÑO NO LO DICE]: a step left pending for the agent
			// writes no step_ended (B-24 names step_ended); its stop reason
			// is read from the output_retry that ended it. P-85: the
			// step's gate writes turn_kind step; the events written
			// before without it are a step too.
			entry.Kind, entry.Status = str(data, "turn_kind"), "agent_pending"
			if entry.Kind == "" {
				entry.Kind = "step"
			}
			if entry.Kind == "step" {
				entry.StopReason = "agent_pending"
			}
			if entry.Kind != "step" && entry.Kind != "understanding" {
				continue
			}
		case ev.Type == store.EventStateChanged && (ev.Actor == "human" && str(data, "kind") == "research" ||
			str(data, "kind") == "judge") && str(data, "to") == "open":
			// B-72: a round you started («Buscar evidencia»); one the agent
			// asked for follows its step (below). Backlog-r3 3.6: a judge
			// round, the app's or your audit's, where it starts.
			obj, err := s.db.GetObject(r.Context(), projectID, ev.ObjectID)
			if err != nil || obj.Type != store.ObjectRound {
				continue
			}
			entry.Kind, entry.Label = "round", obj.Label
		case ev.Type == store.EventStopReport:
			entry.Kind, entry.Text, entry.StopReason = "stop_report", str(data, "answer"), str(data, "stop_reason")
			entry.Cause = str(data, "cause")
			entry.Incomplete, _ = data["principal_failed"].(bool)
		case ev.Type == store.EventStateChanged && ev.AgentRole == string(provider.RoleJudge) &&
			str(data, "to") == "open" && !reported[ev.TurnID]:
			// P-86: the questions of a defense prepared after your answer,
			// with no stop report; those of a stop show in its report.
			aq, err := s.db.GetObject(r.Context(), projectID, ev.ObjectID)
			if err != nil || aq.Type != store.ObjectAudienceQuestion {
				continue
			}
			b := bodyMap(aq)
			entry.Kind, entry.Label, entry.State = "audience_question", aq.Label, aq.State
			entry.Text = str(b, "question")
			entry.NotCovered, _ = b["not_covered"].(bool)
			if sec, err := s.db.GetObject(r.Context(), projectID, str(data, "section")); err == nil {
				entry.Section = str(bodyMap(sec), "title")
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
		if entry.Kind == "step" && entry.StopReason != "" && !reported[ev.TurnID] {
			entry.StoppedWithoutReport = true
		}
		out = append(out, entry)
		// B-64: the round the step asked for, its card right after it.
		if entry.Kind == "step" && str(data, "round") != "" {
			out = append(out, ConversationEntry{Kind: "round", Label: str(data, "round"), CreatedAt: ev.CreatedAt})
		}
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
