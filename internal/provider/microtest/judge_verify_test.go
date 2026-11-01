//go:build meta

// Recordings of real judge outputs for the verification tests (backlog-r3
// 3.6): one project with the section the cycle tests judge
// (internal/cycle/judge_test.go: the same request, criterion, cases,
// premise, source and text), the real judge (cycle.Runner.RunJudgeVerify
// and RunJudgeRecheck: the embedded instructions and
// salida-juez-verificacion, its one retry) against Meta, and each call's
// output saved under internal/provider/testdata/judge-verify/ only when it
// is what its test needs, so the tests keep their meaning.
//
// What the design requires, and nothing more (P-71): the fixed shape, which
// the app validates (the flaw on the section, its passage literal in the
// text, one verdict per flaw); no internal label in a scenario; and per
// scenario: «flaws-two» finds two flaws or more (only the first two are
// recorded as such if it finds more, it is logged); «recheck-still-open»
// says still_open on the uncorrected text.
//
// It runs only with -tags=meta and META_API_KEY set:
//
//	make microtest-judge-verify
//
// No Meta call fails this test: what was saved and what was not is logged,
// with the cost of each call.
package microtest

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"sync"
	"testing"

	"github.com/LaurenteEber/tupaqnin/internal/cycle"
	"github.com/LaurenteEber/tupaqnin/internal/provider"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

const judgeVerifyDir = "../testdata/judge-verify"

// The texts of the cycle tests (round_test.go, judge_test.go).
const (
	judgeRequest  = "Escribe el estado del arte sobre microplásticos en agua potable."
	judgeCriteria = "Solo estudios de campo, no de laboratorio."
	judgeCounts   = "Un muestreo de grifos en Lima."
	judgeNotCount = "Un ensayo con agua sintética."
	judgePremise  = "El agua embotellada trae más partículas que la del grifo."
	judgeSource   = "En 2023 los conteos fueron 60 veces mayores. Los filtros finos retienen más partículas."
	judgeText     = "# Muestra\n\nLos conteos subieron 60 veces en 2023. Revisamos los filtros finos."
	judgeClaim    = "Los conteos subieron 60 veces en 2023."
	judgePassage  = "En 2023 los conteos fueron 60 veces mayores."
)

// judgeRouter sends the judge's calls to Meta, recording the last one.
type judgeRouter struct {
	meta provider.Provider
	mu   sync.Mutex
	last provider.Response
	cost float64
}

func (j *judgeRouter) Respond(ctx context.Context, req provider.Request, onDelta func(provider.Delta)) (provider.Response, error) {
	resp, err := j.meta.Respond(ctx, req, onDelta)
	if err == nil {
		j.mu.Lock()
		j.last, j.cost = resp, j.cost+resp.Usage.CostUSD
		j.mu.Unlock()
	}
	return resp, err
}

func seedJudgeProject(t *testing.T) *store.DB {
	t.Helper()
	ctx := context.Background()
	db, err := store.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := db.Writer.ExecContext(ctx, `INSERT INTO project (id, created_at) VALUES ('p1', '2026-11-01T00:00:00.000Z')`); err != nil {
		t.Fatalf("seed project: %v", err)
	}
	human := store.EventInput{Type: store.EventStateChanged, Actor: "human"}
	edge := store.EventInput{Type: store.EventEdge, Actor: "human"}
	err = db.Write(ctx, "p1", func(tx *store.Tx) error {
		req, err := tx.CreateObject(store.ObjectRequest, "", map[string]any{"text": judgeRequest}, human)
		if err != nil {
			return err
		}
		crit, err := tx.CreateObject(store.ObjectCriterion, "current", map[string]any{"text": judgeCriteria, "request_id": req.ID}, human)
		if err != nil {
			return err
		}
		for _, c := range []struct {
			text   string
			counts bool
		}{{judgeCounts, true}, {judgeNotCount, false}} {
			if _, err := tx.CreateObject(store.ObjectUnderstandingCase, "unconfirmed",
				map[string]any{"text": c.text, "counts": c.counts, "criterion_id": crit.ID}, human); err != nil {
				return err
			}
		}
		if _, err := tx.CreateObject(store.ObjectPremise, "current", map[string]any{"text": judgePremise}, human); err != nil {
			return err
		}
		src, err := tx.CreateObject(store.ObjectSource, "", map[string]any{"kind": "file", "filename": "estudio.md", "text": judgeSource}, human)
		if err != nil {
			return err
		}
		if _, err := tx.CreateObject(store.ObjectDecision, "human", map[string]any{"statement": "Comparar por veces.", "origin": "human"}, human); err != nil {
			return err
		}
		sec, err := tx.CreateObject(store.ObjectSection, "draft", map[string]any{"title": "Muestra", "text": judgeText,
			"role": map[string]any{"value": "other", "state": "confirmed"}, "open_changes": []any{}}, human)
		if err != nil {
			return err
		}
		if _, err := tx.AddEdge(store.EdgeUses, sec.ID, src.ID, "", nil, edge); err != nil {
			return err
		}
		cit, err := tx.CreateObject(store.ObjectCitation, "", map[string]any{"section_id": sec.ID, "anchor": judgeClaim, "passage": judgePassage}, human)
		if err != nil {
			return err
		}
		_, err = tx.AddEdge(store.EdgeComesFrom, cit.ID, src.ID, "", nil, edge)
		return err
	})
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	return db
}

