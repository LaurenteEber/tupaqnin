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
		pendings = append(pendings, Pending{Kind: PendingSourceNotSupporting, About: c})
	}

	events, err := db.EventsSince(ctx, projectID, 0)
	if err != nil {
		return "", nil, err
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
			if eventData(ev)["result"] == "agent_pending" {
				open = true
			}
		case store.EventStepEnded:
			open = false
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
