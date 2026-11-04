package cycle_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/LaurenteEber/tupaqnin/internal/cycle"
	"github.com/LaurenteEber/tupaqnin/internal/graph"
	"github.com/LaurenteEber/tupaqnin/internal/provider"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// B3-20 against B-106: «el tope nunca te impide firmar». At a cap, or when Wonder
// fails, the signoff asks only about the critical decisions; the thesis question
// is skipped, and a section with no critical decisions signs brief.

func cappedGate(db *store.DB) *cycle.Gate {
	g := cycle.NewGate(db)
	g.StoppedBy = func(context.Context, string) (string, error) { return "project_cap", nil }
	return g
}

// noCritical is the case of P-127: a section whose decisions are not critical.
func noCritical(t *testing.T, db *store.DB) {
	t.Helper()
	sec := seedThesisSection(t, db, "Alcance", plainRole)
	seedThesisDecision(t, db, "Limitar el alcance al piloto.", sec)
}

// criticalAndMark has a critical decision and your «aquí se concluye»: Wonder
// gets dec-1 and sec-1.
func criticalAndMark(t *testing.T, db *store.DB) {
	t.Helper()
	seedSignable(t, db, true)
	markSection(t, db, "sec-1")
}

func thesisSkippedEvents(t *testing.T, db *store.DB) []string {
	t.Helper()
	events, err := db.EventsSince(context.Background(), "p1", 0)
	if err != nil {
		t.Fatal(err)
	}
	var reasons []string
	for _, ev := range events {
		var data map[string]any
		if json.Unmarshal(ev.Data, &data) != nil {
			continue
		}
		if data["signoff_step"] == graph.StepThesisSkipped {
			reason, _ := data["reason"].(string)
			reasons = append(reasons, reason)
		}
		if reason, ok := data["thesis_skipped"].(string); ok {
			reasons = append(reasons, reason)
		}
	}
	return reasons
}

func TestB320_AtTheCapASectionWithNoCriticalDecisionsSignsBrief(t *testing.T) {
	db := openTemp(t)
	noCritical(t, db)
	g := cappedGate(db)
	ctx := context.Background()
	open := openSection(t, g, "sec-1")
	if !open.Brief {
		t.Fatalf("at the cap it must open brief: %+v", open)
	}
	if err := g.RecordExplanation(ctx, "p1", open.Label, "Lo explico sin mirar."); err != nil {
		t.Fatal(err)
	}
	if v := view(t, db, open.Label); v.Thesis != "" || !v.CanSign {
		t.Fatalf("view: %+v", v)
	}
	if err := g.Sign(ctx, "p1", open.Label); err != nil {
		t.Fatalf("Sign: %v", err)
	}
	if got := thesisSkippedEvents(t, db); len(got) != 1 || got[0] != cycle.ThesisSkippedCap {
		t.Fatalf("the history must say the question was skipped by the cap: %v", got)
	}
}

func TestB320_AtTheCapAMarkedSectionWithNoDecisionsSignsBrief(t *testing.T) {
	db := openTemp(t)
	sec := seedThesisSection(t, db, "Conclusiones", markedRole)
	if open := openSection(t, cappedGate(db), sec); !open.Brief {
		t.Fatalf("at the cap your mark adds no question: %+v", open)
	}
}

func TestB320_AtTheCapTheSignoffAsksOnlyTheCriticalDecisions(t *testing.T) {
	db := openTemp(t)
	criticalAndMark(t, db)
	g := cappedGate(db)
	ctx := context.Background()
	open := openSection(t, g, "sec-1")
	if open.Brief {
		t.Fatalf("critical decisions keep their question: %+v", open)
	}
	if brief := wonderBrief(t, db, open.Label); strings.Contains(brief, "Thesis of this section") {
		t.Fatalf("Wonder was asked about the thesis:\n%s", brief)
	}
	v := view(t, db, open.Label)
	if v.Thesis != "" || len(v.CriticalDecisions) != 1 {
		t.Fatalf("view: %+v", v)
	}
	if err := g.RecordExplanation(ctx, "p1", open.Label, "Lo explico sin mirar."); err != nil {
		t.Fatal(err)
	}
	// An entry for the thesis is now one Wonder must not write.
	if res := endRound(t, g, open.Label, allMatch("dec-1", "sec-1")); res.Status != cycle.StatusRetry {
		t.Fatalf("an entry for sec-1: %+v", res)
	}
	signCriticalOnly(t, g, db, open.Label)
}

