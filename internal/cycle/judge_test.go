package cycle_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/LaurenteEber/tupaqnin/internal/cycle"
	"github.com/LaurenteEber/tupaqnin/internal/graph"
	"github.com/LaurenteEber/tupaqnin/internal/provider"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// Backlog-r3 3.6: the judge verifies a section with one correction batch
// and one recheck (fichas §2 «Verificar», §7; bloque-1 §4; B-67; K-3, K-4).

const (
	judgedText = "# Muestra\n\nLos conteos subieron 60 veces en 2023. Revisamos los filtros finos."
	flawOne    = "Los conteos subieron 60 veces en 2023."
	flawTwo    = "Revisamos los filtros finos."
)

// riskStep writes a new section, in one step whose closing risk is risk,
// without any pending of the agent. The figure is out of the text so no
// rule leaves a pending.
func riskStep(risk string) string {
	raw, _ := json.Marshal(map[string]any{"stretch": "build", "triage_line": "Construir: escribo la muestra.",
		"closing_risk": risk, "note": "Escribí la muestra.",
		"new_text": []map[string]any{{"section_id": "new-1", "base_version": 0, "anchor": "", "claim_origin": "model_knowledge",
			"operation": "insert_after", "text": "# Muestra\n\nLa muestra cubre la cuenca del Rímac."}},
		"proposed_roles": []map[string]any{{"section_id": "new-1", "role": "other", "title": "Muestra"}}})
	return string(raw)
}

// judgeFixture is a recording of one judge's call.
func judgeVerifyFixture(t *testing.T, out map[string]any) provider.Fixture {
	t.Helper()
	text, err := json.Marshal(out)
	if err != nil {
		t.Fatal(err)
	}
	fx := textFixture(t, string(text))
	fx.TurnKind, fx.Deltas = provider.TurnJudgeVerify, nil
	return fx
}

func oneFlaw(section, passage, risk string) map[string]any {
	return map[string]any{"section": section, "passage": passage, "risk": risk,
		"scenario": "1. El destinatario pide la fuente. 2. No aparece."}
}

func verifyFlaws(t *testing.T, list ...map[string]any) provider.Fixture {
	return judgeVerifyFixture(t, map[string]any{"flaws": list, "recheck": []any{}})
}

func verifyRecheck(t *testing.T, list ...map[string]any) provider.Fixture {
	return judgeVerifyFixture(t, map[string]any{"flaws": []any{}, "recheck": list})
}

func flawStillOpen(n int, passage string) map[string]any {
	return map[string]any{"flaw": n, "verdict": "still_open", "passage": passage}
}

func flawResolved(n int) map[string]any {
	return map[string]any{"flaw": n, "verdict": "resolved", "passage": nil}
}

// seedJudged creates the round_test project (request, criterion with its
// cases, premise, src-1, dec-1) and sec-1 with judgedText and a citation
// on src-1.
func seedJudged(t *testing.T, db *store.DB) {
	t.Helper()
	seedRound(t, db, false)
	ctx := context.Background()
	human := store.EventInput{Type: store.EventStateChanged, Actor: "human"}
	edge := store.EventInput{Type: store.EventEdge, Actor: "human"}
	err := db.Write(ctx, "p1", func(tx *store.Tx) error {
		sec, err := tx.CreateObject(store.ObjectSection, "draft", map[string]any{"title": "Muestra", "text": judgedText,
			"role": map[string]any{"value": "other", "state": "confirmed"}, "open_changes": []any{}}, human)
		if err != nil {
			return err
		}
		src, err := tx.GetObject(ctx, "p1", "src-1")
		if err != nil {
			return err
		}
		if _, err := tx.AddEdge(store.EdgeUses, sec.ID, src.ID, "", nil, edge); err != nil {
			return err
		}
		cit, err := tx.CreateObject(store.ObjectCitation, "", map[string]any{"section_id": sec.ID, "anchor": flawOne,
			"passage": "En 2023 los conteos fueron 60 veces mayores."}, human)
		if err != nil {
			return err
		}
		_, err = tx.AddEdge(store.EdgeComesFrom, cit.ID, src.ID, "", nil, edge)
		return err
	})
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
}

func judgeRounds(t *testing.T, db *store.DB) []store.Object {
	t.Helper()
	all, err := db.ListObjects(context.Background(), "p1", store.ObjectRound)
	if err != nil {
		t.Fatal(err)
	}
	out := []store.Object{}
	for _, o := range all {
		var b cycle.RoundBody
		if json.Unmarshal(o.Body, &b) == nil && b.Kind == "judge" {
			out = append(out, o)
		}
	}
	return out
}

func judgeBody(t *testing.T, o store.Object) cycle.RoundBody {
	t.Helper()
	var b cycle.RoundBody
	if err := json.Unmarshal(o.Body, &b); err != nil {
		t.Fatal(err)
	}
	return b
}

func itemsText(items []json.RawMessage) string {
	var b strings.Builder
	for _, it := range items {
		b.Write(it)
		b.WriteString("\n")
	}
	// The items are JSON: compare with their strings decoded.
	out := strings.ReplaceAll(b.String(), `\n`, "\n")
	out = strings.ReplaceAll(out, `\u003e`, ">")
	return strings.ReplaceAll(out, `\u003c`, "<")
}

func flawOQs(t *testing.T, db *store.DB) []store.Object {
	t.Helper()
	all, err := db.ListObjects(context.Background(), "p1", store.ObjectOpenQuestion)
	if err != nil {
		t.Fatal(err)
	}
	out := []store.Object{}
	for _, o := range all {
		var b struct {
			Kind string `json:"kind"`
		}
		if json.Unmarshal(o.Body, &b) == nil && b.Kind == cycle.OpenQuestionJudgeFlaw {
			out = append(out, o)
		}
	}
	return out
}

// «una sección que queda sin pendientes con un paso de riesgo
// figures_and_calculations llama al juez antes del informe»: the round
// comes before the stop report, Spark high, no principal instance.
func TestB67_ASectionLeftWithoutPendingsAfterARiskStepCallsTheJudgeBeforeTheReport(t *testing.T) {
	db, runner, cap, _ := newCaptureRunner(t, textFixture(t, riskStep("figures_and_calculations")))
	if err := runner.RunTurn(context.Background(), "p1", "turn-1", "Escribe la muestra."); err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	if len(cap.verifies) != 1 {
		t.Fatalf("judge calls: %d, want 1", len(cap.verifies))
	}
	v := cap.verifies[0]
	if v.InstanceID != "" || v.Role != provider.RoleJudge || v.Effort != provider.EffortHigh || v.Model != provider.ModelSpark13 {
		t.Fatalf("judge call: instance %q role %q %s %s", v.InstanceID, v.Role, v.Model, v.Effort)
	}
	rounds := judgeRounds(t, db)
	if len(rounds) != 1 || rounds[0].State != "closed" {
		t.Fatalf("judge rounds: %+v", rounds)
	}
	body := judgeBody(t, rounds[0])
	if body.DelegationReason != cycle.JudgeReasonRisk || len(body.Risks) != 1 || body.Risks[0] != "figures_and_calculations" ||
		body.JudgeStatus != cycle.JudgeDone || body.SectionVersion < 1 {
		t.Fatalf("round body: %+v", body)
	}
	// Before the report: the round's creation precedes the stop report.
	events, err := db.EventsSince(context.Background(), "p1", 0)
	if err != nil {
		t.Fatal(err)
	}
	created, reported := 0, 0
	for _, ev := range events {
		if ev.ObjectID == rounds[0].ID && created == 0 {
			created = ev.Seq
		}
		if ev.Type == store.EventStopReport {
			reported = ev.Seq
		}
	}
	if created == 0 || reported == 0 || created >= reported {
		t.Fatalf("round at %d, report at %d", created, reported)
	}
	// The judge's model_call has no principal instance (B-62, 3.2).
	var judged int
	if err := db.Reader.QueryRow(`SELECT COUNT(*) FROM model_call WHERE role = 'judge' AND (instance_id IS NULL OR instance_id = '')`).Scan(&judged); err != nil {
		t.Fatal(err)
	}
	if judged != 1 {
		t.Fatalf("judge model_calls without an instance: %d, want 1", judged)
	}
}

