package server_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/LaurenteEber/tupaqnin/internal/server"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// B-24 and U-5: after a reload the conversation still shows each
// message, the agent's reply (R-5) and why the chain stopped.
func TestB24_StopReasonIsReadAfterReload(t *testing.T) {
	h, db, _ := newAPI(t)
	projectID := newProject(t, h, nil)
	err := db.Write(context.Background(), projectID, func(tx *store.Tx) error {
		events := []store.EventInput{
			{Type: store.EventHumanMessage, Actor: "human", TurnID: "t1",
				Data: map[string]any{"text": "¿En qué te basaste?"}},
			{Type: store.EventStepEnded, Actor: "agent", AgentRole: "principal", TurnID: "t1",
				Data: map[string]any{"turn_kind": "step", "triage_line": "Respondo tu pregunta.",
					"reply": "En el material que subiste.", "note": "Solo respondí.", "stop_reason": "no_work"}},
			{Type: store.EventOutputRetry, Actor: "agent", AgentRole: "principal", TurnID: "t2",
				Data: map[string]any{"turn_kind": "step", "attempt": 2, "result": "agent_pending"}},
		}
		for _, ev := range events {
			if _, err := tx.AppendEvent(ev); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("seed events: %v", err)
	}
	got := decode[[]server.ConversationEntry](t, do(t, h, http.MethodGet, "/api/projects/"+projectID+"/conversation", nil))
	if len(got) != 3 {
		t.Fatalf("entries: %+v", got)
	}
	if got[0].Kind != "message" || got[0].Text != "¿En qué te basaste?" {
		t.Fatalf("message: %+v", got[0])
	}
	if got[1].Kind != "step" || got[1].Reply != "En el material que subiste." || got[1].StopReason != "no_work" {
		t.Fatalf("step: %+v", got[1])
	}
	if got[2].Status != "agent_pending" || got[2].StopReason != "agent_pending" {
		t.Fatalf("pending step: %+v", got[2])
	}
}

// U-1: the person edits the project's title in the header.
func TestU1_PersonRenamesTheProject(t *testing.T) {
	h, _, _ := newAPI(t)
	projectID := newProject(t, h, nil)
	if rec := do(t, h, http.MethodPut, "/api/projects/"+projectID+"/name", map[string]any{"name": "  "}); rec.Code != http.StatusBadRequest {
		t.Fatalf("empty name: %d", rec.Code)
	}
	if rec := do(t, h, http.MethodPut, "/api/projects/"+projectID+"/name", map[string]any{"name": "Pobreza y cerebro"}); rec.Code != http.StatusNoContent {
		t.Fatalf("rename: %d (%s)", rec.Code, rec.Body.String())
	}
	for _, p := range decode[[]store.Project](t, do(t, h, http.MethodGet, "/api/projects", nil)) {
		if p.ID == projectID && p.Name != "Pobreza y cerebro" {
			t.Fatalf("name: %q", p.Name)
		}
	}
}

// U-2: the summary names the principal's model, its effort and the
// context window, before any call.
func TestU2_SummaryShowsTheModel(t *testing.T) {
	h, _, _ := newAPI(t)
	projectID := newProject(t, h, nil)
	got := decode[server.SummaryView](t, do(t, h, http.MethodGet, "/api/projects/"+projectID+"/summary", nil))
	if got.Model != "muse-spark-1.3" || got.Effort != "medium" || got.ContextWindow != 1_048_576 {
		t.Fatalf("summary: %+v", got)
	}
}