// signCriticalOnly takes a signoff through dec-1 alone: the question, then its
// judgment, as before B-106.
func signCriticalOnly(t *testing.T, g *cycle.Gate, db *store.DB, sign string) {
	t.Helper()
	ctx := context.Background()
	first := strings.Replace(string(allMatch("dec-1")), "not_applicable", "¿De qué supuesto sale?", 1)
	if res := endRound(t, g, sign, json.RawMessage(first)); res.Status != cycle.StatusAccepted {
		t.Fatalf("first round: %+v", res)
	}
	if err := g.RecordAnswers(ctx, "p1", sign, "De la muestra.", ""); err != nil {
		t.Fatalf("RecordAnswers: %v", err)
	}
	second := strings.Replace(first, `"answer": null`, `"answer": "answers"`, 1)
	if res := endRound(t, g, sign, json.RawMessage(second)); res.Outcome != cycle.OutcomeCoincides {
		t.Fatalf("second round: %+v", res)
	}
	if !view(t, db, sign).CanSign {
		t.Fatal("it must be signable")
	}
	if err := g.Sign(ctx, "p1", sign); err != nil {
		t.Fatalf("Sign: %v", err)
	}
}

// The real StoppedBy, through the runner: the project on its signoff balance
// opens brief, calls no model and signs, and the balance is untouched.
func TestB320_TheProjectOnItsSignoffBalanceSpendsNothingOnTheThesis(t *testing.T) {
	db := openTemp(t)
	noCritical(t, db)
	spend(t, db, "p1", 3-provider.SignoffReserveUSD)
	c := &captureProvider{inner: provider.NewFake()}
	r := cycle.NewRunner(db, c, nil, &fakePublisher{})
	t.Cleanup(r.Close)
	ctx := context.Background()
	open, err := r.RequestSignoff(ctx, "p1", "sec-1")
	if err != nil || !open.Opened || !open.Brief {
		t.Fatalf("RequestSignoff: %+v, %v", open, err)
	}
	turn, err := r.SignoffExplanation("p1", open.Label, "Lo explico sin mirar.")
	if err != nil || turn != "" {
		t.Fatalf("a brief signoff starts no turn: %q, %v", turn, err)
	}
	if len(c.sent) != 0 {
		t.Fatalf("model calls: %d", len(c.sent))
	}
	if err := cycle.NewGate(db).Sign(ctx, "p1", open.Label); err != nil {
		t.Fatalf("Sign: %v", err)
	}
}

// A signoff opened below the cap that reaches it before you explain: sending the
// explanation drops the question and calls no model.
func TestB320_TheCapArrivingBeforeYourExplanationSkipsTheQuestion(t *testing.T) {
	db := openTemp(t)
	noCritical(t, db)
	c := &captureProvider{inner: provider.NewFake()}
	r := cycle.NewRunner(db, c, nil, &fakePublisher{})
	t.Cleanup(r.Close)
	ctx := context.Background()
	open, err := r.RequestSignoff(ctx, "p1", "sec-1")
	if err != nil || open.Brief {
		t.Fatalf("below the cap the thesis is asked: %+v, %v", open, err)
	}
	spend(t, db, "p1", 3-provider.SignoffReserveUSD)
	turn, err := r.SignoffExplanation("p1", open.Label, "Lo explico sin mirar.")
	if err != nil || turn != "" || len(c.sent) != 0 {
		t.Fatalf("turn %q, err %v, calls %d", turn, err, len(c.sent))
	}
	if v := view(t, db, open.Label); !v.Brief || v.Thesis != "" || !v.CanSign {
		t.Fatalf("view: %+v", v)
	}
	if got := thesisSkippedEvents(t, db); len(got) != 1 || got[0] != cycle.ThesisSkippedCap {
		t.Fatalf("history: %v", got)
	}
}

