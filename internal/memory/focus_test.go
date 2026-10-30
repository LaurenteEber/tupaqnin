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
