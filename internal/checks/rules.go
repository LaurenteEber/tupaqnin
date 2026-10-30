// Package checks runs the app's rules after each change, without a
// model (flujos §1, fichas §3; backlog-r2 2.1): after an agent step and
// after each save of yours that changes the fingerprint (B-36). The
// rules read the change's own transaction and never stop the agent:
// what they find enters the next triage (flujos §1, «sin bucles»).
// Their pendings come from the functions of internal/graph (B-15); this
// package only orders them and says what each change found.
package checks

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/LaurenteEber/tupaqnin/internal/graph"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// Kind is who made the change.
type Kind string

const (
	// AgentStep is an applied step of the principal.
	AgentStep Kind = "agent_step"
	// YourEdit is a save of yours that changed the fingerprint (B-36).
	YourEdit Kind = "your_edit"
)

// Stretches of the triage (fichas §8.3, B-43).
const (
	StretchDirect   = "direct"
	StretchDiscover = "discover"
	StretchBuild    = "build"
	StretchVerify   = "verify"
)

// Why a direct step is reclassified as build (rule 0, fichas §3 «Vía
// directa»): it touches a figure, a citation, a section with role
// conclusion (recommendation counts the same, R-8) or a criterion.
const (
	TouchesFigure     = "figure"
	TouchesCitation   = "citation"
	TouchesConclusion = "conclusion"
	TouchesCriterion  = "criterion"
)

// Finding kinds besides the graph's pending kinds.
const (
	// FindingReclassified: rule 0 moved a direct step to build.
	FindingReclassified = "reclassified"
	// FindingCritical: rule 3, a critical decision that a touched section
	// uses. Its card, if it is the agent's and just became critical, is
	// opened by the criticality comparison of the same transaction (T-5).
	FindingCritical = "critical"
	// FindingCriticalEdit: rule 3 on your edit (R4, B-49): it removed or
	// changed the anchor of a figure whose decision is critical. The
	// caller opens its card, which asks only for the «porque».
	FindingCriticalEdit = "critical_edit"
)

// Change is an applied agent step or an edit of yours with its base
// version (backlog-r2 2.1). Sections are the ids of the sections the
// change wrote; their current text is read in the transaction.
type Change struct {
	Kind     Kind
	Sections []string

	// An agent step: its proposed stretch, its new texts, whether it
	// brought figure links or citations, whether a new decision applies a
	// criterion, and the sections where it declared a decision (a new
	// decision with used_in to it, or a used decision).
	Stretch   string
	Texts     []string
	Figures   bool
	Citations bool
	Criteria  bool
	Decided   map[string]bool

	// Your edit: the section's text at the base version, before the save.
	BaseText string

	// Changed are the ids of the decisions, figures and sources whose
	// content this change changed (rule 4, B-45), with who changed them
	// and the turn, for the events rule 4 writes.
	Changed []string
	Actor   string
	TurnID  string
}

// Finding is one thing a rule found. Section is a label; About is what
// it is about (a label, a number in the text or an anchor); Decision is
// the label of the decision of rule 3.
type Finding struct {
	Rule int `json:"rule"`
	// Change is the label of the change rule 4 entered (chg-n).
	Change   string   `json:"change,omitempty"`
	Kind     string   `json:"kind"`
	Section  string   `json:"section,omitempty"`
	About    string   `json:"about,omitempty"`
	Decision string   `json:"decision,omitempty"`
	Reasons  []string `json:"reasons,omitempty"`
}

