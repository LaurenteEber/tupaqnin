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
	// ProbeKindQueued is a probe the app queued (P-72): what is
	// protected and the sections it goes against.
	ProbeKindQueued = "queued"
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
			if p.Cause == CauseDoesNotSupport {
				p.Passage = passage // P-110: after seen, so one pending per citation stays
			}
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

// IsAcceptedText says an event wrote the text of a proposal you accepted (B-75):
// the agent's text, which no step's checker saw (P-114). The links of the same
// acceptance carry accepted_by too, without texts.
func IsAcceptedText(ev store.Event) bool {
	if ev.Type != store.EventEdit && ev.Type != store.EventStateChanged {
		return false
	}
	data := eventData(ev)
	_, texts := data["texts"]
	return texts && data["accepted_by"] == "human"
}

// AcceptedUnchecked is «sin comprobar» for an accepted proposal's text (P-114):
// no agent text reaches a signoff unchecked. Only a pass that read that version
// clears it: a step's pass gets that step's passage, and a batch already running
// when you accepted read the text before. One that fails clears nothing and the
// next trigger sends it again, as a queued probe (P-74).
func AcceptedUnchecked(events []store.Event, section store.Object) bool {
	accepted, read := 0, 0
	for _, ev := range events {
		if IsAcceptedText(ev) && ev.ObjectID == section.ID {
			accepted = ev.NewVersion
		}
		if ev.Type != store.EventProbe {
			continue
		}
		if saw, ok := eventData(ev)["saw_versions"].(map[string]any); ok {
			if v, ok := saw[section.Label].(float64); ok {
				read = max(read, int(v))
			}
		}
	}
	return read < accepted
}

// QueuedProbe is a probe of what is protected (by label) against the
// current text of some sections (by label) that the app queued and the
// checker has not run yet (P-72): the probe of a new premise (B-46,
// estados §6, fichas §3) and, in 2.8, of a new criterion. Seq is its
// probe event with kind queued.
type QueuedProbe struct {
	Seq       int
	Protected string
	Sections  []string
}

// PendingProbes are the queued probes still pending (P-72), in the
// order they were queued. Each is a probe event with kind queued and
// closes only with the probe events of its own valid judgment, which
// carry its seq in data.queued_probe. A failed run (its output_retry)
// does not close it: it stays queued and is retried at every trigger,
// so the existing text is never left uncontrasted (P-74, B-50).
// [EL DISEÑO NO LO DICE]: how a queued probe is kept; the history has
// no event for it, so it is a probe event with kind queued (as the
// candidates are kept in probe with kind candidate) and no mutable flag.
func PendingProbes(events []store.Event) []QueuedProbe {
	var queued []QueuedProbe
	closed := map[int]bool{}
	for _, ev := range events {
		if ev.Type != store.EventProbe {
			continue
		}
		data := eventData(ev)
		switch {
		case ev.Type == store.EventProbe && data["kind"] == ProbeKindQueued:
			qp := QueuedProbe{Seq: ev.Seq}
			qp.Protected, _ = data["protected"].(string)
			list, _ := data["queued_sections"].([]any)
			for _, l := range list {
				if s, ok := l.(string); ok {
					qp.Sections = append(qp.Sections, s)
				}
			}
			queued = append(queued, qp)
		default:
			if seq, ok := data["queued_probe"].(float64); ok {
				closed[int(seq)] = true
			}
		}
	}
	var out []QueuedProbe
	for _, qp := range queued {
		if !closed[qp.Seq] {
			out = append(out, qp)
		}
	}
	return out
}

// probePending reports whether a pending queued probe goes against the
// section: it is «sin comprobar» until that probe runs (P-72, B-50).
func probePending(events []store.Event, sectionLabel string) bool {
	for _, qp := range PendingProbes(events) {
		for _, s := range qp.Sections {
			if s == sectionLabel {
				return true
			}
		}
	}
	return false
}
