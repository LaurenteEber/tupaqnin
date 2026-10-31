package cycle_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LaurenteEber/tupaqnin/internal/checks"
	"github.com/LaurenteEber/tupaqnin/internal/cycle"
	"github.com/LaurenteEber/tupaqnin/internal/graph"
	"github.com/LaurenteEber/tupaqnin/internal/memory"
	"github.com/LaurenteEber/tupaqnin/internal/provider"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// Piece 2.3 (backlog-r2): the checker and its probes against the
// contradiction. Its outputs are hand-made recordings under
// internal/provider/testdata/checker/; Eber records real ones later.

const checkerPremise = "El agua embotellada es más segura que la del grifo."

// seedCheckerProject is seedStepProject with a premise: the probes of a
// change on sec-1 are req-1, crit-1, prem-1 and dec-1 (yours).
func seedCheckerProject(t *testing.T) *store.DB {
	t.Helper()
	db := seedStepProject(t)
	err := db.Write(context.Background(), "p1", func(tx *store.Tx) error {
		req, err := tx.GetObject(context.Background(), "p1", "req-1")
		if err != nil {
			return err
		}
		_, err = tx.CreateObject(store.ObjectPremise, "current",
			map[string]any{"text": checkerPremise, "request_id": req.ID, "exploratory": false},
			store.EventInput{Type: store.EventStateChanged, Actor: "human"})
		return err
	})
	if err != nil {
		t.Fatalf("seed premise: %v", err)
	}
	return db
}

func checkerFixture(t *testing.T, name string) provider.Fixture {
	t.Helper()
	fx, err := provider.LoadFixture(filepath.Join("..", "provider", "testdata", "checker", name+".json"))
	if err != nil {
		t.Fatalf("LoadFixture: %v", err)
	}
	return fx
}

// stepWriting is a step that replaces «Cierre.» of sec-1 with text.
func stepWriting(t *testing.T, text, extra string) provider.Fixture {
	t.Helper()
	out := `{"stretch":"build","triage_line":"Construir.","closing_risk":"none","note":"n",` + extra +
		`"new_text":[{"section_id":"sec-1","base_version":1,"anchor":"Cierre.","claim_origin":"material","operation":"replace","text":` +
		mustJSON(t, text) + `}]}`
	return textFixture(t, out)
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return string(b)
}

// runCheckedTurn runs one turn over db: the given step, then a step
// without work that stops the chain; the checker gets the checker
// recordings among fixtures.
func runCheckedTurn(t *testing.T, db *store.DB, wrap func(provider.Provider) provider.Provider, fixtures ...provider.Fixture) *captureProvider {
	t.Helper()
	fixtures = append(fixtures, loadAppFixture(t, "04-principal-no-work.json"))
	fake := provider.NewFake(fixtures...)
	fake.Recorder = db
	var inner provider.Provider = fake
	if wrap != nil {
		inner = wrap(fake)
	}
	cap := &captureProvider{inner: inner}
	count := func(ctx context.Context, input []json.RawMessage) (int, error) {
		return fake.CountTokens(ctx, provider.PrincipalModel, input)
	}
	runner := cycle.NewRunner(db, cap, count, &fakePublisher{})
	if err := runner.RunTurn(context.Background(), "p1", "turn-1", "Escribe el cierre."); err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	return cap
}

func probeEvents(t *testing.T, db *store.DB, kind string) []map[string]any {
	t.Helper()
	var out []map[string]any
	for _, ev := range eventsOfType(t, db, store.EventProbe) {
		data := eventData(t, ev)
		if data["kind"] == kind {
			out = append(out, data)
		}
	}
	return out
}

func briefOf(t *testing.T, req provider.Request) string {
	t.Helper()
	if len(req.Input) != 1 {
		t.Fatalf("checker input: %d items, want 1 (a clean context)", len(req.Input))
	}
	return lastUserText(t, req.Input)
}

