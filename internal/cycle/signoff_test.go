package cycle_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LaurenteEber/tupaqnin/internal/cycle"
	"github.com/LaurenteEber/tupaqnin/internal/graph"
	"github.com/LaurenteEber/tupaqnin/internal/provider"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

const signText = "Comparamos solo estudios con el mismo tamaño de partícula.\n\nAsí la comparación es justa."

// seedSignable creates a section (sec-1, role conclusion) and, when
// critical, an agent decision (dec-1) used in it: critical because the
// section is a conclusion (T-5). No card is open, so nothing is pending.
func seedSignable(t *testing.T, db *store.DB, critical bool) (sectionID, decisionID string) {
	t.Helper()
	err := db.Write(context.Background(), "p1", func(tx *store.Tx) error {
		sec, err := tx.CreateObject(store.ObjectSection, "draft", map[string]any{
			"title": "Regla de comparación", "text": signText,
			"role":         map[string]any{"value": "conclusion", "state": "confirmed"},
			"open_changes": []any{},
		}, store.EventInput{Type: store.EventStateChanged, Actor: "agent"})
		if err != nil {
			return err
		}
		sectionID = sec.ID
		if !critical {
			return nil
		}
		dec, err := tx.CreateObject(store.ObjectDecision, "agent", map[string]any{
			"statement": "Comparar solo estudios con el mismo tamaño de partícula.", "instead_of": "Comparar los conteos tal cual.",
			"would_choose_if": "Todos usaran el mismo filtro.", "assumption": false, "origin": "agent", "human_marked_critical": false,
		}, store.EventInput{Type: store.EventStateChanged, Actor: "agent"})
		if err != nil {
			return err
		}
		decisionID = dec.ID
		_, err = tx.AddEdge(store.EdgeUsedIn, dec.ID, sec.ID, "", store.EventInput{Type: store.EventEdge, Actor: "agent"})
		return err
	})
	if err != nil {
		t.Fatalf("seed signable: %v", err)
	}
	return sectionID, decisionID
}

func wonderOutput(t *testing.T, name string) json.RawMessage {
	t.Helper()
	return recordedOutput(t, filepath.Join("signoff", name+".json"))
}

// fixtureOut is a recording's Wonder output: the tests read their
// expectations from it, so real recordings (make microtest-signoff) keep
// them meaningful.
func fixtureOut(t *testing.T, name string) graph.WonderOutput {
	t.Helper()
	var out graph.WonderOutput
	if err := json.Unmarshal(wonderOutput(t, name), &out); err != nil {
		t.Fatalf("decode %s: %v", name, err)
	}
	return out
}

func openSignoff(t *testing.T, g *cycle.Gate) string {
	t.Helper()
	open, err := g.RequestSignoff(context.Background(), "p1", "sec-1")
	if err != nil || !open.Opened {
		t.Fatalf("RequestSignoff: %+v, %v", open, err)
	}
	return open.Label
}

func round(t *testing.T, g *cycle.Gate, sign, fixture string) cycle.SignoffRoundResult {
	t.Helper()
	res, err := g.EndSignoffRound(context.Background(), "p1", "turn-w", sign, 1, wonderOutput(t, fixture),
		provider.ContrasterModel+" "+provider.ContrasterEffort, false)
	if err != nil {
		t.Fatalf("EndSignoffRound %s: %v", fixture, err)
	}
	if res.Status != cycle.StatusAccepted {
		t.Fatalf("EndSignoffRound %s: status %v (%v)", fixture, res.Status, res.Errors)
	}
	return res
}

func view(t *testing.T, db *store.DB, sign string) graph.SignoffView {
	t.Helper()
	v, err := graph.SignoffDetail(context.Background(), db, "p1", sign)
	if err != nil {
		t.Fatalf("SignoffDetail: %v", err)
	}
	return v
}

func sectionState(t *testing.T, db *store.DB, label string) string {
	t.Helper()
	obj, err := db.GetObject(context.Background(), "p1", label)
	if err != nil {
		t.Fatalf("GetObject %s: %v", label, err)
	}
	return obj.State
}

func TestWonderSchemaCopyIsIdenticalToDesign(t *testing.T) {
	embedded, err := os.ReadFile(filepath.Join("schema", "wonder_output.schema.json"))
	if err != nil {
		t.Fatalf("embedded schema: %v", err)
	}
	design, err := os.ReadFile(filepath.Join("..", "..", "docs", "diseno", "bloque-3", "salida-wonder.schema.json"))
	if err != nil {
		t.Fatalf("design schema: %v", err)
	}
	if string(embedded) != string(design) {
		t.Fatal("embedded Wonder schema differs from docs/")
	}
}

