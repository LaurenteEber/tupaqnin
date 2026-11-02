package provider_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/LaurenteEber/tupaqnin/internal/provider"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// B3-20, B3-22 row 2: a call that finished with a known cost is charged,
// although the request's context was cancelled by then.
func TestB3_20_AFinishedCallIsChargedWhenTheClientAborts(t *testing.T) {
	db, err := store.Open(context.Background(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := db.Writer.ExecContext(context.Background(),
		`INSERT INTO project (id, created_at) VALUES ('p1', '2026-10-28T00:00:00.000Z')`); err != nil {
		t.Fatal(err)
	}
	item, _ := json.Marshal(map[string]any{"type": "message", "role": "assistant",
		"content": []map[string]string{{"type": "output_text", "text": "{}"}}})
	fake := provider.NewFake(provider.Fixture{TurnKind: provider.TurnStep, Items: []json.RawMessage{item},
		Usage: provider.Usage{CostUSD: 0.07}})
	fake.Recorder = db
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := fake.Respond(ctx, provider.Request{ProjectID: "p1", Role: provider.RolePrincipal,
		TurnKind: provider.TurnStep, Model: provider.PrincipalModel}, nil); err != nil {
		t.Fatalf("Respond: %v", err)
	}
	spent, err := db.ProjectSpend(context.Background(), "p1")
	if err != nil || spent != 0.07 {
		t.Fatalf("spent %v, %v; want 0.07", spent, err)
	}
}