// B2-7, flujos §1: a recorded probe «contradicts» on the premise opens a
// «por aclarar» with the checker's question and options (B-44), marked
// «no lo declaró el agente»; each probe leaves a probe event with what
// is protected, the verdict, the passage and whether the principal had
// declared it. One checker call per step, Spark 1.3 low, clean context.
func TestB2x7_ContradictsOnThePremiseOpensAnOpenQuestionUndeclaredByTheAgent(t *testing.T) {
	db := seedCheckerProject(t)
	newText := "El agua del grifo es igual de segura que la embotellada."
	cap := runCheckedTurn(t, db, nil, stepWriting(t, newText, ""), checkerFixture(t, "contradicts-premise"))

	if len(cap.checker) != 1 {
		t.Fatalf("checker calls: %d, want 1 (the step without work brings nothing new)", len(cap.checker))
	}
	req := cap.checker[0]
	if req.Model != provider.CheckerModel || req.Effort != provider.EffortLow || req.Role != provider.RoleChecker || req.InstanceID != "" {
		t.Fatalf("checker request: model %q effort %q role %q instance %q", req.Model, req.Effort, req.Role, req.InstanceID)
	}
	brief := briefOf(t, req)
	for _, want := range []string{"prem-1 (premise): " + checkerPremise, "req-1 (request)", "crit-1 (criterion)",
		"dec-1 (decision, human)", "New passage of section sec-1", newText} {
		if !strings.Contains(brief, want) {
			t.Fatalf("brief lacks %q:\n%s", want, brief)
		}
	}
	if strings.Contains(brief, "dec-2") {
		t.Fatalf("brief carries dec-2, an agent decision neither critical nor confirmed:\n%s", brief)
	}

	oq, body := object(t, db, "oq-1")
	if oq.Type != store.ObjectOpenQuestion || oq.State != "open" {
		t.Fatalf("oq-1: %s %s, want an open open_question", oq.Type, oq.State)
	}
	sec, _ := object(t, db, "sec-1")
	prem, _ := object(t, db, "prem-1")
	if body["undeclared_by_agent"] != true || body["origin"] != "checker" || body["kind"] != "contradiction" ||
		body["protected_id"] != prem.ID || body["section_id"] != sec.ID || body["new_passage"] != newText ||
		!strings.HasPrefix(body["question"].(string), "¿El agua del grifo") {
		t.Fatalf("oq-1 body: %v", body)
	}
	if opts, _ := body["options"].([]any); len(opts) != 2 {
		t.Fatalf("oq-1 options: %v, want the checker's two", body["options"])
	}
	if deps := anyStrings(body["depends_on"]); len(deps) != 1 || deps[0] != sec.ID {
		t.Fatalf("oq-1 depends_on: %v, want the section (the premise is not a decision)", deps)
	}

	probes := probeEvents(t, db, graph.ProbeKindProbe)
	if len(probes) != 4 {
		t.Fatalf("probe events: %d, want one per probe (4)", len(probes))
	}
	verdicts := map[string]any{}
	for _, p := range probes {
		verdicts[p["protected"].(string)] = p["verdict"]
		if p["declared"] != false || strings.Join(anyStrings(p["sections"]), ",") != "sec-1" {
			t.Fatalf("probe event: %v, want declared false over sec-1", p)
		}
		if p["protected"] == "prem-1" && p["passage"] != newText {
			t.Fatalf("prem-1 probe passage: %v", p["passage"])
		}
	}
	if verdicts["prem-1"] != "contradicts" || verdicts["req-1"] != "no" || verdicts["dec-1"] != "no" {
		t.Fatalf("verdicts: %v", verdicts)
	}
	cands := probeEvents(t, db, graph.ProbeKindCandidate)
	if len(cands) != 1 || cands[0]["open_question"] != "oq-1" || cands[0]["type"] != "contradiction" {
		t.Fatalf("candidate events: %v", cands)
	}
	// The checker creates no edges (No hacer): nothing goes out of oq-1.
	if edges, err := db.EdgesFrom(context.Background(), "p1", oq.ID); err != nil || len(edges) != 0 {
		t.Fatalf("edges from oq-1: %v, %v", edges, err)
	}
}

