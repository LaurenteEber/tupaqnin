//go:build meta

package walkthrough

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/LaurenteEber/tupaqnin/internal/memory"
	"github.com/LaurenteEber/tupaqnin/internal/provider"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// recorder keeps every call's deltas and response, in order, so a
// passing run becomes the fake's recorded answers.
type recorder struct {
	inner provider.Provider
	mu    sync.Mutex
	calls []provider.Fixture
	last  string
}

func (r *recorder) Respond(ctx context.Context, req provider.Request, onDelta func(provider.Delta)) (provider.Response, error) {
	var deltas []string
	var gap time.Duration
	var prev time.Time
	resp, err := r.inner.Respond(ctx, req, func(d provider.Delta) {
		now := time.Now()
		if !prev.IsZero() {
			gap += now.Sub(prev)
		}
		prev = now
		deltas = append(deltas, d.Text)
		if onDelta != nil {
			onDelta(d)
		}
	})
	r.mu.Lock()
	defer r.mu.Unlock()
	if err != nil {
		// Never the raw provider body (AGENTS.md rule 9): the code only.
		r.last = fmt.Sprintf("%s call failed: %T", req.TurnKind, err)
		if pe, ok := err.(*provider.ProviderError); ok {
			r.last = fmt.Sprintf("%s call failed: %s %s", req.TurnKind, pe.Type, pe.Code)
		}
		return resp, err
	}
	fx := provider.Fixture{TurnKind: req.TurnKind, Deltas: deltas, Items: resp.RawItems, Usage: resp.Usage}
	if n := len(deltas); n > 1 {
		fx.DeltaDelay = gap / time.Duration(n-1)
	}
	fx.Name = fmt.Sprintf("%02d-%s", len(r.calls)+1, req.TurnKind)
	r.calls = append(r.calls, fx)
	r.last = fmt.Sprintf("%s (%s)", fx.Name, resp.Text)
	if resp.Text == "" && len(resp.ToolCalls) > 0 {
		r.last = fmt.Sprintf("%s (tool calls: %d)", fx.Name, len(resp.ToolCalls))
	}
	return resp, nil
}

func (r *recorder) lastOutput() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.last
}

// save replaces the recorded answers with this run's calls.
func (r *recorder) save(t *testing.T) {
	t.Helper()
	old, _ := filepath.Glob(filepath.Join(recordedDir, "*.json"))
	for _, name := range old {
		if err := os.Remove(name); err != nil {
			t.Fatalf("remove %s: %v", name, err)
		}
	}
	for _, fx := range r.calls {
		b, err := json.MarshalIndent(fx, "", "  ")
		if err != nil {
			t.Fatalf("marshal %s: %v", fx.Name, err)
		}
		if err := os.WriteFile(filepath.Join(recordedDir, fx.Name+".json"), b, 0o644); err != nil {
			t.Fatalf("write %s: %v", fx.Name, err)
		}
	}
	t.Logf("recorded answers saved: %d calls in %s", len(r.calls), recordedDir)
}

// TestDemoCheck runs the case 5 walkthrough against Meta (make
// demo-check, TQ_PROVIDER=meta, key from the keychain). Eber runs it.
// A failing run names the step and the model's last output and leaves
// the recorded answers as they were; a passing run replaces them.
func TestDemoCheck(t *testing.T) {
	key := os.Getenv("META_API_KEY")
	if key == "" || os.Getenv("TQ_PROVIDER") != "meta" {
		t.Skip("demo-check needs TQ_PROVIDER=meta and META_API_KEY (make demo-check)")
	}
	rec := &recorder{}
	w := newWalk(t, func(db *store.DB) (provider.Provider, memory.TokenCounter) {
		meta := provider.NewMeta(key, db, slog.New(slog.NewTextHandler(io.Discard, nil)))
		rec.inner = meta
		return rec, func(ctx context.Context, input []json.RawMessage) (int, error) {
			return meta.CountTokens(ctx, provider.PrincipalModel, input)
		}
	}, rec.lastOutput)
	signed := w.run()
	w.orderedEvents(stepEvents(signed))
	spent, _ := w.db.ProjectSpend(context.Background(), w.project)
	t.Logf("walkthrough passed: %d model calls, %.4f USD", len(rec.calls), spent)
	rec.save(t)
}
