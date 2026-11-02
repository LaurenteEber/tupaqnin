package graph

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/LaurenteEber/tupaqnin/internal/fingerprint"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// Proposals (backlog-r4 4.1, specification §1.1, estados §5): what the
// agent wanted to write and the person decides. They are not pendings: an
// open proposal blocks nothing (estados §1, B-76).

// OperationReplaceAll is the proposal your edit displaced (B-73 d): it
// replaces the section's text, without an anchor.
const OperationReplaceAll = "replace_all"

// Why a proposal was born (B-73, B-77).
const (
	ProposalFromYourEdit     = "your_edit"
	ProposalFromSigned       = "signed"
	ProposalFromChanged      = "changed_since_signoff"
	ProposalFromSignoff      = "in_signoff"
	ProposalFromPlan         = "plan_confirmed"
	ProposalFromDisplacement = "displaced"
)

// Why a proposal was discarded (B-75, B-76, B-73 d).
const (
	ProposalDiscarded       = "discarded"
	ProposalSignedWithout   = "signed_without"
	ProposalAnchorNotUnique = "anchor_not_unique"
	// ProposalDecisionRevoked: it cites a decision revoked after it was born
	// (B-82); accepting it asks for a new step.
	ProposalDecisionRevoked = "decision_revoked"
	ProposalSectionChanged  = "section_changed"
)

// ProposalEdit is one text of the step on the section: T-4 keeps its
// anchor and operation.
type ProposalEdit struct {
	Anchor      string `json:"anchor"`
	Operation   string `json:"operation"`
	Text        string `json:"text"`
	ClaimOrigin string `json:"claim_origin,omitempty"`
}

// ProposalFigure and ProposalCitation are links that come back when the
// proposal is accepted (B-73); their ids are internal, like any body's.
type ProposalFigure struct {
	Anchor     string `json:"figure_anchor"`
	DecisionID string `json:"decision_id"`
}

type ProposalCitation struct {
	Anchor   string `json:"claim_anchor"`
	SourceID string `json:"source_id"`
	Passage  string `json:"passage"`
}

// ProposalClose is a handled_changes entry stored with the proposal (A-1).
type ProposalClose struct {
	ChangeID string `json:"change_id"`
	Result   string `json:"result"`
	Reason   string `json:"reason,omitempty"`
}

// ProposalNewSection is the section a proposal would create (B-77).
type ProposalNewSection struct {
	Title string `json:"title"`
	Role  string `json:"role"`
}

// ProposalDecision is a new decision of the step for that section, with
// its «en vez de» (B-74).
type ProposalDecision struct {
	Label     string `json:"label"`
	Statement string `json:"statement"`
	InsteadOf string `json:"instead_of"`
}

// ProposalContrast is what the app builds when the proposal is born
// (B-74): what leaves and what enters, the decisions, the triage line and
// the note. Version and Fingerprint are the current section's then, so the
// view can say it moved.
type ProposalContrast struct {
	Version     int                `json:"version"`
	Fingerprint string             `json:"fingerprint"`
	Removed     []string           `json:"removed"`
	Added       []string           `json:"added"`
	Decisions   []ProposalDecision `json:"decisions"`
	TriageLine  string             `json:"triage_line"`
	Note        string             `json:"note"`
}

// ProposalBody is the body of a proposal (specification §1.1, one per
// section and step).
// [EL DISEÑO NO LO DICE]: a step with several texts on one section is one
// proposal with one anchor in §1.1: the first goes in anchor, operation and
// text, the others in more_edits. used_decisions and used_sources are the
// step's, linked at accepting as a step links what it writes. origin and why
// say how it was born and how it closed; turn_id is the step's, for the
// history by block.
type ProposalBody struct {
	SectionID      string              `json:"section_id"`
	BaseVersion    int                 `json:"base_version"`
	Anchor         string              `json:"anchor"`
	Operation      string              `json:"operation"`
	Text           string              `json:"text"`
	ClaimOrigin    string              `json:"claim_origin,omitempty"`
	MoreEdits      []ProposalEdit      `json:"more_edits,omitempty"`
	Contrast       ProposalContrast    `json:"contrast"`
	Figures        []ProposalFigure    `json:"figures"`
	Citations      []ProposalCitation  `json:"citations"`
	HandledChanges []ProposalClose     `json:"handled_changes"`
	NewSection     *ProposalNewSection `json:"new_section,omitempty"`
	UsedDecisions  []string            `json:"used_decisions,omitempty"`
	UsedSources    []string            `json:"used_sources,omitempty"`
	Origin         string              `json:"origin"`
	Why            string              `json:"why,omitempty"`
	TurnID         string              `json:"turn_id,omitempty"`
}

// Edits are the proposal's texts in the step's order.
func (b ProposalBody) Edits() []ProposalEdit {
	if b.Operation == "" {
		return nil
	}
	first := ProposalEdit{Anchor: b.Anchor, Operation: b.Operation, Text: b.Text, ClaimOrigin: b.ClaimOrigin}
	return append([]ProposalEdit{first}, b.MoreEdits...)
}

