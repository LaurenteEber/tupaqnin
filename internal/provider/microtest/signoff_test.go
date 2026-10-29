//go:build meta

// Recordings of real Wonder outputs for the signoff tests (backlog-r1
// 1.7): each scenario seeds the same project the cycle tests seed, runs
// the real signoff rounds (graph.WonderBrief, the contraster
// instructions and the Wonder schema) against Meta, and saves a round's
// output under internal/provider/testdata/signoff/ only when its marks
// are the ones the scenario needs, so the tests keep their meaning.
//
// It runs only with -tags=meta and META_API_KEY set:
//
//	make microtest-signoff
//
// No Meta call fails this test: what was saved and what was not is
// logged. new-n-in-decision.json stays hand-made: Wonder is not asked to
// break rule 4.
package microtest

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/LaurenteEber/tupaqnin/internal/cycle"
	"github.com/LaurenteEber/tupaqnin/internal/graph"
	"github.com/LaurenteEber/tupaqnin/internal/provider"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

const signoffDir = "../testdata/signoff"

// The section and decision of the cycle tests (signoff_test.go).
const (
	signText      = "Comparamos solo estudios con el mismo tamaño de partícula.\n\nAsí la comparación es justa."
	signStatement = "Comparar solo estudios con el mismo tamaño de partícula."
)

// recorder keeps the last call's deltas and response.
type recorder struct {
	inner  provider.Provider
	mu     sync.Mutex
	deltas []string
	resp   provider.Response
	start  time.Time
	gap    time.Duration
}

func (r *recorder) Respond(ctx context.Context, req provider.Request, onDelta func(provider.Delta)) (provider.Response, error) {
	r.mu.Lock()
	r.deltas, r.gap = nil, 0
	r.mu.Unlock()
	var last time.Time
	resp, err := r.inner.Respond(ctx, req, func(d provider.Delta) {
		r.mu.Lock()
		now := time.Now()
		if !last.IsZero() {
			r.gap += now.Sub(last)
		}
		last = now
		r.deltas = append(r.deltas, d.Text)
		r.mu.Unlock()
		if onDelta != nil {
			onDelta(d)
		}
	})
	if err == nil {
		r.mu.Lock()
		r.resp = resp
		r.mu.Unlock()
	}
	return resp, err
}

func (r *recorder) save(t *testing.T, name string) {
	t.Helper()
	r.mu.Lock()
	defer r.mu.Unlock()
	fx := provider.Fixture{Name: name, TurnKind: provider.TurnSignoff, Deltas: r.deltas, Items: r.resp.RawItems, Usage: r.resp.Usage}
	if n := len(r.deltas); n > 1 {
		fx.DeltaDelay = r.gap / time.Duration(n-1)
	}
	b, err := json.MarshalIndent(fx, "", "  ")
	if err != nil {
		t.Logf("%s: marshal: %v", name, err)
		return
	}
	if err := os.WriteFile(filepath.Join(signoffDir, name+".json"), b, 0o644); err != nil {
		t.Logf("%s: write: %v", name, err)
		return
	}
	t.Logf("recording saved: %s", name)
}

// scenario is one signoff of the seeded section: a fresh project, its
// gate and a runner over the recorder.
type scenario struct {
	t    *testing.T
	db   *store.DB
	gate *cycle.Gate
	run  *cycle.Runner
	rec  *recorder
	sign string
}

func newScenario(t *testing.T, meta provider.Provider, text string) *scenario {
	t.Helper()
	ctx := context.Background()
	db, err := store.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := db.Writer.ExecContext(ctx, `INSERT INTO project (id, created_at) VALUES ('p1', '2026-10-29T00:00:00.000Z')`); err != nil {
		t.Fatalf("seed project: %v", err)
	}
	err = db.Write(ctx, "p1", func(tx *store.Tx) error {
		sec, err := tx.CreateObject(store.ObjectSection, "draft", map[string]any{
			"title": "Regla de comparación", "text": text,
			"role":         map[string]any{"value": "conclusion", "state": "confirmed"},
			"open_changes": []any{},
		}, store.EventInput{Type: store.EventStateChanged, Actor: "agent"})
		if err != nil {
			return err
		}
		dec, err := tx.CreateObject(store.ObjectDecision, "agent", map[string]any{
			"statement": signStatement, "instead_of": "Comparar los conteos tal cual.",
			"would_choose_if": "Todos usaran el mismo filtro.", "assumption": false, "origin": "agent", "human_marked_critical": false,
		}, store.EventInput{Type: store.EventStateChanged, Actor: "agent"})
		if err != nil {
			return err
		}
		_, err = tx.AddEdge(store.EdgeUsedIn, dec.ID, sec.ID, "", store.EventInput{Type: store.EventEdge, Actor: "agent"})
		return err
	})
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	rec := &recorder{inner: meta}
	s := &scenario{t: t, db: db, gate: cycle.NewGate(db), run: cycle.NewRunner(db, rec, nil, nil), rec: rec}
	open, err := s.gate.RequestSignoff(ctx, "p1", "sec-1")
	if err != nil || !open.Opened {
		t.Fatalf("RequestSignoff: %+v, %v", open, err)
	}
	s.sign = open.Label
	return s
}