// S5 «todo coincide y se firma»; fichas §2: coincide = every critical
// decision match, the origin question answers (E-3); sign stores
// specification §4, creates explains edges and leaves the section signed.
func TestE3_EverythingCoincidesAndSigns(t *testing.T) {
	db := openTemp(t)
	sectionID, decisionID := seedSignable(t, db, true)
	g := cycle.NewGate(db)
	ctx := context.Background()
	sign := openSignoff(t, g)

	if err := g.RecordExplanation(ctx, "p1", sign, "Comparo solo estudios con el mismo tamaño porque si no la diferencia es del filtro."); err != nil {
		t.Fatalf("RecordExplanation: %v", err)
	}
	round(t, g, sign, "round1-all-match")
	v := view(t, db, sign)
	if v.Coincides || v.CanSign {
		t.Fatalf("origin question not answered yet: coincides must be false, got %+v", v)
	}
	if err := g.Sign(ctx, "p1", sign); !errors.Is(err, cycle.ErrSignoffDoesNotCoincide) {
		t.Fatalf("Sign before coinciding: got %v, want ErrSignoffDoesNotCoincide", err)
	}
	if v.OriginQuestion == "" {
		t.Fatal("the origin question from Wonder must be stored")
	}
	if err := g.RecordAnswers(ctx, "p1", sign, "Supongo que todos cuentan desde el mismo tamaño mínimo.", ""); err != nil {
		t.Fatalf("RecordAnswers: %v", err)
	}
	round(t, g, sign, "round2-all-match-answers")
	if v = view(t, db, sign); !v.Coincides || !v.CanSign {
		t.Fatalf("all match and the origin question answers: want coincides and can sign, got %+v", v)
	}
	if err := g.Sign(ctx, "p1", sign); err != nil {
		t.Fatalf("Sign: %v", err)
	}

	if got := sectionState(t, db, "sec-1"); got != "signed" {
		t.Fatalf("section state: got %q, want signed", got)
	}
	if v = view(t, db, sign); v.Phase != graph.SignoffSigned || v.CanSign {
		t.Fatalf("phase: got %+v, want signed", v)
	}
	signObj, _ := db.GetObject(ctx, "p1", sign)
	edges, _ := db.EdgesFrom(ctx, "p1", signObj.ID)
	if len(edges) != 1 || edges[0].Type != store.EdgeExplains || edges[0].ToID != decisionID {
		t.Fatalf("explains edges: got %+v, want one to the critical decision", edges)
	}
	body, _ := graph.ParseSignoff(signObj)
	if body.SectionID != sectionID || body.SectionVersion != 1 || body.SectionFingerprint == "" ||
		len(body.Critical) != 1 || body.Critical[0].ID != decisionID || body.Explanation == "" ||
		body.OriginAnswer == "" || body.ContrastedBy != "muse-wonder-1.0 high" || len(body.Rounds) != 2 {
		t.Fatalf("what the signoff stores (§4): got %+v", body)
	}
}

// S5 «falta en tu explicación: el mensaje nombra qué falta, no la
// respuesta, y llega un caso nuevo».
func TestC2_MissingInExplanationNamesWhatIsMissingAndBringsANewCase(t *testing.T) {
	db := openTemp(t)
	seedSignable(t, db, true)
	g := cycle.NewGate(db)
	sign := openSignoff(t, g)
	if err := g.RecordExplanation(context.Background(), "p1", sign, "Comparo los estudios entre sí."); err != nil {
		t.Fatalf("RecordExplanation: %v", err)
	}
	res := round(t, g, sign, "missing-in-explanation")
	if res.Outcome != cycle.OutcomeCase {
		t.Fatalf("outcome: got %q, want case", res.Outcome)
	}
	v := view(t, db, sign)
	rec := fixtureOut(t, "missing-in-explanation")
	if len(v.Verdicts) != 1 || v.Verdicts[0].Verdict != "missing_in_explanation" || v.Verdicts[0].Missing == "" ||
		v.Verdicts[0].Missing != *rec.Decisions[0].Missing {
		t.Fatalf("verdicts: got %+v, want what is missing named", v.Verdicts)
	}
	if v.OpenCase == "" || v.OpenCase != *rec.NewCase || v.Coincides || v.CanSign {
		t.Fatalf("a new case must arrive and nothing coincides: got %+v", v)
	}
	// The app never puts the answer on the signoff screen: no statement,
	// no contrast of the decision.
	raw, _ := json.Marshal(v)
	if strings.Contains(string(raw), "Comparar solo estudios con el mismo tamaño de partícula.") ||
		strings.Contains(string(raw), "Comparar los conteos tal cual.") {
		t.Fatalf("the view carries the decision's statement or contrast: %s", raw)
	}
	if err := g.RecordAnswers(context.Background(), "p1", sign, "", ""); !errors.Is(err, cycle.ErrCaseAnswerRequired) {
		t.Fatalf("answers without the case answer: got %v, want ErrCaseAnswerRequired", err)
	}
}