// Asking to sign again at the cap shows the signoff in progress without the question.
func TestB320_AskingAgainAtTheCapSkipsTheQuestionOfTheOneInProgress(t *testing.T) {
	db := openTemp(t)
	noCritical(t, db)
	if open := openSection(t, cycle.NewGate(db), "sec-1"); open.Brief {
		t.Fatalf("below the cap: %+v", open)
	}
	if open := openSection(t, cappedGate(db), "sec-1"); !open.Brief || open.Label != "sign-1" {
		t.Fatalf("at the cap: %+v", open)
	}
}

// The cap arrives with a gap only in the thesis, after your origin answer was
// judged: what is left coincides, its case goes, and you sign with no further call.
func TestB320_TheCapAfterAGapOnlyInTheThesisLeavesItSignable(t *testing.T) {
	db := openTemp(t)
	criticalAndMark(t, db)
	g := cycle.NewGate(db)
	ctx := context.Background()
	open := openSection(t, g, "sec-1")
	if err := g.RecordExplanation(ctx, "p1", open.Label, "Lo explico sin mirar."); err != nil {
		t.Fatal(err)
	}
	if res := endRound(t, g, open.Label, wonderOutput(t, "handmade-thesis-marked-round1")); res.Status != cycle.StatusAccepted {
		t.Fatalf("round 1: %+v", res)
	}
	if err := g.RecordAnswers(ctx, "p1", open.Label, "De la muestra.", ""); err != nil {
		t.Fatal(err)
	}
	gap := `{"decisions": [{"decision": "dec-1", "verdict": "match", "missing": null, "passage": null},
		{"decision": "sec-1", "verdict": "missing_in_explanation", "missing": "qué concluye", "passage": null}],
		"origin_question": {"question": "¿De qué supuesto sale?", "answer": "answers"},
		"previous_case": null, "new_case": "Una región pide abrirlo. ¿Qué le respondes?"}`
	if res := endRound(t, g, open.Label, json.RawMessage(gap)); res.Outcome != cycle.OutcomeCase {
		t.Fatalf("round 2: %+v", res)
	}
	if v := view(t, db, open.Label); v.CanSign || v.OpenCase == "" {
		t.Fatalf("below the cap the gap blocks: %+v", v)
	}
	skipped, err := cappedGate(db).SkipThesisAtCap(ctx, "p1", open.Label)
	if err != nil || !skipped {
		t.Fatalf("SkipThesisAtCap: %v, %v", skipped, err)
	}
	v := view(t, db, open.Label)
	if !v.CanSign || v.OpenCase != "" || v.Thesis != "" || len(v.Verdicts) != 1 || v.Verdicts[0].Decision != "dec-1" {
		t.Fatalf("what is left must coincide: %+v", v)
	}
	if err := g.Sign(ctx, "p1", open.Label); err != nil {
		t.Fatalf("Sign: %v", err)
	}
}