func saveJudgeVerify(t *testing.T, name string, resp provider.Response) {
	t.Helper()
	fx := provider.Fixture{Name: name, TurnKind: provider.TurnJudgeVerify, Deltas: []string{}, Items: resp.RawItems, Usage: resp.Usage}
	b, err := json.MarshalIndent(fx, "", "  ")
	if err != nil {
		t.Logf("%s: marshal: %v", name, err)
		return
	}
	if err := os.WriteFile(filepath.Join(judgeVerifyDir, name+".json"), b, 0o644); err != nil {
		t.Logf("%s: write: %v", name, err)
		return
	}
	t.Logf("recording saved: %s", name)
}

var judgeInternalLabel = regexp.MustCompile(`\b(?:sec|dec|src|crit|prem|case|rnd|hit)-[0-9]+\b`)

func TestMicroTestJudgeVerify(t *testing.T) {
	apiKey := os.Getenv("META_API_KEY")
	if apiKey == "" {
		t.Fatal("META_API_KEY is required: run `make microtest-judge-verify`, which reads it from the keychain")
	}
	ctx := context.Background()
	db := seedJudgeProject(t)
	router := &judgeRouter{meta: provider.NewMeta(apiKey, nil, nil)}
	runner := cycle.NewRunner(db, router, nil, nil)
	defer runner.Close()

	label, err := runner.RunJudgeVerify(ctx, "p1", "sec-1", cycle.JudgeReasonRisk, []string{"figures_and_calculations"})
	if err != nil {
		t.Logf("flaws-two: judge: %v; not saved", err)
		return
	}
	round, err := db.GetObject(ctx, "p1", label)
	if err != nil {
		t.Logf("flaws-two: %v", err)
		return
	}
	var body cycle.RoundBody
	_ = json.Unmarshal(round.Body, &body)
	t.Logf("flaws-two: cost %.4f USD; status %q, %d flaws; last raw text: %s", router.cost, body.JudgeStatus, len(body.Flaws), router.last.Text)
	labelled := false
	for _, f := range body.Flaws {
		labelled = labelled || judgeInternalLabel.MatchString(f.Scenario)
	}
	switch {
	case body.JudgeStatus != cycle.JudgeDone:
		t.Logf("flaws-two: not saved (the hand-made one stays)")
	case len(body.Flaws) < 2:
		t.Logf("flaws-two: fewer than two flaws; not saved (the hand-made one stays)")
	case labelled:
		t.Logf("flaws-two: an internal label in a scenario; not saved (the hand-made one stays)")
	default:
		saveJudgeVerify(t, "flaws-two", router.last)
	}
	if len(body.Flaws) == 0 || round.State != "open" {
		t.Logf("recheck-still-open: no flaws to recheck; not saved")
		return
	}

	// The recheck on the uncorrected text: its flaws should stay open.
	spent := router.cost
	if err := runner.RunJudgeRecheck(ctx, "p1", label); err != nil {
		t.Logf("recheck-still-open: judge: %v; not saved", err)
		return
	}
	round, err = db.GetObject(ctx, "p1", label)
	if err != nil {
		t.Logf("recheck-still-open: %v", err)
		return
	}
	body = cycle.RoundBody{}
	_ = json.Unmarshal(round.Body, &body)
	t.Logf("recheck-still-open: cost %.4f USD; status %q; last raw text: %s", router.cost-spent, body.JudgeStatus, router.last.Text)
	open := false
	for _, f := range body.Flaws {
		open = open || f.Verdict == cycle.FlawStillOpen
	}
	if body.JudgeStatus == cycle.JudgeDone && open {
		saveJudgeVerify(t, "recheck-still-open", router.last)
	} else {
		t.Logf("recheck-still-open: no still_open; not saved (the hand-made one stays)")
	}
}