// S5 «2 casos mal aplicados pausan la firma y la sección queda donde
// estaba» (E-2).
func TestE2_TwoMisappliedCasesPauseTheSignoff(t *testing.T) {
	db := openTemp(t)
	seedSignable(t, db, true)
	g := cycle.NewGate(db)
	ctx := context.Background()
	sign := openSignoff(t, g)
	before := sectionState(t, db, "sec-1")
	if err := g.RecordExplanation(ctx, "p1", sign, "Comparo los estudios entre sí."); err != nil {
		t.Fatalf("RecordExplanation: %v", err)
	}
	round(t, g, sign, "missing-in-explanation")
	for i := 1; i <= 2; i++ {
		if err := g.RecordAnswers(ctx, "p1", sign, "Del filtro común.", "Entra igual."); err != nil {
			t.Fatalf("RecordAnswers %d: %v", i, err)
		}
		res := round(t, g, sign, "case-does-not-apply")
		want := cycle.OutcomeCase
		if i == 2 {
			want = cycle.OutcomePaused
		}
		if res.Outcome != want {
			t.Fatalf("round %d outcome: got %q, want %q", i, res.Outcome, want)
		}
	}
	v := view(t, db, sign)
	if v.Phase != graph.SignoffPaused || v.FailedCases != 2 || v.CanSign {
		t.Fatalf("after 2 failed cases: got %+v, want paused", v)
	}
	if len(eventsOfType(t, db, store.EventSignoffPaused)) != 1 {
		t.Fatal("want one signoff_paused event")
	}
	if got := sectionState(t, db, "sec-1"); got != before {
		t.Fatalf("section state: got %q, want %q (where it was)", got, before)
	}
	if err := g.RecordAnswers(ctx, "p1", sign, "x", "y"); !errors.Is(err, cycle.ErrSignoffNotInProgress) {
		t.Fatalf("answers on a paused signoff: got %v, want ErrSignoffNotInProgress", err)
	}
	again, err := g.RequestSignoff(ctx, "p1", "sec-1")
	if err != nil || !again.Opened || again.Label == sign {
		t.Fatalf("asking again after a pause creates another sign-n: got %+v, %v", again, err)
	}
}

// A case applied well, then everything is contrasted again and
// coincides (flujos §3).
func TestE3_CaseAppliedWellCoincides(t *testing.T) {
	db := openTemp(t)
	seedSignable(t, db, true)
	g := cycle.NewGate(db)
	sign := openSignoff(t, g)
	_ = g.RecordExplanation(context.Background(), "p1", sign, "Comparo los estudios entre sí.")
	round(t, g, sign, "missing-in-explanation")
	if err := g.RecordAnswers(context.Background(), "p1", sign, "Del filtro común.", "No entra: mide otro tamaño."); err != nil {
		t.Fatalf("RecordAnswers: %v", err)
	}
	if res := round(t, g, sign, "case-applies-well"); res.Outcome != cycle.OutcomeCoincides {
		t.Fatalf("outcome: got %q, want coincides", res.Outcome)
	}
	if v := view(t, db, sign); !v.CanSign || v.Cases[0].Result != "applies_well" {
		t.Fatalf("view: got %+v", v)
	}
}