// A gap in a critical decision still blocks, whatever the cap does to the thesis.
func TestB320_ASkippedThesisDoesNotExcuseACriticalGap(t *testing.T) {
	db := openTemp(t)
	criticalAndMark(t, db)
	g := cycle.NewGate(db)
	ctx := context.Background()
	open := openSection(t, g, "sec-1")
	if err := g.RecordExplanation(ctx, "p1", open.Label, "Lo explico sin mirar."); err != nil {
		t.Fatal(err)
	}
	gap := `{"decisions": [{"decision": "dec-1", "verdict": "missing_in_explanation", "missing": "por qué", "passage": null},
		{"decision": "sec-1", "verdict": "match", "missing": null, "passage": null}],
		"origin_question": {"question": "¿De qué supuesto sale?", "answer": null},
		"previous_case": null, "new_case": "Una región pide abrirlo. ¿Qué le respondes?"}`
	if res := endRound(t, g, open.Label, json.RawMessage(gap)); res.Outcome != cycle.OutcomeCase {
		t.Fatalf("round: %+v", res)
	}
	if _, err := cappedGate(db).SkipThesisAtCap(ctx, "p1", open.Label); err != nil {
		t.Fatal(err)
	}
	if v := view(t, db, open.Label); v.CanSign || v.Coincides || v.OpenCase == "" {
		t.Fatalf("the gap in dec-1 and its case must stay: %+v", v)
	}
	if err := g.Sign(ctx, "p1", open.Label); !errors.Is(err, cycle.ErrSignoffDoesNotCoincide) {
		t.Fatalf("Sign: %v", err)
	}
}

// A signoff below every cap keeps asking about the thesis: this is the other path.
func TestB320_BelowTheCapTheThesisIsStillAsked(t *testing.T) {
	db := openTemp(t)
	noCritical(t, db)
	g := cycle.NewGate(db)
	g.StoppedBy = func(context.Context, string) (string, error) { return "", nil }
	open := openSection(t, g, "sec-1")
	if open.Brief || view(t, db, open.Label).Thesis == "" {
		t.Fatalf("below the cap: %+v", open)
	}
	if skipped, err := g.SkipThesisAtCap(context.Background(), "p1", open.Label); err != nil || skipped {
		t.Fatalf("SkipThesisAtCap: %v, %v", skipped, err)
	}
	if got := thesisSkippedEvents(t, db); len(got) != 0 {
		t.Fatalf("history: %v", got)
	}
}

// failingWonder fails every contraster call, as a model that does not come back.
type failingWonder struct {
	err   error
	text  string
	calls int
}

func (f *failingWonder) Respond(context.Context, provider.Request, func(provider.Delta)) (provider.Response, error) {
	f.calls++
	if f.err != nil {
		return provider.Response{}, f.err
	}
	return provider.Response{Text: f.text}, nil
}

func wonderFailures() map[string]*failingWonder {
	return map[string]*failingWonder{
		"a 4xx":                 {err: &provider.ProviderError{Code: "http_400", Type: "invalid_request_error", HTTPStatus: 400}},
		"Wonder and Spark down": {err: &provider.ProviderError{Code: "http_503", Type: "http_error", HTTPStatus: 503}},
		"two bad shapes":        {text: "not json"},
	}
}

func TestB320_WhenWonderFailsASectionWithNoCriticalDecisionsSignsBrief(t *testing.T) {
	for name, prov := range wonderFailures() {
		t.Run(name, func(t *testing.T) {
			db := openTemp(t)
			noCritical(t, db)
			pub := &fakePublisher{}
			r := cycle.NewRunner(db, prov, nil, pub)
			t.Cleanup(r.Close)
			g := cycle.NewGate(db)
			ctx := context.Background()
			open := openSection(t, g, "sec-1")
			if err := g.RecordExplanation(ctx, "p1", open.Label, "Lo explico sin mirar."); err != nil {
				t.Fatal(err)
			}
			if err := r.RunSignoffRound(ctx, "p1", "turn-w", open.Label); err != nil {
				t.Fatalf("RunSignoffRound: %v", err)
			}
			if prov.calls == 0 {
				t.Fatal("Wonder was not called")
			}
			v := view(t, db, open.Label)
			if !v.Brief || v.Thesis != "" || !v.CanSign || v.Phase != graph.SignoffInProgress {
				t.Fatalf("view: %+v", v)
			}
			// Nothing is left to retry: the screen reloads, with no error of the turn.
			if len(pub.errs) != 0 || pub.step(t)["signoff"] != open.Label {
				t.Fatalf("errors %v, steps %d", pub.errs, len(pub.steps))
			}
			if err := g.Sign(ctx, "p1", open.Label); err != nil {
				t.Fatalf("Sign: %v", err)
			}
			if got := thesisSkippedEvents(t, db); len(got) != 1 || got[0] != cycle.ThesisSkippedWonderFailed {
				t.Fatalf("history: %v", got)
			}
		})
	}
}

