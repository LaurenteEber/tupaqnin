package memory_test

import (
	"context"
	"strings"
	"testing"

	"github.com/LaurenteEber/tupaqnin/internal/memory"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// P-51, bloque-2 §2 and §3: the principal sees every computed pending
// of estados §1 (B-15) with its section and passage, and the first one
// is the task in turn. An open card is yours, not the agent's.
func TestP51_ComputedPendingsReachThePrincipalAndTheFirstIsTheTask(t *testing.T) {
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
	const passage = "Con el mismo filtro, la embotellada trae 1,5 veces más."
	if err := db.Write(ctx, "p1", func(tx *store.Tx) error {
		for i, s := range []struct{ title, text string }{
			{"Estudios incluidos", "Incluimos los estudios de campo."},
			{"Síntesis", "Los conteos crudos no se comparan.\n\n" + passage},
		} {
			if _, err := tx.CreateObject(store.ObjectSection, "draft", map[string]any{
				"title": s.title, "position": i + 1, "text": s.text,
				"role": map[string]any{"value": "other", "state": "proposed"}, "open_changes": []any{},
			}, store.EventInput{Type: store.EventStateChanged, Actor: "agent"}); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		t.Fatalf("seed sections: %v", err)
	}

	st, err := memory.New(db).WorkingState(ctx, "p1")
	if err != nil {
		t.Fatalf("WorkingState: %v", err)
	}
	if len(st.InFlight.AgentPending) != 1 {
		t.Fatalf("agent pendings: %q, want the one figure without decision", st.InFlight.AgentPending)
	}
	got := st.InFlight.AgentPending[0]
	for _, want := range []string{"sec-2", "figure_without_decision", "1,5", passage} {
		if !strings.Contains(got, want) {
			t.Fatalf("agent pending %q misses %q", got, want)
		}
	}

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
	if !strings.Contains(task, "First agent pending: sec-2") || !strings.Contains(task, "1,5 veces") {
		t.Fatalf("task is not the first agent pending:\n%s", task)
	}
}
