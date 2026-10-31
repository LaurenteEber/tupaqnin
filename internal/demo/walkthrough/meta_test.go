//go:build meta

package walkthrough

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/LaurenteEber/tupaqnin/internal/graph"
	"github.com/LaurenteEber/tupaqnin/internal/memory"
	"github.com/LaurenteEber/tupaqnin/internal/provider"
	"github.com/LaurenteEber/tupaqnin/internal/server"
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

// campoReason answers the «por aclarar» on «campo» with a «porque».
const campoReason = "Porque mi criterio pide muestrear donde se toma el agua; un piloto de laboratorio no lo es."

// TestDemoCheckSlice2 runs the slice-2 stretch of case 5 against Meta
// (backlog-r2 2.10, «Listo cuando»; make demo-check runs it with
// TestDemoCheck, TQ_PROVIDER=meta, key from the keychain). Eber runs it:
// after the understanding and the agent's chain, the «por aclarar» on
// «campo» (D1) is open and the signoff of each section it blocks lists
// it; once answered with an option and a «porque», it no longer blocks.
// A failing run names the step and the model's last output.
func TestDemoCheckSlice2(t *testing.T) {
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
	ctx := context.Background()

	// 1–4: the demo case, the request sent, the understanding and the
	// agent's chain (B-17).
	var created server.CreateProjectResponse
	w.do("1 demo case", http.MethodPost, "/api/projects", map[string]any{"template": "case-5"}, http.StatusCreated, &created)
	w.project = created.ProjectID
	w.do("2 request", http.MethodPost, w.api("/request/submit"), nil, http.StatusAccepted, nil)

	// 7 (slice 2): the «por aclarar» on «campo».
	var todo server.TodoView
	w.do("7 campo", http.MethodGet, w.api("/todo"), nil, http.StatusOK, &todo)
	var campo server.OpenQuestionView
	var open []string
	// P-82: an open question depends on sections or decisions, never on
	// a criterion (the app rejects one). What counts is an open «por
	// aclarar» that names the pilot or «campo» (case- and
	// accent-insensitive) and blocks «Estudios incluidos», where the pilot
	// is classified.
	for _, q := range todo.OpenQuestions {
		open = append(open, q.Question)
		if !namesWord(q.Question, "piloto") && !namesWord(q.Question, "campo") {
			continue
		}
		for _, sec := range q.Sections {
			if namesWord(sec.Title, "Estudios incluidos") {
				campo = q
			}
		}
		if campo.Label != "" {
			break
		}
	}
	if campo.Label == "" {
		w.fail("7 campo", "no open «por aclarar» naming the pilot or «campo» that blocks «Estudios incluidos»; open: %q", open)
	}

	// Its sections cannot be signed while it is open.
	signoff := func(step, label string) (int, server.SignoffBlocked) {
		req := httptest.NewRequest(http.MethodPost, w.api("/sections/"+label+"/signoff"), nil)
		res := httptest.NewRecorder()
		w.h.ServeHTTP(res, req)
		var blocked server.SignoffBlocked
		if res.Code == http.StatusConflict {
			_ = json.Unmarshal(res.Body.Bytes(), &blocked)
		}
		return res.Code, blocked
	}
	lists := func(b server.SignoffBlocked) bool {
		for _, p := range b.Pendings {
			if p.Kind == graph.PendingOpenQuestion && p.About == campo.Label {
				return true
			}
		}
		return false
	}
	for _, sec := range campo.Sections {
		code, blocked := signoff("7 campo blocks", sec.Label)
		if code != http.StatusConflict || !lists(blocked) {
			w.fail("7 campo blocks", "signoff of %q: %d %+v, want it blocked by %s", sec.Title, code, blocked, campo.Label)
		}
	}

	// Answered with an option and a «porque», it no longer blocks.
	option := 0
	w.do("7 campo answered", http.MethodPost, w.api("/open-questions/"+campo.Label+"/answer"),
		map[string]any{"option": option, "reason": campoReason}, http.StatusOK, nil)
	for _, sec := range campo.Sections {
		if code, blocked := signoff("7 campo answered", sec.Label); code == http.StatusConflict && lists(blocked) {
			w.fail("7 campo answered", "signoff of %q still lists %s after the answer: %+v", sec.Title, campo.Label, blocked)
		}
	}
	spent, _ := w.db.ProjectSpend(ctx, w.project)
	t.Logf("slice-2 stretch passed: %d model calls, %.4f USD", len(rec.calls), spent)
}