func TestB320_WhenWonderFailsTheNextRoundAsksOnlyTheCriticalDecisions(t *testing.T) {
	for name, prov := range wonderFailures() {
		t.Run(name, func(t *testing.T) {
			db := openTemp(t)
			criticalAndMark(t, db)
			pub := &fakePublisher{}
			r := cycle.NewRunner(db, prov, nil, pub)
			t.Cleanup(r.Close)
			g := cycle.NewGate(db)
			ctx := context.Background()
			open := openSection(t, g, "sec-1")
			if err := g.RecordExplanation(ctx, "p1", open.Label, "Lo explico sin mirar."); err != nil {
				t.Fatal(err)
			}
			if err := r.RunSignoffRound(ctx, "p1", "turn-w", open.Label); err != nil {
				t.Fatalf("RunSignoffRound: %v", err)
			}
			v := view(t, db, open.Label)
			if v.Brief || v.Thesis != "" || v.CanSign {
				t.Fatalf("critical decisions stay to be asked: %+v", v)
			}
			if len(pub.errs) != 1 {
				t.Fatalf("the turn's error stays, as before: %v", pub.errs)
			}
			if brief := wonderBrief(t, db, open.Label); strings.Contains(brief, "Thesis of this section") {
				t.Fatalf("the next call asks about the thesis:\n%s", brief)
			}
			signCriticalOnly(t, g, db, open.Label)
		})
	}
}

// A cap that refuses Wonder's call is the same: a section with no critical
// decisions signs brief.
func TestB320_ACapRefusingWondersCallSignsBrief(t *testing.T) {
	db := openTemp(t)
	noCritical(t, db)
	prov := &failingWonder{text: "not called"}
	r := cycle.NewRunner(db, prov, nil, &fakePublisher{})
	t.Cleanup(r.Close)
	g := cycle.NewGate(db)
	ctx := context.Background()
	open := openSection(t, g, "sec-1")
	if err := g.RecordExplanation(ctx, "p1", open.Label, "Lo explico sin mirar."); err != nil {
		t.Fatal(err)
	}
	spend(t, db, "p1", 3) // past the balance too
	if err := r.RunSignoffRound(ctx, "p1", "turn-w", open.Label); err != nil {
		t.Fatalf("RunSignoffRound: %v", err)
	}
	if prov.calls != 0 {
		t.Fatalf("the gate must refuse it: %d calls", prov.calls)
	}
	if v := view(t, db, open.Label); !v.Brief || !v.CanSign {
		t.Fatalf("view: %+v", v)
	}
	if err := g.Sign(ctx, "p1", open.Label); err != nil {
		t.Fatalf("Sign: %v", err)
	}
	if got := thesisSkippedEvents(t, db); len(got) != 1 || got[0] != cycle.ThesisSkippedCap {
		t.Fatalf("history: %v", got)
	}
}

// Spark contrasting when Wonder is down is no failure of the signoff: the round
// comes back, so the thesis stays asked.
func TestB320_SparkContrastingKeepsTheThesis(t *testing.T) {
	db := openTemp(t)
	noCritical(t, db)
	fixtures, err := provider.AppFixtures()
	if err != nil {
		t.Fatal(err)
	}
	fake := provider.NewFake(fixtures...)
	fake.FitSignoff = true
	prov := &unavailableWonder{fake: fake, err: &provider.ProviderError{Code: "http_503", Type: "http_error", HTTPStatus: 503}}
	r := cycle.NewRunner(db, prov, nil, &fakePublisher{})
	t.Cleanup(r.Close)
	g := cycle.NewGate(db)
	ctx := context.Background()
	open := openSection(t, g, "sec-1")
	if err := g.RecordExplanation(ctx, "p1", open.Label, "Lo explico sin mirar."); err != nil {
		t.Fatal(err)
	}
	if err := r.RunSignoffRound(ctx, "p1", "turn-w", open.Label); err != nil {
		t.Fatalf("RunSignoffRound: %v", err)
	}
	if v := view(t, db, open.Label); v.Brief || v.Thesis == "" || !v.WonderUnavailable || v.Rounds != 1 {
		t.Fatalf("view: %+v", v)
	}
}