// «una sin riesgos no»: closing_risk none calls no judge and creates no
// round; the chain still stops with its report (the previous behavior).
func TestB67_ASectionWithoutRisksDoesNotCallTheJudge(t *testing.T) {
	db, runner, cap, _ := newCaptureRunner(t, textFixture(t, riskStep("none")))
	if err := runner.RunTurn(context.Background(), "p1", "turn-1", "Escribe la muestra."); err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	if len(cap.verifies) != 0 || len(judgeRounds(t, db)) != 0 {
		t.Fatalf("judge calls %d, rounds %d", len(cap.verifies), len(judgeRounds(t, db)))
	}
	reports := stopReports(t, db)
	if len(reports) != 1 || reports[0].StopReason != cycle.StopSectionReady || !reports[0].ProposeSignoff ||
		len(reports[0].JudgeNotVerified) != 0 {
		t.Fatalf("reports: %+v", reports)
	}
}

// A section with an agent's pending does not call the judge yet (B-67 a:
// «la deja sin pendientes del agente»), whatever its risk.
func TestB67_ASectionWithAgentPendingsDoesNotCallTheJudgeYet(t *testing.T) {
	step := strings.Replace(pendingFigureStep, `"closing_risk": "none"`, `"closing_risk": "figures_and_calculations"`, 1)
	db, runner, cap, _ := newCaptureRunner(t, textFixture(t, step), textFixture(t, `{"stretch":"direct","triage_line":"x","closing_risk":"none","note":"nada"}`))
	if err := runner.RunTurn(context.Background(), "p1", "turn-1", "Escribe la muestra."); err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	if len(cap.verifies) != 0 {
		t.Fatalf("judge calls: %d, want 0", len(cap.verifies))
	}
	if got, err := cycle.NewGate(db).ShouldVerify(context.Background(), "p1", "sec-1"); err != nil || got {
		t.Fatalf("ShouldVerify = %v, %v", got, err)
	}
}

// «desde su última verificación»: a verified section with no later risk
// step needs none; a later risk step brings it back.
func TestB67_ShouldVerifyCountsFromTheVersionTheLastJudgeRoundKeeps(t *testing.T) {
	db, runner, _, _ := newCaptureRunner(t, textFixture(t, riskStep("sources_and_citations")))
	gate := cycle.NewGate(db)
	ctx := context.Background()
	if err := runner.RunTurn(ctx, "p1", "turn-1", "Escribe la muestra."); err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	if got, err := gate.ShouldVerify(ctx, "p1", "sec-1"); err != nil || got {
		t.Fatalf("after its verification: ShouldVerify = %v, %v", got, err)
	}
	sec, err := db.GetObject(ctx, "p1", "sec-1")
	if err != nil {
		t.Fatal(err)
	}
	step, _ := json.Marshal(map[string]any{"stretch": "build", "triage_line": "x", "closing_risk": "sources_and_citations", "note": "n",
		"new_text": []map[string]any{{"section_id": "sec-1", "base_version": sec.CurrentVersion, "anchor": "La muestra cubre la cuenca del Rímac.",
			"claim_origin": "model_knowledge", "operation": "insert_after", "text": "Y también la del Chillón."}}})
	if _, err := gate.ApplyChainStep(ctx, "p1", "turn-2", json.RawMessage(step), false); err != nil {
		t.Fatalf("ApplyChainStep: %v", err)
	}
	if got, err := gate.ShouldVerify(ctx, "p1", "sec-1"); err != nil || !got {
		t.Fatalf("after a later risk step: ShouldVerify = %v, %v", got, err)
	}
}

// «"Auditar sección" lo llama siempre»: with no risk at all, the judge is
// called; a section without text and a label that is not a section refuse.
func TestB67_AuditAlwaysCallsTheJudge(t *testing.T) {
	db, runner, cap, _ := newCaptureRunner(t)
	seedJudged(t, db)
	label, turnID, err := runner.AuditSection("p1", "sec-1")
	if err != nil || label == "" || turnID == "" {
		t.Fatalf("AuditSection: %q %q %v", label, turnID, err)
	}
	waitJudgeClosed(t, db, label)
	if len(cap.verifies) != 1 {
		t.Fatalf("judge calls: %d, want 1", len(cap.verifies))
	}
	rounds := judgeRounds(t, db)
	if len(rounds) != 1 || rounds[0].State != "closed" || judgeBody(t, rounds[0]).DelegationReason != cycle.JudgeReasonAudit {
		t.Fatalf("rounds: %+v", rounds)
	}
	if _, _, err := runner.AuditSection("p1", "src-1"); !errors.Is(err, cycle.ErrNotSection) {
		t.Fatalf("audit of a source: %v", err)
	}
	runner.Close()
}

// The audit takes the project's turn slot (P-20).
func TestB67_AuditDuringATurnIsRefused(t *testing.T) {
	db, runner, cap, _ := newCaptureRunner(t)
	seedJudged(t, db)
	release := make(chan struct{})
	held := &heldJudge{inner: cap, release: release, entered: make(chan struct{})}
	r2 := cycle.NewRunner(db, held, runnerCount, nil)
	if _, _, err := r2.AuditSection("p1", "sec-1"); err != nil {
		t.Fatalf("AuditSection: %v", err)
	}
	<-held.entered
	if _, _, err := r2.AuditSection("p1", "sec-1"); !errors.Is(err, cycle.ErrTurnRunning) {
		t.Fatalf("second audit: %v", err)
	}
	close(release)
	waitJudgeClosed(t, db, judgeRounds(t, db)[0].Label)
	r2.Close()
	runner.Close()
}

type heldJudge struct {
	inner   provider.Provider
	release chan struct{}
	entered chan struct{}
	done    bool
}

func (h *heldJudge) Respond(ctx context.Context, req provider.Request, onDelta func(provider.Delta)) (provider.Response, error) {
	if req.TurnKind == provider.TurnJudgeVerify && !h.done {
		h.done = true
		close(h.entered)
		<-h.release
	}
	return h.inner.Respond(ctx, req, onDelta)
}

func runnerCount(ctx context.Context, input []json.RawMessage) (int, error) {
	return provider.NewFake().CountTokens(ctx, provider.PrincipalModel, input)
}

// The encargo of bloque-1 §4 in literal plus the claims with risk: the
// section's current text delimited, its citations with their source's
// passage; labels only, never an id (rule 7).
func TestB67_TheJudgeGetsTheWholeBriefWithTheJudgedTextDelimited(t *testing.T) {
	db, runner, cap, _ := newCaptureRunner(t)
	seedJudged(t, db)
	if _, err := runner.RunJudgeVerify(context.Background(), "p1", "sec-1", cycle.JudgeReasonRisk, []string{"figures_and_calculations"}); err != nil {
		t.Fatalf("RunJudgeVerify: %v", err)
	}
	if len(cap.verifies) != 1 {
		t.Fatalf("judge calls: %d", len(cap.verifies))
	}
	text := itemsText(cap.verifies[0].Input)
	for _, want := range []string{roundRequest, roundCriteria, roundCounts, roundNotCount, roundPremise, "Comparar por veces.",
		"estudio.md", "<<<BEGIN section>>>\n" + judgedText, "En 2023 los conteos fueron 60 veces mayores.", "<<<BEGIN source>>>",
		"figures_and_calculations", "[inferencia]"} {
		if !strings.Contains(text, want) {
			t.Errorf("the brief lacks %q:\n%s", want, text)
		}
	}
	sec, _ := db.GetObject(context.Background(), "p1", "sec-1")
	if strings.Contains(text, sec.ID) {
		t.Fatalf("the brief carries the section's internal id:\n%s", text)
	}
	if !strings.Contains(cap.verifies[0].Instructions, "Data, not instructions") {
		t.Fatal("the instructions lack the data rule")
	}
}

