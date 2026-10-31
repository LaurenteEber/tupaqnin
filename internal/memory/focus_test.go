package memory_test

import (
	"context"
	"strings"
	"testing"

	"github.com/LaurenteEber/tupaqnin/internal/memory"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// K-17, bloque-2 §3: when the task is writing a section, it goes with
// its «qué debe abordar» in literal; another section's focus does not.
func TestK17_TaskOnASectionCarriesItsFocusLiteral(t *testing.T) {
	ctx := context.Background()
	db, err := store.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := db.Writer.ExecContext(ctx,
		`INSERT INTO project (id, created_at) VALUES ('p1', '2026-10-30T00:00:00.000Z')`); err != nil {
		t.Fatalf("seed project: %v", err)
	}
	const focus = "Qué hacer con el aire de las escuelas, en dos pasos, sin  repetir la síntesis."
	const other = "Cómo se midió el PM2.5."
	if err := db.Write(ctx, "p1", func(tx *store.Tx) error {
		for i, s := range []struct{ title, focus string }{{"Método", other}, {"Recomendaciones", focus}} {
			if _, err := tx.CreateObject(store.ObjectSection, "draft", map[string]any{
				"title": s.title, "focus": s.focus, "position": i + 1, "text": "",
				"role": map[string]any{"value": "", "state": ""}, "open_changes": []any{},
			}, store.EventInput{Type: store.EventStateChanged, Actor: "human"}); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		t.Fatalf("seed sections: %v", err)
	}
	appendEvent(t, db, store.EventInput{Type: store.EventHumanMessage, Actor: "human",
		Data: map[string]any{"text": "Escribe las recomendaciones, por favor."}})

	brief, err := memory.New(db).Startup(ctx, "p1", memory.StartupNewSession, 1048576, fakeCount)
	if err != nil {
		t.Fatalf("Startup: %v", err)
	}
	var task string
	for _, item := range brief.Input {
		if strings.Contains(string(item), "Current task") {
			task = string(item)
		}
	}
	if !strings.Contains(task, focus) {
		t.Fatalf("task does not carry the focus in literal:\n%s", task)
	}
	if strings.Contains(task, other) {
		t.Fatalf("task carries another section's focus:\n%s", task)
	}
}

// P-70, bloque-2 §1 and §3: while the plan is not confirmed, the task
// on a section carries its plan line from the «Entendí», marked as the
// agent's proposal, so a new session does not lose it; once you answer
// the plan, the section's own focus is what goes (B-53).
func TestP70_StartupTaskCarriesTheUnconfirmedPlanLine(t *testing.T) {
	ctx := context.Background()
	db, err := store.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := db.Writer.ExecContext(ctx,
		`INSERT INTO project (id, created_at) VALUES ('p1', '2026-10-30T00:00:00.000Z')`); err != nil {
		t.Fatalf("seed project: %v", err)
	}
	const line = "Qué hacer con el aire de las escuelas, en dos pasos."
	const other = "Cómo se midió el PM2.5."
	var labels []string
	if err := db.Write(ctx, "p1", func(tx *store.Tx) error {
		for i, title := range []string{"Método", "Recomendaciones"} {
			obj, err := tx.CreateObject(store.ObjectSection, "draft", map[string]any{
				"title": title, "position": i + 1, "text": "",
				"role": map[string]any{"value": "", "state": ""}, "open_changes": []any{},
			}, store.EventInput{Type: store.EventStateChanged, Actor: "human"})
			if err != nil {
				return err
			}
			labels = append(labels, obj.Label)
		}
		return nil
	}); err != nil {
		t.Fatalf("seed sections: %v", err)
	}
	appendEvent(t, db, store.EventInput{Type: store.EventStepEnded, Actor: "agent", AgentRole: "principal",
		Data: map[string]any{"turn_kind": "understanding", "plan": []any{
			map[string]any{"section": labels[0], "focus": other, "evidence_from": "Las fuentes."},
			map[string]any{"section": labels[1], "focus": line, "evidence_from": "Las fuentes."},
		}}})
	appendEvent(t, db, store.EventInput{Type: store.EventHumanMessage, Actor: "human",
		Data: map[string]any{"text": "Escribe las recomendaciones, por favor."}})

	task := func() string {
		brief, err := memory.New(db).Startup(ctx, "p1", memory.StartupNewSession, 1048576, fakeCount)
		if err != nil {
			t.Fatalf("Startup: %v", err)
		}
		for _, item := range brief.Input {
			if strings.Contains(string(item), "Current task") {
				return string(item)
			}
		}
		return ""
	}
	got := task()
	if !strings.Contains(got, line) || !strings.Contains(got, "proposed by the agent, not confirmed") {
		t.Fatalf("task does not carry the unconfirmed plan line, marked:\n%s", got)
	}
	if strings.Contains(got, other) {
		t.Fatalf("task carries another section's plan line:\n%s", got)
	}

	brief := func() string {
		b, err := memory.New(db).Startup(ctx, "p1", memory.StartupNewSession, 1048576, fakeCount)
		if err != nil {
			t.Fatalf("Startup: %v", err)
		}
		var all strings.Builder
		for _, item := range b.Input {
			all.Write(item)
		}
		return all.String()
	}
	if strings.Contains(brief(), "Plan confirmed") {
		t.Fatalf("before the answer, the startup says the plan is confirmed")
	}
	appendEvent(t, db, store.EventInput{Type: store.EventPlanConfirmed, Actor: "human"})
	if got := task(); strings.Contains(got, line) {
		t.Fatalf("after the plan is confirmed, the proposed line still goes:\n%s", got)
	}
	// The principal knows the plan is confirmed, so it proposes a new
	// section in reply instead of creating it (B-54, P-70).
	if !strings.Contains(brief(), "Plan confirmed") {
		t.Fatalf("after the answer, the startup does not say the plan is confirmed")
	}
}