// B2-7: a probe «doubt» opens it too (fichas §8.2: a doubt is treated
// as a contradiction).
func TestB2x7_DoubtAlsoOpensAnOpenQuestion(t *testing.T) {
	db := seedCheckerProject(t)
	runCheckedTurn(t, db, nil, stepWriting(t, "El agua del grifo es igual de segura que la embotellada.", ""),
		checkerFixture(t, "doubt-premise"))
	oq, body := object(t, db, "oq-1")
	if oq.State != "open" || body["undeclared_by_agent"] != true {
		t.Fatalf("oq-1: %s %v, want open and undeclared by the agent", oq.State, body)
	}
	for _, p := range probeEvents(t, db, graph.ProbeKindProbe) {
		if p["protected"] == "prem-1" && p["verdict"] != "doubt" {
			t.Fatalf("prem-1 verdict: %v, want doubt", p["verdict"])
		}
	}
}

// B2-7: when the principal had proposed it as «por aclarar» (its
// proposed open question depends on the section), the checker does not
// open a second one and the probe says declared.
func TestB2x7_DeclaredByThePrincipalOpensNoSecondOpenQuestion(t *testing.T) {
	db := seedCheckerProject(t)
	proposed := `"proposed_open_questions":[{"id":"new-1","question":"¿Grifo o embotellada?","options":[` +
		`{"option":"Grifo","contrast":"c","consequence":"q","risk":"r"},{"option":"Embotellada","contrast":"c","consequence":"q","risk":"r"}],` +
		`"depends_on":["sec-1"]}],`
	runCheckedTurn(t, db, nil, stepWriting(t, "El agua del grifo es igual de segura que la embotellada.", proposed),
		checkerFixture(t, "contradicts-premise"))
	if objs, err := db.ListObjects(context.Background(), "p1", store.ObjectOpenQuestion); err != nil || len(objs) != 0 {
		t.Fatalf("open questions: %v, %v; want none from the checker", objs, err)
	}
	for _, p := range probeEvents(t, db, graph.ProbeKindProbe) {
		if p["protected"] == "prem-1" && p["declared"] != true {
			t.Fatalf("prem-1 probe: %v, want declared", p)
		}
	}
}