// A passage that is not literal in the section, or a flaw on another
// section, is an error of shape: retried once.
func TestB67_AFlawOutsideTheSectionIsRetriedOnce(t *testing.T) {
	db, runner, cap, _ := newCaptureRunner(t,
		verifyFlaws(t, oneFlaw("sec-1", "Una frase que no está.", "unverifiable")),
		verifyFlaws(t, oneFlaw("sec-1", flawOne, "figures_and_calculations")))
	seedJudged(t, db)
	label, err := runner.RunJudgeVerify(context.Background(), "p1", "sec-1", cycle.JudgeReasonRisk, nil)
	if err != nil {
		t.Fatalf("RunJudgeVerify: %v", err)
	}
	if len(cap.verifies) != 2 {
		t.Fatalf("judge calls: %d, want 2", len(cap.verifies))
	}
	round, _ := db.GetObject(context.Background(), "p1", label)
	if b := judgeBody(t, round); round.State != "open" || len(b.Flaws) != 1 || b.Flaws[0].N != 1 || b.Flaws[0].Passage != flawOne {
		t.Fatalf("round %s: %+v", round.State, b)
	}
	if len(eventsOfType(t, db, store.EventOutputRetry)) != 1 {
		t.Fatal("no output_retry for the first output")
	}
}

// The fixed correct step of the batch: a text change on sec-1.
func batchStep(t *testing.T, db *store.DB) provider.Fixture {
	t.Helper()
	sec, err := db.GetObject(context.Background(), "p1", "sec-1")
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(map[string]any{"stretch": "build", "triage_line": "Verificar: corrijo.", "closing_risk": "none", "note": "Corregí.",
		"new_text": []map[string]any{{"section_id": "sec-1", "base_version": sec.CurrentVersion, "anchor": "Revisamos los filtros finos.",
			"claim_origin": "model_knowledge", "operation": "insert_after", "text": "Revisamos además los gruesos."}}})
	return textFixture(t, string(raw))
}

// «una falla descartada no llega al lote»: flaw_discarded is written, the
// batch carries only the other one, and the recheck asks only for it.
func TestB67_ADiscardedFlawDoesNotReachTheBatch(t *testing.T) {
	db, runner, cap, _ := newCaptureRunner(t,
		verifyFlaws(t, oneFlaw("sec-1", flawOne, "figures_and_calculations"), oneFlaw("sec-1", flawTwo, "unverifiable")))
	seedJudged(t, db)
	ctx := context.Background()
	label, err := runner.RunJudgeVerify(ctx, "p1", "sec-1", cycle.JudgeReasonRisk, nil)
	if err != nil {
		t.Fatalf("RunJudgeVerify: %v", err)
	}
	if err := runner.DiscardFlaw(ctx, "p1", label, 1); err != nil {
		t.Fatalf("DiscardFlaw: %v", err)
	}
	if evs := eventsOfType(t, db, store.EventFlawDiscarded); len(evs) != 1 || evs[0].Actor != "human" {
		t.Fatalf("flaw_discarded: %+v", evs)
	}
	if err := runner.DiscardFlaw(ctx, "p1", label, 1); !errors.Is(err, cycle.ErrFlawDiscardedTwice) {
		t.Fatalf("second discard: %v", err)
	}
	if err := runner.DiscardFlaw(ctx, "p1", label, 9); !errors.Is(err, cycle.ErrFlawNotFound) {
		t.Fatalf("unknown flaw: %v", err)
	}
	runner2Fake := provider.NewFake(batchStep(t, db), verifyRecheck(t, flawResolved(2)))
	cap.inner = recordingFake(db, runner2Fake)
	if err := runner.RunTurn(ctx, "p1", "turn-1", "Sigue."); err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	batch := itemsText(cap.sent[0])
	if strings.Contains(batch, flawOne) && strings.Contains(batch, "<<<BEGIN section>>>\n"+flawOne) {
		t.Fatalf("the discarded flaw reached the batch:\n%s", batch)
	}
	if !strings.Contains(batch, "<<<BEGIN section>>>\n"+flawTwo) {
		t.Fatalf("the batch lacks the other flaw:\n%s", batch)
	}
	if err := runner.DiscardFlaw(ctx, "p1", label, 2); !errors.Is(err, cycle.ErrFlawTooLate) {
		t.Fatalf("discard after the batch started: %v", err)
	}
}

func recordingFake(db *store.DB, f *provider.Fake) *provider.Fake {
	f.Recorder = db
	return f
}

// «sin descartes, todas llegan».
func TestB67_WithoutDiscardsEveryFlawReachesTheBatch(t *testing.T) {
	db, runner, cap, _ := newCaptureRunner(t,
		verifyFlaws(t, oneFlaw("sec-1", flawOne, "figures_and_calculations"), oneFlaw("sec-1", flawTwo, "unverifiable")))
	seedJudged(t, db)
	ctx := context.Background()
	if _, err := runner.RunJudgeVerify(ctx, "p1", "sec-1", cycle.JudgeReasonRisk, nil); err != nil {
		t.Fatalf("RunJudgeVerify: %v", err)
	}
	cap.inner = recordingFake(db, provider.NewFake(batchStep(t, db)))
	if err := runner.RunTurn(ctx, "p1", "turn-1", "Sigue."); err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	batch := itemsText(cap.sent[0])
	for _, want := range []string{"<<<BEGIN section>>>\n" + flawOne, "<<<BEGIN section>>>\n" + flawTwo, "1. El destinatario pide la fuente."} {
		if !strings.Contains(batch, want) {
			t.Fatalf("the batch lacks %q:\n%s", want, batch)
		}
	}
}

// All flaws discarded: no batch, no recheck, the round closes.
func TestB67_DiscardingEveryFlawClosesTheRoundWithoutBatch(t *testing.T) {
	db, runner, cap, _ := newCaptureRunner(t, verifyFlaws(t, oneFlaw("sec-1", flawOne, "figures_and_calculations")))
	seedJudged(t, db)
	ctx := context.Background()
	label, err := runner.RunJudgeVerify(ctx, "p1", "sec-1", cycle.JudgeReasonRisk, nil)
	if err != nil {
		t.Fatalf("RunJudgeVerify: %v", err)
	}
	if err := runner.DiscardFlaw(ctx, "p1", label, 1); err != nil {
		t.Fatalf("DiscardFlaw: %v", err)
	}
	round, _ := db.GetObject(ctx, "p1", label)
	if round.State != "closed" || len(cap.verifies) != 1 {
		t.Fatalf("round %s, judge calls %d", round.State, len(cap.verifies))
	}
	if err := runner.DiscardFlaw(ctx, "p1", label, 1); !errors.Is(err, cycle.ErrFlawTooLate) {
		t.Fatalf("discard on a closed round: %v", err)
	}
}

