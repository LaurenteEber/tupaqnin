package cycle_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/LaurenteEber/tupaqnin/internal/cycle"
	"github.com/LaurenteEber/tupaqnin/internal/graph"
	"github.com/LaurenteEber/tupaqnin/internal/provider"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// B-106: besides the critical decisions, Wonder asks about the section's
// thesis. The recordings named handmade-thesis-* are hand-made: Eber re-records
// the signoff with make microtest-signoff.

const wonderMark = provider.ContrasterModel + " " + provider.ContrasterEffort

var (
	markedRole   = map[string]any{"value": "conclusion", "state": "confirmed"}
	proposedRole = map[string]any{"value": "conclusion", "state": "proposed"}
	plainRole    = map[string]any{"value": "other", "state": "proposed"}
)

func seedThesisSection(t *testing.T, db *store.DB, title string, role map[string]any) string {
	t.Helper()
	var label string
	err := db.Write(context.Background(), "p1", func(tx *store.Tx) error {
		sec, err := tx.CreateObject(store.ObjectSection, "draft", map[string]any{
			"title": title, "text": "Primer párrafo de " + title + ".\n\nAsí se concluye " + title + ".",
			"role": role, "open_changes": []any{},
		}, store.EventInput{Type: store.EventStateChanged, Actor: "agent"})
		label = sec.Label
		return err
	})
	if err != nil {
		t.Fatalf("seed section: %v", err)
	}
	return label
}