// ParseProposal reads a proposal's body.
func ParseProposal(obj store.Object) (ProposalBody, error) {
	var b ProposalBody
	if err := json.Unmarshal(obj.Body, &b); err != nil {
		return ProposalBody{}, fmt.Errorf("graph: parse proposal %s: %w", obj.Label, err)
	}
	return b, nil
}

// ProposalView is a proposal for the page: labels, what it says and, when
// the current text moved since its contrast, that it did (backlog-r4 4.1).
type ProposalView struct {
	Label          string              `json:"label"`
	State          string              `json:"state"`
	Section        string              `json:"section"`
	BaseVersion    int                 `json:"base_version"`
	Anchor         string              `json:"anchor"`
	Operation      string              `json:"operation"`
	Text           string              `json:"text"`
	Contrast       ProposalContrast    `json:"contrast"`
	Figures        []string            `json:"figures"`
	Citations      []string            `json:"citations"`
	NewSection     *ProposalNewSection `json:"new_section,omitempty"`
	Origin         string              `json:"origin"`
	Why            string              `json:"why,omitempty"`
	VersionChanged bool                `json:"version_changed"`
}

// Proposals lists the project's proposals in the order they were born;
// with a section label, only those on it.
func Proposals(ctx context.Context, q store.Querier, projectID, sectionLabel string) ([]ProposalView, error) {
	objects, err := q.ListObjects(ctx, projectID, store.ObjectProposal)
	if err != nil {
		return nil, err
	}
	out := []ProposalView{}
	for _, obj := range objects {
		b, err := ParseProposal(obj)
		if err != nil {
			return nil, err
		}
		v := ProposalView{Label: obj.Label, State: obj.State, BaseVersion: b.BaseVersion, Anchor: b.Anchor,
			Operation: b.Operation, Text: b.Text, Contrast: b.Contrast, Figures: []string{}, Citations: []string{},
			NewSection: b.NewSection, Origin: b.Origin, Why: b.Why}
		for _, f := range b.Figures {
			v.Figures = append(v.Figures, f.Anchor)
		}
		for _, c := range b.Citations {
			v.Citations = append(v.Citations, c.Anchor)
		}
		if b.SectionID != "" {
			sec, err := q.GetObject(ctx, projectID, b.SectionID)
			if err != nil {
				return nil, err
			}
			v.Section = sec.Label
			v.VersionChanged = fingerprint.Section(bodyString(sec, "text")) != b.Contrast.Fingerprint
		}
		if sectionLabel != "" && v.Section != sectionLabel {
			continue
		}
		out = append(out, v)
	}
	return out, nil
}

// OpenProposals are the open ones of a section: the list at signing (B-76).
func OpenProposals(ctx context.Context, q store.Querier, projectID, sectionLabel string) ([]ProposalView, error) {
	all, err := Proposals(ctx, q, projectID, sectionLabel)
	if err != nil {
		return nil, err
	}
	out := []ProposalView{}
	for _, v := range all {
		if v.State == "open" {
			out = append(out, v)
		}
	}
	return out, nil
}

// ProposalLine is what the principal reads of its proposal, in «lo que
// cambió» (backlog-r4 4.1): its state, so it does not repeat it, with
// labels only (rule 7).
func ProposalLine(label, state string, b ProposalBody, section string) string {
	where := section
	if b.NewSection != nil {
		where = fmt.Sprintf("the new section «%s»", b.NewSection.Title)
	}
	switch {
	case state == "accepted":
		return fmt.Sprintf("Your proposal %s on %s was accepted by the person: its text is in the document now.", label, where)
	case state == "discarded" && b.Why == ProposalSignedWithout:
		return fmt.Sprintf("Your proposal %s on %s was discarded: the person signed %s without it. Do not propose it again.", label, where, where)
	case proposalAsksForAStep(state, b):
		return ProposalStepNotice(label, section)
	case state == "discarded":
		return fmt.Sprintf("Your proposal %s on %s was discarded by the person. Do not propose it again.", label, where)
	}
	return fmt.Sprintf("Your proposal %s on %s is open: the person accepts or discards it. It blocks nothing: go on with the rest and do not propose it again.",
		label, where)
}

// ProposalRead is what the principal reads of its proposal when it reaches
// it whole (backlog-r4 4.1): its line and, while it is open, its text.
func ProposalRead(label, state string, b ProposalBody, section string) string {
	line := ProposalLine(label, state, b, section)
	if state == "open" && b.Text != "" {
		return line + "\nIts text:\n" + b.Text
	}
	return line
}

func proposalAsksForAStep(state string, b ProposalBody) bool {
	return state == "discarded" && (b.Why == ProposalAnchorNotUnique || b.Why == ProposalSectionChanged || b.Why == ProposalDecisionRevoked)
}

// ProposalStepNotice is the line of a proposal that asks for a new step
// (B-75), with labels only: «lo que cambió» delivers it once, by the delivery
// table, in a live conversation or in a startup's list, whatever step ended
// meanwhile (a step at the model when you accepted was sent before it).
func ProposalStepNotice(label, section string) string {
	return fmt.Sprintf("The person accepted your proposal %s, but %s no longer has the text it was written against: it was not applied. "+
		"Write a new step for %s against its current text", label, section, section)
}