// S5 «falta en el texto: pendiente del agente» (B-14, B-15): the signoff
// is cancelled with reason missing_in_text and the section keeps the
// passage as a pending until its fingerprint changes.
func TestB15_MissingInTextLeavesAnAgentPendingWithThePassage(t *testing.T) {
	db := openTemp(t)
	sectionID, _ := seedSignable(t, db, true)
	g := cycle.NewGate(db)
	ctx := context.Background()
	sign := openSignoff(t, g)
	_ = g.RecordExplanation(ctx, "p1", sign, "Comparo solo estudios con el mismo tamaño.")
	if res := round(t, g, sign, "missing-in-text"); res.Outcome != cycle.OutcomeMissingInText {
		t.Fatalf("outcome: got %q, want missing_in_text", res.Outcome)
	}
	v := view(t, db, sign)
	if v.Phase != graph.SignoffCancelled || v.Cancelled != graph.CancelMissingInText {
		t.Fatalf("phase: got %+v, want cancelled missing_in_text", v)
	}
	status, pendings, err := graph.SectionStatus(ctx, db, "p1", "sec-1")
	if err != nil {
		t.Fatalf("SectionStatus: %v", err)
	}
	passage := *fixtureOut(t, "missing-in-text").Decisions[0].Passage
	if status != graph.StatusDraft || len(pendings) != 1 || pendings[0].Kind != graph.PendingMissingInText ||
		pendings[0].About != passage {
		t.Fatalf("status/pendings: got %s %+v, want draft with missing_in_text and its passage", status, pendings)
	}
	// The agent fixes the text: the fingerprint changes and it closes.
	err = db.Write(ctx, "p1", func(tx *store.Tx) error {
		sec, err := tx.Object(sectionID)
		if err != nil {
			return err
		}
		var b map[string]any
		_ = json.Unmarshal(sec.Body, &b)
		b["text"] = signText + " Ningún estudio queda fuera sin decirlo."
		_, err = tx.UpdateObject(sectionID, sec.CurrentVersion, sec.State, b, store.EventInput{Type: store.EventEdit, Actor: "agent"})
		return err
	})
	if err != nil {
		t.Fatalf("edit: %v", err)
	}
	if status, pendings, _ = graph.SectionStatus(ctx, db, "p1", "sec-1"); status != graph.StatusReady || len(pendings) != 0 {
		t.Fatalf("after the fingerprint changed: got %s %+v, want ready", status, pendings)
	}
}

// S5 «sección sin decisiones críticas: firma breve» (A-4): no Wonder,
// brief_signoff with «pregunta de origen: no aplica».
func TestA4_SectionWithoutCriticalDecisionsSignsBrief(t *testing.T) {
	db := openTemp(t)
	sectionID, _ := seedSignable(t, db, false)
	g := cycle.NewGate(db)
	ctx := context.Background()
	open, err := g.RequestSignoff(ctx, "p1", "sec-1")
	if err != nil || !open.Opened || !open.Brief {
		t.Fatalf("RequestSignoff: %+v, %v; want a brief signoff", open, err)
	}
	if err := g.Sign(ctx, "p1", open.Label); !errors.Is(err, cycle.ErrSignoffDoesNotCoincide) {
		t.Fatalf("brief signoff without explanation: got %v", err)
	}
	// Through the runner: the explanation starts no Wonder turn; the fake
	// has no recordings, so any model call would fail.
	r := cycle.NewRunner(db, provider.NewFake(), nil, nil)
	turnID, err := r.SignoffExplanation("p1", open.Label, "Cuenta cómo se compararon los estudios.")
	if err != nil || turnID != "" {
		t.Fatalf("brief explanation: turn %q, %v; want no turn", turnID, err)
	}
	if err := g.Sign(ctx, "p1", open.Label); err != nil {
		t.Fatalf("Sign: %v", err)
	}
	if got := sectionState(t, db, "sec-1"); got != "signed" {
		t.Fatalf("section: got %q, want signed", got)
	}
	briefs := eventsOfType(t, db, store.EventBriefSignoff)
	if len(briefs) != 1 || !strings.Contains(string(briefs[0].Data), `"origin_question":"not_applicable"`) {
		t.Fatalf("brief_signoff: got %+v", briefs)
	}
	var calls int
	_ = db.Writer.QueryRowContext(ctx, `SELECT COUNT(*) FROM model_call`).Scan(&calls)
	if calls != 0 {
		t.Fatalf("model calls: got %d, want none", calls)
	}
	_ = sectionID
}