// seedThesisDecision creates an agent decision used in the given sections.
func seedThesisDecision(t *testing.T, db *store.DB, statement string, sections ...string) string {
	t.Helper()
	var label string
	err := db.Write(context.Background(), "p1", func(tx *store.Tx) error {
		dec, err := tx.CreateObject(store.ObjectDecision, "agent", map[string]any{
			"statement": statement, "instead_of": "Lo contrario.", "would_choose_if": "Cambiara el pedido.",
			"assumption": false, "origin": "agent", "human_marked_critical": false,
		}, store.EventInput{Type: store.EventStateChanged, Actor: "agent"})
		if err != nil {
			return err
		}
		label = dec.Label
		for _, s := range sections {
			sec, err := tx.GetObject(context.Background(), "p1", s)
			if err != nil {
				return err
			}
			if _, err := tx.AddEdge(store.EdgeUsedIn, dec.ID, sec.ID, "", nil, store.EventInput{Type: store.EventEdge, Actor: "agent"}); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("seed decision: %v", err)
	}
	return label
}

func openSection(t *testing.T, g *cycle.Gate, section string) cycle.SignoffOpen {
	t.Helper()
	open, err := g.RequestSignoff(context.Background(), "p1", section)
	if err != nil || !open.Opened {
		t.Fatalf("RequestSignoff %s: %+v, %v", section, open, err)
	}
	return open
}

func wonderBrief(t *testing.T, db *store.DB, sign string) string {
	t.Helper()
	obj, err := db.GetObject(context.Background(), "p1", sign)
	if err != nil {
		t.Fatalf("GetObject %s: %v", sign, err)
	}
	brief, err := graph.WonderBrief(context.Background(), db, "p1", obj.ID)
	if err != nil {
		t.Fatalf("WonderBrief: %v", err)
	}
	return brief
}

// allMatch is one Wonder output marking the labels as match, with no origin
// question (a signoff without critical decisions).
func allMatch(labels ...string) json.RawMessage {
	var entries []string
	for _, l := range labels {
		entries = append(entries, fmt.Sprintf(`{"decision": %q, "verdict": "match", "missing": null, "passage": null}`, l))
	}
	return json.RawMessage(`{"decisions": [` + strings.Join(entries, ",") + `],
		"origin_question": {"question": "not_applicable", "answer": null}, "previous_case": null, "new_case": null}`)
}

// coincideOnThesis is the Wonder round of a signoff that only asks about the
// thesis, every entry match (B-106): it leaves it coinciding.
func coincideOnThesis(t *testing.T, db *store.DB, g *cycle.Gate, sign string) {
	t.Helper()
	ctx := context.Background()
	obj, err := db.GetObject(ctx, "p1", sign)
	if err != nil {
		t.Fatalf("GetObject %s: %v", sign, err)
	}
	body, err := graph.ParseSignoff(obj)
	if err != nil {
		t.Fatalf("ParseSignoff: %v", err)
	}
	labels, err := graph.NewLabeler(ctx, db, "p1")
	if err != nil {
		t.Fatalf("NewLabeler: %v", err)
	}
	section, err := db.GetObject(ctx, "p1", body.SectionID)
	if err != nil {
		t.Fatalf("GetObject: %v", err)
	}
	point, err := body.ThesisPoint(labels, section.Label)
	if err != nil || point == "" {
		t.Fatalf("ThesisPoint: %q, %v", point, err)
	}
	if res := endRound(t, g, sign, allMatch(point)); res.Outcome != cycle.OutcomeCoincides {
		t.Fatalf("round on the thesis: %+v", res)
	}
}

func endRound(t *testing.T, g *cycle.Gate, sign string, output json.RawMessage) cycle.SignoffRoundResult {
	t.Helper()
	res, err := g.EndSignoffRound(context.Background(), "p1", "turn-w", sign, 1, output, wonderMark, false)
	if err != nil {
		t.Fatalf("EndSignoffRound: %v", err)
	}
	return res
}

// signSection opens, explains, contrasts and signs a section with the labels
// Wonder must mark, leaving it signed.
func signSection(t *testing.T, db *store.DB, g *cycle.Gate, section string, labels ...string) string {
	t.Helper()
	ctx := context.Background()
	open := openSection(t, g, section)
	if err := g.RecordExplanation(ctx, "p1", open.Label, "Lo explico sin mirar."); err != nil {
		t.Fatalf("RecordExplanation: %v", err)
	}
	output := allMatch(labels...)
	if len(view(t, db, open.Label).CriticalDecisions) > 0 {
		// With critical decisions the origin question comes first, then its judgment.
		first := strings.Replace(string(output), "not_applicable", "¿De qué supuesto sale?", 1)
		if res := endRound(t, g, open.Label, json.RawMessage(first)); res.Status != cycle.StatusAccepted {
			t.Fatalf("first round of %s: %+v", section, res)
		}
		if err := g.RecordAnswers(ctx, "p1", open.Label, "De la muestra.", ""); err != nil {
			t.Fatalf("RecordAnswers: %v", err)
		}
		output = json.RawMessage(strings.Replace(first, `"answer": null`, `"answer": "answers"`, 1))
	}
	if res := endRound(t, g, open.Label, output); res.Status != cycle.StatusAccepted || res.Outcome != cycle.OutcomeCoincides {
		t.Fatalf("round of %s: %+v", section, res)
	}
	if err := g.Sign(ctx, "p1", open.Label); err != nil {
		t.Fatalf("Sign %s: %v", section, err)
	}
	return open.Label
}

// A section with your mark gets a question about what it concludes: the entry
// is required, and its verdict reaches the screen in words.
func TestB106_MarkedThesisGetsAQuestionAboutIt(t *testing.T) {
	db := openTemp(t)
	sec := seedThesisSection(t, db, "Conclusiones", markedRole)
	seedThesisDecision(t, db, "Recomendar el piloto.", sec)
	g := cycle.NewGate(db)
	ctx := context.Background()
	open := openSection(t, g, sec)

	brief := wonderBrief(t, db, open.Label)
	if !strings.Contains(brief, "## Thesis of this section") || !strings.Contains(brief, "with the label sec-1") {
		t.Fatalf("the brief does not ask about the thesis:\n%s", brief)
	}
	if err := g.RecordExplanation(ctx, "p1", open.Label, "Recomiendo el piloto."); err != nil {
		t.Fatalf("RecordExplanation: %v", err)
	}
	res, err := g.EndSignoffRound(ctx, "p1", "turn-w", open.Label, 1, wonderOutput(t, "handmade-thesis-marked-missing-only-decision"), wonderMark, false)
	if err != nil || res.Status != cycle.StatusRetry || !strings.Contains(strings.Join(res.Errors, " "), "sec-1") {
		t.Fatalf("an output without the thesis entry: %+v, %v; want a retry naming sec-1", res, err)
	}
	if res := endRound(t, g, open.Label, wonderOutput(t, "handmade-thesis-marked-round1")); res.Status != cycle.StatusAccepted {
		t.Fatalf("round 1: %+v", res)
	}
	v := view(t, db, open.Label)
	if v.Thesis != graph.ThesisConclusion || len(v.Verdicts) != 2 {
		t.Fatalf("view: %+v", v)
	}
	if w := v.Verdicts[1]; w.Thesis != graph.ThesisConclusion || w.Number != 0 || w.Verdict != "match" {
		t.Fatalf("thesis verdict: %+v", w)
	}
	if v.Verdicts[0].Thesis != "" || v.Verdicts[0].Number != 1 {
		t.Fatalf("the critical decision keeps its number: %+v", v.Verdicts[0])
	}
	if err := g.RecordAnswers(ctx, "p1", open.Label, "De que la muestra es la misma.", ""); err != nil {
		t.Fatalf("RecordAnswers: %v", err)
	}
	if res := endRound(t, g, open.Label, wonderOutput(t, "handmade-thesis-marked-round2")); res.Outcome != cycle.OutcomeCoincides {
		t.Fatalf("round 2: %+v", res)
	}
	if err := g.Sign(ctx, "p1", open.Label); err != nil {
		t.Fatalf("Sign: %v", err)
	}
}

// A section with your mark and no decisions is not brief: the question about it
// is what the signoff is for. It costs one Wonder call, with no origin question.
func TestB106_MarkedThesisWithoutDecisionsIsNotBrief(t *testing.T) {
	db := openTemp(t)
	sec := seedThesisSection(t, db, "Conclusiones", markedRole)
	g := cycle.NewGate(db)
	if open := openSection(t, g, sec); open.Brief {
		t.Fatalf("a marked section opened brief: %+v", open)
	}
	if err := g.LeaveSignoff(context.Background(), "p1", "sign-1"); err != nil {
		t.Fatalf("LeaveSignoff: %v", err)
	}
	signed := signSection(t, db, g, sec, sec)
	if v := view(t, db, signed); v.OriginQuestion != "" || v.Phase != graph.SignoffSigned {
		t.Fatalf("signed: %+v", v)
	}
}

// Without your mark, a role the principal only proposed is no mark: the thesis
// is the first decision that holds the section up, not the section.
func TestB106_AProposedRoleIsNotAMark(t *testing.T) {
	db := openTemp(t)
	sec := seedThesisSection(t, db, "Conclusiones", proposedRole)
	seedThesisDecision(t, db, "Recomendar el piloto.", sec)
	g := cycle.NewGate(db)
	open := openSection(t, g, sec)
	if v := view(t, db, open.Label); v.Thesis != "" {
		t.Fatalf("the proposed role added a thesis point: %+v", v)
	}
}

// No mark: the first decision that holds it up, even if it is not critical. It
// stays out of the critical list, and the signoff has no origin question.
func TestB106_NoMarkAsksTheFirstDecisionEvenIfNotCritical(t *testing.T) {
	db := openTemp(t)
	sec := seedThesisSection(t, db, "Contexto", plainRole)
	dec := seedThesisDecision(t, db, "Medir el piloto en tres meses.", sec)
	seedThesisDecision(t, db, "Publicar el informe.", sec)
	g := cycle.NewGate(db)
	ctx := context.Background()
	open := openSection(t, g, sec)
	if open.Brief {
		t.Fatalf("opened brief: %+v", open)
	}
	v := view(t, db, open.Label)
	if v.Thesis != graph.ThesisDecision || len(v.CriticalDecisions) != 0 {
		t.Fatalf("view: %+v", v)
	}
	brief := wonderBrief(t, db, open.Label)
	if !strings.Contains(brief, "Medir el piloto") || strings.Contains(brief, "Publicar el informe") {
		t.Fatalf("the brief does not name the first decision alone:\n%s", brief)
	}
	if !strings.Contains(brief, "Not applicable") {
		t.Fatalf("the brief must say the origin question does not apply:\n%s", brief)
	}
	if err := g.RecordExplanation(ctx, "p1", open.Label, "Medimos en tres meses."); err != nil {
		t.Fatalf("RecordExplanation: %v", err)
	}
	if res := endRound(t, g, open.Label, allMatch(dec)); res.Outcome != cycle.OutcomeCoincides {
		t.Fatalf("round: %+v", res)
	}
	v = view(t, db, open.Label)
	if !v.CanSign || v.OriginQuestion != "" || v.Verdicts[0].Thesis != graph.ThesisDecision {
		t.Fatalf("after the round: %+v", v)
	}
	if err := g.Sign(ctx, "p1", open.Label); err != nil {
		t.Fatalf("Sign: %v", err)
	}
	critical, _, err := graph.Critical(ctx, db, "p1", dec)
	if err != nil {
		t.Fatalf("Critical: %v", err)
	}
	if critical {
		t.Fatal("the thesis decision became critical")
	}
	obj, _ := db.GetObject(ctx, "p1", dec)
	edges, _ := db.EdgesTo(ctx, "p1", obj.ID)
	for _, e := range edges {
		if e.Type == store.EdgeExplains {
			t.Fatalf("an explains edge to a decision that is not critical: %+v", e)
		}
	}
}

// A first decision that is critical adds no question: it is already asked.
func TestB106_AFirstDecisionAlreadyCriticalAddsNoPoint(t *testing.T) {
	db := openTemp(t)
	sec := seedThesisSection(t, db, "Conclusiones", proposedRole)
	dec := seedThesisDecision(t, db, "Recomendar el piloto.", sec)
	g := cycle.NewGate(db)
	open := openSection(t, g, sec)
	if strings.Contains(wonderBrief(t, db, open.Label), "## Thesis of this section") {
		t.Fatal("the brief added a point for a decision already critical")
	}
	if err := g.RecordExplanation(context.Background(), "p1", open.Label, "Recomiendo el piloto."); err != nil {
		t.Fatalf("RecordExplanation: %v", err)
	}
	// One entry only, as before: an extra one is rejected.
	other := seedThesisSection(t, db, "Otra", plainRole)
	if res, _ := g.EndSignoffRound(context.Background(), "p1", "turn-w", open.Label, 1, allMatch(dec, other), wonderMark, false); res.Status != cycle.StatusRetry {
		t.Fatalf("an entry for another section: %+v; want a retry", res)
	}
}

// Two sections with different decisions do not get the same question, whether
// each has its own or they share one: the second passes over what the first
// signature already answered for, and an output about it is rejected.
func TestB106_TwoSectionsWithDifferentDecisionsDoNotGetTheSameQuestion(t *testing.T) {
	db := openTemp(t)
	first := seedThesisSection(t, db, "Diagnóstico", plainRole)
	second := seedThesisSection(t, db, "Recomendación", plainRole)
	shared := seedThesisDecision(t, db, "Abrir hasta las 8 pm.", first, second) // critical: two sections
	seedThesisDecision(t, db, "Hacer un piloto en dos sedes.", second)
	g := cycle.NewGate(db)
	signSection(t, db, g, first, shared)

	open := openSection(t, g, second)
	v := view(t, db, open.Label)
	if v.Thesis != graph.ThesisDecision || len(v.CriticalDecisions) != 1 || v.CriticalDecisions[0] != shared {
		t.Fatalf("second signoff: %+v", v)
	}
	brief := wonderBrief(t, db, open.Label)
	if i := strings.Index(brief, "## Thesis of this section"); i < 0 || !strings.Contains(brief[i:], "Hacer un piloto") {
		t.Fatalf("the thesis is not the decision the first signature did not answer for:\n%s", brief)
	}
	if err := g.RecordExplanation(context.Background(), "p1", open.Label, "Abrimos hasta las 8 y hacemos un piloto."); err != nil {
		t.Fatalf("RecordExplanation: %v", err)
	}
	if res, _ := g.EndSignoffRound(context.Background(), "p1", "turn-w", open.Label, 1,
		json.RawMessage(strings.ReplaceAll(string(wonderOutput(t, "handmade-thesis-marked-round1")), "sec-1", first)), wonderMark, false); res.Status != cycle.StatusRetry {
		t.Fatalf("an output about another point: %+v; want a retry", res)
	}
}

// Each section with only its own decision is asked about that one.
func TestB106_SectionsWithTheirOwnDecisionsAreAskedAboutThem(t *testing.T) {
	db := openTemp(t)
	first := seedThesisSection(t, db, "Uno", plainRole)
	second := seedThesisSection(t, db, "Dos", plainRole)
	one := seedThesisDecision(t, db, "Decisión del uno.", first)
	two := seedThesisDecision(t, db, "Decisión del dos.", second)
	g := cycle.NewGate(db)
	signSection(t, db, g, first, one)
	open := openSection(t, g, second)
	brief := wonderBrief(t, db, open.Label)
	if !strings.Contains(brief, "Decisión del dos.") || strings.Contains(brief, "Decisión del uno.") {
		t.Fatalf("second brief:\n%s", brief)
	}
	if res := endRoundAfterExplaining(t, g, open.Label, allMatch(two)); res.Status != cycle.StatusAccepted {
		t.Fatalf("round: %+v", res)
	}
}

func endRoundAfterExplaining(t *testing.T, g *cycle.Gate, sign string, output json.RawMessage) cycle.SignoffRoundResult {
	t.Helper()
	if err := g.RecordExplanation(context.Background(), "p1", sign, "Lo explico sin mirar."); err != nil {
		t.Fatalf("RecordExplanation: %v", err)
	}
	return endRound(t, g, sign, output)
}

// Two sections whose only decision is the same one may repeat it.
func TestB106_TwoSectionsWhoseOnlyDecisionIsTheSameMayRepeatIt(t *testing.T) {
	db := openTemp(t)
	first := seedThesisSection(t, db, "Uno", plainRole)
	second := seedThesisSection(t, db, "Dos", plainRole)
	shared := seedThesisDecision(t, db, "La misma decisión.", first, second)
	g := cycle.NewGate(db)
	signSection(t, db, g, first, shared)
	open := openSection(t, g, second)
	obj, err := db.GetObject(context.Background(), "p1", open.Label)
	if err != nil {
		t.Fatalf("GetObject: %v", err)
	}
	body, err := graph.ParseSignoff(obj)
	if err != nil {
		t.Fatalf("ParseSignoff: %v", err)
	}
	dec, _ := db.GetObject(context.Background(), "p1", shared)
	if body.Thesis == nil || body.Thesis.DecisionID != dec.ID {
		t.Fatalf("thesis: %+v, want the shared decision", body.Thesis)
	}
}

// A signature of the same section does not count as asked elsewhere, nor does a
// signoff of another section that was left unsigned.
func TestB106_OnlyAnotherSectionsSignatureCountsAsAsked(t *testing.T) {
	db := openTemp(t)
	first := seedThesisSection(t, db, "Uno", plainRole)
	second := seedThesisSection(t, db, "Dos", plainRole)
	shared := seedThesisDecision(t, db, "La compartida.", first, second)
	seedThesisDecision(t, db, "La del dos.", second)
	g := cycle.NewGate(db)
	left := openSection(t, g, first)
	if err := g.LeaveSignoff(context.Background(), "p1", left.Label); err != nil {
		t.Fatalf("LeaveSignoff: %v", err)
	}
	open := openSection(t, g, second)
	if v := view(t, db, open.Label); v.Thesis != "" || len(v.CriticalDecisions) != 1 || v.CriticalDecisions[0] != shared {
		t.Fatalf("a left signoff counted as asked: %+v", v)
	}
}

// R3′ on the thesis point: Wonder names the gap, the history of that point is
// offered, your explanation is not asked again (it cannot be rewritten), and a
// new case of one line closes the gap once applied well.
func TestB106_R3Prime_HoldsOnTheThesisPoint(t *testing.T) {
	db := openTemp(t)
	sec := seedThesisSection(t, db, "Conclusiones", markedRole)
	seedThesisDecision(t, db, "Recomendar el piloto.", sec)
	g := cycle.NewGate(db)
	ctx := context.Background()
	open := openSection(t, g, sec)
	if err := g.RecordExplanation(ctx, "p1", open.Label, "Recomiendo el piloto."); err != nil {
		t.Fatalf("RecordExplanation: %v", err)
	}
	if res := endRound(t, g, open.Label, wonderOutput(t, "handmade-thesis-marked-gap")); res.Outcome != cycle.OutcomeCase {
		t.Fatalf("round with a gap in the thesis: %+v", res)
	}
	v := view(t, db, open.Label)
	w := v.Verdicts[1]
	if w.Thesis != graph.ThesisConclusion || w.Verdict != "missing_in_explanation" || !w.HistoryOffered || w.Block != 2 {
		t.Fatalf("thesis verdict: %+v, want its history offered on the last block", w)
	}
	if v.OpenCase == "" || v.Coincides || v.CanSign {
		t.Fatalf("a gap in the thesis must bring a new case and block signing: %+v", v)
	}
	if err := g.RecordExplanation(ctx, "p1", open.Label, "Repito la explicación."); !errors.Is(err, cycle.ErrExplanationGiven) {
		t.Fatalf("repeating the explanation: %v, want ErrExplanationGiven", err)
	}
	if err := g.RecordAnswers(ctx, "p1", open.Label, "De la muestra.", "Le digo que no, por el personal."); err != nil {
		t.Fatalf("RecordAnswers: %v", err)
	}
	if res := endRound(t, g, open.Label, wonderOutput(t, "handmade-thesis-marked-case-well")); res.Outcome != cycle.OutcomeCoincides {
		t.Fatalf("case applied well: %+v", res)
	}
	if !view(t, db, open.Label).CanSign {
		t.Fatal("after applying the case, it must be signable")
	}
}

// A gap in a signoff with no origin question needs its new case: without one
// the person could not go on, so the output is rejected.
func TestB106_AGapWithoutOriginNeedsANewCase(t *testing.T) {
	db := openTemp(t)
	sec := seedThesisSection(t, db, "Contexto", plainRole)
	dec := seedThesisDecision(t, db, "Medir en tres meses.", sec)
	g := cycle.NewGate(db)
	open := openSection(t, g, sec)
	if err := g.RecordExplanation(context.Background(), "p1", open.Label, "Medimos."); err != nil {
		t.Fatalf("RecordExplanation: %v", err)
	}
	out := json.RawMessage(fmt.Sprintf(`{"decisions": [{"decision": %q, "verdict": "missing_in_explanation", "missing": "cuándo", "passage": null}],
		"origin_question": {"question": "not_applicable", "answer": null}, "previous_case": null, "new_case": null}`, dec))
	res, err := g.EndSignoffRound(context.Background(), "p1", "turn-w", open.Label, 1, out, wonderMark, false)
	if err != nil || res.Status != cycle.StatusRetry {
		t.Fatalf("gap with no case: %+v, %v; want a retry", res, err)
	}
}

// The thesis keeps the signing gate's meaning: a thesis decision revoked after
// the signoff opened is no longer what you explained.
func TestB106_AThesisDecisionRevokedMeanwhileOutdatesTheSignoff(t *testing.T) {
	db := openTemp(t)
	sec := seedThesisSection(t, db, "Contexto", plainRole)
	dec := seedThesisDecision(t, db, "Medir en tres meses.", sec)
	g := cycle.NewGate(db)
	ctx := context.Background()
	open := openSection(t, g, sec)
	if res := endRoundAfterExplaining(t, g, open.Label, allMatch(dec)); res.Outcome != cycle.OutcomeCoincides {
		t.Fatalf("round: %+v", res)
	}
	obj, _ := db.GetObject(ctx, "p1", dec)
	err := db.Write(ctx, "p1", func(tx *store.Tx) error {
		_, err := tx.UpdateObject(obj.ID, obj.CurrentVersion, "revoked", json.RawMessage(obj.Body),
			store.EventInput{Type: store.EventStateChanged, Actor: "human"})
		return err
	})
	if err != nil {
		t.Fatalf("revoke: %v", err)
	}
	if err := g.Sign(ctx, "p1", open.Label); !errors.Is(err, cycle.ErrSignoffOutdated) {
		t.Fatalf("Sign: %v, want ErrSignoffOutdated", err)
	}
}

// A section with no decisions and no mark stays brief, and calls no model.
func TestB106_NoDecisionsAndNoMarkStaysBrief(t *testing.T) {
	db := openTemp(t)
	sec := seedThesisSection(t, db, "Contexto", plainRole)
	g := cycle.NewGate(db)
	open := openSection(t, g, sec)
	if !open.Brief {
		t.Fatalf("want brief: %+v", open)
	}
	r := cycle.NewRunner(db, provider.NewFake(), nil, nil)
	if turn, err := r.SignoffExplanation("p1", open.Label, "Cuenta el contexto."); err != nil || turn != "" {
		t.Fatalf("brief explanation: %q, %v; want no turn", turn, err)
	}
	if err := g.Sign(context.Background(), "p1", open.Label); err != nil {
		t.Fatalf("Sign: %v", err)
	}
}

// The critical decisions' flow is unchanged: with no mark and a critical first
// decision, the same recordings, the same origin question and the same gate.
func TestB106_CriticalFlowUnchanged(t *testing.T) {
	db := openTemp(t)
	seedSignable(t, db, true)
	g := cycle.NewGate(db)
	ctx := context.Background()
	sign := openSignoff(t, g)
	if strings.Contains(wonderBrief(t, db, sign), "## Thesis of this section") {
		t.Fatal("the brief gained a thesis point")
	}
	if err := g.RecordExplanation(ctx, "p1", sign, "Comparo con el mismo tamaño."); err != nil {
		t.Fatalf("RecordExplanation: %v", err)
	}
	if res := round(t, g, sign, "round1-all-match"); res.Outcome == cycle.OutcomeCoincides {
		t.Fatalf("without an origin answer it must not coincide: %+v", res)
	}
	v := view(t, db, sign)
	if v.Thesis != "" || v.OriginQuestion == "" || len(v.Verdicts) != 1 || v.Verdicts[0].Number != 1 {
		t.Fatalf("view: %+v", v)
	}
	if err := g.Sign(ctx, "p1", sign); !errors.Is(err, cycle.ErrSignoffDoesNotCoincide) {
		t.Fatalf("Sign before coinciding: %v", err)
	}
}

// A signoff opened before B-106 has no thesis in its body: it reads, contrasts
// and signs as it did, full or brief, even if its section has what would be a
// thesis today.
func TestB106_OldSignoffInProgressReads(t *testing.T) {
	db := openTemp(t)
	sectionID, decisionID := seedSignable(t, db, true)
	ctx := context.Background()
	dec, _ := db.GetObject(ctx, "p1", decisionID)
	sec, _ := db.GetObject(ctx, "p1", sectionID)
	body := map[string]any{"section_id": sectionID, "section_version": sec.CurrentVersion, "brief": false,
		"critical_decisions": []map[string]any{{"id": decisionID, "version": dec.CurrentVersion}}, "origin_assumption_id": "",
		"explanation": "", "origin_question": "", "origin_answer": "", "cases": []any{}, "failed_cases": 0,
		"coincides": false, "contrasted_by": "", "wonder_unavailable": false, "rounds": []any{}}
	var label string
	err := db.Write(ctx, "p1", func(tx *store.Tx) error {
		sign, err := tx.CreateObject(store.ObjectSignoff, "", body, store.EventInput{Type: store.EventStateChanged, Actor: "human",
			Data: map[string]any{"signoff_step": graph.StepRequested, "section": sec.Label, "brief": false}})
		label = sign.Label
		return err
	})
	if err != nil {
		t.Fatalf("seed old signoff: %v", err)
	}
	g := cycle.NewGate(db)
	if v := view(t, db, label); v.Thesis != "" || v.Brief {
		t.Fatalf("old signoff view: %+v", v)
	}
	if strings.Contains(wonderBrief(t, db, label), "## Thesis of this section") {
		t.Fatal("an old signoff gained a thesis point")
	}
	if err := g.RecordExplanation(ctx, "p1", label, "Comparo."); err != nil {
		t.Fatalf("RecordExplanation: %v", err)
	}
	round(t, g, label, "round1-all-match")
	if err := g.RecordAnswers(ctx, "p1", label, "De la misma muestra.", ""); err != nil {
		t.Fatalf("RecordAnswers: %v", err)
	}
	if res := round(t, g, label, "round2-all-match-answers"); res.Outcome != cycle.OutcomeCoincides {
		t.Fatalf("old signoff round 2: %+v", res)
	}
	if err := g.Sign(ctx, "p1", label); err != nil {
		t.Fatalf("old signoff Sign: %v", err)
	}
}

// markSection is what «Confirm» does to a section's proposed role.
func markSection(t *testing.T, db *store.DB, label string) {
	t.Helper()
	ctx := context.Background()
	obj, err := db.GetObject(ctx, "p1", label)
	if err != nil {
		t.Fatalf("GetObject %s: %v", label, err)
	}
	var body map[string]any
	if err := json.Unmarshal(obj.Body, &body); err != nil {
		t.Fatalf("body of %s: %v", label, err)
	}
	body["role"] = markedRole
	err = db.Write(ctx, "p1", func(tx *store.Tx) error {
		_, err := tx.UpdateObject(obj.ID, obj.CurrentVersion, obj.State, body,
			store.EventInput{Type: store.EventRoleChanged, Actor: "human", ObjectID: obj.ID})
		return err
	})
	if err != nil {
		t.Fatalf("mark %s: %v", label, err)
	}
}

// The jury's path with TQ_PROVIDER=fake: «Confirm» marks the sections, and the
// app's fake has to answer the signoff whatever the section's labels are. Two
// signoffs in a row, since the recordings replay in turn: one with a critical
// decision and a mark, one with only the mark and no origin question.
func TestB106_TheAppsFakeSignsAMarkedSection(t *testing.T) {
	ctx := context.Background()
	db := openTemp(t)
	seedSignable(t, db, true)
	marked := seedThesisSection(t, db, "Conclusiones", markedRole)
	fixtures, err := provider.AppFixtures()
	if err != nil {
		t.Fatalf("AppFixtures: %v", err)
	}
	fake := provider.NewFake(fixtures...)
	fake.FitSignoff = true
	r := cycle.NewRunner(db, fake, nil, nil)
	g := cycle.NewGate(db)
	markSection(t, db, "sec-1")

	sign := func(section string, wantRounds int) {
		t.Helper()
		open := openSection(t, g, section)
		if err := g.RecordExplanation(ctx, "p1", open.Label, "Lo concluyo por el mismo tamaño mínimo."); err != nil {
			t.Fatalf("RecordExplanation: %v", err)
		}
		for round := 1; round <= wantRounds; round++ {
			if err := r.RunSignoffRound(ctx, "p1", "turn-w", open.Label); err != nil {
				t.Fatalf("round %d: %v", round, err)
			}
			if round < wantRounds {
				if err := g.RecordAnswers(ctx, "p1", open.Label, "De que la muestra es la misma.", ""); err != nil {
					t.Fatalf("RecordAnswers: %v", err)
				}
			}
		}
		if v := view(t, db, open.Label); v.Thesis != graph.ThesisConclusion || !v.CanSign {
			t.Fatalf("after %d rounds: thesis %q, can sign %v: %+v", wantRounds, v.Thesis, v.CanSign, v)
		}
		if err := g.Sign(ctx, "p1", open.Label); err != nil {
			t.Fatalf("Sign %s: %v", section, err)
		}
	}
	sign("sec-1", 2)
	sign(marked, 1)
}

// Outside the app's fake nothing is fitted: a test's recording stays what the
// model said, so the checks on Wonder's output keep seeing it.
func TestB106_ARecordingIsNotFittedUnlessAsked(t *testing.T) {
	ctx := context.Background()
	db := openTemp(t)
	seedSignable(t, db, true)
	fixtures, err := provider.AppFixtures()
	if err != nil {
		t.Fatalf("AppFixtures: %v", err)
	}
	r := cycle.NewRunner(db, provider.NewFake(fixtures...), nil, nil)
	g := cycle.NewGate(db)
	markSection(t, db, "sec-1")
	open := openSection(t, g, "sec-1")
	if err := g.RecordExplanation(ctx, "p1", open.Label, "Lo concluyo."); err != nil {
		t.Fatalf("RecordExplanation: %v", err)
	}
	if err := r.RunSignoffRound(ctx, "p1", "turn-w", open.Label); err != nil {
		t.Fatalf("round: %v", err)
	}
	if v := view(t, db, open.Label); v.Rounds != 0 {
		t.Fatalf("a recording without the thesis entry was accepted: %+v", v)
	}
}
