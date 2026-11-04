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
// break rule 4. The thesis scenarios (B-106, R3′) log what each round asked.
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
	calls  int
	spent  float64
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
		r.calls++
		r.spent += resp.Usage.CostUSD
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
	return seededScenario(t, meta, text, map[string]any{"value": "conclusion", "state": "proposed"}, true)
}

// seededScenario is a scenario with this role on the section and, if decision,
// the decision used in it. A role of yours (state confirmed) is the «aquí se
// concluye» mark (B-106). The role decides whether the decision is critical.
func seededScenario(t *testing.T, meta provider.Provider, text string, role map[string]any, decision bool) *scenario {
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
			"role":         role,
			"open_changes": []any{},
		}, store.EventInput{Type: store.EventStateChanged, Actor: "agent"})
		if err != nil || !decision {
			return err
		}
		dec, err := tx.CreateObject(store.ObjectDecision, "agent", map[string]any{
			"statement": signStatement, "instead_of": "Comparar los conteos tal cual.",
			"would_choose_if": "Todos usaran el mismo filtro.", "assumption": false, "origin": "agent", "human_marked_critical": false,
		}, store.EventInput{Type: store.EventStateChanged, Actor: "agent"})
		if err != nil {
			return err
		}
		_, err = tx.AddEdge(store.EdgeUsedIn, dec.ID, sec.ID, "", nil, store.EventInput{Type: store.EventEdge, Actor: "agent"})
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

// thesisEntry is the verdict of the point B-106 adds, from the view.
func thesisEntry(v graph.SignoffView) (graph.WonderVerdict, bool) {
	for _, w := range v.Verdicts {
		if w.Thesis != "" {
			return w, true
		}
	}
	return graph.WonderVerdict{}, false
}

// report logs what the round asked about the thesis: seeded text only, for Eber to read.
func (s *scenario) report(name string, v graph.SignoffView, out graph.WonderOutput) {
	w, ok := thesisEntry(v)
	s.t.Logf("%s: critical %d · thesis asked %q · thesis verdict %q · history offered %v · missing %q · origin question %q · new case %q · coincides %v · can sign %v",
		name, len(v.CriticalDecisions), v.Thesis, w.Verdict, ok && w.HistoryOffered, w.Missing, v.OriginQuestion, v.OpenCase, v.Coincides, v.CanSign)
	if out.PreviousCase != nil {
		s.t.Logf("%s: previous case judged %q", name, *out.PreviousCase)
	}
}

func thesisMatches(v graph.SignoffView) bool {
	w, ok := thesisEntry(v)
	return ok && w.Verdict == "match"
}

func thesisGap(v graph.SignoffView) bool {
	w, ok := thesisEntry(v)
	return ok && w.Verdict == "missing_in_explanation" && w.HistoryOffered && v.OpenCase != ""
}

// thesisScenarios are B-106 and R3′ against real Wonder. Their recordings keep their
// own names: the tests replay the hand-made ones, whose order and labels they pin.
func thesisScenarios(t *testing.T, meta provider.Provider, spent func(*scenario)) {
	t.Helper()
	ctx := context.Background()
	marked := map[string]any{"value": "conclusion", "state": "confirmed"}
	plain := map[string]any{"value": "other", "state": "confirmed"}
	const concludes = "Comparo solo estudios con el mismo tamaño de partícula, porque con filtros distintos la diferencia saldría del filtro y no del agua. " +
		"Por eso concluyo que la comparación es justa: mide el agua y no el filtro."
	const onlyTheRule = "Comparo solo estudios con el mismo tamaño de partícula, porque con filtros distintos la diferencia saldría del filtro y no del agua."
	const origin = "Del supuesto de que todos los estudios cuentan desde el mismo tamaño mínimo de partícula."

	// Your mark and a critical decision, explained whole: the entry matches.
	e := seededScenario(t, meta, signText, marked, true)
	_ = e.gate.RecordExplanation(ctx, "p1", e.sign, concludes)
	if v, out, ok := e.round(); ok {
		e.report("thesis-marked-round1", v, out)
		e.keep("thesis-marked-round1", thesisMatches(v) && out.OriginQuestion.Answer == nil)
		_ = e.gate.RecordAnswers(ctx, "p1", e.sign, origin, "")
		if v2, out2, ok := e.round(); ok {
			e.report("thesis-marked-round2", v2, out2)
			e.keep("thesis-marked-round2", thesisMatches(v2) && v2.Coincides)
		}
	}
	spent(e)

	// R3′: the explanation leaves out what the section concludes. A gap in the
	// thesis, with its history and a new case, and the case applied well closes it.
	f := seededScenario(t, meta, signText, marked, true)
	_ = f.gate.RecordExplanation(ctx, "p1", f.sign, onlyTheRule)
	if v, out, ok := f.round(); ok {
		f.report("thesis-marked-gap", v, out)
		f.keep("thesis-marked-gap", thesisGap(v))
		if v.OpenCase != "" {
			_ = f.gate.RecordAnswers(ctx, "p1", f.sign, origin,
				"Le digo que no: con otro tamaño de partícula la diferencia saldría del filtro, así que la conclusión de que la comparación es justa ya no valdría.")
			if v2, out2, ok := f.round(); ok {
				f.report("thesis-marked-case-well", v2, out2)
				f.keep("thesis-marked-case-well", out2.PreviousCase != nil && *out2.PreviousCase == "applies_well" && v2.Coincides)
			}
		}
	}
	spent(f)

	// Your mark with no decisions: the thesis alone, with no origin question.
	g := seededScenario(t, meta, signText, marked, false)
	_ = g.gate.RecordExplanation(ctx, "p1", g.sign, concludes)
	if v, out, ok := g.round(); ok {
		g.report("thesis-marked-only", v, out)
		g.keep("thesis-marked-only", thesisMatches(v) && v.OriginQuestion == "" && v.Coincides)
	}
	spent(g)

	// No mark, and a decision that is not critical: that decision is the thesis.
	h := seededScenario(t, meta, signText, plain, true)
	_ = h.gate.RecordExplanation(ctx, "p1", h.sign, onlyTheRule)
	if v, out, ok := h.round(); ok {
		h.report("thesis-decision", v, out)
		h.keep("thesis-decision", v.Thesis == graph.ThesisDecision && len(v.CriticalDecisions) == 0 && thesisMatches(v))
	}
	spent(h)
}

func TestMicroTestWonderSignoff(t *testing.T) {
	apiKey := os.Getenv("META_API_KEY")
	if apiKey == "" {
		t.Fatal("META_API_KEY is required: run `make microtest-signoff`, which reads it from the keychain")
	}
	meta := provider.NewMeta(apiKey, nil, nil)
	ctx := context.Background()
	calls, usd := 0, 0.0
	spent := func(s *scenario) {
		calls += s.rec.calls
		usd += s.rec.spent
	}
	defer func() { t.Logf("Wonder: %d calls, %.4f USD", calls, usd) }()
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
	for _, s := range []*scenario{a, b, c, d} {
		spent(s)
	}

	thesisScenarios(t, meta, spent)
}
