//go:build meta

// Recordings of real checker outputs for the checker tests (backlog-r2
// 2.3, P-68): each scenario seeds the project the cycle tests seed
// (seedCheckerProject), writes the scenario's passage into sec-1 as the
// step does, runs the real checker call (checks.Probes, the embedded
// checker instructions and schema, its one retry) against Meta, and
// saves the output under internal/provider/testdata/checker/ only when
// it is the judgment the scenario needs, so the tests keep their
// meaning.
//
// It runs only with -tags=meta and META_API_KEY set:
//
//	make microtest-checker
//
// No Meta call fails this test: what was saved and what was not is
// logged. malformed.json and source-not-supporting.json stay hand-made:
// the checker is not asked to break its shape, and the citation's pair
// needs the principal's step.
package microtest

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/LaurenteEber/tupaqnin/internal/checks"
	"github.com/LaurenteEber/tupaqnin/internal/cycle"
	"github.com/LaurenteEber/tupaqnin/internal/provider"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

const checkerDir = "../testdata/checker"

// The seeded section's text, and the premise (cycle tests, apply_test.go
// and checker_test.go).
const (
	checkerSectionText = "La síntesis compara estudios. Los estudios usan filtros. Cierre."
	checkerPremise     = "El agua embotellada es más segura que la del grifo."
)

// saveChecker writes the recorder's last call as a checker recording.
func saveChecker(t *testing.T, r *recorder, name string) {
	t.Helper()
	r.mu.Lock()
	defer r.mu.Unlock()
	fx := provider.Fixture{Name: name, TurnKind: provider.TurnChecker, Deltas: r.deltas, Items: r.resp.RawItems, Usage: r.resp.Usage}
	if n := len(r.deltas); n > 1 {
		fx.DeltaDelay = r.gap / time.Duration(n-1)
	}
	b, err := json.MarshalIndent(fx, "", "  ")
	if err != nil {
		t.Logf("%s: marshal: %v", name, err)
		return
	}
	if err := os.WriteFile(filepath.Join(checkerDir, name+".json"), b, 0o644); err != nil {
		t.Logf("%s: write: %v", name, err)
		return
	}
	t.Logf("recording saved: %s", name)
}

// seedChecker is the cycle tests' project: req-1, crit-1, sec-1, src-1
// (read), dec-1 (yours), dec-2 (the agent's, used in sec-1) and prem-1.
func seedChecker(t *testing.T) *store.DB {
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
		req, err := tx.CreateObject(store.ObjectRequest, "", map[string]any{"text": "Escribe la síntesis."}, human)
		if err != nil {
			return err
		}
		if _, err := tx.CreateObject(store.ObjectCriterion, "current",
			map[string]any{"text": "Comparar conteos.", "request_id": req.ID}, human); err != nil {
			return err
		}
		sec, err := tx.CreateObject(store.ObjectSection, "draft", map[string]any{
			"title": "Síntesis", "text": checkerSectionText,
			"role": map[string]any{"value": "other", "state": "confirmed"}, "open_changes": []any{},
		}, human)
		if err != nil {
			return err
		}
		src, err := tx.CreateObject(store.ObjectSource, "", map[string]any{
			"kind": "file", "filename": "estudio.md",
			"text": "En 2023 los conteos fueron 60 veces mayores. Los filtros finos retienen más partículas.",
		}, human)
		if err != nil {
			return err
		}
		if err := tx.RecordDelivery(store.Delivery{Via: store.ViaRead, ObjectID: src.ID, Version: src.CurrentVersion}); err != nil {
			return err
		}
		if _, err := tx.CreateObject(store.ObjectDecision, "human", map[string]any{
			"statement": "Usar el informe del comité.", "origin": "human",
		}, human); err != nil {
			return err
		}
		dec, err := tx.CreateObject(store.ObjectDecision, "agent", map[string]any{
			"statement": "Comparar por promedio.", "instead_of": "Comparar por mediana.",
			"would_choose_if": "Hubiera valores extremos.", "assumption": false, "origin": "agent",
		}, store.EventInput{Type: store.EventStateChanged, Actor: "agent"})
		if err != nil {
			return err
		}
		if _, err := tx.AddEdge(store.EdgeUsedIn, dec.ID, sec.ID, "", nil, store.EventInput{Type: store.EventEdge, Actor: "agent"}); err != nil {
			return err
		}
		_, err = tx.CreateObject(store.ObjectPremise, "current",
			map[string]any{"text": checkerPremise, "request_id": req.ID, "exploratory": false}, human)
		return err
	})
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	return db
}