// S4 «al pedir firmar: si quedan pendientes, la respuesta los lista y no
// abre la firma».
func TestB14_AskingToSignWithPendingsListsThemAndOpensNothing(t *testing.T) {
	db := openTemp(t)
	sectionID, _ := seedSignable(t, db, true)
	ctx := context.Background()
	err := db.Write(ctx, "p1", func(tx *store.Tx) error {
		sec, _ := tx.Object(sectionID)
		var b map[string]any
		_ = json.Unmarshal(sec.Body, &b)
		b["text"] = signText + " La diferencia es de 60 veces."
		_, err := tx.UpdateObject(sectionID, sec.CurrentVersion, sec.State, b, store.EventInput{Type: store.EventEdit, Actor: "human"})
		return err
	})
	if err != nil {
		t.Fatalf("edit: %v", err)
	}
	open, err := cycle.NewGate(db).RequestSignoff(ctx, "p1", "sec-1")
	if err != nil {
		t.Fatalf("RequestSignoff: %v", err)
	}
	if open.Opened || len(open.Pendings) != 1 || open.Pendings[0].Kind != graph.PendingFigureWithoutDecision {
		t.Fatalf("got %+v, want not opened with the figure pending", open)
	}
	if signs, _ := db.ListObjects(ctx, "p1", store.ObjectSignoff); len(signs) != 0 {
		t.Fatalf("a sign-n was created: %d", len(signs))
	}
}

// K-11: each answer of yours records act_timed.
func TestK11_EachAnswerOfTheSignoffRecordsActTimed(t *testing.T) {
	db := openTemp(t)
	seedSignable(t, db, true)
	g := cycle.NewGate(db)
	sign := openSignoff(t, g)
	_ = g.RecordExplanation(context.Background(), "p1", sign, "Comparo los estudios entre sí.")
	round(t, g, sign, "missing-in-explanation")
	_ = g.RecordAnswers(context.Background(), "p1", sign, "Del filtro común.", "Entra igual.")
	var answers []string
	for _, ev := range eventsOfType(t, db, store.EventActTimed) {
		var data map[string]any
		_ = json.Unmarshal(ev.Data, &data)
		if data["act"] != "signoff" {
			t.Fatalf("act_timed: got %v", data)
		}
		answers = append(answers, data["part"].(string))
	}
	if strings.Join(answers, ",") != "explanation,origin,case" {
		t.Fatalf("act_timed answers: got %v, want explanation, origin, case", answers)
	}
}

// Rule 4 of specification §3, here too: a new-n in decision is rejected.
func TestR4_WonderNewNInDecisionIsRejected(t *testing.T) {
	db := openTemp(t)
	seedSignable(t, db, true)
	g := cycle.NewGate(db)
	sign := openSignoff(t, g)
	_ = g.RecordExplanation(context.Background(), "p1", sign, "Comparo los estudios entre sí.")
	res, err := g.EndSignoffRound(context.Background(), "p1", "turn-w", sign, 1, wonderOutput(t, "new-n-in-decision"),
		provider.ContrasterModel+" "+provider.ContrasterEffort, false)
	if err != nil {
		t.Fatalf("EndSignoffRound: %v", err)
	}
	if res.Status != cycle.StatusRetry || !strings.Contains(strings.Join(res.Errors, ";"), "new-1 is a new-n") {
		t.Fatalf("got %+v, want a retry naming new-1", res)
	}
	if retries := eventsOfType(t, db, store.EventOutputRetry); len(retries) != 1 {
		t.Fatalf("output_retry: got %d, want 1", len(retries))
	}
	if v := view(t, db, sign); v.Rounds != 0 {
		t.Fatalf("a rejected round is not recorded: %+v", v)
	}
	// The second failure is visible and leaves no agent pending.
	res, _ = g.EndSignoffRound(context.Background(), "p1", "turn-w", sign, 2, wonderOutput(t, "new-n-in-decision"),
		provider.ContrasterModel+" "+provider.ContrasterEffort, false)
	if res.Status != cycle.StatusFailed {
		t.Fatalf("second failure: got %v, want failed", res.Status)
	}
	if _, pendings, _ := graph.SectionStatus(context.Background(), db, "p1", "sec-1"); len(pendings) != 0 {
		t.Fatalf("a failed Wonder round leaves no pending: %+v", pendings)
	}
}

// unavailableWonder fails every Wonder call with err and serves the
// rest from the fake.
type unavailableWonder struct {
	fake  *provider.Fake
	err   error
	calls []string
}

func (u *unavailableWonder) Respond(ctx context.Context, req provider.Request, onDelta func(provider.Delta)) (provider.Response, error) {
	u.calls = append(u.calls, req.Model+" "+req.Effort)
	if req.Model == provider.ContrasterModel {
		return provider.Response{}, u.err
	}
	return u.fake.Respond(ctx, req, onDelta)
}

func signoffFixture(t *testing.T, name string) provider.Fixture {
	t.Helper()
	fx, err := provider.LoadFixture(filepath.Join("..", "provider", "testdata", "signoff", name+".json"))
	if err != nil {
		t.Fatalf("LoadFixture: %v", err)
	}
	return fx
}

