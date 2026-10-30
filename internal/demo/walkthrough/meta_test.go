//go:build meta

package walkthrough

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
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

// TestDemoCheck runs the case 5 walkthrough against Meta (make
// demo-check, TQ_PROVIDER=meta, key from the keychain). Eber runs it.
// It stops after Wonder's first round (P-57): Wonder's cases are never
// answered with fixed text; a person signs in the browser. A failing
// run names the step and the model's last output. The recorded answers
// stay as they are: they feed the fake's run up to «signed», which this
// run does not reach.
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
	section, sign, historyDecision := w.toSignoff()
	w.wonderFirstRound(sign)
	want, names := stepEvents(section, historyDecision)
	w.orderedEvents(want[:len(want)-afterFirstRound], names[:len(names)-afterFirstRound])
	spent, _ := w.db.ProjectSpend(context.Background(), w.project)
	t.Logf("walkthrough passed up to Wonder's first round: %d model calls, %.4f USD", len(rec.calls), spent)
}