// judge writes passage in place of «Cierre.» of sec-1, as the cycle
// tests' step does, and runs the real checker on that change. ok is
// false when the checker failed its shape twice or the call failed.
func judge(t *testing.T, meta provider.Provider, passage string) (*recorder, checks.CheckerOutput, bool) {
	t.Helper()
	ctx := context.Background()
	db := seedChecker(t)
	var change checks.Change
	err := db.Write(ctx, "p1", func(tx *store.Tx) error {
		sec, err := tx.GetObject(ctx, "p1", "sec-1")
		if err != nil {
			return err
		}
		body := map[string]any{}
		_ = json.Unmarshal(sec.Body, &body)
		body["text"] = strings.Replace(checkerSectionText, "Cierre.", passage, 1)
		if _, err := tx.UpdateObject(sec.ID, sec.CurrentVersion, sec.State, body,
			store.EventInput{Type: store.EventEdit, Actor: "agent"}); err != nil {
			return err
		}
		change = checks.Change{Kind: checks.AgentStep, Sections: []string{sec.ID},
			Passages: []checks.Passage{{Section: sec.ID, Text: passage}}, Actor: "agent", TurnID: "rec"}
		return nil
	})
	if err != nil {
		t.Fatalf("write passage: %v", err)
	}
	probes, err := checks.Probes(ctx, db, "p1", change)
	if err != nil {
		t.Fatalf("Probes: %v", err)
	}
	rec := &recorder{inner: meta}
	out, ok, err := cycle.NewChecker(rec).CallChecker(ctx, db, "p1", "rec", change, probes)
	if err != nil {
		t.Logf("checker call: %v", err)
		return rec, out, false
	}
	if !ok {
		t.Logf("checker failed its shape twice: not saved")
	}
	return rec, out, ok
}

func verdictOf(out checks.CheckerOutput, protected string) string {
	for _, p := range out.Probes {
		if p.Protected == protected {
			return p.Verdict
		}
	}
	return ""
}

// onlyVerdict: protected has verdict, every other probe «no».
func onlyVerdict(out checks.CheckerOutput, protected, verdict string) bool {
	for _, p := range out.Probes {
		if (p.Protected == protected) != (p.Verdict == verdict) {
			return false
		}
	}
	return verdictOf(out, protected) == verdict
}

// allNo: every probe «no».
func allNo(out checks.CheckerOutput) bool {
	for _, p := range out.Probes {
		if p.Verdict != checks.VerdictNo {
			return false
		}
	}
	return len(out.Probes) > 0
}

// candidateTypes are the candidates' types in order, each with its new
// passage.
func candidateTypes(out checks.CheckerOutput) []string {
	var got []string
	for _, c := range out.Candidates {
		got = append(got, c.Type+":"+c.NewPassage)
	}
	return got
}

// contradiction: one contradiction candidate on protected in sec-1,
// with a question and the options of B-44. What the design asks and
// nothing more (P-71): its new passage is literal in what was written
// (fichas §3: «pasajes literales»), the question is not empty (the
// design fixes no wording), and there are two options or more (B-44).
// One candidate only is the scenario's: the cycle tests read oq-1 as
// this one.
func contradiction(out checks.CheckerOutput, protected, written string) bool {
	if len(out.Candidates) != 1 {
		return false
	}
	c := out.Candidates[0]
	return c.Type == "contradiction" && c.Section == "sec-1" &&
		strings.TrimSpace(c.NewPassage) != "" && strings.Contains(written, c.NewPassage) &&
		c.Protected != nil && *c.Protected == protected &&
		c.Question != nil && strings.TrimSpace(*c.Question) != "" && len(c.Options) >= 2
}

// logJudgment shows what Meta answered when it is not the scenario's
// (P-71): the verdict of each probe, and the type, passage and number of
// options of each candidate.
func logJudgment(t *testing.T, name string, out checks.CheckerOutput) {
	t.Helper()
	var b strings.Builder
	fmt.Fprintf(&b, "%s: what Meta answered\n", name)
	for _, p := range out.Probes {
		passage := ""
		if p.Passage != nil {
			passage = *p.Passage
		}
		fmt.Fprintf(&b, "  probe %s: %s «%s»\n", p.Protected, p.Verdict, passage)
	}
	if len(out.Candidates) == 0 {
		b.WriteString("  no candidates\n")
	}
	for _, c := range out.Candidates {
		protected := ""
		if c.Protected != nil {
			protected = " on " + *c.Protected
		}
		fmt.Fprintf(&b, "  candidate %s%s in %s: «%s» · %d options\n", c.Type, protected, c.Section, c.NewPassage, len(c.Options))
	}
	t.Log(b.String())
}

func keepChecker(t *testing.T, rec *recorder, name string, out checks.CheckerOutput, ok bool) {
	t.Helper()
	if ok {
		saveChecker(t, rec, name)
		return
	}
	t.Logf("%s: the judgment is not the scenario's; not saved (the hand-made one stays)", name)
	logJudgment(t, name, out)
}