// B-20 (c): Wonder answers 5xx after its retry, so Spark 1.3 xhigh
// contrasts and the signoff says so.
func TestB20c_WonderUnavailableFallsBackToSpark13XHigh(t *testing.T) {
	db := openTemp(t)
	seedSignable(t, db, true)
	prov := &unavailableWonder{fake: provider.NewFake(signoffFixture(t, "round1-all-match")),
		err: &provider.ProviderError{Code: "http_503", Type: "http_error", HTTPStatus: 503}}
	r := cycle.NewRunner(db, prov, nil, nil)
	g := cycle.NewGate(db)
	sign := openSignoff(t, g)
	if err := g.RecordExplanation(context.Background(), "p1", sign, "Comparo solo estudios con el mismo tamaño."); err != nil {
		t.Fatalf("RecordExplanation: %v", err)
	}
	if err := r.RunSignoffRound(context.Background(), "p1", "turn-w", sign); err != nil {
		t.Fatalf("RunSignoffRound: %v", err)
	}
	want := []string{"muse-wonder-1.0 high", "muse-wonder-1.0 high", "muse-spark-1.3 xhigh"}
	if strings.Join(prov.calls, ",") != strings.Join(want, ",") {
		t.Fatalf("calls: got %v, want %v (Wonder, its retry, then Spark)", prov.calls, want)
	}
	v := view(t, db, sign)
	if !v.WonderUnavailable || v.ContrastedBy != "muse-spark-1.3 xhigh" || v.Rounds != 1 {
		t.Fatalf("the signoff must say Spark contrasted: got %+v", v)
	}
}

// B-20 (c): a 4xx validation error does not count: it is an error of
// the turn, with no fallback and no round.
func TestB20c_ValidationErrorIsATurnErrorNotUnavailable(t *testing.T) {
	db := openTemp(t)
	seedSignable(t, db, true)
	prov := &unavailableWonder{fake: provider.NewFake(signoffFixture(t, "round1-all-match")),
		err: &provider.ProviderError{Code: "http_400", Type: "invalid_request_error", HTTPStatus: 400}}
	r := cycle.NewRunner(db, prov, nil, nil)
	g := cycle.NewGate(db)
	sign := openSignoff(t, g)
	_ = g.RecordExplanation(context.Background(), "p1", sign, "Comparo solo estudios con el mismo tamaño.")
	_ = r.RunSignoffRound(context.Background(), "p1", "turn-w", sign)
	if len(prov.calls) != 1 {
		t.Fatalf("calls: got %v, want one Wonder call and no fallback", prov.calls)
	}
	if v := view(t, db, sign); v.Rounds != 0 || v.Phase != graph.SignoffInProgress {
		t.Fatalf("got %+v, want no round and the signoff still in progress", v)
	}
}

func TestB20c_UnavailableIsTimeout5xxOr429Only(t *testing.T) {
	for _, c := range []struct {
		err  error
		want bool
	}{
		{context.DeadlineExceeded, true},
		{&provider.ProviderError{Code: "server_error", HTTPStatus: 500}, true},
		{&provider.ProviderError{Code: "http_503", HTTPStatus: 503}, true},
		{&provider.ProviderError{Code: "rate_limit", HTTPStatus: 429}, true},
		{&provider.ProviderError{Code: "http_400", HTTPStatus: 400}, false},
		{&provider.ProviderError{Code: "invalid", HTTPStatus: 422}, false},
		{errors.New("other"), false},
	} {
		if got := provider.Unavailable(c.err); got != c.want {
			t.Fatalf("Unavailable(%v): got %v, want %v", c.err, got, c.want)
		}
	}
	if !provider.Known(provider.ModelWonder10) || provider.Known("muse-wonder-9") {
		t.Fatal("Known must read models.go")
	}
}

// Leaving the signoff cancels it with reason left (B-14, 7g).
func TestB14_LeavingCancelsTheSignoff(t *testing.T) {
	db := openTemp(t)
	seedSignable(t, db, true)
	g := cycle.NewGate(db)
	sign := openSignoff(t, g)
	if err := g.LeaveSignoff(context.Background(), "p1", sign); err != nil {
		t.Fatalf("LeaveSignoff: %v", err)
	}
	if v := view(t, db, sign); v.Phase != graph.SignoffCancelled || v.Cancelled != graph.CancelLeft {
		t.Fatalf("got %+v, want cancelled left", v)
	}
	if active, ok, _ := graph.ActiveSignoff(context.Background(), db, "p1", mustID(t, db, "sec-1")); ok {
		t.Fatalf("no signoff should be in progress: %+v", active)
	}
}

