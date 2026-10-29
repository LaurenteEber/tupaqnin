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
	// PendingSupersededDecision is «texto con una decisión reemplazada»
	// (B-16). [EL DISEÑO NO LO DICE]: the glossary has no code name for
	// it; this is the smallest English one.
	PendingSupersededDecision = "superseded_decision_in_text"
)

// Pending is one thing a section lacks: its kind and what it is about,
// by label or by the number in the text.
type Pending struct {
	Kind  string `json:"kind"`
	About string `json:"about,omitempty"`
}

// SectionStatus is the one function every view uses for a section's
// state and its pendings (specification §4, estados §1). The pendings of
// slice 1 are the open cards of the decisions the section uses and the
// agent's pendings, computed (B-15), «falta en el texto» included.
func SectionStatus(ctx context.Context, db *store.DB, projectID, label string) (Status, []Pending, error) {
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
		if used {
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
	if malformedOutputOpen(events) {
		pendings = append(pendings, Pending{Kind: PendingMalformedOutput})
	}
	superseded, err := supersededDecisionsInText(ctx, db, projectID, section, events)
	if err != nil {
		return "", nil, err
	}
	for _, d := range superseded {
		pendings = append(pendings, Pending{Kind: PendingSupersededDecision, About: d})
	}
	// «Falta en el texto» señalado por Wonder en la firma (B-14, B-15):
	// the passage of the signoff's event, until the section's fingerprint
	// changes.
	for _, passage := range missingInText(section, events) {
		pendings = append(pendings, Pending{Kind: PendingMissingInText, About: passage})
	}

	// Ready = text and no pendings (B-21): a section without text stays
	// draft and is not offered for signing.
	status := Status(section.State)
	if status == StatusDraft && len(pendings) == 0 && strings.TrimSpace(bodyString(section, "text")) != "" {
		status = StatusReady
	}
	return status, pendings, nil
}

func usedIn(ctx context.Context, db *store.DB, projectID, decisionID, sectionID string) (bool, error) {
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

// supersededDecisionsInText is «texto con una decisión reemplazada»
// (B-15, B-16): a decision of the person born from a card's other option
// and used in the section, until an agent step that touches the section
// declares it among the decisions it uses.
func supersededDecisionsInText(ctx context.Context, db *store.DB, projectID string, section store.Object, events []store.Event) ([]string, error) {
	born := map[string]int{} // decision id → seq of its birth from a card
	for _, ev := range events {
		data := eventData(ev)
		if ev.Type == store.EventStateChanged && data["via"] == "card" && data["replaces"] != nil {
			born[ev.ObjectID] = ev.Seq
		}
	}
	var out []string
	for decID, seq := range born {
		used, err := usedIn(ctx, db, projectID, decID, section.ID)
		if err != nil {
			return nil, err
		}
		if !used {
			continue
		}
		dec, err := db.GetObject(ctx, projectID, decID)
		if err != nil {
			return nil, err
		}
		if !declaredAfter(events, seq, section.Label, dec.Label) {
			out = append(out, dec.Label)
		}
	}
	return out, nil
}

// declaredAfter reports an agent step after seq that wrote the section
// and declared the decision in used_decisions (its step_ended records
// both).
func declaredAfter(events []store.Event, seq int, sectionLabel, decisionLabel string) bool {
	for _, ev := range events {
		if ev.Seq <= seq || ev.Type != store.EventStepEnded {
			continue
		}
		data := eventData(ev)
		if contains(data["written_sections"], sectionLabel) && contains(data["used_decisions"], decisionLabel) {
			return true
		}
	}
	return false
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
