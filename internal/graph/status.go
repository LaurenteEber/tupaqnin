package graph

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// Status is a section's state (estados §1). Ready is never stored: it
// is a draft with no pendings.
type Status string

const (
	StatusDraft               Status = "draft"
	StatusReady               Status = "ready"
	StatusSigned              Status = "signed"
	StatusChangedSinceSignoff Status = "changed_since_signoff"
	StatusStale               Status = "stale"
)

// Pending kinds of slice 1 (glossary §9, B-15, B-16).
const (
	PendingOpenCard              = "open_card"
	PendingFigureWithoutDecision = "figure_without_decision"
	PendingSourceNotSupporting   = "source_not_supporting"
	PendingMalformedOutput       = "malformed_output"
	// Slice 2 (glossary §9, backlog-r2 2.1).
	PendingTextWithoutDecision = "text_without_decision"
	PendingInternalCodes       = "internal_codes"
	// From the checker (glossary §9, backlog-r2 2.3, B-50).
	PendingProcessNarrative = "process_narrative"
	// PendingUnchecked is «sin comprobar» (B-50): the checker failed
	// twice on a change of the section.
	// [EL DISEÑO NO LO DICE]: the glossary has no code name for it; this
	// is the smallest English one.
	PendingUnchecked = "unchecked"
	// PendingOpenQuestion is an open «por aclarar» (glossary §9).
	PendingOpenQuestion = "open_question"
)

// Causes of «fuente que no sostiene» (backlog-r2 2.3): the passage is
// not literal in its source (the app), or it is and does not support
// the claim (the checker, A-2).
const (
	CausePassageNotFound = "passage_not_found"
	CauseDoesNotSupport  = "does_not_support"
)

// Pending is one thing a section lacks: its kind and what it is about,
// by label or by the number in the text.
type Pending struct {
	Kind  string `json:"kind"`
	About string `json:"about,omitempty"`
	// Number names an open card on the signoff screen by its decision's
	// place among the critical decisions, from 1 (P-62, as P-59); 0
	// elsewhere.
	Number int `json:"number,omitempty"`
	// Cause tells the two causes of source_not_supporting apart
	// (CausePassageNotFound, CauseDoesNotSupport); empty on other kinds.
	Cause string `json:"cause,omitempty"`
	// Passage is what the checker marked on a does_not_support citation: the
	// text the pending closes with (P-110). For the agent, not for the page.
	Passage string `json:"-"`
}

// IsYours reports a pending only you can close: an open card or an open «por
// aclarar» (glossary §9, human_pending). All the others are the agent's.
func (p Pending) IsYours() bool {
	return p.Kind == PendingOpenCard || p.Kind == PendingOpenQuestion
}

// PendingsAtCap splits a section's pendings for the signoff (B3-20, Eber
// 2-nov): with a cap stopping the agent, its pendings do not block, since it
// can no longer close them; yours still do. Below the caps every pending
// blocks. stoppedBy is what Runner.StoppedBy answers.
func PendingsAtCap(pendings []Pending, stoppedBy string) (blocking, atCap []Pending) {
	blocking, atCap = []Pending{}, []Pending{}
	for _, p := range pendings {
		if stoppedBy == "" || p.IsYours() {
			blocking = append(blocking, p)
		} else {
			atCap = append(atCap, p)
		}
	}
	return blocking, atCap
}