// TestMicroTestCheckerExceeds records exceeds_reviewed (backlog-r3 3.5,
// B-68): sec-1 uses src-1, of a round whose inventory says «2 of 5 results
// unread», and the passage says everything was reviewed. It replaces the
// hand-made exceeds-reviewed.json only when the checker marks that. It runs
// with `make microtest-checker-exceeds`.
func TestMicroTestCheckerExceeds(t *testing.T) {
	apiKey := os.Getenv("META_API_KEY")
	if apiKey == "" {
		t.Fatal("META_API_KEY is required: run `make microtest-checker-exceeds`, which reads it from the keychain")
	}
	meta := provider.NewMeta(apiKey, nil, nil)
	ctx := context.Background()
	db := seedChecker(t)
	const passage = "Se revisaron todos los estudios."
	var change checks.Change
	err := db.Write(ctx, "p1", func(tx *store.Tx) error {
		sec, err := tx.GetObject(ctx, "p1", "sec-1")
		if err != nil {
			return err
		}
		src, err := tx.GetObject(ctx, "p1", "src-1")
		if err != nil {
			return err
		}
		body := map[string]any{}
		_ = json.Unmarshal(sec.Body, &body)
		body["text"] = strings.Replace(checkerSectionText, "Cierre.", passage, 1)
		if _, err := tx.UpdateObject(sec.ID, sec.CurrentVersion, sec.State, body,
			store.EventInput{Type: store.EventEdit, Actor: "agent"}); err != nil {
			return err
		}
		app := store.EventInput{Type: store.EventStateChanged, Actor: "app"}
		if _, err := tx.AddEdge(store.EdgeUses, sec.ID, src.ID, "", nil, store.EventInput{Type: store.EventEdge, Actor: "app"}); err != nil {
			return err
		}
		if _, err := tx.CreateObject(store.ObjectRound, "closed", map[string]any{"kind": "research",
			"question": "¿Qué conteos comparan los estudios?",
			"parts":    []map[string]any{{"sources": []map[string]any{{"source_id": src.ID}}}},
			"hits":     []map[string]any{{"label": "hit-4", "title": "Conteos en Lima"}, {"label": "hit-5", "title": "Conteos en Quito"}},
			"coverage_inventory": []map[string]any{{"part": 1, "status": "returned", "sources": []string{src.ID}, "sources_read": []string{src.ID},
				"hits": []string{"hit-1", "hit-2", "hit-3", "hit-4", "hit-5"}, "hits_read": []string{"hit-1", "hit-2", "hit-3"}, "gaps": []string{}}}}, app); err != nil {
			return err
		}
		change = checks.Change{Kind: checks.AgentStep, Sections: []string{sec.ID},
			Passages: []checks.Passage{{Section: sec.ID, Text: passage}}, Actor: "agent", TurnID: "rec"}
		return nil
	})
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	probes, err := checks.Probes(ctx, db, "p1", change)
	if err != nil {
		t.Fatalf("Probes: %v", err)
	}
	rec := &recorder{inner: meta}
	out, ok, err := cycle.NewChecker(rec).CallChecker(ctx, db, "p1", "rec", change, probes)
	if err != nil || !ok {
		t.Logf("checker call: ok %v, %v: not saved", ok, err)
		return
	}
	got := candidateTypes(out)
	keepChecker(t, rec, "exceeds-reviewed", out, allNo(out) && len(got) == 1 && got[0] == "exceeds_reviewed:"+passage)
}

func TestMicroTestChecker(t *testing.T) {
	apiKey := os.Getenv("META_API_KEY")
	if apiKey == "" {
		t.Fatal("META_API_KEY is required: run `make microtest-checker`, which reads it from the keychain")
	}
	meta := provider.NewMeta(apiKey, nil, nil)

	// A contradiction of the premise (TestB2x7_ContradictsOnThePremise…).
	const tap = "El agua del grifo es igual de segura que la embotellada."
	if rec, out, ok := judge(t, meta, tap); ok {
		keepChecker(t, rec, "contradicts-premise", out,
			onlyVerdict(out, "prem-1", checks.VerdictContradicts) && contradiction(out, "prem-1", tap))
	}

	// The same passage judged «doubt» (TestB2x7_DoubtAlsoOpens…): saved
	// only if the checker doubts instead of contradicting.
	if rec, out, ok := judge(t, meta, tap); ok {
		keepChecker(t, rec, "doubt-premise", out,
			onlyVerdict(out, "prem-1", checks.VerdictDoubt) && contradiction(out, "prem-1", tap))
	}

	// A contradiction of your decision dec-1 (TestP68_…).
	const thesis = "Usamos solo datos de tesis, no el informe del comité."
	if rec, out, ok := judge(t, meta, thesis); ok {
		keepChecker(t, rec, "contradicts-decision", out,
			onlyVerdict(out, "dec-1", checks.VerdictContradicts) && contradiction(out, "dec-1", thesis))
	}

	// «Relato del proceso» (TestB15_ProcessNarrative…).
	const narrative = "Tras la revisión del martes, ajustamos el filtro."
	if rec, out, ok := judge(t, meta, narrative); ok {
		got := candidateTypes(out)
		keepChecker(t, rec, "process-narrative", out, allNo(out) && len(got) == 1 && got[0] == "process_narrative:"+narrative)
	}

	// Two candidates, one of which leaves the text while the checker
	// answers (TestB50_CandidateWhosePassageLeftTheText…).
	const kept = "Usamos solo filtros finos."
	if rec, out, ok := judge(t, meta, narrative+" "+kept); ok {
		got := strings.Join(candidateTypes(out), " | ")
		keepChecker(t, rec, "discarded-passage", out, allNo(out) &&
			got == "process_narrative:"+narrative+" | written_without_decision:"+kept)
	}
}