// RunRules runs rules 0 to 5 in that order, inside the change's
// transaction q (flujos §1): 0 a direct step that touches a figure, a
// citation, a conclusion or a criterion becomes build (only on an agent
// step); 1 figures without decision; 2 text without decision (only on
// an agent step, in a conclusion section, B-42); 3 criticality (on your
// edit, the R4 card of B-49); 4 warnings to other sections (piece 2.2);
// 5 internal codes. No rule stops the chain of steps.
func RunRules(ctx context.Context, q store.Querier, projectID string, change Change) ([]Finding, error) {
	sections := make([]store.Object, 0, len(change.Sections))
	for _, id := range change.Sections {
		sec, err := q.GetObject(ctx, projectID, id)
		if err != nil {
			return nil, err
		}
		sections = append(sections, sec)
	}
	var out []Finding

	// Rule 0: the direct route, checked after the change (fichas §2,
	// Triaje; B-43).
	if change.Kind == AgentStep && change.Stretch == StretchDirect {
		if reasons := directTouches(change, sections); len(reasons) > 0 {
			out = append(out, Finding{Rule: 0, Kind: FindingReclassified, About: StretchBuild, Reasons: reasons})
		}
	}

	// Rule 1: figures without decision.
	for _, sec := range sections {
		figures, err := graph.FiguresWithoutDecision(ctx, q, projectID, sec.ID)
		if err != nil {
			return nil, err
		}
		for _, n := range figures {
			out = append(out, Finding{Rule: 1, Kind: graph.PendingFigureWithoutDecision, Section: sec.Label, About: n})
		}
	}

	// Rule 2: text without decision, found by the app only in a
	// conclusion section; elsewhere the checker marks it (B-42). Your
	// edits do not trigger it: your critical edit opens the R4 card.
	if change.Kind == AgentStep {
		for _, sec := range sections {
			if isConclusion(sec) && !change.Decided[sec.ID] {
				out = append(out, Finding{Rule: 2, Kind: graph.PendingTextWithoutDecision, Section: sec.Label})
			}
		}
	}

	// Rule 3: criticality (T-5).
	critical, err := graph.CriticalAll(ctx, q, projectID)
	if err != nil {
		return nil, err
	}
	if change.Kind == YourEdit {
		edits, err := criticalEdits(ctx, q, projectID, change.BaseText, sections, critical)
		if err != nil {
			return nil, err
		}
		out = append(out, edits...)
	} else {
		used, err := criticalUsed(ctx, q, projectID, sections, critical)
		if err != nil {
			return nil, err
		}
		out = append(out, used...)
	}

	// Rule 4: warnings to other sections (A-1, B-45). It writes the
	// changes in the same transaction, so it needs it.
	if len(change.Changed) > 0 {
		tx, ok := q.(*store.Tx)
		if !ok {
			return nil, errors.New("checks: rule 4 needs the change's transaction")
		}
		exclude := map[string]bool{}
		for _, id := range change.Sections {
			exclude[id] = true
		}
		warned, err := Warn(ctx, tx, change.Actor, change.TurnID, change.Changed, exclude)
		if err != nil {
			return nil, err
		}
		out = append(out, warned...)
	}

	// Rule 5: internal codes, from the current text. In an agent step a
	// new-n in the text is already rejected by rule 4 of the gate
	// (specification §3); a label of the project is not, and this finds
	// it.
	for _, sec := range sections {
		codes, err := graph.InternalCodes(ctx, q, projectID, sec.ID)
		if err != nil {
			return nil, err
		}
		for _, c := range codes {
			out = append(out, Finding{Rule: 5, Kind: graph.PendingInternalCodes, Section: sec.Label, About: c})
		}
	}
	return out, nil
}

// directTouches is rule 0's test after the change.
// [EL DISEÑO NO LO DICE]: «toca una cifra» is read as a number in the
// step's new text or a figure link; «toca un criterio» as a new decision
// that applies a criterion, since the agent cannot write a criterion.
func directTouches(change Change, sections []store.Object) []string {
	var reasons []string
	figure := change.Figures
	for _, t := range change.Texts {
		if len(graph.Numbers(t)) > 0 {
			figure = true
		}
	}
	if figure {
		reasons = append(reasons, TouchesFigure)
	}
	if change.Citations {
		reasons = append(reasons, TouchesCitation)
	}
	for _, sec := range sections {
		if isConclusion(sec) {
			reasons = append(reasons, TouchesConclusion)
			break
		}
	}
	if change.Criteria {
		reasons = append(reasons, TouchesCriterion)
	}
	return reasons
}