// failedCaseThenGap leaves a signoff with a judged case that did not apply and a
// new case open, with a gap only in the thesis. Below the cap that case can still
// close it.
func failedCaseThenGap(t *testing.T, g *cycle.Gate) string {
	t.Helper()
	ctx := context.Background()
	open := openSection(t, g, "sec-1")
	if err := g.RecordExplanation(ctx, "p1", open.Label, "Lo explico sin mirar."); err != nil {
		t.Fatal(err)
	}
	round := func(previous, origin, newCase string) {
		t.Helper()
		out := `{"decisions": [{"decision": "dec-1", "verdict": "match", "missing": null, "passage": null},
			{"decision": "sec-1", "verdict": "missing_in_explanation", "missing": "qué concluye", "passage": null}],
			"origin_question": {"question": "¿De qué supuesto sale?", "answer": ` + origin + `},
			"previous_case": ` + previous + `, "new_case": "` + newCase + `"}`
		if res := endRound(t, g, open.Label, json.RawMessage(out)); res.Status != cycle.StatusAccepted {
			t.Fatalf("round: %+v", res)
		}
	}
	round("null", "null", "Caso A")
	if err := g.RecordAnswers(ctx, "p1", open.Label, "De la muestra.", "Respuesta A"); err != nil {
		t.Fatal(err)
	}
	round(`"does_not_apply"`, `"answers"`, "Caso B")
	return open.Label
}

func caseWellOnDec1() json.RawMessage {
	return json.RawMessage(`{"decisions": [{"decision": "dec-1", "verdict": "match", "missing": null, "passage": null}],
		"origin_question": {"question": "¿De qué supuesto sale?", "answer": "answers"},
		"previous_case": "applies_well", "new_case": null}`)
}

// After a failed case the open one is the way out: dropping it would leave the last
// judged case failed for good, with no way to sign.
func TestB320_ASkipAfterAFailedCaseKeepsTheCaseThatCanCloseIt(t *testing.T) {
	db := openTemp(t)
	criticalAndMark(t, db)
	g := cycle.NewGate(db)
	ctx := context.Background()
	sign := failedCaseThenGap(t, g)
	if skipped, err := cappedGate(db).SkipThesisAtCap(ctx, "p1", sign); err != nil || !skipped {
		t.Fatalf("SkipThesisAtCap: %v, %v", skipped, err)
	}
	if v := view(t, db, sign); v.OpenCase != "Caso B" || v.CanSign {
		t.Fatalf("the case must stay: %+v", v)
	}
	if err := g.RecordAnswers(ctx, "p1", sign, "", "Respuesta B"); err != nil {
		t.Fatalf("RecordAnswers: %v", err)
	}
	if res := endRound(t, g, sign, caseWellOnDec1()); res.Outcome != cycle.OutcomeCoincides {
		t.Fatalf("round: %+v", res)
	}
	if err := g.Sign(ctx, "p1", sign); err != nil {
		t.Fatalf("Sign: %v", err)
	}
}