// Asking twice while one is in progress returns the same sign-n.
func TestB14_AskingAgainWhileInProgressReturnsTheSameSignoff(t *testing.T) {
	db := openTemp(t)
	seedSignable(t, db, true)
	g := cycle.NewGate(db)
	first := openSignoff(t, g)
	if second := openSignoff(t, g); second != first {
		t.Fatalf("got %s then %s, want the same signoff", first, second)
	}
}

func mustID(t *testing.T, db *store.DB, label string) string {
	t.Helper()
	obj, err := db.GetObject(context.Background(), "p1", label)
	if err != nil {
		t.Fatalf("GetObject %s: %v", label, err)
	}
	return obj.ID
}

func stepFixture(name, text string) provider.Fixture {
	item, _ := json.Marshal(map[string]any{"type": "message", "role": "assistant",
		"content": []map[string]string{{"type": "output_text", "text": text}}})
	return provider.Fixture{Name: name, TurnKind: provider.TurnStep, Items: []json.RawMessage{item}}
}

// B-14 in the runner: the step's text on the section in signoff is not
// applied, and the next chained step tells the agent so (the chain of
// B-17 is unchanged otherwise).
func TestB14_TheNextChainedStepCarriesTheRejectionForTheAgent(t *testing.T) {
	onSigning := stepFixture("on-signing", `{"triage_line": "Construir.", "closing_risk": "none", "note": "n",
	  "new_text": [{"section_id": "sec-1", "base_version": 1, "anchor": "Así la comparación es justa.", "operation": "replace", "text": "Así es justa."}]}`)
	db, runner, cap, _ := newCaptureRunner(t, onSigning, loadAppFixture(t, "04-principal-no-work.json"))
	seedSignable(t, db, true)
	sign := openSignoff(t, cycle.NewGate(db))
	if err := runner.RunTurn(context.Background(), "p1", "turn-1", "Mejora la sección."); err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	if len(cap.sent) != 2 {
		t.Fatalf("provider calls: %d, want 2", len(cap.sent))
	}
	got := lastUserText(t, cap.sent[1])
	if !strings.HasPrefix(got, "sec-1 is being signed by the person") || !strings.HasSuffix(got, cycle.ContinueMessage) {
		t.Fatalf("second step message: %q", got)
	}
	if obj, _ := db.GetObject(context.Background(), "p1", "sec-1"); obj.CurrentVersion != 1 {
		t.Fatalf("sec-1 was written: version %d", obj.CurrentVersion)
	}
	if v := view(t, db, sign); v.Phase != graph.SignoffInProgress {
		t.Fatalf("the signoff stays in progress: %+v", v)
	}
}

// «Preguntar al agente» asks for an answer without new text (B-14):
// the principal is told so, and nothing of its answer is applied.
func TestB14_AskingTheAgentFromTheSignoffWritesNoText(t *testing.T) {
	db, runner, cap, pub := newCaptureRunner(t, loadAppFixture(t, "01-principal-valid.json"))
	seedSignable(t, db, true)
	sign := openSignoff(t, cycle.NewGate(db))
	if err := runner.RunSignoffAsk(context.Background(), "p1", "turn-a", sign, "¿Por qué se dejan fuera estudios?"); err != nil {
		t.Fatalf("RunSignoffAsk: %v", err)
	}
	if len(cap.sent) != 1 {
		t.Fatalf("calls: %d, want 1", len(cap.sent))
	}
	if got := lastUserText(t, cap.sent[0]); got != cycle.SignoffAskMessage("sec-1", "¿Por qué se dejan fuera estudios?") {
		t.Fatalf("message: %q", got)
	}
	if sections, _ := db.ListObjects(context.Background(), "p1", store.ObjectSection); len(sections) != 1 {
		t.Fatalf("sections: %d, want 1 (the answer writes nothing)", len(sections))
	}
	if n := len(eventsOfType(t, db, store.EventStepEnded)); n != 0 {
		t.Fatalf("step_ended: %d, want 0", n)
	}
	if step := pub.step(t); step["status"] != "accepted" || step["note"] == "" {
		t.Fatalf("published step: %v", step)
	}
}

