package graph

import (
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// Candidate types of the checker's fixed output
// (salida-comprobador.schema.json, fichas §7).
const (
	CandidateContradiction          = "contradiction"
	CandidateDoubtfulTerm           = "doubtful_term"
	CandidateProcessNarrative       = "process_narrative"
	CandidateWrittenWithoutDecision = "written_without_decision"
	CandidateSourceNotSupporting    = "source_not_supporting"
	CandidateExceedsReviewed        = "exceeds_reviewed"
)

// What a probe event records in its data.kind (backlog-r2 2.3): the
// verdict of one probe, or one candidate (stored «en probe con kind:
// candidate»).
const (
	ProbeKindProbe     = "probe"
	ProbeKindCandidate = "candidate"
)

// checkerPendings are the agent's pendings the checker left on a
// section (flujos §1, B-15 as precised in backlog-r2 2.1): «relato del
// proceso», «escrito sin decisión» outside conclusions and «fuente que
// no sostiene» with the cause does_not_support. Each stays in its probe
// event with its passage and closes when that passage is no longer
// literal in the section; a discarded candidate (B-50) never opens.
func checkerPendings(events []store.Event, section store.Object) []Pending {
	text := Compare(bodyString(section, "text"))
	var out []Pending
	seen := map[Pending]bool{}
	for _, ev := range events {
		if ev.Type != store.EventProbe {
			continue
		}
		data := eventData(ev)
		if data["kind"] != ProbeKindCandidate || data["section"] != section.Label || data["discarded"] == true {
			continue
		}
		passage, _ := data["new_passage"].(string)
		if len(text.Find(passage)) == 0 {
			continue
		}
		var p Pending
		switch data["type"] {
		case CandidateProcessNarrative:
			p = Pending{Kind: PendingProcessNarrative, About: passage}
		case CandidateWrittenWithoutDecision:
			p = Pending{Kind: PendingTextWithoutDecision, About: passage}
		case CandidateSourceNotSupporting:
			citation, _ := data["citation"].(string)
			p = Pending{Kind: PendingSourceNotSupporting, About: citation, Cause: CauseDoesNotSupport}
		default:
			continue
		}
		if !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}
	return out
}

// uncheckedOpen is «sin comprobar» (B-50): the checker failed twice on
// a change of the section (an output_retry of turn kind checker with
// result unchecked), until the next valid judgment over it (a probe
// event whose sections include it).
func uncheckedOpen(events []store.Event, sectionLabel string) bool {
	open := false
	for _, ev := range events {
		data := eventData(ev)
		switch {
		case ev.Type == store.EventOutputRetry && data["turn_kind"] == "checker" &&
			data["result"] == "unchecked" && contains(data["sections"], sectionLabel):
			open = true
		case ev.Type == store.EventProbe && contains(data["sections"], sectionLabel):
			open = false
		}
	}
	return open
}
