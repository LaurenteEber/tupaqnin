//go:build meta

// Recording of a real judge output for the defense tests (backlog-r2
// 2.6): the scenario seeds the project the cycle tests seed (seedDefense
// in internal/cycle/defense_test.go, with sec-1 as the step leaves it),
// runs the real judge call (cycle.CallJudgeDefense: the embedded
// instructions and schema, its one retry) against Meta, and saves the
// output under internal/provider/testdata/judge/ only when it is what
// the design requires of the judge, so the tests keep their meaning.
//
// What the design requires, and nothing more (P-71): the fixed shape of
// salida-juez-defensa.schema.json (3 to 5 questions, each «no cubre»
// with its statement) with every entry on the section in question
// (fichas §7), which the call itself validates; and that no question or
// statement carries an internal label (fichas §8.2: written for the
// reader, never «según dec-4»). The design fixes no wording.
//
// It runs only with -tags=meta and META_API_KEY set:
//
//	make microtest-judge-defense
//
// No Meta call fails this test: what was saved and what was not is
// logged, with what Meta answered on a mismatch.
package microtest

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/LaurenteEber/tupaqnin/internal/cycle"
	"github.com/LaurenteEber/tupaqnin/internal/provider"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

const judgeDir = "../testdata/judge"

// The section text as the cycle tests' step leaves it (defenseText with
// defenseStep applied).
const judgeSectionText = "Comparamos solo estudios con el mismo tamaño de partícula.\n\nAsí la comparación es justa y repetible."

var internalLabel = regexp.MustCompile(`\b[a-z]{2,5}-[0-9]+\b`)

// seedJudge is the cycle tests' defense project: req-1 with the audience
// given, crit-1, sec-1 with the role given, and dec-1 (yours) used in it.
func seedJudge(t *testing.T, audience, role string) *store.DB {
	t.Helper()
	ctx := context.Background()
	db, err := store.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := db.Writer.ExecContext(ctx, `INSERT INTO project (id, created_at) VALUES ('p1', '2026-10-31T00:00:00.000Z')`); err != nil {
		t.Fatalf("seed project: %v", err)
	}
	human := store.EventInput{Type: store.EventStateChanged, Actor: "human"}
	err = db.Write(ctx, "p1", func(tx *store.Tx) error {
		req, err := tx.CreateObject(store.ObjectRequest, "", map[string]any{
			"text": "Escribe la regla de comparación de los estudios.", "audience": audience}, human)
		if err != nil {
			return err
		}
		if _, err := tx.CreateObject(store.ObjectCriterion, "current",
			map[string]any{"text": "Comparar conteos de partículas.", "request_id": req.ID}, human); err != nil {
			return err
		}
		sec, err := tx.CreateObject(store.ObjectSection, "draft", map[string]any{
			"title": "Regla de comparación", "text": judgeSectionText,
			"role": map[string]any{"value": role, "state": "confirmed"}, "open_changes": []any{},
		}, human)
		if err != nil {
			return err
		}
		dec, err := tx.CreateObject(store.ObjectDecision, "human", map[string]any{
			"statement": "Comparar solo estudios con el mismo tamaño de partícula.", "origin": "human",
			"instead_of": "Comparar los conteos tal cual.",
		}, human)
		if err != nil {
			return err
		}
		_, err = tx.AddEdge(store.EdgeUsedIn, dec.ID, sec.ID, "", nil, store.EventInput{Type: store.EventEdge, Actor: "human"})
		return err
	})
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	return db
}

func saveJudge(t *testing.T, r *recorder, name string) {
	t.Helper()
	r.mu.Lock()
	defer r.mu.Unlock()
	fx := provider.Fixture{Name: name, TurnKind: provider.TurnJudgeDefense, Deltas: r.deltas, Items: r.resp.RawItems, Usage: r.resp.Usage}
	if n := len(r.deltas); n > 1 {
		fx.DeltaDelay = r.gap / time.Duration(n-1)
	}
	b, err := json.MarshalIndent(fx, "", "  ")
	if err != nil {
		t.Logf("%s: marshal: %v", name, err)
		return
	}
	if err := os.WriteFile(filepath.Join(judgeDir, name+".json"), b, 0o644); err != nil {
		t.Logf("%s: write: %v", name, err)
		return
	}
	t.Logf("recording saved: %s", name)
}

// labelsIn are the internal labels a question or statement carries.
func labelsIn(out cycle.DefenseOutput) []string {
	var got []string
	for _, q := range out.AudienceQuestions {
		got = append(got, internalLabel.FindAllString(q.Question, -1)...)
	}
	for _, n := range out.NotCovered {
		got = append(got, internalLabel.FindAllString(n.Statement, -1)...)
	}
	return got
}

// logDefense shows what Meta answered when it is not what the design
// requires: each question and statement with its section, and the
// errors of each failed attempt.
func logDefense(t *testing.T, name string, db *store.DB, r *recorder, out cycle.DefenseOutput) {
	t.Helper()
	var b strings.Builder
	fmt.Fprintf(&b, "%s: what Meta answered\n", name)
	for _, q := range out.AudienceQuestions {
		fmt.Fprintf(&b, "  question on %s: «%s»\n", q.Section, q.Question)
	}
	for _, n := range out.NotCovered {
		fmt.Fprintf(&b, "  not covered on %s: «%s»\n", n.Section, n.Statement)
	}
	if events, err := db.EventsSince(context.Background(), "p1", 0); err == nil {
		for _, ev := range events {
			if ev.Type == store.EventOutputRetry {
				fmt.Fprintf(&b, "  output_retry: %s\n", ev.Data)
			}
		}
	}
	r.mu.Lock()
	fmt.Fprintf(&b, "  last raw text: %s\n", r.resp.Text)
	r.mu.Unlock()
	t.Log(b.String())
}

func TestMicroTestJudgeDefense(t *testing.T) {
	apiKey := os.Getenv("META_API_KEY")
	if apiKey == "" {
		t.Fatal("META_API_KEY is required: run `make microtest-judge-defense`, which reads it from the keychain")
	}
	meta := provider.NewMeta(apiKey, nil, nil)

	// A section with role conclusion and no audience (B-38: its role asks
	// for the defense), as TestK1_ConclusionSection… seeds it.
	const name = "conclusion-section"
	db := seedJudge(t, "", "conclusion")
	rec := &recorder{inner: meta}
	out, ok, err := cycle.CallJudgeDefense(context.Background(), rec, db, "p1", "rec", "sec-1")
	switch {
	case err != nil:
		t.Logf("%s: judge call: %v; not saved", name, err)
	case !ok:
		t.Logf("%s: the judge failed its shape twice; not saved (the hand-made one stays)", name)
		logDefense(t, name, db, rec, out)
	case len(labelsIn(out)) > 0:
		t.Logf("%s: the judge wrote internal labels %v; not saved (the hand-made one stays)", name, labelsIn(out))
		logDefense(t, name, db, rec, out)
	default:
		saveJudge(t, rec, name)
	}
}
