package cycle_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/LaurenteEber/tupaqnin/internal/cycle"
	"github.com/LaurenteEber/tupaqnin/internal/graph"
	"github.com/LaurenteEber/tupaqnin/internal/memory"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// agentLine is the line the agent receives for a pending kind (and cause).
func agentLine(t *testing.T, db *store.DB, kind string) string {
	t.Helper()
	lines, err := memory.New(db).AgentPendings(context.Background(), "p1")
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range lines {
		if strings.Contains(line, kind) {
			return line
		}
	}
	t.Fatalf("no line for %q in %v", kind, lines)
	return ""
}

func wantLine(t *testing.T, line string, wants ...string) {
	t.Helper()
	for _, want := range wants {
		if !strings.Contains(line, want) {
			t.Errorf("line misses %q: %s", want, line)
		}
	}
}

// P-111: every agent pending says its passage and what closes it, from what
// computes it: FiguresWithoutDecision closes when linked or out of the text.
func TestP111_AFigureWithoutDecisionSaysHowItCloses(t *testing.T) {
	db := seedStepProject(t)
	if applied := applyRecorded(t, db, stepFile(t, "figure-datum-loose")); !applied.Applied {
		t.Fatalf("not applied: %v", applied.Errors)
	}
	wantLine(t, agentLine(t, db, graph.PendingFigureWithoutDecision), "passage: «", "7 casos",
		"link it to a decision with figure_links or take it out of the text")
}

// InternalCodes closes when the code leaves the text.
func TestP111_AnInternalCodeSaysHowItCloses(t *testing.T) {
	db := seedStepProject(t)
	out := `{"stretch":"build","triage_line":"Construir.","closing_risk":"none","note":"n",
		"new_text":[{"section_id":"sec-1","base_version":1,"anchor":"Cierre.","claim_origin":"material","operation":"replace","text":"Según dec-2 y el Formulario F-40."}]}`
	if applied := applyRecorded(t, db, json.RawMessage(out)); !applied.Applied {
		t.Fatalf("ApplyStep: %v", applied.Errors)
	}
	wantLine(t, agentLine(t, db, graph.PendingInternalCodes), "internal_codes: dec-2", "passage: «", "Según dec-2 y el Formulario F-40.»",
		"rewrite this passage without the code")
}

// A checker's process narrative closes when its passage leaves the text.
func TestP111_AProcessNarrativeSaysHowItCloses(t *testing.T) {
	db := seedCheckerProject(t)
	passage := "Tras la revisión del martes, ajustamos el filtro."
	runCheckedTurn(t, db, nil, stepWriting(t, passage, ""), checkerFixture(t, "process-narrative"))
	wantLine(t, agentLine(t, db, graph.PendingProcessNarrative), passage, "rewrite this passage")
}

// Rule 2's «escrito sin decisión» closes with a decision used in the section.
func TestP111_TextWithoutDecisionFromTheRuleSaysHowItCloses(t *testing.T) {
	db := seedConclusionWithoutDecision(t)
	wantLine(t, agentLine(t, db, graph.PendingTextWithoutDecision), "sec-2",
		"declare in new_decisions the decision this section rests on, with this section in used_in_sections")
}

// P-111, B-42: a decision declared later, used in the section, closes rule 2's
// pending though that step writes no text: nothing else could close it.
func TestB42_ADecisionDeclaredInALaterStepClosesTextWithoutDecision(t *testing.T) {
	db := seedConclusionWithoutDecision(t)
	later := `{"stretch":"build","triage_line":"Construir.","closing_risk":"none","note":"n",
		"new_decisions":[{"id":"new-1","statement":"Concluir por veces.","instead_of":"Concluir por conteos.",
			"would_choose_if":"Todos usaran el mismo filtro.","assumption":false,"used_in_sections":["sec-2"]}]}`
	if applied := applyRecorded(t, db, json.RawMessage(later)); !applied.Applied {
		t.Fatalf("later step: %v", applied.Errors)
	}
	if got := anyStrings(lastStepEnded(t, db)["decided_sections"]); strings.Join(got, ",") != "sec-2" {
		t.Fatalf("decided_sections: %v", got)
	}
	if _, pendings := status(t, db, "sec-2"); hasPending(pendings, graph.PendingTextWithoutDecision, "") {
		t.Fatalf("sec-2 pendings: %v, want it closed", pendings)
	}
}