// The whole chain: judge → batch step → exactly one recheck; a still_open
// opens one «por aclarar» per flaw with B-67's two options, blocking the
// section; the round closes and no second recheck or round follows.
func TestB67_ExactlyOneRecheckAndStillOpenOpensAnOpenQuestion(t *testing.T) {
	first := riskStep("figures_and_calculations")
	passage := "La muestra cubre la cuenca del Rímac."
	correction, _ := json.Marshal(map[string]any{"stretch": "build", "triage_line": "Verificar: corrijo.", "closing_risk": "figures_and_calculations", "note": "Corregí.",
		"new_text": []map[string]any{{"section_id": "sec-1", "base_version": 1, "anchor": passage,
			"claim_origin": "model_knowledge", "operation": "insert_after", "text": "Revisamos además los gruesos."}}})
	db, runner, cap, _ := newCaptureRunner(t,
		textFixture(t, first), textFixture(t, string(correction)),
		verifyFlaws(t, oneFlaw("sec-1", passage, "figures_and_calculations"), oneFlaw("sec-1", passage, "unverifiable")),
		verifyRecheck(t, flawStillOpen(1, passage), flawResolved(2)))
	if err := runner.RunTurn(context.Background(), "p1", "turn-1", "Escribe la muestra."); err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	if len(cap.verifies) != 2 {
		t.Fatalf("judge calls: %d, want 2 (one round, one recheck)", len(cap.verifies))
	}
	if got := cap.verifies[1].JudgeRecheck; len(got) != 2 || got[0] != 1 || got[1] != 2 {
		t.Fatalf("recheck list: %v", got)
	}
	if text := itemsText(cap.verifies[1].Input); !strings.Contains(text, "Revisamos además los gruesos.") ||
		!strings.Contains(text, "Flaws to recheck") {
		t.Fatalf("the recheck lacks the correction or the flaws:\n%s", text)
	}
	rounds := judgeRounds(t, db)
	if len(rounds) != 1 || rounds[0].State != "closed" {
		t.Fatalf("judge rounds: %+v", rounds)
	}
	body := judgeBody(t, rounds[0])
	if body.Flaws[0].Verdict != "still_open" || body.Flaws[1].Verdict != "resolved" || body.Flaws[0].OpenQuestionID == "" {
		t.Fatalf("flaws: %+v", body.Flaws)
	}
	oqs := flawOQs(t, db)
	if len(oqs) != 1 || oqs[0].State != "open" {
		t.Fatalf("flaw «por aclarar»: %+v", oqs)
	}
	var b struct {
		Options []struct {
			Code string `json:"code"`
		} `json:"options"`
		NewPassage string `json:"new_passage"`
	}
	if err := json.Unmarshal(oqs[0].Body, &b); err != nil {
		t.Fatal(err)
	}
	if len(b.Options) != 2 || b.Options[0].Code != cycle.FlawOptionRedoPassage || b.Options[1].Code != cycle.FlawOptionAcceptRisk || b.NewPassage != passage {
		t.Fatalf("options: %+v", b)
	}
	clear, err := graph.SectionClear(context.Background(), db, "p1", "sec-1")
	if err != nil || clear {
		t.Fatalf("SectionClear = %v, %v (the open question blocks the signoff)", clear, err)
	}
	// The correction's own risk does not bring the judge back (B-67: never
	// another round after the batch).
	if got, err := cycle.NewGate(db).ShouldVerify(context.Background(), "p1", "sec-1"); err != nil || got {
		t.Fatalf("after the recheck: ShouldVerify = %v, %v", got, err)
	}
}

// «un hallazgo nuevo del juez en la reverificación no abre otra ronda»:
// it opens no round and no «por aclarar»; it stays in the round's body.
func TestB67_ANewFindingInTheRecheckOpensNoRound(t *testing.T) {
	db, runner, cap, _ := newCaptureRunner(t, verifyFlaws(t, oneFlaw("sec-1", flawOne, "figures_and_calculations")),
		judgeVerifyFixture(t, map[string]any{"flaws": []any{oneFlaw("sec-1", flawTwo, "unverifiable")}, "recheck": []any{flawResolved(1)}}))
	seedJudged(t, db)
	ctx := context.Background()
	label, err := runner.RunJudgeVerify(ctx, "p1", "sec-1", cycle.JudgeReasonRisk, nil)
	if err != nil {
		t.Fatalf("RunJudgeVerify: %v", err)
	}
	if err := runner.RunJudgeRecheck(ctx, "p1", label); err != nil {
		t.Fatalf("RunJudgeRecheck: %v", err)
	}
	rounds := judgeRounds(t, db)
	if len(rounds) != 1 || rounds[0].State != "closed" || len(flawOQs(t, db)) != 0 {
		t.Fatalf("rounds %+v, questions %d", rounds, len(flawOQs(t, db)))
	}
	if b := judgeBody(t, rounds[0]); len(b.LateFlaws) != 1 || b.LateFlaws[0].Passage != flawTwo {
		t.Fatalf("late flaws: %+v", b.LateFlaws)
	}
	// A second recheck is never made.
	if err := runner.RunJudgeRecheck(ctx, "p1", label); err != nil {
		t.Fatalf("second recheck: %v", err)
	}
	if len(cap.verifies) != 2 {
		t.Fatalf("judge calls: %d, want 2", len(cap.verifies))
	}
}

// A judge that fails twice: the round closes with no flaws, the report
// says it could not verify, the signoff is not blocked, and the failed
// round does not count as «última verificación».
func TestB67_AJudgeThatFailsTwiceClosesWithNoFlawsAndBlocksNothing(t *testing.T) {
	db, runner, cap, _ := newCaptureRunner(t, textFixture(t, riskStep("figures_and_calculations")))
	cap.verifyFails = true
	ctx := context.Background()
	if err := runner.RunTurn(ctx, "p1", "turn-1", "Escribe la muestra."); err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	if len(cap.verifies) != 2 {
		t.Fatalf("judge calls: %d, want 2 (one retry)", len(cap.verifies))
	}
	rounds := judgeRounds(t, db)
	if len(rounds) != 1 || rounds[0].State != "closed" {
		t.Fatalf("rounds: %+v", rounds)
	}
	if b := judgeBody(t, rounds[0]); b.JudgeStatus != cycle.JudgeFailed || len(b.Flaws) != 0 {
		t.Fatalf("body: %+v", b)
	}
	reports := stopReports(t, db)
	if len(reports) != 1 || len(reports[0].JudgeNotVerified) != 1 || reports[0].JudgeNotVerified[0] != "sec-1" || !reports[0].ProposeSignoff {
		t.Fatalf("reports: %+v", reports)
	}
	if clear, err := graph.SectionClear(ctx, db, "p1", "sec-1"); err != nil || !clear {
		t.Fatalf("SectionClear = %v, %v", clear, err)
	}
	retries := eventsOfType(t, db, store.EventOutputRetry)
	var last map[string]any
	_ = json.Unmarshal(retries[len(retries)-1].Data, &last)
	if last["result"] != cycle.JudgeFailed || last["turn_kind"] != string(provider.TurnJudgeVerify) {
		t.Fatalf("output_retry: %v", last)
	}
	if got, err := cycle.NewGate(db).ShouldVerify(ctx, "p1", "sec-1"); err != nil || !got {
		t.Fatalf("after a failed round: ShouldVerify = %v, %v, want true", got, err)
	}
}

// A chain tries a failed section once; the next chain is its next
// opportunity.
func TestB67_AFailedJudgeIsNotRetriedInTheSameChain(t *testing.T) {
	second := `{"stretch":"build","triage_line":"x","closing_risk":"none","note":"n","new_text":[{"section_id":"new-1","base_version":0,"anchor":"","claim_origin":"model_knowledge","operation":"insert_after","text":"# Otra\n\nOtra parte."}],"proposed_roles":[{"section_id":"new-1","role":"other","title":"Otra"}]}`
	db, runner, cap, _ := newCaptureRunner(t, textFixture(t, riskStep("figures_and_calculations")), textFixture(t, second))
	cap.verifyFails = true
	if err := runner.RunTurn(context.Background(), "p1", "turn-1", "Escribe."); err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	if len(judgeRounds(t, db)) != 1 {
		t.Fatalf("judge rounds in one chain: %d", len(judgeRounds(t, db)))
	}
}