// SectionStatus is the one function every view uses for a section's
// state and its pendings (specification §4, estados §1). The pendings of
// slice 1 are the open cards of the decisions the section uses and the
// agent's pendings, computed (B-15), «falta en el texto» included.
func SectionStatus(ctx context.Context, db store.Querier, projectID, label string) (Status, []Pending, error) {
	section, err := db.GetObject(ctx, projectID, label)
	if err != nil {
		return "", nil, err
	}
	pendings := []Pending{}

	// Open cards of the decisions used in the section (estados §3: an
	// open card blocks the signoff of the sections that use it).
	cards, err := db.ListObjects(ctx, projectID, store.ObjectCard)
	if err != nil {
		return "", nil, err
	}
	for _, card := range cards {
		if card.State != "open" {
			continue
		}
		used, err := usedIn(ctx, db, projectID, bodyString(card, "decision_id"), section.ID)
		if err != nil {
			return "", nil, err
		}
		// The card of your critical edit (R4, B-49) keeps the section it
		// was opened on pending until you answer it.
		if used || bodyString(card, "section_id") == section.ID {
			pendings = append(pendings, Pending{Kind: PendingOpenCard, About: card.Label})
		}
	}

	// Open «por aclarar» (estados §4: it blocks the signoff of all that
	// depends on it): the section it blocks, or one that uses a decision
	// it blocks (backlog-r2 2.4).
	questions, err := OpenQuestionsBlocking(ctx, db, projectID, section.ID)
	if err != nil {
		return "", nil, err
	}
	for _, q := range questions {
		pendings = append(pendings, Pending{Kind: PendingOpenQuestion, About: q})
	}

	figures, err := FiguresWithoutDecision(ctx, db, projectID, section.ID)
	if err != nil {
		return "", nil, err
	}
	for _, n := range figures {
		pendings = append(pendings, Pending{Kind: PendingFigureWithoutDecision, About: n})
	}
	citations, err := SourcesNotSupporting(ctx, db, projectID, section.ID)
	if err != nil {
		return "", nil, err
	}
	for _, c := range citations {
		pendings = append(pendings, Pending{Kind: PendingSourceNotSupporting, About: c, Cause: CausePassageNotFound})
	}

	events, err := db.EventsSince(ctx, projectID, 0)
	if err != nil {
		return "", nil, err
	}
	// The checker's (backlog-r2 2.3): its candidates while their passage
	// is literal in the text, and «sin comprobar» (B-50), also while a
	// queued probe over the section is pending (P-72).
	pendings = append(pendings, checkerPendings(events, section)...)
	if uncheckedOpen(events, section.Label) || probePending(events, section.Label) || acceptedUnchecked(events, section) {
		pendings = append(pendings, Pending{Kind: PendingUnchecked})
	}
	// «Escrito sin decisión» (rule 2, B-42) and «códigos internos»
	// (rule 5), slice 2.
	if textWithoutDecision(events, section.Label) {
		pendings = append(pendings, Pending{Kind: PendingTextWithoutDecision})
	}
	codes, err := InternalCodes(ctx, db, projectID, section.ID)
	if err != nil {
		return "", nil, err
	}
	for _, c := range codes {
		pendings = append(pendings, Pending{Kind: PendingInternalCodes, About: c})
	}
	// «Afirmación sin fuente» (R-1, backlog-r2 2.11): computed from the
	// current text, only when the document asks for evidence (B-51).
	unsourced, err := UnsourcedClaims(ctx, db, projectID, section.ID)
	if err != nil {
		return "", nil, err
	}
	for _, passage := range unsourced {
		pendings = append(pendings, Pending{Kind: PendingUnsourcedClaim, About: passage})
	}
	if malformedOutputOpen(events) {
		pendings = append(pendings, Pending{Kind: PendingMalformedOutput})
	}
	// «Falta en el texto» señalado por Wonder en la firma (B-14, B-15):
	// the passage of the signoff's event, until the section's fingerprint
	// changes.
	for _, passage := range missingInText(section, events) {
		pendings = append(pendings, Pending{Kind: PendingMissingInText, About: passage})
	}

	// Ready = text, no pendings (B-21) and no open changes: a section
	// without text stays draft and is not offered for signing. Open
	// changes are not pendings, but they count here in any state (A-1,
	// E-4; backlog-r2 2.2): a draft never shows ready with one.
	status := Status(section.State)
	if status == StatusDraft && len(pendings) == 0 && len(OpenChanges(section)) == 0 &&
		strings.TrimSpace(bodyString(section, "text")) != "" {
		status = StatusReady
	}
	return status, pendings, nil
}

func usedIn(ctx context.Context, db store.Querier, projectID, decisionID, sectionID string) (bool, error) {
	edges, err := db.EdgesFrom(ctx, projectID, decisionID)
	if err != nil {
		return false, err
	}
	for _, e := range edges {
		if e.Type == store.EdgeUsedIn && e.ToID == sectionID {
			return true, nil
		}
	}
	return false, nil
}

func eventData(ev store.Event) map[string]any {
	var data map[string]any
	_ = json.Unmarshal(ev.Data, &data)
	return data
}

// LeavesAgentPending reports whether an output_retry left its output
// pending for the agent (fichas §7). The stop report's never does
// (P-75 (c)): estados §1 ties the agent's pendings to a section and the
// report writes in none; a history written before P-75 may still carry
// one with result agent_pending, and it is ignored.
func LeavesAgentPending(ev store.Event) bool {
	if ev.Type != store.EventOutputRetry {
		return false
	}
	data := eventData(ev)
	return data["result"] == "agent_pending" && data["turn_kind"] != "stop_report"
}

// malformedOutputOpen is «retorno sin la forma fija» (B-15): the last
// output_retry that left the step pending for the agent, until the next
// valid step end.
// [EL DISEÑO NO LO DICE]: the return names no section; it counts for
// every section until it closes.
func malformedOutputOpen(events []store.Event) bool {
	open := false
	for _, ev := range events {
		switch ev.Type {
		case store.EventOutputRetry:
			if LeavesAgentPending(ev) {
				open = true
			}
		case store.EventStepEnded:
			open = false
		case store.EventStopReport:
			// [EL DISEÑO NO LO DICE]: a stop at the project's cap closes it: no
			// step can, and a failed step says nothing about the built text
			// (B3-20: the cap never keeps you from signing it).
			// The mode's last stop says it in its cause (B-83).
			if data := eventData(ev); data["stop_reason"] == "project_cap" || data["cause"] == "project_cap" {
				open = false
			}
		}
	}
	return open
}

func contains(list any, want string) bool {
	items, _ := list.([]any)
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}

// OpenQuestionsBlocking returns the labels of the open «por aclarar»
// that block a section (backlog-r2 2.4): by a current blocks edge to
// it, or to a decision used in it.
// [EL DISEÑO NO LO DICE]: a section that uses a blocked decision counts
// as depending on the question, as with an open card (estados §3).
func OpenQuestionsBlocking(ctx context.Context, db store.Querier, projectID, sectionID string) ([]string, error) {
	questions, err := db.ListObjects(ctx, projectID, store.ObjectOpenQuestion)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, q := range questions {
		if q.State != "open" {
			continue
		}
		edges, err := db.EdgesFrom(ctx, projectID, q.ID)
		if err != nil {
			return nil, err
		}
		blocks := false
		for _, e := range edges {
			if e.Type != store.EdgeBlocks {
				continue
			}
			if e.ToID == sectionID {
				blocks = true
				break
			}
			used, err := usedIn(ctx, db, projectID, e.ToID, sectionID)
			if err != nil {
				return nil, err
			}
			if used {
				blocks = true
				break
			}
		}
		if blocks {
			out = append(out, q.Label)
		}
	}
	return out, nil
}
