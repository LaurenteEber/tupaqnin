package graph

import (
	"context"
	"encoding/json"

	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// UnappliedDecision is an answer of yours that changes a section's text and has
// not reached it (B-100): nothing of the agent touched the section since. It is
// computed from the history, so it survives a restart. Text is literal.
type UnappliedDecision struct {
	Section string `json:"section"`
	Title   string `json:"title"`
	Text    string `json:"text"`
	Reason  string `json:"reason"`
	// Decision is the decision born from the answer; empty when an answer to a
	// «por aclarar» made none.
	Decision string `json:"decision,omitempty"`
	// Source is the card or «por aclarar» you answered. Never shown.
	Source string `json:"-"`
	// Ready: the section has no card or «por aclarar» of yours open, so the agent
	// can apply it (one batch, not one step per answer).
	Ready bool `json:"ready"`
}

// UnappliedDecisions lists them oldest first. An answer records what it changes
// in its event (applies_to, applies_text); the agent's next event on the section
// (its text, a handled change) or a proposal for it (B-73) closes it.
func UnappliedDecisions(ctx context.Context, q store.Querier, projectID string) ([]UnappliedDecision, error) {
	events, err := q.EventsSince(ctx, projectID, 0)
	if err != nil {
		return nil, err
	}
	type open struct {
		section, text, reason, decision, source string
		// during: the turn whose step was in flight when you answered. It did not
		// read your answer, so what it writes closes nothing until it ends.
		during string
	}
	var pending []open
	for _, ev := range events {
		var data struct {
			AppliesTo   []string `json:"applies_to"`
			AppliesText string   `json:"applies_text"`
			Reason      string   `json:"applies_reason"`
			During      string   `json:"applies_during"`
			Decision    string   `json:"applies_decision"`
			Section     string   `json:"section"`
			To          string   `json:"to"`
		}
		_ = json.Unmarshal(ev.Data, &data)
		for _, section := range data.AppliesTo {
			pending = append(pending, open{section: section, text: data.AppliesText, reason: data.Reason, decision: data.Decision, source: ev.ObjectID, during: data.During})
		}
		if ev.Actor != "agent" {
			continue
		}
		kept := pending[:0]
		for _, p := range pending {
			if p.during != "" && p.during == ev.TurnID {
				if ev.Type == store.EventStepEnded {
					p.during = ""
				}
				kept = append(kept, p)
				continue
			}
			if ev.ObjectID == p.section || data.To == "open" && data.Section != "" && sectionNamed(ctx, q, projectID, data.Section) == p.section {
				continue
			}
			kept = append(kept, p)
		}
		pending = kept
	}
	out := make([]UnappliedDecision, 0, len(pending))
	for _, p := range pending {
		sec, err := q.GetObject(ctx, projectID, p.section)
		if err != nil {
			return nil, err
		}
		var body struct {
			Title string `json:"title"`
		}
		_ = json.Unmarshal(sec.Body, &body)
		_, pendings, err := SectionStatus(ctx, q, projectID, sec.Label)
		if err != nil {
			return nil, err
		}
		ready := true
		for _, pending := range pendings {
			ready = ready && !pending.IsYours()
		}
		out = append(out, UnappliedDecision{Section: sec.Label, Title: body.Title, Text: p.text, Reason: p.reason, Decision: p.decision, Source: p.source, Ready: ready})
	}
	return out, nil
}

// sectionNamed is the internal id of a proposal's section label, "" if none.
func sectionNamed(ctx context.Context, q store.Querier, projectID, label string) string {
	sec, err := q.GetObject(ctx, projectID, label)
	if err != nil {
		return ""
	}
	return sec.ID
}