// B-65: the judge reserves before each call against the project's cap;
// no call when it does not fit, the round closes and the report says so.
func TestB65_TheJudgeReservesBeforeEachCallAgainstTheProjectCap(t *testing.T) {
	db, runner, cap, _ := newCaptureRunner(t)
	seedJudged(t, db)
	runner.ProjectCapUSD = 0.01
	label, err := runner.RunJudgeVerify(context.Background(), "p1", "sec-1", cycle.JudgeReasonRisk, nil)
	if err != nil {
		t.Fatalf("RunJudgeVerify: %v", err)
	}
	if len(cap.verifies) != 0 {
		t.Fatalf("judge calls: %d, want 0", len(cap.verifies))
	}
	round, _ := db.GetObject(context.Background(), "p1", label)
	if b := judgeBody(t, round); round.State != "closed" || b.JudgeStatus != cycle.JudgeProjectCap {
		t.Fatalf("round %s: %+v", round.State, b)
	}
}

// B-67: the flaw's two options reach the principal once you answer: what
// you chose, with your indication, between labels only (rule 7).
func TestB67_AnsweringRedoGivesThePrincipalYourIndication(t *testing.T) {
	db, runner, cap, _ := newCaptureRunner(t, verifyFlaws(t, oneFlaw("sec-1", flawOne, "figures_and_calculations")),
		verifyRecheck(t, flawStillOpen(1, flawOne)))
	seedJudged(t, db)
	ctx := context.Background()
	label, err := runner.RunJudgeVerify(ctx, "p1", "sec-1", cycle.JudgeReasonRisk, nil)
	if err != nil {
		t.Fatalf("RunJudgeVerify: %v", err)
	}
	if err := runner.RunJudgeRecheck(ctx, "p1", label); err != nil {
		t.Fatalf("RunJudgeRecheck: %v", err)
	}
	oqs := flawOQs(t, db)
	if len(oqs) != 1 {
		t.Fatalf("questions: %d", len(oqs))
	}
	// A live instance, so the answer arrives in what changed.
	cap.inner = recordingFake(db, provider.NewFake(textFixture(t, `{"stretch":"direct","triage_line":"x","closing_risk":"none","note":"n"}`)))
	if err := runner.RunTurn(ctx, "p1", "turn-1", "Hola."); err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	zero := 0
	const indication = "Di que son conteos de Lima, porque solo medimos ahí."
	if _, err := runner.AnswerOpenQuestion(ctx, "p1", oqs[0].Label, &zero, indication); err != nil {
		t.Fatalf("AnswerOpenQuestion: %v", err)
	}
	// B-100: the answer starts the step that takes your indication.
	cycle.WaitTurnsForTest(runner)
	text := itemsText(cap.sent[len(cap.sent)-1])
	if !strings.Contains(text, indication) || !strings.Contains(text, "<<<BEGIN passage>>>") {
		t.Fatalf("the principal lacks your indication:\n%s", text)
	}
	if strings.Contains(text, `"depends_on"`) || strings.Contains(text, oqs[0].ID) {
		t.Fatalf("the principal got the body with ids:\n%s", text)
	}
}

// pantallas: «Fallas por revisar» in «Lo que te toca» until the batch
// starts, and each flaw's passage placed for its underline.
func TestB67_FlawsShowInYourTodoAndUnderlinedUntilTheBatchStarts(t *testing.T) {
	db, runner, _, _ := newCaptureRunner(t, verifyFlaws(t, oneFlaw("sec-1", flawOne, "figures_and_calculations")))
	seedJudged(t, db)
	ctx := context.Background()
	label, err := runner.RunJudgeVerify(ctx, "p1", "sec-1", cycle.JudgeReasonRisk, nil)
	if err != nil {
		t.Fatalf("RunJudgeVerify: %v", err)
	}
	frame, err := graph.ComputeFrame(ctx, db, "p1", graph.FrameLayers{})
	if err != nil {
		t.Fatal(err)
	}
	if len(frame.Todo.FlawsToReview) != 1 || frame.Todo.FlawsToReview[0].Round != label || frame.Todo.FlawsToReview[0].Flaws != 1 {
		t.Fatalf("todo: %+v", frame.Todo)
	}
	warnings, err := graph.SectionWarnings(ctx, db, "p1", "sec-1")
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, w := range warnings {
		if w.Kind == graph.WarningFlaw && w.Passage == flawOne && w.Block > 0 && w.End > w.Start && w.Target == label {
			found = true
		}
	}
	if !found {
		t.Fatalf("no flaw warning: %+v", warnings)
	}
}

// The round card shows what the page needs: the risks, the flaws with
// their status, labels only.
func TestB67_TheRoundViewShowsFlawsWithLabelsOnly(t *testing.T) {
	db, runner, _, _ := newCaptureRunner(t, verifyFlaws(t, oneFlaw("sec-1", flawOne, "figures_and_calculations")))
	seedJudged(t, db)
	ctx := context.Background()
	label, err := runner.RunJudgeVerify(ctx, "p1", "sec-1", cycle.JudgeReasonRisk, []string{"figures_and_calculations"})
	if err != nil {
		t.Fatalf("RunJudgeVerify: %v", err)
	}
	view, err := runner.Round(ctx, "p1", label)
	if err != nil {
		t.Fatal(err)
	}
	if view.Kind != "judge" || view.Section != "sec-1" || view.SectionTitle != "Muestra" || view.Reason != cycle.JudgeReasonRisk ||
		len(view.Risks) != 1 || len(view.Flaws) != 1 || view.Flaws[0].N != 1 || view.Flaws[0].Scenario == "" || view.BatchStarted {
		t.Fatalf("view: %+v", view)
	}
}

// «con salidas grabadas»: the recordings of make microtest-judge-verify
// (hand-made until Eber runs it) pass the app's validation on the section
// they were recorded for, and give the round its flaws and its question.
func TestB67_TheRecordedOutputsGiveTheFlawsAndTheQuestion(t *testing.T) {
	load := func(name string) provider.Fixture {
		fx, err := provider.LoadFixture(filepath.Join("..", "provider", "testdata", "judge-verify", name+".json"))
		if err != nil {
			t.Fatalf("recording %s: %v", name, err)
		}
		return fx
	}
	db, runner, cap, _ := newCaptureRunner(t, load("flaws-two"), load("recheck-still-open"))
	seedJudged(t, db)
	ctx := context.Background()
	label, err := runner.RunJudgeVerify(ctx, "p1", "sec-1", cycle.JudgeReasonRisk, []string{"figures_and_calculations"})
	if err != nil {
		t.Fatalf("RunJudgeVerify: %v", err)
	}
	if err := runner.RunJudgeRecheck(ctx, "p1", label); err != nil {
		t.Fatalf("RunJudgeRecheck: %v", err)
	}
	round, _ := db.GetObject(ctx, "p1", label)
	b := judgeBody(t, round)
	if round.State != "closed" || len(b.Flaws) != 2 || b.Flaws[0].Verdict != cycle.FlawStillOpen || len(flawOQs(t, db)) != 1 || len(cap.verifies) != 2 {
		t.Fatalf("round %s: %+v; calls %d", round.State, b, len(cap.verifies))
	}
}

