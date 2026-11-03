package checks

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/LaurenteEber/tupaqnin/internal/graph"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// OpenQuestion is what opens one «por aclarar» (backlog-r2 2.4): from
// the principal's proposed_open_questions or from the checker's
// candidate (B-44).
type OpenQuestion struct {
	Question string
	// Options are the proposer's, each with option, contrast,
	// consequence and risk.
	Options []map[string]any
	// DependsOn are the ids of the sections and decisions it blocks.
	DependsOn []string
	Variants  []string
	// Premise is the id of the premise it contradicts, or "": the premise
	// goes contradicted while it is open (estados §6).
	Premise string
	// Extra fields of its body (origin, the checker's passages,
	// undeclared_by_agent…).
	Extra map[string]any
	// Event is the creation event; the app's edges and moves use its
	// actor and turn.
	Event store.EventInput
}

// The scope «por aclarar» (estados §4, B-68): a kind of its own and the
// app's two options, codes the page translates: complete the review with
// another round, with its cost, or declare the limit in the text.
const (
	OpenQuestionScope         = "scope"
	ScopeOptionCompleteReview = "complete_review"
	ScopeOptionDeclareLimit   = "declare_limit"
)

// ScopeOptions are a scope «por aclarar»'s options, wherever it opens from.
func ScopeOptions() []map[string]any {
	return []map[string]any{
		{"option": "Complete the review with another round, with its cost", "code": ScopeOptionCompleteReview,
			"contrast": "", "consequence": "", "risk": ""},
		{"option": "Declare the limit in the text", "code": ScopeOptionDeclareLimit,
			"contrast": "", "consequence": "", "risk": ""}}
}

// Kinds of what a «por aclarar» keeps with its first option (body
// keeps.kind).
const (
	KeepDecision = "decision"
	KeepPremise  = "premise"
)

