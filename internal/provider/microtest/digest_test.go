//go:build meta

// Recordings of real digest outputs for the digest tests (backlog-r3
// 3.4): each scenario seeds the round the cycle tests build
// (internal/cycle/digest_test.go: the same request, criterion, premise,
// sources and verified findings), runs the real digest (cycle.Runner.
// Digest: the embedded instructions and salida-digesto, its one retry)
// against Meta, and saves the digest call's output under
// internal/provider/testdata/digest/ only when it is what the scenario
// needs, so the tests keep their meaning. The checker the digest calls
// afterwards answers from the fake (no Meta call).
//
// What the design requires, and nothing more (P-71): the fixed shape,
// which the app validates (findings in the list, the contrast because
// the round has an expectation); no internal label in what the person
// reads (answer, outline, contrast, about); and per scenario: in
// «discarded-unnamed» the outline uses finding 1; in
// «conflict-two-sources» the contradiction between findings 1 and 2 is
// listed (R-9: never resolved in silence).
//
// It runs only with -tags=meta and META_API_KEY set:
//
//	make microtest-digest
//
// No Meta call fails this test: what was saved and what was not is
// logged, with the cost of each digest call.
package microtest

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/LaurenteEber/tupaqnin/internal/cycle"
	"github.com/LaurenteEber/tupaqnin/internal/provider"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

const digestDir = "../testdata/digest"

// The texts of the cycle tests (round_test.go, digest_test.go).
const (
	digestRequest  = "Escribe el estado del arte sobre microplásticos en agua potable."
	digestCriteria = "Solo estudios de campo, no de laboratorio."
	digestPremise  = "El agua embotellada trae más partículas que la del grifo."
	digestSource1  = "En 2023 los conteos fueron 60 veces mayores. Los filtros finos retienen más partículas."
	digestSource2  = "Un segundo estudio. En 2023 los conteos fueron similares en ambos tipos de agua."
	digestPassage1 = "En 2023 los conteos fueron 60 veces mayores."
	digestPassage2 = "En 2023 los conteos fueron similares en ambos tipos de agua."
)

// digestRouter sends the digest's calls to Meta, recording the last
// one, and every other call (the checker's) to the fake.
type digestRouter struct {
	meta provider.Provider
	fake provider.Provider
	mu   sync.Mutex
	last provider.Response
	cost float64
}

func (d *digestRouter) Respond(ctx context.Context, req provider.Request, onDelta func(provider.Delta)) (provider.Response, error) {
	if req.TurnKind != provider.TurnDigest {
		return d.fake.Respond(ctx, req, onDelta)
	}
	resp, err := d.meta.Respond(ctx, req, onDelta)
	if err == nil {
		d.mu.Lock()
		d.last, d.cost = resp, d.cost+resp.Usage.CostUSD
		d.mu.Unlock()
	}
	return resp, err
}

// seedDigestRound seeds the project and a round whose readers returned
// these findings, already verified as entered on their project source.
func seedDigestRound(t *testing.T, findings [][2]string) *store.DB {
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
		req, err := tx.CreateObject(store.ObjectRequest, "", map[string]any{"text": digestRequest}, human)
		if err != nil {
			return err
		}
		if _, err := tx.CreateObject(store.ObjectCriterion, "current", map[string]any{"text": digestCriteria, "request_id": req.ID}, human); err != nil {
			return err
		}
		if _, err := tx.CreateObject(store.ObjectPremise, "current", map[string]any{"text": digestPremise}, human); err != nil {
			return err
		}
		ids := map[string]string{}
		s1, err := tx.CreateObject(store.ObjectSource, "", map[string]any{"kind": "file", "filename": "estudio.md", "text": digestSource1}, human)
		if err != nil {
			return err
		}
		s2, err := tx.CreateObject(store.ObjectSource, "", map[string]any{"kind": "file", "filename": "segundo.md", "text": digestSource2}, human)
		if err != nil {
			return err
		}
		ids["src-1"], ids["src-2"] = s1.ID, s2.ID
		body := cycle.RoundBody{Kind: "research", Question: "¿Qué tamaño mínimo detecta cada método?",
			Doubt: "Si los estudios con distinto tamaño mínimo se pueden comparar.", DelegationReason: "independent_parts",
			Scope: "Literatura publicada sobre métodos de detección.", Expectation: digestPremise,
			TurnID: "digest-microtest", Hits: []cycle.HitRecord{}}
		for i, f := range findings {
			part := i + 1
			body.Parts = append(body.Parts, cycle.RoundPart{Question: "¿Qué dicen las cifras?", Where: []string{"project"},
				Queries: []string{}, Sources: []cycle.RoundPartSource{{SourceID: ids[f[1]], WhatFor: "Las cifras."}}})
			out, _ := json.Marshal(map[string]any{"findings": []map[string]any{{"kind": "fact", "passage": f[0],
				"source_id": f[1], "hit": nil, "reference": nil, "note": nil}}, "covered": []string{}, "sources_read": []string{f[1]}})
			body.Readers = append(body.Readers, cycle.ReaderResult{Part: part, Status: cycle.ReaderReturned, Output: out})
			body.Findings = append(body.Findings, cycle.FindingCheck{Part: part, Finding: 1,
				Verification: cycle.VerificationEntered, SourceID: ids[f[1]]})
		}
		_, err = tx.CreateObject(store.ObjectRound, "open", body, human)
		return err
	})
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	return db
}

