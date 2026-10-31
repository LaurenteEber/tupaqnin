package server_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/LaurenteEber/tupaqnin/internal/cycle"
	"github.com/LaurenteEber/tupaqnin/internal/graph"
	"github.com/LaurenteEber/tupaqnin/internal/server"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// seedOpenQuestion writes text on sec-1 and applies a step that proposes
// a «por aclarar» on it (backlog-r2 2.4), as the runner does.
func seedOpenQuestion(t *testing.T, db *store.DB, projectID string) {
	t.Helper()
	ctx := context.Background()
	err := db.Write(ctx, projectID, func(tx *store.Tx) error {
		sec, err := tx.GetObject(ctx, projectID, "sec-1")
		if err != nil {
			return err
		}
		_, err = tx.UpdateObject(sec.ID, sec.CurrentVersion, sec.State, map[string]any{
			"title": "Síntesis", "text": "La síntesis compara estudios.",
			"role": map[string]any{"value": "other", "state": "confirmed"}, "open_changes": []any{}},
			store.EventInput{Type: store.EventEdit, Actor: "human"})
		return err
	})
	if err != nil {
		t.Fatalf("seed text: %v", err)
	}
	out := []byte(`{"stretch":"build","triage_line":"Construir.","closing_risk":"none","note":"n",` +
		`"proposed_open_questions":[{"id":"new-1","question":"¿El piloto D1 cuenta como estudio de campo?",` +
		`"options":[{"option":"Cuenta","contrast":"Planta real.","consequence":"Entra.","risk":"Mezcla."},` +
		`{"option":"No cuenta","contrast":"Laboratorio.","consequence":"Queda fuera.","risk":"Se pierde un dato."}],` +
		`"depends_on":["sec-1"],"variants":["Con D1","Sin D1"]}]}`)
	gate := cycle.NewGate(db)
	if res, err := gate.EndStep(ctx, projectID, "turn-1", 1, out); err != nil || res.Status != cycle.StatusAccepted {
		t.Fatalf("EndStep: %+v, %v", res, err)
	}
	if applied, err := gate.ApplyStep(ctx, projectID, "turn-1", out); err != nil || !applied.Applied {
		t.Fatalf("ApplyStep: %+v, %v", applied, err)
	}
}

// P4 [2], backlog-r2 2.4: GET /todo lists the open «por aclarar» with
// its options and what it blocks; «Te toca» of the frame has its line;
// the answer route keeps it open without an option or with «ok», and
// resolves it with an option and a «porque».
func TestP4_TodoListsTheOpenQuestionAndTheAnswerRouteResolvesIt(t *testing.T) {
	h, db, _ := newAPI(t)
	projectID := newProject(t, h, nil)
	seedUnderstanding(t, db, projectID)
	seedOpenQuestion(t, db, projectID)
	base := "/api/projects/" + projectID

	todo := decode[server.TodoView](t, do(t, h, http.MethodGet, base+"/todo", nil))
	if len(todo.OpenQuestions) != 1 {
		t.Fatalf("todo: %+v, want one open question", todo)
	}
	q := todo.OpenQuestions[0]
	if q.Label != "oq-1" || q.Question != "¿El piloto D1 cuenta como estudio de campo?" || len(q.Options) != 2 ||
		q.Options[1].Option != "No cuenta" || q.Options[1].Risk != "Se pierde un dato." || q.Options[0].Keep ||
		len(q.Sections) != 1 || q.Sections[0].Title != "Síntesis" || len(q.Variants) != 2 || q.UndeclaredByAgent {
		t.Fatalf("open question: %+v", q)
	}
	frame := getFrame(t, h, projectID)
	if len(frame.Todo.OpenQuestions) != 1 || frame.Todo.OpenQuestions[0].Label != "oq-1" ||
		frame.Todo.OpenQuestions[0].Question != q.Question {
		t.Fatalf("frame todo: %+v, want the open question's line", frame.Todo)
	}
	for _, s := range frame.Sections {
		if s.Label == "sec-1" && s.Status != graph.StatusDraft {
			t.Fatalf("sec-1 in the frame: %s, want draft", s.Status)
		}
	}

	answer := base + "/open-questions/oq-1/answer"
	cases := []struct {
		body any
		code int
		err  string
	}{
		{map[string]any{"option": nil, "reason": "B, como dije"}, http.StatusUnprocessableEntity, "option_required"},
		{map[string]any{"option": 0, "reason": "ok"}, http.StatusUnprocessableEntity, "invalid_reason"},
		{map[string]any{"option": 2, "reason": "Porque se midió en planta."}, http.StatusBadRequest, "invalid_option"},
	}
	for _, c := range cases {
		rec := do(t, h, http.MethodPost, answer, c.body)
		if rec.Code != c.code || decode[map[string]string](t, rec)["error"] != c.err {
			t.Fatalf("answer %v: %d %s, want %d %s", c.body, rec.Code, rec.Body.String(), c.code, c.err)
		}
	}
	if rec := do(t, h, http.MethodPost, base+"/open-questions/oq-9/answer", map[string]any{"option": 0, "reason": "Porque sí."}); rec.Code != http.StatusNotFound {
		t.Fatalf("unknown: %d, want 404", rec.Code)
	}
	if todo := decode[server.TodoView](t, do(t, h, http.MethodGet, base+"/todo", nil)); len(todo.OpenQuestions) != 1 {
		t.Fatalf("todo after invalid answers: %+v, want it still open", todo)
	}

	rec := do(t, h, http.MethodPost, answer, map[string]any{"option": 1, "reason": "Porque se midió en laboratorio."})
	if got := decode[server.AnswerOpenQuestionResponse](t, rec); rec.Code != http.StatusOK || got.OpenQuestion != "oq-1" || got.NewDecision != "" {
		t.Fatalf("answer: %d %s", rec.Code, rec.Body.String())
	}
	if todo := decode[server.TodoView](t, do(t, h, http.MethodGet, base+"/todo", nil)); len(todo.OpenQuestions) != 0 {
		t.Fatalf("todo after answer: %+v, want empty", todo)
	}
	if rec := do(t, h, http.MethodPost, answer, map[string]any{"option": 0, "reason": "Porque cambié de idea."}); rec.Code != http.StatusConflict {
		t.Fatalf("answer again: %d, want 409", rec.Code)
	}
	if frame := getFrame(t, h, projectID); len(frame.Todo.OpenQuestions) != 0 {
		t.Fatalf("frame todo after answer: %+v", frame.Todo)
	}
}