// Open creates the open_question open and its blocks edges to what it
// depends on (specification §1.2: the app creates them). When it blocks
// a decision of yours or a confirmed one, or contradicts the premise,
// its first option is the app's «mantener» (estados §2 and §6), marked
// keep: true, with the decision's statement or the premise's text.
// Opening it moves to changed_since_signoff every signed section it
// blocks, directly or through a blocked decision used in it (estados
// §1), and the contradicted premise to contradicted. The blocked
// decision does not change state; objecting does (OpenObjection).
// [EL DISEÑO NO LO DICE]: the keep option's contrast, consequence and
// risk; they are empty. With two decisions of yours blocked, the first
// in depends_on is the one kept. A signed section that uses a blocked
// decision counts as blocked, like an open card's.
func Open(ctx context.Context, tx *store.Tx, oq OpenQuestion) (store.Object, error) {
	pid := tx.ProjectID()
	app := store.EventInput{Type: store.EventStateChanged, Actor: "app", TurnID: oq.Event.TurnID}

	var keeps map[string]any
	options := []map[string]any{}
	var blockedDecisions, blockedSections []store.Object
	for _, id := range oq.DependsOn {
		obj, err := tx.GetObject(ctx, pid, id)
		if err != nil {
			return store.Object{}, err
		}
		switch obj.Type {
		case store.ObjectDecision:
			blockedDecisions = append(blockedDecisions, obj)
			if keeps == nil && (obj.State == "human" || obj.State == "confirmed") {
				keeps = map[string]any{"kind": KeepDecision, "id": obj.ID}
				options = append(options, keepOption(bodyText(obj, "statement")))
			}
		case store.ObjectSection:
			blockedSections = append(blockedSections, obj)
		}
	}
	var premise store.Object
	if oq.Premise != "" {
		p, err := tx.GetObject(ctx, pid, oq.Premise)
		if err != nil {
			return store.Object{}, err
		}
		premise = p
		if keeps == nil {
			keeps = map[string]any{"kind": KeepPremise, "id": p.ID}
			options = append(options, keepOption(bodyText(p, "text")))
		}
	}
	options = append(options, oq.Options...)
	variants := oq.Variants
	if variants == nil {
		variants = []string{}
	}
	body := map[string]any{}
	for k, v := range oq.Extra {
		body[k] = v
	}
	body["question"], body["options"], body["depends_on"], body["variants"] = oq.Question, options, oq.DependsOn, variants
	if keeps != nil {
		body["keeps"] = keeps
	}
	obj, err := tx.CreateObject(store.ObjectOpenQuestion, "open", body, oq.Event)
	if err != nil {
		return store.Object{}, err
	}

	edge := store.EventInput{Type: store.EventEdge, Actor: "app", TurnID: oq.Event.TurnID}
	for _, id := range oq.DependsOn {
		if _, err := tx.AddEdge(store.EdgeBlocks, obj.ID, id, "", nil, edge); err != nil {
			return store.Object{}, err
		}
	}

	moved := map[string]bool{}
	move := func(sec store.Object) error {
		if sec.State != "signed" || moved[sec.ID] {
			return nil
		}
		moved[sec.ID] = true
		var b map[string]any
		_ = json.Unmarshal(sec.Body, &b)
		ev := app
		ev.Data = map[string]any{"label": sec.Label, "from": "signed", "to": "changed_since_signoff",
			"why": "open_question", "open_question": obj.Label}
		_, err := tx.UpdateObject(sec.ID, sec.CurrentVersion, "changed_since_signoff", b, ev)
		if errors.Is(err, store.ErrNoChange) {
			return nil
		}
		return err
	}
	for _, sec := range blockedSections {
		if err := move(sec); err != nil {
			return store.Object{}, err
		}
	}
	for _, dec := range blockedDecisions {
		edges, err := tx.EdgesFrom(ctx, pid, dec.ID)
		if err != nil {
			return store.Object{}, err
		}
		for _, e := range edges {
			if e.Type != store.EdgeUsedIn {
				continue
			}
			sec, err := tx.GetObject(ctx, pid, e.ToID)
			if err != nil {
				return store.Object{}, err
			}
			if err := move(sec); err != nil {
				return store.Object{}, err
			}
		}
	}

	if premise.ID != "" && premise.State == "current" {
		var b map[string]any
		_ = json.Unmarshal(premise.Body, &b)
		ev := app
		ev.Data = map[string]any{"label": premise.Label, "from": "current", "to": "contradicted", "open_question": obj.Label}
		if _, err := tx.UpdateObject(premise.ID, premise.CurrentVersion, "contradicted", b, ev); err != nil {
			return store.Object{}, err
		}
	}
	return obj, nil
}

// OriginObjection marks the «por aclarar» of an objection of yours (E-1).
const OriginObjection = "objection"

// OriginChecker is the origin of a «por aclarar» the checker opened.
const OriginChecker = "checker"