// The embedded schema is the design's, byte for byte.
func TestB67_EmbeddedJudgeVerifySchemaEqualsTheDesign(t *testing.T) {
	embedded, err := os.ReadFile(filepath.Join("schema", "judge_verify_output.schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	design, err := os.ReadFile(filepath.Join("..", "..", "docs", "diseno", "bloque-3", "salida-juez-verificacion.schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	if string(embedded) != string(design) {
		t.Fatal("embedded schema differs from salida-juez-verificacion.schema.json")
	}
}

// waitJudgeClosed waits for the judge round in the background to close.
func waitJudgeClosed(t *testing.T, db *store.DB, label string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if round, err := db.GetObject(context.Background(), "p1", label); err == nil && round.State == "closed" {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("the judge round did not close")
}

// F1 (backlog-r3 3.6: «para que la sección vuelva a verificarse en la
// siguiente oportunidad»): the stop report resets the touched sections, so
// the next chain walks the failed ones too, also when it writes another
// section.
func TestB67_AFailedSectionIsReverifiedInTheNextChain(t *testing.T) {
	db, runner, cap, _ := newCaptureRunner(t, textFixture(t, riskStep("figures_and_calculations")))
	cap.verifyFails = true
	ctx := context.Background()
	if err := runner.RunTurn(ctx, "p1", "t1", "Escribe."); err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	first := len(cap.verifies)
	cap.verifyFails = false
	cap.inner = recordingFake(db, provider.NewFake(textFixture(t, riskStep("none"))))
	if err := runner.RunTurn(ctx, "p1", "t2", "Otra sección."); err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	if len(cap.verifies) != first+1 {
		t.Fatalf("judge calls: %d, want %d", len(cap.verifies), first+1)
	}
	rounds := judgeRounds(t, db)
	if len(rounds) != 2 || judgeBody(t, rounds[1]).JudgeStatus != cycle.JudgeDone {
		t.Fatalf("rounds: %+v", rounds)
	}
	if got, err := cycle.NewGate(db).ShouldVerify(ctx, "p1", "sec-1"); err != nil || got {
		t.Fatalf("after it verified: ShouldVerify = %v, %v", got, err)
	}
}

// Decided: a chain whose step does no work is an opportunity too: the
// section may be the last one, and nothing else would ever verify it.
func TestB67_ANextChainWithoutWorkReverifiesAFailedSection(t *testing.T) {
	db, runner, cap, _ := newCaptureRunner(t, textFixture(t, riskStep("figures_and_calculations")))
	cap.verifyFails = true
	ctx := context.Background()
	if err := runner.RunTurn(ctx, "p1", "t1", "Escribe."); err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	reports, first := len(stopReports(t, db)), len(cap.verifies)
	cap.verifyFails = false
	cap.inner = recordingFake(db, provider.NewFake(textFixture(t, `{"stretch":"direct","triage_line":"x","closing_risk":"none","note":"n"}`)))
	if err := runner.RunTurn(ctx, "p1", "t2", "¿Cómo vas?"); err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	if len(cap.verifies) != first+1 || len(judgeRounds(t, db)) != 2 {
		t.Fatalf("judge calls %d, rounds %d", len(cap.verifies), len(judgeRounds(t, db)))
	}
	// P-75 (b) holds: no work since the stop, no report.
	if got := len(stopReports(t, db)); got != reports {
		t.Fatalf("reports: %d, want %d", got, reports)
	}
	if got, err := cycle.JudgeNotVerified(ctx, db, "p1"); err != nil || len(got) != 0 {
		t.Fatalf("JudgeNotVerified = %v, %v", got, err)
	}
}

// A no-work chain whose reverification finds flaws goes on with their
// batch and its recheck.
func TestB67_ANoWorkChainThatFindsFlawsRunsTheirBatch(t *testing.T) {
	passage := "La muestra cubre la cuenca del Rímac."
	db, runner, cap, _ := newCaptureRunner(t, textFixture(t, riskStep("figures_and_calculations")))
	cap.verifyFails = true
	ctx := context.Background()
	if err := runner.RunTurn(ctx, "p1", "t1", "Escribe."); err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	cap.verifyFails = false
	first, steps := len(cap.verifies), len(cap.sent)
	noWork := `{"stretch":"direct","triage_line":"x","closing_risk":"none","note":"n"}`
	cap.inner = recordingFake(db, provider.NewFake(textFixture(t, noWork),
		verifyFlaws(t, oneFlaw("sec-1", passage, "unverifiable")), verifyRecheck(t, flawResolved(1))))
	if err := runner.RunTurn(ctx, "p1", "t2", "¿Cómo vas?"); err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	if len(cap.verifies) != first+2 || len(cap.sent) != steps+2 {
		t.Fatalf("judge calls +%d, steps +%d", len(cap.verifies)-first, len(cap.sent)-steps)
	}
	rounds := judgeRounds(t, db)
	if rounds[len(rounds)-1].State != "closed" || !strings.Contains(itemsText(cap.sent[len(cap.sent)-1]), "<<<BEGIN section>>>\n"+passage) {
		t.Fatalf("round %s; batch:\n%s", rounds[len(rounds)-1].State, itemsText(cap.sent[len(cap.sent)-1]))
	}
}

// No cost loop: while the judge keeps failing, one attempt (its two calls)
// per chain, however many steps the chain takes.
func TestB67_AJudgeThatKeepsFailingGetsOneAttemptPerChain(t *testing.T) {
	db, runner, cap, _ := newCaptureRunner(t, textFixture(t, riskStep("figures_and_calculations")))
	cap.verifyFails = true
	ctx := context.Background()
	if err := runner.RunTurn(ctx, "p1", "t1", "Escribe."); err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	if len(cap.verifies) != 2 {
		t.Fatalf("first chain: %d judge calls", len(cap.verifies))
	}
	// A chain of several steps: each writes a figure that leaves a pending,
	// so ShouldStop does not stop it.
	cap.inner = recordingFake(db, provider.NewFake(textFixture(t, pendingFigureStep),
		textFixture(t, `{"stretch":"direct","triage_line":"x","closing_risk":"none","note":"n"}`)))
	if err := runner.RunTurn(ctx, "p1", "t2", "Sigue."); err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	if len(cap.verifies) != 4 || len(judgeRounds(t, db)) != 2 {
		t.Fatalf("second chain: %d judge calls, %d rounds", len(cap.verifies), len(judgeRounds(t, db)))
	}
}

// After project_cap: the section is reverified when the budget allows; a
// chain over the cap creates no other round and makes no call.
func TestB67_AfterTheProjectCapTheSectionIsReverifiedWhenItFits(t *testing.T) {
	dear := textFixture(t, riskStep("figures_and_calculations"))
	dear.Usage.CostUSD = 0.08
	db, runner, cap, _ := newCaptureRunner(t, dear)
	// A dear judge call before: the principal's and the checker's reserves
	// fit; the judge's, its dearest call (B-89), does not.
	if err := db.RecordModelCall(context.Background(), store.ModelCall{ProjectID: "p1", Role: "judge", CostUSD: 0.10}); err != nil {
		t.Fatal(err)
	}
	runner.ProjectCapUSD = 0.25 + provider.SignoffReserveUSD
	ctx := context.Background()
	if err := runner.RunTurn(ctx, "p1", "t1", "Escribe."); err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	if len(cap.verifies) != 0 || len(judgeRounds(t, db)) != 1 {
		t.Fatalf("over the cap: %d calls, %d rounds", len(cap.verifies), len(judgeRounds(t, db)))
	}
	cap.inner = recordingFake(db, provider.NewFake(textFixture(t, riskStep("none"))))
	if err := runner.RunTurn(ctx, "p1", "t2", "Otra."); err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	if len(cap.verifies) != 0 || len(judgeRounds(t, db)) != 1 {
		t.Fatalf("still over the cap: %d calls, %d rounds", len(cap.verifies), len(judgeRounds(t, db)))
	}
	runner.ProjectCapUSD = 0
	cap.inner = recordingFake(db, provider.NewFake(textFixture(t, riskStep("none"))))
	if err := runner.RunTurn(ctx, "p1", "t3", "Otra más."); err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	if len(cap.verifies) != 1 || len(judgeRounds(t, db)) != 2 {
		t.Fatalf("within the cap: %d calls, %d rounds", len(cap.verifies), len(judgeRounds(t, db)))
	}
}

// A3: an internal error in the audit's chain publishes one error, not two.
func TestB67_AnAuditWhoseChainFailsPublishesOneError(t *testing.T) {
	db, _, cap, pub := newCaptureRunner(t, verifyFlaws(t, oneFlaw("sec-1", flawOne, "unverifiable")))
	seedJudged(t, db)
	failing := func(context.Context, []json.RawMessage) (int, error) { return 0, errors.New("no count") }
	runner := cycle.NewRunner(db, cap, failing, pub)
	defer runner.Close()
	if _, _, err := runner.AuditSection("p1", "sec-1"); err != nil {
		t.Fatalf("AuditSection: %v", err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) && len(publishedErrors(pub)) == 0 {
		time.Sleep(5 * time.Millisecond)
	}
	time.Sleep(50 * time.Millisecond)
	if got := publishedErrors(pub); len(got) != 1 || got[0] != "turn_failed" {
		t.Fatalf("errors published: %v", got)
	}
}

func publishedErrors(p *fakePublisher) []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]string{}, p.errs...)
}

// Backlog-r4 v0.8, hueco 4 (B-67 with B-73 b): the judge's round covers the section
// the step closed and each signed section where that step or its batch left a proposal.

const signedPassage = "Los conteos subieron 60 veces en 2023."

// seedSignedAndDraft is sec-1 signed (judgedText) and sec-2, a draft the steps close.
func seedSignedAndDraft(t *testing.T, db *store.DB) {
	t.Helper()
	seedJudged(t, db)
	err := db.Write(context.Background(), "p1", func(tx *store.Tx) error {
		_, err := tx.CreateObject(store.ObjectSection, "draft", map[string]any{"title": "Cierre", "text": "# Cierre\n\nLa muestra alcanza.",
			"role": map[string]any{"value": "other", "state": "confirmed"}, "open_changes": []any{}},
			store.EventInput{Type: store.EventStateChanged, Actor: "human"})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	setSectionSigned(t, db, "sec-1")
}

func judgedOnSigned(text string) map[string]any {
	return map[string]any{"section_id": "sec-1", "base_version": 1, "anchor": signedPassage,
		"claim_origin": "model_knowledge", "operation": "insert_after", "text": text}
}

func judgedOnDraft(text string) map[string]any {
	return map[string]any{"section_id": "sec-2", "base_version": 1, "anchor": "La muestra alcanza.",
		"claim_origin": "model_knowledge", "operation": "insert_after", "text": text}
}

func stepOn(risk string, texts ...map[string]any) string {
	raw, _ := json.Marshal(map[string]any{"stretch": "build", "triage_line": "Construir: sigo.",
		"closing_risk": risk, "note": "Sigo.", "new_text": texts})
	return string(raw)
}

// The step that closes sec-2 also leaves a proposal on signed sec-1: the first call
// has sec-1's text and may name a flaw there; its still_open moves sec-1 to «cambió».
func TestB67_ARoundCoversTheSignedSectionWhereItsStepLeftAProposal(t *testing.T) {
	db, runner, cap, _ := newCaptureRunner(t,
		textFixture(t, stepOn("figures_and_calculations", judgedOnSigned("Añado una nota al diagnóstico."), judgedOnDraft("Se revisa la muestra."))),
		verifyFlaws(t, oneFlaw("sec-1", signedPassage, "figures_and_calculations")),
		textFixture(t, stepOn("none", judgedDraftAt(2, "Se revisa otra vez."))),
		verifyRecheck(t, flawStillOpen(1, signedPassage)))
	seedSignedAndDraft(t, db)
	if err := runner.RunTurn(context.Background(), "p1", "turn-1", "Revisa."); err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	if len(cap.verifies) != 2 {
		t.Fatalf("judge calls: %d, want the first and the recheck", len(cap.verifies))
	}
	for i, call := range cap.verifies {
		if text := itemsText(call.Input); !strings.Contains(text, judgedText) || !strings.Contains(text, "Se revisa la muestra.") {
			t.Fatalf("call %d lacks the current text of a covered section:\n%s", i, text)
		}
	}
	if text := itemsText(cap.verifies[0].Input); strings.Contains(text, "Añado una nota") {
		t.Fatalf("the judge read the proposal, not the current text:\n%s", text)
	}
	if text := itemsText(cap.sent[1]); !strings.Contains(text, "Flaw 1 · risk figures_and_calculations · section sec-1.") {
		t.Fatalf("the batch does not say where the flaw is:\n%s", text)
	}
	oqs := flawOQs(t, db)
	if len(oqs) != 1 || oqs[0].State != "open" {
		t.Fatalf("flaw «por aclarar»: %+v", oqs)
	}
	st, pendings := status(t, db, "sec-1")
	if st != graph.StatusChangedSinceSignoff || !hasPending(pendings, graph.PendingOpenQuestion, oqs[0].Label) {
		t.Fatalf("sec-1: %s %v, want changed_since_signoff with its «por aclarar»", st, pendings)
	}
	if _, pendings := status(t, db, "sec-2"); hasPending(pendings, graph.PendingOpenQuestion, oqs[0].Label) {
		t.Fatalf("the failure in sec-1 blocks sec-2: %v", pendings)
	}
}

// The batch leaves the proposal: the recheck reads sec-1's current text, not the
// proposal's; a failure there moves it to «cambió», and the section stayed signed
// until that recheck.
func TestB67_TheRecheckReadsTheSignedSectionWhereTheBatchLeftAProposal(t *testing.T) {
	passage := "La muestra alcanza."
	db, runner, cap, _ := newCaptureRunner(t,
		textFixture(t, stepOn("figures_and_calculations", judgedOnDraft("Se revisa la muestra."))),
		verifyFlaws(t, oneFlaw("sec-2", passage, "figures_and_calculations")),
		textFixture(t, stepOn("none", judgedOnSigned("Corrijo el diagnóstico."), judgedDraftAt(2, "Se revisa otra vez."))),
		verifyRecheck(t, flawStillOpen(1, signedPassage)))
	seedSignedAndDraft(t, db)
	if err := runner.RunTurn(context.Background(), "p1", "turn-1", "Revisa."); err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	if len(cap.verifies) != 2 {
		t.Fatalf("judge calls: %d", len(cap.verifies))
	}
	if text := itemsText(cap.verifies[0].Input); strings.Contains(text, judgedText) {
		t.Fatalf("the first call read sec-1, where no proposal was left yet:\n%s", text)
	}
	recheck := itemsText(cap.verifies[1].Input)
	if !strings.Contains(recheck, judgedText) || strings.Contains(recheck, "Corrijo el diagnóstico.") {
		t.Fatalf("the recheck reads the current text of sec-1:\n%s", recheck)
	}
	oqs := flawOQs(t, db)
	if len(oqs) != 1 {
		t.Fatalf("flaw «por aclarar»: %+v", oqs)
	}
	if st, _ := status(t, db, "sec-1"); st != graph.StatusChangedSinceSignoff {
		t.Fatalf("sec-1: %s", st)
	}
	var open []store.Object
	all, err := db.ListObjects(context.Background(), "p1", store.ObjectProposal)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range all {
		if p.State == "open" {
			open = append(open, p)
		}
	}
	if len(open) != 1 {
		t.Fatalf("the proposal stays open: %+v", all)
	}
}

// A failure in the closed section leaves the covered signed section signed, and a
// passage literal in neither is the retry's error of shape.
func TestB67_AFailureOnlyInTheClosedSectionLeavesTheSignedSectionSigned(t *testing.T) {
	passage := "La muestra alcanza."
	db, runner, _, _ := newCaptureRunner(t,
		textFixture(t, stepOn("figures_and_calculations", judgedOnDraft("Se revisa la muestra."))),
		verifyFlaws(t, oneFlaw("sec-2", passage, "figures_and_calculations")),
		textFixture(t, stepOn("none", judgedOnSigned("Corrijo el diagnóstico."))),
		verifyRecheck(t, flawStillOpen(1, "Este pasaje no está en ninguna sección.")),
		verifyRecheck(t, flawStillOpen(1, passage)))
	seedSignedAndDraft(t, db)
	if err := runner.RunTurn(context.Background(), "p1", "turn-1", "Revisa."); err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	oqs := flawOQs(t, db)
	if len(oqs) != 1 {
		t.Fatalf("flaw «por aclarar»: %+v", oqs)
	}
	if st, _ := status(t, db, "sec-1"); st != graph.StatusSigned {
		t.Fatalf("sec-1: %s, want signed", st)
	}
	if _, pendings := status(t, db, "sec-2"); !hasPending(pendings, graph.PendingOpenQuestion, oqs[0].Label) {
		t.Fatalf("sec-2 pendings: %v", pendings)
	}
}

// A signed section where no step left a proposal stays out of the round: only the
// section the step closed is judged, as before.
func TestB67_ASignedSectionWithoutAProposalStaysOutOfTheRound(t *testing.T) {
	db, runner, cap, _ := newCaptureRunner(t,
		textFixture(t, stepOn("figures_and_calculations", judgedOnDraft("Se revisa la muestra."))),
		judgeVerifyFixture(t, map[string]any{"flaws": []any{}, "recheck": []any{}}))
	seedSignedAndDraft(t, db)
	if err := runner.RunTurn(context.Background(), "p1", "turn-1", "Revisa."); err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	if len(cap.verifies) != 1 || strings.Contains(itemsText(cap.verifies[0].Input), judgedText) {
		t.Fatalf("the judge read a signed section with no proposal: %d calls", len(cap.verifies))
	}
	if st, _ := status(t, db, "sec-1"); st != graph.StatusSigned {
		t.Fatalf("sec-1: %s", st)
	}
}

// judgedDraftAt is a text on sec-2 from the version the earlier step left.
func judgedDraftAt(version int, text string) map[string]any {
	m := judgedOnDraft(text)
	m["base_version"] = version
	return m
}

// addJudgedDraft is one more draft section the steps close.
func addJudgedDraft(t *testing.T, db *store.DB, title, text string) {
	t.Helper()
	err := db.Write(context.Background(), "p1", func(tx *store.Tx) error {
		_, err := tx.CreateObject(store.ObjectSection, "draft", map[string]any{"title": title, "text": text,
			"role": map[string]any{"value": "other", "state": "confirmed"}, "open_changes": []any{}},
			store.EventInput{Type: store.EventStateChanged, Actor: "human"})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
}

// A recheck that could not verify does not lose the signed section its batch left a
// proposal on: the batch's step carries no risk, so only the round remembers it.
func TestB67_AFailedRecheckLeavesTheBatchsSignedSectionForTheNextChain(t *testing.T) {
	passage := "La muestra alcanza."
	wrong := verifyRecheck(t, flawStillOpen(9, passage))
	db, runner, cap, _ := newCaptureRunner(t,
		textFixture(t, stepOn("figures_and_calculations", judgedOnDraft("Se revisa la muestra."))),
		verifyFlaws(t, oneFlaw("sec-2", passage, "figures_and_calculations")),
		textFixture(t, stepOn("none", judgedOnSigned("Corrijo el diagnóstico."))),
		wrong, wrong,
		loadAppFixture(t, "04-principal-no-work.json"),
		judgeVerifyFixture(t, map[string]any{"flaws": []any{}, "recheck": []any{}}))
	seedSignedAndDraft(t, db)
	ctx := context.Background()
	if err := runner.RunTurn(ctx, "p1", "turn-1", "Revisa."); err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	if len(cap.verifies) != 3 {
		t.Fatalf("judge calls in the chain: %d, want the first and a recheck that failed twice", len(cap.verifies))
	}
	if err := runner.RunTurn(ctx, "p1", "turn-2", "Sigue."); err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	if len(cap.verifies) != 4 || !strings.Contains(itemsText(cap.verifies[3].Input), judgedText) {
		t.Fatalf("the next chain did not read the signed section: %d calls", len(cap.verifies))
	}
}

// A flaw in a covered section shows there, not in the round's section.
func TestB67_AFlawInACoveredSectionIsShownInThatSection(t *testing.T) {
	db := openTemp(t)
	seedSignedAndDraft(t, db)
	ctx := context.Background()
	err := db.Write(ctx, "p1", func(tx *store.Tx) error {
		closed, err := tx.GetObject(ctx, "p1", "sec-2")
		if err != nil {
			return err
		}
		signed, err := tx.GetObject(ctx, "p1", "sec-1")
		if err != nil {
			return err
		}
		_, err = tx.CreateObject(store.ObjectRound, "open", cycle.RoundBody{Kind: "judge", SectionID: closed.ID, Covered: []string{signed.ID},
			JudgeStatus: cycle.JudgeDone, Flaws: []cycle.JudgeFlaw{{N: 1, Section: "sec-1", Passage: signedPassage, Risk: "unverifiable", Scenario: "1. Falla."}}},
			store.EventInput{Type: store.EventStateChanged, Actor: "app"})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	flawIn := func(label string) []graph.Warning {
		list, err := graph.SectionWarnings(ctx, db, "p1", label)
		if err != nil {
			t.Fatal(err)
		}
		var out []graph.Warning
		for _, w := range list {
			if w.Kind == graph.WarningFlaw {
				out = append(out, w)
			}
		}
		return out
	}
	if got := flawIn("sec-1"); len(got) != 1 || got[0].End == 0 {
		t.Fatalf("sec-1: %+v, want the flaw underlined", got)
	}
	if got := flawIn("sec-2"); len(got) != 0 {
		t.Fatalf("sec-2: %+v, want none", got)
	}
}

// One batch reads a signed section once, also with two rounds waiting.
func TestB67_OneBatchReadsASignedSectionOnce(t *testing.T) {
	two := judgedOnDraft("Se revisa la muestra.")
	three := map[string]any{"section_id": "sec-3", "base_version": 1, "anchor": "Tres alcanza.",
		"claim_origin": "model_knowledge", "operation": "insert_after", "text": "Se revisa la tercera."}
	db, runner, cap, _ := newCaptureRunner(t,
		textFixture(t, stepOn("figures_and_calculations", two, three)),
		verifyFlaws(t, oneFlaw("sec-2", "La muestra alcanza.", "figures_and_calculations")),
		verifyFlaws(t, oneFlaw("sec-3", "Tres alcanza.", "figures_and_calculations")),
		textFixture(t, stepOn("none", judgedOnSigned("Corrijo."))),
		verifyRecheck(t, flawStillOpen(1, signedPassage)),
		verifyRecheck(t, flawStillOpen(1, "Tres alcanza.")))
	seedSignedAndDraft(t, db)
	addJudgedDraft(t, db, "Tres", "# Tres\n\nTres alcanza.")
	if err := runner.RunTurn(context.Background(), "p1", "turn-1", "Revisa."); err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	reads := 0
	for _, call := range cap.verifies[2:] {
		if strings.Contains(itemsText(call.Input), judgedText) {
			reads++
		}
	}
	if len(cap.verifies) != 4 || reads != 1 {
		t.Fatalf("judge calls %d, reads of the signed section in the batch: %d", len(cap.verifies), reads)
	}
	if st, _ := status(t, db, "sec-1"); st != graph.StatusChangedSinceSignoff {
		t.Fatalf("sec-1: %s", st)
	}
}