// The same when Wonder fails on your answer to that case: your answer stays, to be
// judged by the next round.
func TestB320_WonderFailingOnYourAnswerKeepsIt(t *testing.T) {
	db := openTemp(t)
	criticalAndMark(t, db)
	g := cycle.NewGate(db)
	ctx := context.Background()
	sign := failedCaseThenGap(t, g)
	if err := g.RecordAnswers(ctx, "p1", sign, "", "Respuesta B"); err != nil {
		t.Fatal(err)
	}
	r := cycle.NewRunner(db, &failingWonder{err: &provider.ProviderError{Code: "http_400", Type: "invalid_request_error", HTTPStatus: 400}}, nil, &fakePublisher{})
	t.Cleanup(r.Close)
	if err := r.RunSignoffRound(ctx, "p1", "turn-w", sign); err != nil {
		t.Fatal(err)
	}
	v := view(t, db, sign)
	if v.CanSign || v.Thesis != "" {
		t.Fatalf("view: %+v", v)
	}
	if err := g.RecordAnswers(ctx, "p1", sign, "De la muestra, otra vez.", ""); err != nil {
		t.Fatal(err)
	}
	if res := endRound(t, g, sign, caseWellOnDec1()); res.Outcome != cycle.OutcomeCoincides {
		t.Fatalf("the case is judged by the next round: %+v", res)
	}
}

// An answer written after the last round was judged by nobody: its marks cannot
// open signing, whatever they said of the answer before it.
func TestB320_AnAnswerWonderNeverJudgedDoesNotCoincide(t *testing.T) {
	db := openTemp(t)
	criticalAndMark(t, db)
	g := cycle.NewGate(db)
	ctx := context.Background()
	open := openSection(t, g, "sec-1")
	if err := g.RecordExplanation(ctx, "p1", open.Label, "Lo explico sin mirar."); err != nil {
		t.Fatal(err)
	}
	if res := endRound(t, g, open.Label, wonderOutput(t, "handmade-thesis-marked-round1")); res.Status != cycle.StatusAccepted {
		t.Fatalf("round 1: %+v", res)
	}
	if err := g.RecordAnswers(ctx, "p1", open.Label, "De la muestra.", ""); err != nil {
		t.Fatal(err)
	}
	gap := `{"decisions": [{"decision": "dec-1", "verdict": "match", "missing": null, "passage": null},
		{"decision": "sec-1", "verdict": "missing_in_explanation", "missing": "qué concluye", "passage": null}],
		"origin_question": {"question": "¿De qué supuesto sale?", "answer": "answers"},
		"previous_case": null, "new_case": "Caso B"}`
	if res := endRound(t, g, open.Label, json.RawMessage(gap)); res.Outcome != cycle.OutcomeCase {
		t.Fatalf("round 2: %+v", res)
	}
	if err := g.RecordAnswers(ctx, "p1", open.Label, "no sé", "Respuesta B"); err != nil {
		t.Fatal(err)
	}
	pub := &fakePublisher{}
	r := cycle.NewRunner(db, &failingWonder{err: &provider.ProviderError{Code: "http_400", Type: "invalid_request_error", HTTPStatus: 400}}, nil, pub)
	t.Cleanup(r.Close)
	if err := r.RunSignoffRound(ctx, "p1", "turn-w", open.Label); err != nil {
		t.Fatal(err)
	}
	if v := view(t, db, open.Label); v.Coincides || v.CanSign {
		t.Fatalf("nobody judged «no sé»: %+v", v)
	}
	if len(pub.errs) != 1 {
		t.Fatalf("there is something to retry: %v", pub.errs)
	}
}

// K-11: the time you took counts from when the signoff last asked you something, and
// the app dropping a question asks nothing.
func TestB320_TheSkipDoesNotResetYourTime(t *testing.T) {
	db := openTemp(t)
	noCritical(t, db)
	r := cycle.NewRunner(db, &failingWonder{text: "not called"}, nil, &fakePublisher{})
	t.Cleanup(r.Close)
	ctx := context.Background()
	open, err := r.RequestSignoff(ctx, "p1", "sec-1")
	if err != nil || open.Brief {
		t.Fatalf("RequestSignoff: %+v, %v", open, err)
	}
	// created_at has millisecond precision: a wait the measure cannot miss.
	time.Sleep(50 * time.Millisecond)
	spend(t, db, "p1", 3-provider.SignoffReserveUSD)
	if _, err := r.SignoffExplanation("p1", open.Label, "Lo explico sin mirar."); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, ev := range eventsOfType(t, db, store.EventActTimed) {
		var data map[string]any
		if err := json.Unmarshal(ev.Data, &data); err != nil {
			t.Fatal(err)
		}
		if data["part"] == "explanation" {
			found = true
			if ms, _ := data["duration_ms"].(float64); ms < 40 {
				t.Fatalf("duration_ms %v: it counted from the skip", ms)
			}
		}
	}
	if !found {
		t.Fatal("no act_timed for the explanation")
	}
}