// OpenObjection moves the decision to in_open_question and opens the question of
// B-81, inside tx: the app's two options from the decision itself, «mantener» and
// «la otra», each with its contrast («elegiría la otra si»), its consequence (the
// sections that use it) and its risk (the reasons it is critical, codes the page
// translates). Its body keeps the previous state, for the history, and the
// decision's label, for the principal (rule 7: never its id in a line).
// [EL DISEÑO NO LO DICE]: the question's text; it is your motive, literal (rule
// 8): the page names the decision in what it blocks. Both options carry the same
// contrast, consequence and risk: the decision has one of each.
func OpenObjection(ctx context.Context, tx *store.Tx, dec store.Object, reason string) (store.Object, error) {
	pid := tx.ProjectID()
	dependsOn := []string{dec.ID}
	titles := []string{}
	edges, err := tx.EdgesFrom(ctx, pid, dec.ID)
	if err != nil {
		return store.Object{}, err
	}
	for _, e := range edges {
		if e.Type != store.EdgeUsedIn {
			continue
		}
		sec, err := tx.GetObject(ctx, pid, e.ToID)
		if err != nil {
			return store.Object{}, err
		}
		dependsOn = append(dependsOn, sec.ID)
		if title := bodyText(sec, "title"); title != "" {
			titles = append(titles, title)
		}
	}
	critical, err := graph.CriticalAll(ctx, tx, pid)
	if err != nil {
		return store.Object{}, err
	}
	var body map[string]any
	_ = json.Unmarshal(dec.Body, &body)
	if _, err := tx.UpdateObject(dec.ID, dec.CurrentVersion, "in_open_question", body,
		store.EventInput{Type: store.EventStateChanged, Actor: "human", Reason: reason,
			Data: map[string]any{"label": dec.Label, "from": dec.State, "to": "in_open_question", "via": OriginObjection}}); err != nil {
		return store.Object{}, err
	}
	option := func(text string, mark string) map[string]any {
		return map[string]any{"option": text, mark: true, "contrast": bodyText(dec, "would_choose_if"),
			"consequence": strings.Join(titles, ", "), "risk": "", "risk_reasons": critical[dec.ID]}
	}
	// P-115: the question is the decision, literal; your reason goes apart, and the
	// page says «¿Mantener…?» around it.
	return Open(ctx, tx, OpenQuestion{Question: bodyText(dec, "statement"),
		Options:   []map[string]any{option(bodyText(dec, "statement"), "keep"), option(bodyText(dec, "instead_of"), "other")},
		DependsOn: dependsOn,
		Extra: map[string]any{"origin": OriginObjection, "keeps": map[string]any{"kind": KeepDecision, "id": dec.ID},
			"previous_state": dec.State, "objected_decision": dec.Label, "objection_reason": reason, "undeclared_by_agent": false},
		Event: store.EventInput{Type: store.EventStateChanged, Actor: "human", Reason: reason,
			Data: map[string]any{"via": OriginObjection, "decision": dec.Label, "previous_state": dec.State}}})
}

func keepOption(text string) map[string]any {
	return map[string]any{"option": text, "keep": true, "contrast": "", "consequence": "", "risk": ""}
}

// PremiseProbe is the change that probes the current text against a
// new version of the premise (B-46 for the premise, backlog-r2 2.4):
// the sections that use a decision depending on the premise
// (depends_on_assumption), or, if none, every section with text. Each
// passage is the section's current text.
// [EL DISEÑO NO LO DICE]: B-46 is written for a criterion (decisions
// that apply it); for the premise its edge is depends_on_assumption.
func PremiseProbe(ctx context.Context, q store.Querier, projectID, premiseID string) (Change, error) {
	return probeThrough(ctx, q, projectID, premiseID, store.EdgeDependsOnAssumption)
}

// CriterionProbe is the change that probes the current text against a
// new version of a criterion (B-46, backlog-r2 2.8): the sections that
// use a decision applying the criterion (applies), or, if none, every
// section with text. Each passage is the section's current text.
func CriterionProbe(ctx context.Context, q store.Querier, projectID, criterionID string) (Change, error) {
	return probeThrough(ctx, q, projectID, criterionID, store.EdgeApplies)
}

// probeThrough is B-46's probe of a new version of protected: the
// sections where a decision linked to it by edge (decision → protected)
// is used, or, if none, every section with text, in document order.
func probeThrough(ctx context.Context, q store.Querier, projectID, protectedID string, edge store.EdgeType) (Change, error) {
	change := Change{Kind: YourEdit, Actor: "human"}
	in, err := q.EdgesTo(ctx, projectID, protectedID)
	if err != nil {
		return change, err
	}
	affected := map[string]bool{}
	for _, e := range in {
		if e.Type != edge {
			continue
		}
		out, err := q.EdgesFrom(ctx, projectID, e.FromID)
		if err != nil {
			return change, err
		}
		for _, u := range out {
			if u.Type == store.EdgeUsedIn {
				affected[u.ToID] = true
			}
		}
	}
	sections, err := graph.OrderedSections(ctx, q, projectID)
	if err != nil {
		return change, err
	}
	add := func(only bool) {
		for _, sec := range sections {
			text := bodyText(sec, "text")
			if strings.TrimSpace(text) == "" || only && !affected[sec.ID] {
				continue
			}
			change.Sections = append(change.Sections, sec.ID)
			change.Passages = append(change.Passages, Passage{Section: sec.ID, Text: text})
		}
	}
	add(true)
	if len(change.Passages) == 0 {
		add(false)
	}
	return change, nil
}