// isConclusion: role conclusion, proposed or confirmed (K-2, R-8);
// recommendation counts the same.
func isConclusion(sec store.Object) bool {
	var b struct {
		Role struct {
			Value string `json:"value"`
		} `json:"role"`
	}
	_ = json.Unmarshal(sec.Body, &b)
	return b.Role.Value == "conclusion" || b.Role.Value == "recommendation"
}

// criticalUsed lists the critical decisions the touched sections use.
func criticalUsed(ctx context.Context, q store.Querier, projectID string, sections []store.Object,
	critical map[string][]graph.Reason) ([]Finding, error) {
	in := map[string]bool{}
	for _, sec := range sections {
		in[sec.ID] = true
	}
	decisions, err := q.ListObjects(ctx, projectID, store.ObjectDecision)
	if err != nil {
		return nil, err
	}
	var out []Finding
	for _, dec := range decisions {
		reasons := critical[dec.ID]
		if len(reasons) == 0 || dec.State == "superseded" || dec.State == "revoked" {
			continue
		}
		edges, err := q.EdgesFrom(ctx, projectID, dec.ID)
		if err != nil {
			return nil, err
		}
		for _, e := range edges {
			if e.Type == store.EdgeUsedIn && in[e.ToID] {
				sec, _ := q.GetObject(ctx, projectID, e.ToID)
				out = append(out, Finding{Rule: 3, Kind: FindingCritical, Section: sec.Label,
					Decision: dec.Label, Reasons: reasonStrings(reasons)})
			}
		}
	}
	return out, nil
}

// criticalEdits is B-49: your edit removed or changed the anchor of a
// figure of the section (it was in the base text and is not in the new
// one) whose decision, by covers, is critical, whatever that decision's
// state.
// [EL DISEÑO NO LO DICE]: «la decisión de la cita» has no edge in the
// data model (a citation only comes_from its source, glossary §3), so
// only figures open the card.
func criticalEdits(ctx context.Context, q store.Querier, projectID, baseText string, sections []store.Object,
	critical map[string][]graph.Reason) ([]Finding, error) {
	base := graph.Compare(baseText)
	figures, err := q.ListObjects(ctx, projectID, store.ObjectFigure)
	if err != nil {
		return nil, err
	}
	var out []Finding
	for _, sec := range sections {
		now := graph.Compare(sectionText(sec))
		seen := map[string]bool{}
		for _, fig := range figures {
			anchor := bodyString(fig, "anchor")
			if bodyString(fig, "section_id") != sec.ID || anchor == "" {
				continue
			}
			if len(base.Find(anchor)) == 0 || len(now.Find(anchor)) > 0 {
				continue // not in the base, or still there
			}
			edges, err := q.EdgesFrom(ctx, projectID, fig.ID)
			if err != nil {
				return nil, err
			}
			for _, e := range edges {
				if e.Type != store.EdgeCovers || len(critical[e.ToID]) == 0 || seen[e.ToID] {
					continue
				}
				dec, err := q.GetObject(ctx, projectID, e.ToID)
				if err != nil {
					return nil, err
				}
				seen[e.ToID] = true
				out = append(out, Finding{Rule: 3, Kind: FindingCriticalEdit, Section: sec.Label,
					About: anchor, Decision: dec.Label, Reasons: reasonStrings(critical[e.ToID])})
			}
		}
	}
	return out, nil
}

func reasonStrings(reasons []graph.Reason) []string {
	out := make([]string, 0, len(reasons))
	for _, r := range reasons {
		out = append(out, string(r))
	}
	return out
}

func bodyString(obj store.Object, key string) string {
	var b map[string]any
	_ = json.Unmarshal(obj.Body, &b)
	s, _ := b[key].(string)
	return s
}

func sectionText(sec store.Object) string { return bodyString(sec, "text") }