// Answering a case the skip just took away is no error: the screen reloads and shows
// what is left to answer, the origin question.
func TestB320_AnAnswerToACaseTheSkipDroppedIsNoError(t *testing.T) {
	db := openTemp(t)
	criticalAndMark(t, db)
	r := cycle.NewRunner(db, &failingWonder{text: "not called"}, nil, &fakePublisher{})
	t.Cleanup(r.Close)
	g := cycle.NewGate(db)
	ctx := context.Background()
	open := openSection(t, g, "sec-1")
	if err := g.RecordExplanation(ctx, "p1", open.Label, "Lo explico sin mirar."); err != nil {
		t.Fatal(err)
	}
	if res := endRound(t, g, open.Label, wonderOutput(t, "handmade-thesis-marked-gap")); res.Outcome != cycle.OutcomeCase {
		t.Fatalf("round: %+v", res)
	}
	spend(t, db, "p1", 3-provider.SignoffReserveUSD)
	turn, err := r.SignoffAnswers("p1", open.Label, "", "Le digo que no.")
	if err != nil || turn != "" {
		t.Fatalf("turn %q, err %v", turn, err)
	}
	if v := view(t, db, open.Label); v.OpenCase != "" || v.OriginQuestion == "" || v.CanSign {
		t.Fatalf("view: %+v", v)
	}
}

// A failure with no thesis to skip stays the turn's error, as before: a critical
// section with no thesis point, coinciding, where your new answer's round fails.
func TestB320_AFailureWithNothingToSkipStaysTheTurnsError(t *testing.T) {
	db := openTemp(t)
	seedSignable(t, db, true)
	g := cycle.NewGate(db)
	ctx := context.Background()
	open := openSection(t, g, "sec-1")
	if err := g.RecordExplanation(ctx, "p1", open.Label, "Lo explico sin mirar."); err != nil {
		t.Fatal(err)
	}
	first := strings.Replace(string(allMatch("dec-1")), "not_applicable", "¿De qué supuesto sale?", 1)
	if res := endRound(t, g, open.Label, json.RawMessage(first)); res.Status != cycle.StatusAccepted {
		t.Fatalf("round 1: %+v", res)
	}
	if err := g.RecordAnswers(ctx, "p1", open.Label, "De la muestra.", ""); err != nil {
		t.Fatal(err)
	}
	second := strings.Replace(first, `"answer": null`, `"answer": "answers"`, 1)
	if res := endRound(t, g, open.Label, json.RawMessage(second)); res.Outcome != cycle.OutcomeCoincides {
		t.Fatalf("round 2: %+v", res)
	}
	if err := g.RecordAnswers(ctx, "p1", open.Label, "Otra respuesta.", ""); err != nil {
		t.Fatal(err)
	}
	pub := &fakePublisher{}
	r := cycle.NewRunner(db, &failingWonder{err: &provider.ProviderError{Code: "http_400", Type: "invalid_request_error", HTTPStatus: 400}}, nil, pub)
	t.Cleanup(r.Close)
	if err := r.RunSignoffRound(ctx, "p1", "turn-w", open.Label); err != nil {
		t.Fatal(err)
	}
	if len(pub.errs) != 1 || pub.errs[0] != "provider_error" || len(pub.steps) != 0 {
		t.Fatalf("errors %v, steps %d", pub.errs, len(pub.steps))
	}
}