// round runs one real round and returns the view and the round's marks.
func (s *scenario) round() (graph.SignoffView, graph.WonderOutput, bool) {
	ctx := context.Background()
	if err := s.run.RunSignoffRound(ctx, "p1", "rec", s.sign); err != nil {
		s.t.Logf("round: %v", err)
		return graph.SignoffView{}, graph.WonderOutput{}, false
	}
	v, err := graph.SignoffDetail(ctx, s.db, "p1", s.sign)
	if err != nil || v.ContrastedBy != provider.ContrasterModel+" "+provider.ContrasterEffort {
		s.t.Logf("round not contrasted by Wonder (%q), %v: not saved", v.ContrastedBy, err)
		return v, graph.WonderOutput{}, false
	}
	var out graph.WonderOutput
	if err := json.Unmarshal([]byte(s.rec.resp.Text), &out); err != nil {
		return v, out, false
	}
	return v, out, true
}

func allVerdicts(out graph.WonderOutput, verdict string) bool {
	for _, d := range out.Decisions {
		if d.Verdict != verdict {
			return false
		}
	}
	return len(out.Decisions) > 0
}

func anyVerdict(out graph.WonderOutput, verdict string) bool {
	for _, d := range out.Decisions {
		if d.Verdict == verdict {
			return true
		}
	}
	return false
}

func (s *scenario) keep(name string, ok bool) {
	if ok {
		s.rec.save(s.t, name)
	} else {
		s.t.Logf("%s: the marks are not the scenario's; not saved (the hand-made one stays)", name)
	}
}

func TestMicroTestWonderSignoff(t *testing.T) {
	apiKey := os.Getenv("META_API_KEY")
	if apiKey == "" {
		t.Fatal("META_API_KEY is required: run `make microtest-signoff`, which reads it from the keychain")
	}
	meta := provider.NewMeta(apiKey, nil, nil)
	ctx := context.Background()
	const good = "Comparo solo estudios con el mismo tamaño de partícula, porque con filtros distintos la diferencia saldría del filtro y no del agua."
	const poor = "Comparo los estudios entre sí."

	// All match, then the origin answer that answers.
	a := newScenario(t, meta, signText)
	_ = a.gate.RecordExplanation(ctx, "p1", a.sign, good)
	if _, out, ok := a.round(); ok {
		a.keep("round1-all-match", allVerdicts(out, "match") && out.OriginQuestion.Answer == nil && out.NewCase == nil)
		_ = a.gate.RecordAnswers(ctx, "p1", a.sign, "Del supuesto de que todos los estudios cuentan desde el mismo tamaño mínimo de partícula.", "")
		if v, _, ok := a.round(); ok {
			a.keep("round2-all-match-answers", v.Coincides)
		}
	}

	// Missing in your explanation, then a case that does not apply.
	b := newScenario(t, meta, signText)
	_ = b.gate.RecordExplanation(ctx, "p1", b.sign, poor)
	if v, out, ok := b.round(); ok {
		b.keep("missing-in-explanation", anyVerdict(out, "missing_in_explanation") && v.OpenCase != "")
		if v.OpenCase != "" {
			_ = b.gate.RecordAnswers(ctx, "p1", b.sign, "Del filtro común.", "Sí, entra igual: todos los estudios se comparan.")
			if v2, out2, ok := b.round(); ok {
				b.keep("case-does-not-apply", out2.PreviousCase != nil && *out2.PreviousCase == "does_not_apply" && v2.OpenCase != "")
			}
		}
	}

	// A case applied well, and then everything coincides.
	c := newScenario(t, meta, signText)
	_ = c.gate.RecordExplanation(ctx, "p1", c.sign, poor)
	if v, _, ok := c.round(); ok && v.OpenCase != "" {
		_ = c.gate.RecordAnswers(ctx, "p1", c.sign, "Del supuesto de que todos cuentan desde el mismo tamaño mínimo.",
			"No entra: con otro tamaño de partícula la comparación ya no mide lo mismo, por eso la regla lo deja fuera.")
		if v2, out2, ok := c.round(); ok {
			c.keep("case-applies-well", out2.PreviousCase != nil && *out2.PreviousCase == "applies_well" && v2.Coincides)
		}
	}

	// Missing in the text: the explanation gives a reason the text does
	// not say.
	d := newScenario(t, meta, signText)
	_ = d.gate.RecordExplanation(ctx, "p1", d.sign, good+" El texto debería decir que los estudios con otro filtro quedan fuera.")
	if _, out, ok := d.round(); ok {
		d.keep("missing-in-text", anyVerdict(out, "missing_in_text"))
	}
}