// A decision used in another section does not close it.
func TestB42_ADecisionUsedInAnotherSectionLeavesItOpen(t *testing.T) {
	db := seedConclusionWithoutDecision(t)
	later := `{"stretch":"build","triage_line":"Construir.","closing_risk":"none","note":"n",
		"new_decisions":[{"id":"new-1","statement":"Concluir por veces.","instead_of":"Concluir por conteos.",
			"would_choose_if":"Todos usaran el mismo filtro.","assumption":false,"used_in_sections":["sec-1"]}]}`
	if applied := applyRecorded(t, db, json.RawMessage(later)); !applied.Applied {
		t.Fatalf("later step: %v", applied.Errors)
	}
	if _, pendings := status(t, db, "sec-2"); !hasPending(pendings, graph.PendingTextWithoutDecision, "") {
		t.Fatalf("sec-2 pendings: %v, want it open", pendings)
	}
}

func seedConclusionWithoutDecision(t *testing.T) *store.DB {
	t.Helper()
	db := seedStepProject(t)
	first := `{"stretch":"build","triage_line":"Construir.","closing_risk":"none","note":"n",
		"proposed_roles":[{"section_id":"new-1","role":"conclusion","title":"Conclusiones"}],
		"new_text":[{"section_id":"new-1","base_version":0,"anchor":"","claim_origin":"material","operation":"insert_after","text":"La embotellada trae más partículas."}]}`
	if applied := applyRecorded(t, db, json.RawMessage(first)); !applied.Applied || applied.Labels["new-1"] != "sec-2" {
		t.Fatalf("first step: %+v", applied)
	}
	return db
}

// The checker's «escrito sin decisión» closes when its passage leaves the text.
func TestP111_TextWithoutDecisionFromTheCheckerSaysHowItCloses(t *testing.T) {
	db := seedCheckerProject(t)
	const passage = "Fin del texto marcado."
	if err := db.Write(context.Background(), "p1", func(tx *store.Tx) error {
		sec, err := tx.GetObject(context.Background(), "p1", "sec-1")
		if err != nil {
			return err
		}
		if _, err := tx.UpdateObject(sec.ID, sec.CurrentVersion, sec.State,
			map[string]any{"title": "Síntesis", "text": passage}, store.EventInput{Type: store.EventEdit, Actor: "human"}); err != nil {
			return err
		}
		_, err = tx.AppendEvent(store.EventInput{Type: store.EventProbe, Actor: "agent", Data: map[string]any{
			"kind": graph.ProbeKindCandidate, "section": "sec-1", "sections": []string{"sec-1"},
			"type": graph.CandidateWrittenWithoutDecision, "new_passage": passage}})
		return err
	}); err != nil {
		t.Fatal(err)
	}
	wantLine(t, agentLine(t, db, graph.PendingTextWithoutDecision), passage, "rewrite this passage")
}

// «Sin comprobar» closes with the checker's next valid judgment.
func TestP111_UncheckedSaysHowItCloses(t *testing.T) {
	db := seedCheckerProject(t)
	runCheckedTurn(t, db, nil, stepWriting(t, "Fin del texto.", ""), checkerFixture(t, "malformed"), checkerFixture(t, "malformed"))
	wantLine(t, agentLine(t, db, graph.PendingUnchecked), "the next step that changes this section runs the checker again")
}

// «Afirmación sin fuente» closes with the citation or the paragraph out.
func TestP111_AnUnsourcedClaimSaysHowItCloses(t *testing.T) {
	db := seedStepProject(t)
	seedWebSource(t, db, "")
	setEvidenceRequired(t, db, true)
	if applied := applyRecorded(t, db, stepExternal("external", "src-2", webPassage)); !applied.Applied {
		t.Fatalf("step not applied: %+v", applied)
	}
	wantLine(t, agentLine(t, db, graph.PendingUnsourcedClaim), "unsourced_claim: La síntesis",
		"cite the literal passage of a source you read or take this paragraph out")
}

// «Falta en el texto» closes when the section's text changes.
func TestP111_MissingInTextSaysHowItCloses(t *testing.T) {
	db := openTemp(t)
	seedSignable(t, db, true)
	g := cycle.NewGate(db)
	sign := openSignoff(t, g)
	_ = g.RecordExplanation(context.Background(), "p1", sign, "Comparo solo estudios con el mismo tamaño.")
	if res := round(t, g, sign, "missing-in-text"); res.Outcome != cycle.OutcomeMissingInText {
		t.Fatalf("outcome: %q", res.Outcome)
	}
	wantLine(t, agentLine(t, db, graph.PendingMissingInText), *fixtureOut(t, "missing-in-text").Decisions[0].Passage,
		"write what is missing into the section")
}

// A citation whose passage is not in its source says the claim it backs.
func TestP111_ACitationNotFoundInItsSourceSaysHowItCloses(t *testing.T) {
	db := seedCheckerProject(t)
	runCheckedTurn(t, db, nil, textFixture(t, string(stepFile(t, "citations"))), checkerFixture(t, "source-not-supporting"))
	wantLine(t, agentLine(t, db, graph.CausePassageNotFound), "cit-2", "passage: «", "rewrite this passage")
}