// The question is your human_message and the answer stays in the history
// next to it, not applied (Eber, 29-oct).
func TestB14_AskTheAgentRecordsYourQuestionAndItsAnswerNotApplied(t *testing.T) {
	db, runner, _, _ := newCaptureRunner(t, loadAppFixture(t, "01-principal-valid.json"))
	seedSignable(t, db, true)
	sign := openSignoff(t, cycle.NewGate(db))
	if err := runner.RunSignoffAsk(context.Background(), "p1", "turn-a", sign, "¿Por qué se dejan fuera estudios?"); err != nil {
		t.Fatalf("RunSignoffAsk: %v", err)
	}
	msgs := eventsOfType(t, db, store.EventHumanMessage)
	if len(msgs) != 1 {
		t.Fatalf("human_message: %d, want 1", len(msgs))
	}
	data := eventData(t, msgs[0])
	if data["text"] != "¿Por qué se dejan fuera estudios?" || data["applied"] != false ||
		!strings.Contains(data["response"].(string), "Intenté una primera sección mínima") {
		t.Fatalf("human_message data: %v, want the question, its answer and applied false", data)
	}
}

// Spec §9: the frame shows your current signoff explanation on the
// critical decision the signoff explains.
func TestM1_FrameShowsTheCurrentSignoffExplanation(t *testing.T) {
	db := openTemp(t)
	seedSignable(t, db, true)
	g := cycle.NewGate(db)
	ctx := context.Background()
	sign := openSignoff(t, g)
	frameDecision := func() graph.FrameDecision {
		f, err := graph.ComputeFrame(ctx, db, "p1", graph.FrameLayers{})
		if err != nil || len(f.Sections) != 1 || len(f.Sections[0].Decisions) != 1 {
			t.Fatalf("frame: %+v, %v", f, err)
		}
		return f.Sections[0].Decisions[0]
	}
	const explanation = "Comparo solo estudios con el mismo tamaño porque si no la diferencia es del filtro."
	if err := g.RecordExplanation(ctx, "p1", sign, explanation); err != nil {
		t.Fatalf("RecordExplanation: %v", err)
	}
	if d := frameDecision(); d.SignoffExplanation != "" {
		t.Fatalf("explanation shown while signing: %q (not yet a signoff)", d.SignoffExplanation)
	}
	round(t, g, sign, "round1-all-match")
	if err := g.RecordAnswers(ctx, "p1", sign, "Supongo que todos cuentan desde el mismo tamaño mínimo.", ""); err != nil {
		t.Fatalf("RecordAnswers: %v", err)
	}
	round(t, g, sign, "round2-all-match-answers")
	if err := g.Sign(ctx, "p1", sign); err != nil {
		t.Fatalf("Sign: %v", err)
	}
	if d := frameDecision(); d.SignoffExplanation != explanation {
		t.Fatalf("frame explanation: %q, want the signed one", d.SignoffExplanation)
	}
}

// P-41 corrected (flujos §3): «Ver la historia de ese punto» is never
// offered before your first explanation, and shows again, while the
// signoff is in progress, once Wonder names what is missing.
func TestP41_HistoryOfThatPointShowsAgainAfterWonderNamesWhatIsMissing(t *testing.T) {
	db := openTemp(t)
	sectionID, decisionID := seedSignable(t, db, true)
	// The decision's point in the text: a figure that covers it.
	err := db.Write(context.Background(), "p1", func(tx *store.Tx) error {
		fig, err := tx.CreateObject(store.ObjectFigure, "", map[string]any{"section_id": sectionID,
			"anchor": "Así la comparación es justa."}, store.EventInput{Type: store.EventStateChanged, Actor: "agent"})
		if err != nil {
			return err
		}
		_, err = tx.AddEdge(store.EdgeCovers, fig.ID, decisionID, "", store.EventInput{Type: store.EventEdge, Actor: "agent"})
		return err
	})
	if err != nil {
		t.Fatalf("seed figure: %v", err)
	}
	g := cycle.NewGate(db)
	sign := openSignoff(t, g)
	for _, v := range view(t, db, sign).Verdicts {
		if v.HistoryOffered {
			t.Fatalf("history offered before the first explanation: %+v", v)
		}
	}
	if err := g.RecordExplanation(context.Background(), "p1", sign, "Comparo los estudios entre sí."); err != nil {
		t.Fatalf("RecordExplanation: %v", err)
	}
	round(t, g, sign, "missing-in-explanation")
	v := view(t, db, sign)
	if v.Phase != graph.SignoffInProgress || len(v.Verdicts) != 1 || v.Verdicts[0].Block == 0 || !v.Verdicts[0].HistoryOffered {
		t.Fatalf("after «falta en tu explicación»: %+v", v.Verdicts)
	}
}