// B2-7, bloque-2 §5: the probes always carry every current critical
// decision, even when the change does not touch it; besides, the
// confirmed ones linked to what it touched, never the others; the
// confirmed understanding with its cases, not the unconfirmed ones.
func TestB2x7_ProbesAlwaysIncludeEveryCurrentCriticalDecision(t *testing.T) {
	db := seedCheckerProject(t)
	ctx := context.Background()
	var sec2 string
	err := db.Write(ctx, "p1", func(tx *store.Tx) error {
		human := store.EventInput{Type: store.EventStateChanged, Actor: "human"}
		sec1, _ := tx.GetObject(ctx, "p1", "sec-1")
		crit, _ := tx.GetObject(ctx, "p1", "crit-1")
		s2, err := tx.CreateObject(store.ObjectSection, "draft", map[string]any{
			"title": "Método", "text": "Medimos con filtros.", "role": map[string]any{"value": "other", "state": "confirmed"},
			"open_changes": []any{},
		}, human)
		if err != nil {
			return err
		}
		sec2 = s2.ID
		// dec-3 critical (your mark) in sec-1 only; dec-4 confirmed in
		// sec-1; dec-5 confirmed in sec-2; dec-6 superseded and marked.
		for _, d := range []struct {
			state, statement, section string
			marked                    bool
		}{
			{"agent", "Contar solo microplásticos.", sec1.ID, true},
			{"confirmed", "Usar promedios.", sec1.ID, false},
			{"confirmed", "Medir en laboratorio.", s2.ID, false},
			{"superseded", "Medir en campo.", s2.ID, true},
		} {
			dec, err := tx.CreateObject(store.ObjectDecision, d.state, map[string]any{
				"statement": d.statement, "origin": "agent", "human_marked_critical": d.marked,
			}, human)
			if err != nil {
				return err
			}
			if _, err := tx.AddEdge(store.EdgeUsedIn, dec.ID, d.section, "", nil, store.EventInput{Type: store.EventEdge, Actor: "agent"}); err != nil {
				return err
			}
		}
		for _, c := range []struct{ state, text string }{{"confirmed", "Un estudio de laboratorio cuenta."}, {"unconfirmed", "Una nota de prensa no cuenta."}} {
			if _, err := tx.CreateObject(store.ObjectUnderstandingCase, c.state,
				map[string]any{"text": c.text, "counts": c.state == "confirmed", "criterion_id": crit.ID}, human); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	change := checks.Change{Kind: checks.AgentStep, Sections: []string{sec2},
		Passages: []checks.Passage{{Section: sec2, Text: "Medimos con filtros."}}}
	probes, err := checks.Probes(ctx, db, "p1", change)
	if err != nil {
		t.Fatalf("Probes: %v", err)
	}
	var got []string
	for _, p := range probes {
		got = append(got, p.Protected)
	}
	if want := "req-1,case-1,crit-1,prem-1,dec-1,dec-3,dec-5"; strings.Join(got, ",") != want {
		t.Fatalf("probes: %v, want %s", got, want)
	}
	brief, err := checks.Brief(ctx, db, "p1", change, probes)
	if err != nil {
		t.Fatalf("Brief: %v", err)
	}
	for _, want := range []string{"dec-3 (decision, agent): Contar solo microplásticos.",
		"case-1 (understanding case of crit-1, counts): Un estudio de laboratorio cuenta.", "New passage of section sec-2 «Método»"} {
		if !strings.Contains(brief, want) {
			t.Fatalf("brief lacks %q:\n%s", want, brief)
		}
	}
}

// A-2, backlog-r2 2.3: a citation whose passage is literal in its source
// but does not support the claim leaves «fuente que no sostiene» with
// the cause does_not_support; the one whose passage is not in the
// source keeps it with passage_not_found (status.go, as before). The
// checker receives the pair {claim, source passage, source label and
// fingerprint} only for the citation whose passage is in the source.
func TestA2_CitationWithItsPassageInTheSourceThatDoesNotSupportTheClaim(t *testing.T) {
	db := seedCheckerProject(t)
	cap := runCheckedTurn(t, db, nil, textFixture(t, string(stepFile(t, "citations"))), checkerFixture(t, "source-not-supporting"))
	if len(cap.checker) != 1 {
		t.Fatalf("checker calls: %d, want 1", len(cap.checker))
	}
	brief := briefOf(t, cap.checker[0])
	if !strings.Contains(brief, "cit-1 in sec-1: claim «Los estudios usan filtros.» · source src-1") ||
		!strings.Contains(brief, "«Los filtros finos retienen más partículas.»") || strings.Contains(brief, "cit-2") {
		t.Fatalf("brief citations:\n%s", brief)
	}
	_, pendings := status(t, db, "sec-1")
	var causes []string
	for _, p := range pendings {
		if p.Kind == graph.PendingSourceNotSupporting {
			causes = append(causes, p.About+":"+p.Cause)
		}
	}
	if strings.Join(causes, ",") != "cit-2:passage_not_found,cit-1:does_not_support" {
		t.Fatalf("source_not_supporting pendings: %v (all: %v)", causes, pendings)
	}
	agent, err := memory.New(db).AgentPendings(context.Background(), "p1")
	if err != nil || !strings.Contains(strings.Join(agent, "\n"), "source_not_supporting: cit-1 (does_not_support)") {
		t.Fatalf("agent pendings: %v, %v", agent, err)
	}
	// It closes when the claim leaves the text.
	_, b := object(t, db, "sec-1")
	yourEdit(t, db, "sec-1", strings.Replace(b["text"].(string), "Los estudios usan filtros.", "", 1))
	if _, pendings := status(t, db, "sec-1"); hasPending(pendings, graph.PendingSourceNotSupporting, "cit-1") {
		t.Fatalf("pendings after removing the claim: %v", pendings)
	}
}

// B-15, fichas §3 «relato del proceso»: «tras la revisión del martes»
// leaves the agent's pending process_narrative with its passage, which
// reaches the agent's next triage and closes when the passage leaves
// the text.
func TestB15_ProcessNarrativeLeavesAnAgentPending(t *testing.T) {
	db := seedCheckerProject(t)
	passage := "Tras la revisión del martes, ajustamos el filtro."
	runCheckedTurn(t, db, nil, stepWriting(t, passage, ""), checkerFixture(t, "process-narrative"))
	st, pendings := status(t, db, "sec-1")
	if st != graph.StatusDraft || !hasPending(pendings, graph.PendingProcessNarrative, passage) {
		t.Fatalf("sec-1: %s %v, want draft with process_narrative", st, pendings)
	}
	agent, err := memory.New(db).AgentPendings(context.Background(), "p1")
	if err != nil {
		t.Fatalf("AgentPendings: %v", err)
	}
	if !strings.Contains(strings.Join(agent, "\n"), "sec-1 «Síntesis» · process_narrative: "+passage) {
		t.Fatalf("agent pendings: %v", agent)
	}
	_, b := object(t, db, "sec-1")
	yourEdit(t, db, "sec-1", strings.Replace(b["text"].(string), passage, "Fin.", 1))
	if _, pendings := status(t, db, "sec-1"); hasPending(pendings, graph.PendingProcessNarrative, "") {
		t.Fatalf("pendings after removing the passage: %v", pendings)
	}
}

// editWhileChecking edits sec-1 while the checker answers: the text
// changed while the app waited for it (B-50).
type editWhileChecking struct {
	inner      provider.Provider
	db         *store.DB
	t          *testing.T
	from, with string
}

func (e *editWhileChecking) Respond(ctx context.Context, req provider.Request, onDelta func(provider.Delta)) (provider.Response, error) {
	if req.TurnKind == provider.TurnChecker {
		_, b := object(e.t, e.db, "sec-1")
		yourEdit(e.t, e.db, "sec-1", strings.Replace(b["text"].(string), e.from, e.with, 1))
	}
	return e.inner.Respond(ctx, req, onDelta)
}

// B-50: the checker's results are applied against the current text: a
// candidate whose passage left the section while the app waited for the
// checker is discarded, with its probe event; the others apply.
func TestB50_CandidateWhosePassageLeftTheTextIsDiscardedAndTheOthersApply(t *testing.T) {
	db := seedCheckerProject(t)
	narrative := "Tras la revisión del martes, ajustamos el filtro."
	kept := "Usamos solo filtros finos."
	wrap := func(p provider.Provider) provider.Provider {
		return &editWhileChecking{inner: p, db: db, t: t, from: narrative, with: ""}
	}
	runCheckedTurn(t, db, wrap, stepWriting(t, narrative+" "+kept, ""), checkerFixture(t, "discarded-passage"))
	cands := probeEvents(t, db, graph.ProbeKindCandidate)
	if len(cands) != 2 {
		t.Fatalf("candidate events: %d, want 2", len(cands))
	}
	for _, c := range cands {
		discarded := c["discarded"] == true
		if (c["type"] == "process_narrative") != discarded {
			t.Fatalf("candidate %v: discarded %v", c["type"], discarded)
		}
	}
	_, pendings := status(t, db, "sec-1")
	if hasPending(pendings, graph.PendingProcessNarrative, "") || !hasPending(pendings, graph.PendingTextWithoutDecision, kept) {
		t.Fatalf("pendings: %v, want only the kept candidate's text_without_decision", pendings)
	}
}

// B-50, fichas §7: two shape failures of the checker leave «sin
// comprobar» on the step's section, which blocks its signoff, not the
// work; the next valid judgment over the section closes it.
func TestB50_TwoShapeFailuresLeaveUncheckedAndBlockTheSignoff(t *testing.T) {
	db := seedCheckerProject(t)
	cap := runCheckedTurn(t, db, nil, stepWriting(t, "Fin del texto.", ""),
		checkerFixture(t, "malformed"), checkerFixture(t, "malformed"))
	if len(cap.checker) != 2 || len(cap.checker[1].Input) != 3 {
		t.Fatalf("checker calls: %d, want the call and its retry with the errors", len(cap.checker))
	}
	if len(cap.sent) != 2 {
		t.Fatalf("principal calls: %d, want 2: the failure does not stop the chain", len(cap.sent))
	}
	var results []string
	for _, ev := range eventsOfType(t, db, store.EventOutputRetry) {
		data := eventData(t, ev)
		if data["turn_kind"] == "checker" && ev.AgentRole == "checker" {
			results = append(results, data["result"].(string)+":"+strings.Join(anyStrings(data["sections"]), ","))
		}
	}
	if strings.Join(results, " ") != "retry:sec-1 unchecked:sec-1" {
		t.Fatalf("checker output_retry: %v", results)
	}
	if n := len(eventsOfType(t, db, store.EventProbe)); n != 0 {
		t.Fatalf("probe events: %d, want none from a failed judgment", n)
	}
	st, pendings := status(t, db, "sec-1")
	if st != graph.StatusDraft || len(pendings) != 1 || pendings[0].Kind != graph.PendingUnchecked {
		t.Fatalf("sec-1: %s %v, want draft with unchecked only", st, pendings)
	}
	open, err := cycle.NewGate(db).RequestSignoff(context.Background(), "p1", "sec-1")
	if err != nil || open.Opened || !hasPending(open.Pendings, graph.PendingUnchecked, "") {
		t.Fatalf("RequestSignoff: %+v, %v; want blocked by unchecked", open, err)
	}

	sec, _ := object(t, db, "sec-1")
	change := checks.Change{Kind: checks.AgentStep, Sections: []string{sec.ID}}
	out := checks.CheckerOutput{Probes: []checks.ProbeResult{{Protected: "req-1", Verdict: checks.VerdictNo}}}
	if err := checks.Apply(context.Background(), db, "p1", "turn-2", change, out); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if st, pendings := status(t, db, "sec-1"); st != graph.StatusReady || len(pendings) != 0 {
		t.Fatalf("after a valid judgment: %s %v, want ready", st, pendings)
	}
}

// B-36: your edits go to the checker in one batch when you send a
// message, before the agent's step; an edit already judged does not go
// again.
func TestB36_YourEditsGoToTheCheckerInOneBatchWhenYouSendAMessage(t *testing.T) {
	db := seedCheckerProject(t)
	edited := "La síntesis compara estudios. El agua del grifo es igual de segura."
	yourEdit(t, db, "sec-1", edited)
	fake := provider.NewFake(loadAppFixture(t, "04-principal-no-work.json"))
	fake.Recorder = db
	cap := &captureProvider{inner: fake}
	count := func(ctx context.Context, input []json.RawMessage) (int, error) {
		return fake.CountTokens(ctx, provider.PrincipalModel, input)
	}
	runner := cycle.NewRunner(db, cap, count, &fakePublisher{})
	for _, turn := range []string{"turn-1", "turn-2"} {
		if err := runner.RunTurn(context.Background(), "p1", turn, "Sigue."); err != nil {
			t.Fatalf("RunTurn %s: %v", turn, err)
		}
	}
	if len(cap.checker) != 1 {
		t.Fatalf("checker calls: %d, want 1 (the batch of your edit, once)", len(cap.checker))
	}
	if brief := briefOf(t, cap.checker[0]); !strings.Contains(brief, edited) {
		t.Fatalf("brief lacks your edit:\n%s", brief)
	}
	// The fake without a checker recording judges every probe «no».
	if n := len(probeEvents(t, db, graph.ProbeKindProbe)); n != 4 {
		t.Fatalf("probe events: %d, want 4", n)
	}
}

// B-44: the embedded checker schema is the design's, byte for byte.
func TestB44_EmbeddedCheckerSchemaIsTheDesignSchema(t *testing.T) {
	embedded, err := os.ReadFile(filepath.Join("schema", "checker_output.schema.json"))
	if err != nil {
		t.Fatalf("read embedded schema: %v", err)
	}
	design, err := os.ReadFile(filepath.Join("..", "..", "docs", "diseno", "bloque-3", "salida-comprobador.schema.json"))
	if err != nil {
		t.Fatalf("read design schema: %v", err)
	}
	if string(embedded) != string(design) {
		t.Fatal("embedded schema differs from docs/diseno/bloque-3/salida-comprobador.schema.json")
	}
}
