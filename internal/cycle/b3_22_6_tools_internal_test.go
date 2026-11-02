package cycle

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/LaurenteEber/tupaqnin/internal/provider"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// B3-22 row 6: the agent's tools use the turn's project, never one the
// model names. With the label or the id of another project's source,
// neither the principal's nor a reader's tools find it, and nothing is
// recorded as read.
func TestB3_22_6_AgentToolsReadOnlyTheTurnProject(t *testing.T) {
	ctx := context.Background()
	db, err := store.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	for _, id := range []string{"mine", "other"} {
		if _, err := db.Writer.ExecContext(ctx, `INSERT INTO project (id, created_at) VALUES (?, '2026-11-02T00:00:00.000Z')`, id); err != nil {
			t.Fatalf("seed project: %v", err)
		}
	}
	const secret = "palabra-secreta-del-otro-proyecto"
	var otherID string
	if err := db.Write(ctx, "other", func(tx *store.Tx) error {
		for _, text := range []string{"primero", secret} {
			o, err := tx.CreateObject(store.ObjectSource, "", map[string]any{"kind": "material",
				"filename": "otro.md", "media_type": "text/markdown", "text": text},
				store.EventInput{Type: store.EventStateChanged, Actor: "human"})
			if err != nil {
				return err
			}
			otherID = o.ID
		}
		return nil
	}); err != nil {
		t.Fatalf("seed other: %v", err)
	}
	if err := db.Write(ctx, "mine", func(tx *store.Tx) error {
		_, err := tx.CreateObject(store.ObjectSource, "", map[string]any{"kind": "material",
			"filename": "mio.md", "media_type": "text/markdown", "text": "lo mío"},
			store.EventInput{Type: store.EventStateChanged, Actor: "human"})
		return err
	}); err != nil {
		t.Fatalf("seed mine: %v", err)
	}
	r := NewRunner(db, provider.NewFake(), nil, nil)
	t.Cleanup(r.Close)

	call := func(name string, args map[string]any) provider.ToolCall {
		raw, _ := json.Marshal(args)
		return provider.ToolCall{Name: name, Arguments: string(raw)}
	}
	calls := map[string]provider.ToolCall{
		"label of a source only the other has": call("read_source", map[string]any{"source": "src-2"}),
		"id of the other's source":             call("read_source", map[string]any{"source": otherID}),
		"a project in the arguments":           call("read_source", map[string]any{"source": "src-2", "project_id": "other"}),
		"search":                               call("search_sources", map[string]any{"text": secret}),
	}
	for name, c := range calls {
		out, err := r.runTool(ctx, "mine", "inst-1", c)
		if err != nil {
			t.Fatalf("principal, %s: %v", name, err)
		}
		if strings.Contains(out, secret) {
			t.Errorf("principal, %s, found the other's text: %q", name, out)
		}
		out, err = r.runReaderTool(ctx, "mine", c, map[string]bool{"project": true}, 0, nil)
		if err != nil {
			t.Fatalf("reader, %s: %v", name, err)
		}
		if strings.Contains(out, secret) {
			t.Errorf("reader, %s, found the other's text: %q", name, out)
		}
	}
	// The same tool does read its own project (the check above can fail).
	out, err := r.runTool(ctx, "other", "inst-2", call("read_source", map[string]any{"source": "src-2"}))
	if err != nil || !strings.Contains(out, secret) {
		t.Fatalf("own project: %q, %v", out, err)
	}
}