// labelsInDigest are the internal labels in what the person reads.
func labelsInDigest(d *cycle.DigestOutput) []string {
	texts := append([]string{d.Answer}, d.Outline...)
	if d.ExpectationContrast != nil {
		texts = append(texts, *d.ExpectationContrast)
	}
	for _, c := range d.Contradictions {
		texts = append(texts, c.About)
	}
	var got []string
	for _, s := range texts {
		got = append(got, internalLabel.FindAllString(s, -1)...)
	}
	return got
}

func saveDigest(t *testing.T, name string, resp provider.Response) {
	t.Helper()
	fx := provider.Fixture{Name: name, TurnKind: provider.TurnDigest, Deltas: []string{}, Items: resp.RawItems, Usage: resp.Usage}
	b, err := json.MarshalIndent(fx, "", "  ")
	if err != nil {
		t.Logf("%s: marshal: %v", name, err)
		return
	}
	if err := os.WriteFile(filepath.Join(digestDir, name+".json"), b, 0o644); err != nil {
		t.Logf("%s: write: %v", name, err)
		return
	}
	t.Logf("recording saved: %s", name)
}

func TestMicroTestDigest(t *testing.T) {
	apiKey := os.Getenv("META_API_KEY")
	if apiKey == "" {
		t.Fatal("META_API_KEY is required: run `make microtest-digest`, which reads it from the keychain")
	}
	meta := provider.NewMeta(apiKey, nil, nil)
	scenarios := []struct {
		name     string
		findings [][2]string
		ok       func(d *cycle.DigestOutput) string
	}{
		{"discarded-unnamed", [][2]string{{digestPassage1, "src-1"}}, func(d *cycle.DigestOutput) string {
			for _, f := range d.Findings {
				if f.Finding == 1 {
					return ""
				}
			}
			return "the outline does not use finding 1"
		}},
		{"conflict-two-sources", [][2]string{{digestPassage1, "src-1"}, {digestPassage2, "src-2"}}, func(d *cycle.DigestOutput) string {
			for _, c := range d.Contradictions {
				if c.FindingA+c.FindingB == 3 && c.FindingA != c.FindingB {
					return ""
				}
			}
			return "the contradiction between findings 1 and 2 is not listed"
		}},
	}
	for _, sc := range scenarios {
		db := seedDigestRound(t, sc.findings)
		fake := provider.NewFake()
		router := &digestRouter{meta: meta, fake: fake}
		count := func(ctx context.Context, input []json.RawMessage) (int, error) {
			return fake.CountTokens(ctx, provider.PrincipalModel, input)
		}
		runner := cycle.NewRunner(db, router, count, nil)
		err := runner.Digest(context.Background(), "p1", "rnd-1")
		runner.Close()
		round, gerr := db.GetObject(context.Background(), "p1", "rnd-1")
		var body cycle.RoundBody
		if gerr == nil {
			_ = json.Unmarshal(round.Body, &body)
		}
		t.Logf("%s: digest calls cost %.4f USD; last raw text: %s", sc.name, router.cost, router.last.Text)
		switch {
		case err != nil:
			t.Logf("%s: digest: %v; not saved", sc.name, err)
		case body.DigestStatus != cycle.DigestDone || body.Digest == nil:
			t.Logf("%s: digest status %q; not saved (the hand-made one stays)", sc.name, body.DigestStatus)
		case len(labelsInDigest(body.Digest)) > 0:
			t.Logf("%s: internal labels %v; not saved (the hand-made one stays)", sc.name, labelsInDigest(body.Digest))
		case sc.ok(body.Digest) != "":
			t.Logf("%s: %s; not saved (the hand-made one stays)", sc.name, sc.ok(body.Digest))
		default:
			if errs, _ := cycle.ValidateDigest(router.last.Text, len(sc.findings), true); len(errs) > 0 {
				t.Logf("%s: the last call is not the accepted one (%s); not saved", sc.name, strings.Join(errs, "; "))
				continue
			}
			saveDigest(t, sc.name, router.last)
		}
	}
}
