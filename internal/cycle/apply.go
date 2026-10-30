package cycle

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"unicode"

	"github.com/LaurenteEber/tupaqnin/internal/graph"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// The principal's step, applied (backlog-r1 1.3; specification §3,
// rules 2 to 6). EndStep has already checked the fixed shape (rule 1).

type newTextItem struct {
	SectionID   string `json:"section_id"`
	BaseVersion int    `json:"base_version"`
	Anchor      string `json:"anchor"`
	Operation   string `json:"operation"`
	Text        string `json:"text"`
}

type newDecision struct {
	ID                  string   `json:"id"`
	Statement           string   `json:"statement"`
	InsteadOf           string   `json:"instead_of"`
	WouldChooseIf       string   `json:"would_choose_if"`
	Assumption          bool     `json:"assumption"`
	UsedInSections      []string `json:"used_in_sections"`
	Supersedes          *string  `json:"supersedes"`
	Applies             []string `json:"applies"`
	DependsOnAssumption *string  `json:"depends_on_assumption"`
}

type proposedRole struct {
	SectionID string `json:"section_id"`
	Role      string `json:"role"`
	Title     string `json:"title"`
}

type figureLink struct {
	FigureAnchor string `json:"figure_anchor"`
	DecisionID   string `json:"decision_id"`
}

type citationItem struct {
	ClaimAnchor string `json:"claim_anchor"`
	SourceID    string `json:"source_id"`
	Passage     string `json:"passage"`
}

type stepOutput struct {
	TriageLine            string          `json:"triage_line"`
	ClosingRisk           string          `json:"closing_risk"`
	Note                  string          `json:"note"`
	NewText               []newTextItem   `json:"new_text"`
	NewDecisions          []newDecision   `json:"new_decisions"`
	UsedDecisions         []string        `json:"used_decisions"`
	ProposedRoles         []proposedRole  `json:"proposed_roles"`
	ProposedOpenQuestions json.RawMessage `json:"proposed_open_questions"`
	FigureLinks           []figureLink    `json:"figure_links"`
	UsedSources           []string        `json:"used_sources"`
	Citations             []citationItem  `json:"citations"`
	HandledChanges        json.RawMessage `json:"handled_changes"`
	Deferred              []struct {
		What string `json:"what"`
		Why  string `json:"why"`
	} `json:"deferred"`
}

// ApplyResult is what ApplyStep reports. Applied is false when rules 2
// to 6 rejected the step: Errors then carry a message for the agent,
// with labels only, and nothing was written. Labels maps each new-n to
// the label the app gave it (T-3). Work is true when the step wrote text
// or registered decisions (B-17: the chain stops on a step without
// work).
type ApplyResult struct {
	Applied bool
	Errors  []string
	Labels  map[string]string
	Work    bool
	// Rejected are the new_text items not applied because their section
	// is in signoff or signed (B-14), each with a message for the agent;
	// the rest of the step applied.
	Rejected []string
}

// errStale is a section that changed since the version the agent read
// (B-11): the step is rejected and retried once with the current text.
type errStale struct{ label string }

func (e errStale) Error() string {
	return fmt.Sprintf("%s changed since the version you read; its current text is in the next message. Write against it", e.label)
}

// ApplyStep validates the step against rules 2 to 6 of specification
// §3 and, if it passes, applies it in one transaction: section text,
// the agent's decisions, figures, data and citations, their edges, and
// step_ended with the labels given to each new-n and what slice 1 does
// not apply (B-17). It never calls the model.
func (g *Gate) ApplyStep(ctx context.Context, projectID, turnID string, output json.RawMessage) (ApplyResult, error) {
	errs, _, err := validateOutput(output)
	if err != nil {
		return ApplyResult{}, err
	}
	if len(errs) > 0 {
		return ApplyResult{}, fmt.Errorf("cycle: ApplyStep needs an output that passed EndStep")
	}
	clean, err := cleanOutput(output)
	if err != nil {
		return ApplyResult{}, err
	}
	var out stepOutput
	if err := json.Unmarshal(clean, &out); err != nil {
		return ApplyResult{}, fmt.Errorf("cycle: parse step: %w", err)
	}

	p, err := g.loadProject(ctx, projectID)
	if err != nil {
		return ApplyResult{}, err
	}
	plan, errs := p.validate(out)
	if len(errs) > 0 {
		return ApplyResult{Errors: errs}, nil
	}
	var rejected []string
	for _, r := range plan.rejected {
		rejected = append(rejected, r.message)
	}

	beforeApplyWrite()
	var labels map[string]string
	err = g.db.Write(ctx, projectID, func(tx *store.Tx) error {
		return TrackCriticality(ctx, tx, turnID, func() error {
			var err error
			labels, err = p.apply(tx, turnID, out, plan)
			return err
		})
	})
	var stale errStale
	if errors.As(err, &stale) {
		return ApplyResult{Errors: []string{stale.Error()}}, nil
	}
	if err != nil {
		return ApplyResult{}, err
	}
	return ApplyResult{Applied: true, Labels: labels, Rejected: rejected,
		Work: len(out.NewText) > 0 || len(out.NewDecisions) > 0}, nil
}

// beforeApplyWrite runs between ApplyStep's read and its write; tests
// use it to change the project in between.
var beforeApplyWrite = func() {}

// project is what ApplyStep reads before the write: the label
// translation (only in graph, rule 7 of AGENTS) and current objects.
type project struct {
	labels   *graph.Labeler
	byID     map[string]store.Object
	edgesOut map[string][]store.Edge
	sources  []store.Object
	// inSignoff are the sections with a signoff in progress (B-14).
	inSignoff map[string]bool
}

func (g *Gate) loadProject(ctx context.Context, projectID string) (*project, error) {
	labeler, err := graph.NewLabeler(ctx, g.db, projectID)
	if err != nil {
		return nil, err
	}
	objects, err := g.db.AllObjects(ctx, projectID)
	if err != nil {
		return nil, err
	}
	inSignoff, err := graph.SectionsInSignoff(ctx, g.db, projectID)
	if err != nil {
		return nil, err
	}
	p := &project{labels: labeler, byID: make(map[string]store.Object, len(objects)),
		edgesOut: make(map[string][]store.Edge), inSignoff: inSignoff}
	for _, obj := range objects {
		p.byID[obj.ID] = obj
		if obj.Type == store.ObjectSource {
			p.sources = append(p.sources, obj)
		}
		if obj.Type == store.ObjectDecision || obj.Type == store.ObjectFigure ||
			obj.Type == store.ObjectDatum || obj.Type == store.ObjectSection {
			edges, err := g.db.EdgesFrom(ctx, projectID, obj.ID)
			if err != nil {
				return nil, err
			}
			p.edgesOut[obj.ID] = edges
		}
	}
	return p, nil
}

func (p *project) object(label string) (store.Object, bool) {
	id, err := p.labels.ToID(label)
	if err != nil {
		return store.Object{}, false
	}
	obj, ok := p.byID[id]
	return obj, ok
}

func bodyOf(obj store.Object) map[string]any {
	b := map[string]any{}
	_ = json.Unmarshal(obj.Body, &b)
	return b
}

func bodyStr(b map[string]any, key string) string {
	s, _ := b[key].(string)
	return s
}

// edit is one new_text item located in its section's base text.
type edit struct {
	index      int
	start, end int // byte range of the base text it replaces (empty for an insert)
	text       string
	anchor     string
}

// sectionPlan is the text a step writes in one section.
type sectionPlan struct {
	ref         string // label, or new-n for a section born in this output
	baseVersion int
	base        string // NFC base text
	edits       []edit
	result      string
}

type plan struct {
	sections []*sectionPlan // in the order of their first new_text item
	declared map[string]string
	newRoles map[string]proposedRole
	// rejected are the new_text items on a section in signoff or signed
	// (B-14); the figures and citations whose anchor is only in them are
	// skipped with them.
	rejected      []rejectedText
	skipFigures   map[int]bool
	skipCitations map[int]bool
}

// rejectedText is one new_text item not applied (B-14).
type rejectedText struct {
	section string
	text    string
	message string
}

// lockedMessage tells the agent why its text on a section was not
// applied (B-14): until slice 4 a step on a section in signoff or
// signed is rejected there; the rest of the step applies.
func lockedMessage(label string, signing bool) string {
	if signing {
		return fmt.Sprintf("%s is being signed by the person: your new_text for it was not applied; the rest of your step was. "+
			"Do not write in %s while it is being signed", label, label)
	}
	return fmt.Sprintf("%s is signed (or changed since it was signed): your new_text for it was not applied; the rest of your step was. "+
		"Do not write in it until the person signs it again", label)
}

// rawHTML finds the start of an HTML tag, comment or declaration (rule 3).
// [EL DISEÑO NO LO DICE]: how raw HTML is detected; goldmark (§7) is not
// a dependency yet, so the smallest check rejects any "<" followed by a
// letter, "/", "!" or "?", Markdown autolinks included.
var rawHTML = regexp.MustCompile(`<[A-Za-z/!?]`)

// validate checks rules 2 to 6 and returns the text plan. Errors are
// messages for the agent, one per problem, with labels only.
func (p *project) validate(out stepOutput) (*plan, []string) {
	var errs []string
	fail := func(path, format string, args ...any) {
		errs = append(errs, path+": "+fmt.Sprintf(format, args...))
	}
	pl := &plan{declared: map[string]string{}, newRoles: map[string]proposedRole{},
		skipFigures: map[int]bool{}, skipCitations: map[int]bool{}}

	// Rule 4: each new-n is declared once (a decision or an open
	// question with its id; a new section in proposed_roles).
	declare := func(path, id, kind string) {
		if _, dup := pl.declared[id]; dup {
			fail(path, "%s is declared more than once; declare each new-n once", id)
			return
		}
		pl.declared[id] = kind
	}
	for i, d := range out.NewDecisions {
		declare(fmt.Sprintf("/new_decisions/%d/id", i), d.ID, "decision")
	}
	var openQuestions []struct {
		ID        string   `json:"id"`
		DependsOn []string `json:"depends_on"`
	}
	_ = json.Unmarshal(out.ProposedOpenQuestions, &openQuestions)
	for i, q := range openQuestions {
		declare(fmt.Sprintf("/proposed_open_questions/%d/id", i), q.ID, "open_question")
	}
	for i, r := range out.ProposedRoles {
		if !isNew(r.SectionID) {
			continue
		}
		declare(fmt.Sprintf("/proposed_roles/%d/section_id", i), r.SectionID, "section")
		if strings.TrimSpace(r.Title) == "" {
			fail(fmt.Sprintf("/proposed_roles/%d/title", i), "a new section needs its title")
		}
		pl.newRoles[r.SectionID] = r
	}

	// ref checks a field that takes a label of the given types or a
	// new-n declared as one of kinds; label checks one that takes only
	// what already exists.
	ref := func(path, value string, types []store.ObjectType, kinds ...string) (store.Object, bool) {
		if isNew(value) {
			kind, ok := pl.declared[value]
			if !ok {
				fail(path, "%s is not declared in this output", value)
				return store.Object{}, false
			}
			for _, k := range kinds {
				if k == kind {
					return store.Object{}, true
				}
			}
			fail(path, "%s is declared as a %s, not valid here", value, strings.ReplaceAll(kind, "_", " "))
			return store.Object{}, false
		}
		obj, ok := p.object(value)
		if !ok {
			fail(path, "%s does not exist in this project", value)
			return store.Object{}, false
		}
		for _, t := range types {
			if obj.Type == t {
				return obj, true
			}
		}
		fail(path, "%s is not a %s", value, typeNames(types))
		return store.Object{}, false
	}
	label := func(path, value string, types ...store.ObjectType) (store.Object, bool) {
		if isNew(value) {
			fail(path, "%s is not valid here: this field names what already exists, by its label", value)
			return store.Object{}, false
		}
		return ref(path, value, types)
	}
	section := []store.ObjectType{store.ObjectSection}
	decision := []store.ObjectType{store.ObjectDecision}

	for i, d := range out.NewDecisions {
		path := fmt.Sprintf("/new_decisions/%d", i)
		// Rule 5: complete contrast.
		if strings.TrimSpace(d.Statement) == "" || strings.TrimSpace(d.InsteadOf) == "" ||
			strings.TrimSpace(d.WouldChooseIf) == "" {
			fail(path, "%s needs its complete contrast: statement, instead_of and would_choose_if", d.ID)
		}
		for j, s := range d.UsedInSections {
			ref(fmt.Sprintf("%s/used_in_sections/%d", path, j), s, section, "section")
		}
		for j, c := range d.Applies {
			if obj, ok := ref(fmt.Sprintf("%s/applies/%d", path, j), c,
				[]store.ObjectType{store.ObjectCriterion}); ok && obj.State != "current" {
				fail(fmt.Sprintf("%s/applies/%d", path, j), "%s is not current", c)
			}
		}
		if d.DependsOnAssumption != nil {
			p.checkAssumption(fmt.Sprintf("%s/depends_on_assumption", path), *d.DependsOnAssumption, out, ref, fail)
		}
		if d.Supersedes != nil {
			if old, ok := label(path+"/supersedes", *d.Supersedes, store.ObjectDecision); ok {
				p.checkSupersedes(path+"/supersedes", old, *d.Supersedes, fail) // rule 6
			}
		}
	}
	for i, d := range out.UsedDecisions {
		label(fmt.Sprintf("/used_decisions/%d", i), d, store.ObjectDecision)
	}
	for i, r := range out.ProposedRoles {
		if !isNew(r.SectionID) {
			ref(fmt.Sprintf("/proposed_roles/%d/section_id", i), r.SectionID, section)
		}
	}
	for i, f := range out.FigureLinks {
		ref(fmt.Sprintf("/figure_links/%d/decision_id", i), f.DecisionID, decision, "decision")
	}
	for i, s := range out.UsedSources {
		label(fmt.Sprintf("/used_sources/%d", i), s, store.ObjectSource)
	}
	for i, c := range out.Citations {
		label(fmt.Sprintf("/citations/%d/source_id", i), c.SourceID, store.ObjectSource)
	}
	var handled []struct {
		ChangeID string `json:"change_id"`
	}
	_ = json.Unmarshal(out.HandledChanges, &handled)
	for i, h := range handled {
		if isNew(h.ChangeID) {
			fail(fmt.Sprintf("/handled_changes/%d/change_id", i), "%s is not valid here: name the change by its label", h.ChangeID)
		}
	}
	for i, q := range openQuestions {
		for j, dep := range q.DependsOn {
			ref(fmt.Sprintf("/proposed_open_questions/%d/depends_on/%d", i, j), dep,
				[]store.ObjectType{store.ObjectSection, store.ObjectDecision}, "section", "decision")
		}
	}

	// Rules 2 and 3 and B-11, per section.
	bySection := map[string]*sectionPlan{}
	for i, item := range out.NewText {
		path := fmt.Sprintf("/new_text/%d", i)
		for _, id := range graph.ProvisionalIDsIn(item.Text) {
			fail(path+"/text", "%s is not valid inside the text; refer to it in its field, never in the text", id)
		}
		if hasInvisible(item.Text) {
			fail(path+"/text", "the text carries invisible characters; remove them")
		}
		if rawHTML.MatchString(item.Text) {
			fail(path+"/text", "the text carries raw HTML; write Markdown only")
		}
		sp, ok := bySection[item.SectionID]
		if !ok {
			sp = &sectionPlan{ref: item.SectionID, baseVersion: item.BaseVersion}
			if isNew(item.SectionID) {
				if pl.declared[item.SectionID] != "section" {
					fail(path+"/section_id", "%s is not declared as a new section in proposed_roles", item.SectionID)
					continue
				}
			} else {
				obj, ok := ref(path+"/section_id", item.SectionID, section)
				if !ok {
					continue
				}
				// B-14: a section in signoff or signed rejects only this
				// text, before B-11 looks at its version. P-40: a section
				// in changed_since_signoff too (estados §1: steps arrive as
				// proposals until the new signature; proposals are slice 4).
				if signing := p.inSignoff[obj.ID]; signing || obj.State == "signed" || obj.State == "changed_since_signoff" {
					pl.rejected = append(pl.rejected, rejectedText{section: item.SectionID, text: item.Text,
						message: lockedMessage(item.SectionID, signing)})
					continue
				}
				if obj.CurrentVersion != item.BaseVersion { // B-11
					fail(path+"/base_version", "%s", errStale{label: item.SectionID}.Error())
					continue
				}
				sp.base = graph.Compare(bodyStr(bodyOf(obj), "text")).Orig
			}
			bySection[item.SectionID] = sp
			pl.sections = append(pl.sections, sp)
		} else if sp.baseVersion != item.BaseVersion {
			// [EL DISEÑO NO LO DICE]: texts of one section against two
			// base versions; §3 does not cover it, so it is rejected.
			fail(path+"/base_version", "every text for %s must be written against the same base_version", item.SectionID)
			continue
		}
		if isNew(item.SectionID) && (item.BaseVersion != 0 || item.Anchor != "" || item.Operation != "insert_after") {
			fail(path, "a new section is written with base_version 0, an empty anchor and insert_after")
			continue
		}
		e := edit{index: i, text: item.Text, anchor: item.Anchor}
		if item.Anchor == "" {
			if item.Operation != "insert_after" {
				fail(path+"/anchor", "an empty anchor is only valid with insert_after")
				continue
			}
		} else {
			found := graph.Compare(sp.base).Find(item.Anchor)
			if len(found) != 1 {
				fail(path+"/anchor", "the anchor appears %d times in %s version %d; give a longer anchor that appears once",
					len(found), item.SectionID, item.BaseVersion)
				continue
			}
			e.start, e.end = found[0][0], found[0][1]
			if item.Operation == "insert_after" {
				e.start = e.end
			}
		}
		sp.edits = append(sp.edits, e)
	}
	for _, sp := range pl.sections {
		if msg := overlap(sp); msg != "" {
			fail("/new_text", "%s", msg)
		}
		sp.result = applyEdits(sp.base, sp.edits)
	}

	// Figure and citation anchors are found in the text the step
	// writes (T-4 (3)).
	// [EL DISEÑO NO LO DICE]: they name no section; each must appear
	// once across all the sections the step writes.
	// An anchor found only in a text rejected by B-14 is skipped with it.
	written := func(path, anchor string) (skip bool) {
		n := 0
		for _, sp := range pl.sections {
			n += len(graph.Compare(sp.result).Find(anchor))
		}
		if n == 0 {
			for _, r := range pl.rejected {
				if len(graph.Compare(r.text).Find(anchor)) > 0 {
					return true
				}
			}
		}
		if n != 1 {
			fail(path, "the anchor appears %d times in the text this step writes; give a longer anchor that appears once", n)
		}
		return false
	}
	for i, f := range out.FigureLinks {
		pl.skipFigures[i] = written(fmt.Sprintf("/figure_links/%d/figure_anchor", i), f.FigureAnchor)
		if len(graph.Numbers(f.FigureAnchor)) == 0 {
			fail(fmt.Sprintf("/figure_links/%d/figure_anchor", i), "the anchor of a figure must contain its number")
		}
	}
	for i, c := range out.Citations {
		pl.skipCitations[i] = written(fmt.Sprintf("/citations/%d/claim_anchor", i), c.ClaimAnchor)
	}
	return pl, errs
}

func (p *project) checkAssumption(path, value string, out stepOutput,
	ref func(string, string, []store.ObjectType, ...string) (store.Object, bool), fail func(string, string, ...any)) {
	if isNew(value) {
		if _, ok := ref(path, value, nil, "decision"); !ok {
			return
		}
		for _, d := range out.NewDecisions {
			if d.ID == value && !d.Assumption {
				fail(path, "%s is not marked assumption: true", value)
			}
		}
		return
	}
	obj, ok := ref(path, value, []store.ObjectType{store.ObjectDecision, store.ObjectPremise})
	if !ok || obj.Type == store.ObjectPremise {
		return
	}
	if a, _ := bodyOf(obj)["assumption"].(bool); !a {
		fail(path, "%s is not a decision marked assumption: true", value)
	}
}

// checkSupersedes is rule 6: the agent only supersedes its own current
// decision, outside what is signed (estados §2, fichas §3).
func (p *project) checkSupersedes(path string, old store.Object, label string, fail func(string, string, ...any)) {
	switch {
	case bodyStr(bodyOf(old), "origin") != "agent" || old.State == "human":
		fail(path, "%s is a decision of the human; the agent only supersedes its own decisions", label)
		return
	case old.State == "confirmed":
		fail(path, "%s is confirmed by the human; the agent can not supersede it", label)
		return
	case old.State != "agent":
		fail(path, "%s is not current", label)
		return
	}
	for _, e := range p.edgesOut[old.ID] {
		if e.Type != store.EdgeUsedIn {
			continue
		}
		if sec := p.byID[e.ToID]; sec.State == "signed" || sec.State == "changed_since_signoff" {
			fail(path, "%s is in a signed section; the agent can not supersede it", label)
			return
		}
	}
}

func isNew(s string) bool { return strings.HasPrefix(s, "new-") }

func typeNames(types []store.ObjectType) string {
	names := make([]string, len(types))
	for i, t := range types {
		names[i] = string(t)
	}
	return strings.Join(names, " or ")
}

// hasInvisible reports format characters (zero-width, bidi controls,
// soft hyphen), the invisible content of rule 3.
func hasInvisible(s string) bool {
	for _, r := range s {
		if unicode.Is(unicode.Cf, r) {
			return true
		}
	}
	return false
}

// overlap reports texts of one section that touch the same passage.
// Inserts at the same point do not overlap: they go in output order.
// [EL DISEÑO NO LO DICE]: T-4 forbids overlaps but does not say whether
// two inserts at one point overlap; keeping both in order is smallest.
func overlap(sp *sectionPlan) string {
	for i, a := range sp.edits {
		for _, b := range sp.edits[i+1:] {
			if a.start < b.end && b.start < a.end ||
				a.start == a.end && b.start < a.start && a.start < b.end ||
				b.start == b.end && a.start < b.start && b.start < a.end {
				return fmt.Sprintf("new_text %d and %d overlap in %s; texts of one section may not touch the same passage",
					a.index, b.index, sp.ref)
			}
		}
	}
	return ""
}

// applyEdits applies the edits back to front (T-4 (2)).
func applyEdits(base string, edits []edit) string {
	sorted := append([]edit(nil), edits...)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].start != sorted[j].start {
			return sorted[i].start > sorted[j].start
		}
		return sorted[i].index > sorted[j].index
	})
	out := base
	for _, e := range sorted {
		out = out[:e.start] + e.text + out[e.end:]
	}
	return out
}

// apply writes the validated step. Section text first, then the new
// objects, so each new-n has its label before any edge is created
// (specification §3, rule 4), then the edges and step_ended.
func (p *project) apply(tx *store.Tx, turnID string, out stepOutput, pl *plan) (map[string]string, error) {
	agent := func(t store.EventType) store.EventInput {
		return store.EventInput{Type: t, Actor: "agent", AgentRole: "principal", TurnID: turnID}
	}
	labels := map[string]string{}
	ids := map[string]string{} // new-n → internal id
	resolve := func(value string) string {
		if isNew(value) {
			return ids[value]
		}
		obj, _ := p.object(value)
		return obj.ID
	}
	written := map[string]bool{} // section ids the step wrote
	resultText := map[string]string{}

	for _, sp := range pl.sections {
		// The step's texts and anchors go in its event, so the history by
		// block finds the step (B-10).
		// [EL DISEÑO NO LO DICE]: the edit event records base and new
		// version only; texts and anchors are added to its data.
		texts, anchors := []string{}, []string{}
		for _, e := range sp.edits {
			texts = append(texts, e.text)
			if e.anchor != "" {
				anchors = append(anchors, e.anchor)
			}
		}
		stepEvent := func(t store.EventType) store.EventInput {
			ev := agent(t)
			ev.Data = map[string]any{"texts": texts, "anchors": anchors}
			return ev
		}
		if isNew(sp.ref) {
			role := pl.newRoles[sp.ref]
			obj, err := tx.CreateObject(store.ObjectSection, "draft", map[string]any{
				"title": role.Title, "text": sp.result,
				"role":         map[string]any{"value": role.Role, "state": "proposed"},
				"open_changes": []any{},
			}, stepEvent(store.EventStateChanged))
			if err != nil {
				return nil, err
			}
			ids[sp.ref], labels[sp.ref] = obj.ID, obj.Label
			written[obj.ID], resultText[obj.ID] = true, sp.result
			continue
		}
		obj, _ := p.object(sp.ref)
		b := bodyOf(obj)
		b["text"] = sp.result
		// [EL DISEÑO NO LO DICE]: the event of the agent's text change is
		// not named; edit with actor agent, like the human's (estados §8).
		_, err := tx.UpdateObject(obj.ID, sp.baseVersion, obj.State, b, stepEvent(store.EventEdit))
		if errors.Is(err, store.ErrVersionConflict) {
			return nil, errStale{label: sp.ref} // B-11: changed after the step was read
		}
		if err != nil && !errors.Is(err, store.ErrNoChange) {
			return nil, err
		}
		written[obj.ID], resultText[obj.ID] = true, sp.result
	}
	// A new section declared in proposed_roles without text is still
	// born, empty (K-2).
	// [EL DISEÑO NO LO DICE]: whether it is born without text; it is,
	// like the sections of the understanding.
	for id, role := range pl.newRoles {
		if _, done := ids[id]; done {
			continue
		}
		obj, err := tx.CreateObject(store.ObjectSection, "draft", map[string]any{
			"title": role.Title, "text": "",
			"role":         map[string]any{"value": role.Role, "state": "proposed"},
			"open_changes": []any{},
		}, agent(store.EventStateChanged))
		if err != nil {
			return nil, err
		}
		ids[id], labels[id] = obj.ID, obj.Label
	}
	var rolesNotApplied []any
	for _, r := range out.ProposedRoles {
		if isNew(r.SectionID) {
			continue
		}
		applied, err := proposeRole(tx, r.SectionID, resolve(r.SectionID), r.Role, turnID)
		if err != nil {
			return nil, err
		}
		if !applied {
			rolesNotApplied = append(rolesNotApplied, map[string]any{"section_id": r.SectionID, "role": r.Role})
		}
	}

	for _, d := range out.NewDecisions {
		obj, err := tx.CreateObject(store.ObjectDecision, "agent", map[string]any{
			"statement": d.Statement, "instead_of": d.InsteadOf, "would_choose_if": d.WouldChooseIf,
			"assumption": d.Assumption, "origin": "agent", "human_marked_critical": false,
		}, agent(store.EventStateChanged))
		if err != nil {
			return nil, err
		}
		ids[d.ID], labels[d.ID] = obj.ID, obj.Label
	}

	edges := newEdgeSet(p, tx, agent(store.EventEdge))
	for _, d := range out.NewDecisions {
		decID := ids[d.ID]
		for _, s := range d.UsedInSections {
			if err := edges.add(store.EdgeUsedIn, decID, resolve(s), ""); err != nil {
				return nil, err
			}
		}
		for _, c := range d.Applies {
			if err := edges.add(store.EdgeApplies, decID, resolve(c), ""); err != nil {
				return nil, err
			}
		}
		if d.DependsOnAssumption != nil {
			if err := edges.add(store.EdgeDependsOnAssumption, decID, resolve(*d.DependsOnAssumption), ""); err != nil {
				return nil, err
			}
		}
		if d.Supersedes != nil {
			old, _ := p.object(*d.Supersedes)
			if err := edges.add(store.EdgeSupersedes, decID, old.ID, ""); err != nil {
				return nil, err
			}
			ev := agent(store.EventStateChanged)
			ev.Data = map[string]any{"label": *d.Supersedes, "from": old.State, "to": "superseded", "by": labels[d.ID]}
			if _, err := tx.UpdateObject(old.ID, old.CurrentVersion, "superseded", bodyOf(old), ev); err != nil {
				return nil, err
			}
		}
	}
	sectionIDs := sortedKeys(written)
	for _, d := range out.UsedDecisions {
		for _, sec := range sectionIDs {
			if err := edges.add(store.EdgeUsedIn, resolve(d), sec, ""); err != nil {
				return nil, err
			}
		}
	}
	// [EL DISEÑO NO LO DICE]: used_sources names no section; the uses
	// edge goes from each section the step wrote.
	for _, s := range out.UsedSources {
		for _, sec := range sectionIDs {
			if err := edges.add(store.EdgeUses, sec, resolve(s), ""); err != nil {
				return nil, err
			}
		}
	}

	// Figures (covers, and used_in to the section where the figure is),
	// citations (comes_from when the passage is literal in its source).
	inSection := func(anchor string) string {
		for _, sec := range sectionIDs {
			if len(graph.Compare(resultText[sec]).Find(anchor)) > 0 {
				return sec
			}
		}
		return ""
	}
	linkedValues := map[string]bool{}
	for i, f := range out.FigureLinks {
		if pl.skipFigures[i] {
			continue
		}
		sec := inSection(f.FigureAnchor)
		value := graph.Numbers(f.FigureAnchor)[0].Raw
		// [EL DISEÑO NO LO DICE]: a figure's value is the first number of
		// its anchor.
		fig, err := tx.CreateObject(store.ObjectFigure, "",
			map[string]any{"section_id": sec, "anchor": f.FigureAnchor, "value": value},
			agent(store.EventStateChanged))
		if err != nil {
			return nil, err
		}
		decID := resolve(f.DecisionID)
		if err := edges.add(store.EdgeCovers, fig.ID, decID, f.FigureAnchor); err != nil {
			return nil, err
		}
		if err := edges.add(store.EdgeUsedIn, decID, sec, ""); err != nil {
			return nil, err
		}
		for v := range graph.NumberValues(f.FigureAnchor) {
			linkedValues[v] = true
		}
	}
	for i, c := range out.Citations {
		if pl.skipCitations[i] {
			continue
		}
		cit, err := tx.CreateObject(store.ObjectCitation, "",
			map[string]any{"section_id": inSection(c.ClaimAnchor), "anchor": c.ClaimAnchor, "passage": c.Passage},
			agent(store.EventStateChanged))
		if err != nil {
			return nil, err
		}
		src, _ := p.object(c.SourceID)
		if len(graph.Compare(bodyStr(bodyOf(src), "text")).Find(c.Passage)) > 0 {
			if err := edges.add(store.EdgeComesFrom, cit.ID, src.ID, ""); err != nil {
				return nil, err
			}
		}
		// Otherwise no comes_from: the pending «fuente que no sostiene»
		// (B-15), computed by graph.SourcesNotSupporting.
	}

	// Data (A-3): a number of the new text with no figure link that
	// appears literal, separators normalized, in a source used in the
	// step or in the human's material is linked to that source.
	// [EL DISEÑO NO LO DICE]: «tu material» is read as the project's
	// sources, all uploaded by the human in slice 1.
	if err := p.linkData(tx, out, pl, ids, linkedValues, edges, agent(store.EventStateChanged)); err != nil {
		return nil, err
	}

	notApplied := map[string]any{}
	if len(out.ProposedOpenQuestions) > 0 && string(out.ProposedOpenQuestions) != "null" {
		notApplied["proposed_open_questions"] = out.ProposedOpenQuestions
	}
	if len(out.HandledChanges) > 0 && string(out.HandledChanges) != "null" {
		notApplied["handled_changes"] = out.HandledChanges
	}
	if rolesNotApplied != nil {
		notApplied["proposed_roles"] = rolesNotApplied // P-32
	}
	if len(pl.rejected) > 0 { // B-14
		var texts []any
		for _, r := range pl.rejected {
			texts = append(texts, map[string]any{"section_id": r.section, "text": r.text, "message": r.message})
		}
		notApplied["new_text"] = texts
		var figures, citations []any
		for i, f := range out.FigureLinks {
			if pl.skipFigures[i] {
				figures = append(figures, map[string]any{"figure_anchor": f.FigureAnchor, "decision_id": f.DecisionID})
			}
		}
		for i, c := range out.Citations {
			if pl.skipCitations[i] {
				citations = append(citations, map[string]any{"claim_anchor": c.ClaimAnchor, "source_id": c.SourceID})
			}
		}
		if figures != nil {
			notApplied["figure_links"] = figures
		}
		if citations != nil {
			notApplied["citations"] = citations
		}
	}
	deferred := []any{}
	var longDeferred []int
	for i, d := range out.Deferred {
		deferred = append(deferred, map[string]any{"what": d.What, "why": d.Why})
		if len(strings.Fields(d.What+" "+d.Why)) > 30 { // T-6: logged, never cut
			longDeferred = append(longDeferred, i)
		}
	}
	// The sections the step wrote and the decisions it declared, so the
	// pending «texto con una decisión reemplazada» can close (B-15).
	// [EL DISEÑO NO LO DICE]: step_ended does not list them; they are
	// added to its data.
	writtenLabels := []string{}
	for _, sp := range pl.sections {
		if l, ok := labels[sp.ref]; ok {
			writtenLabels = append(writtenLabels, l)
		} else {
			writtenLabels = append(writtenLabels, sp.ref)
		}
	}
	usedDecisions := append([]string{}, out.UsedDecisions...)
	data := map[string]any{
		"turn_kind": "step", "triage_line": out.TriageLine, "closing_risk": out.ClosingRisk,
		"note": out.Note, "deferred": deferred, "labels": labels, "not_applied": notApplied,
		"written_sections": writtenLabels, "used_decisions": usedDecisions,
	}
	if longDeferred != nil {
		data["long_deferred"] = longDeferred
	}
	ev := agent(store.EventStepEnded)
	ev.Data = data
	if _, err := tx.AppendEvent(ev); err != nil {
		return nil, err
	}
	return labels, nil
}

func (p *project) linkData(tx *store.Tx, out stepOutput, pl *plan, ids map[string]string,
	linked map[string]bool, edges *edgeSet, ev store.EventInput) error {
	used := map[string]bool{}
	var order []store.Object
	for _, s := range out.UsedSources {
		if src, ok := p.object(s); ok && !used[src.ID] {
			used[src.ID] = true
			order = append(order, src)
		}
	}
	for _, src := range p.sources {
		if !used[src.ID] {
			order = append(order, src)
		}
	}
	sourceValues := make([]map[string]bool, len(order))
	for i, src := range order {
		sourceValues[i] = graph.NumberValues(bodyStr(bodyOf(src), "text"))
	}
	for _, item := range out.NewText {
		secID := ids[item.SectionID]
		if !isNew(item.SectionID) {
			obj, _ := p.object(item.SectionID)
			secID = obj.ID
		}
		for _, n := range graph.Numbers(item.Text) {
			if linked[n.Value] {
				continue
			}
			for i, src := range order {
				if !sourceValues[i][n.Value] {
					continue
				}
				dat, err := tx.CreateObject(store.ObjectDatum, "",
					map[string]any{"section_id": secID, "anchor": n.Raw, "value": n.Raw}, ev)
				if err != nil {
					return err
				}
				if err := edges.add(store.EdgeComesFrom, dat.ID, src.ID, ""); err != nil {
					return err
				}
				linked[n.Value] = true
				break
			}
			// Otherwise the pending «cifra sin decisión» (B-15), computed
			// by graph.FiguresWithoutDecision.
		}
	}
	return nil
}

// proposeRole proposes a role for an existing section, in a step or in
// the understanding. A role the person confirmed is never changed back
// to proposed (P-32): the proposal is not applied, and the caller keeps
// it in the step end as not applied, like B-17. It reports whether the
// proposal was applied.
func proposeRole(tx *store.Tx, label, id, role, turnID string) (bool, error) {
	obj, err := tx.Object(id)
	if err != nil {
		return false, err
	}
	b := bodyOf(obj)
	if current, _ := b["role"].(map[string]any); bodyStr(current, "state") == "confirmed" {
		return false, nil
	}
	b["role"] = map[string]any{"value": role, "state": "proposed"}
	_, err = tx.UpdateObject(obj.ID, obj.CurrentVersion, obj.State, b,
		store.EventInput{Type: store.EventRoleChanged, Actor: "agent", AgentRole: "principal", TurnID: turnID,
			Data: map[string]any{"label": label, "role": role, "state": "proposed"}})
	if errors.Is(err, store.ErrNoChange) {
		return true, nil
	}
	return err == nil, err
}

// edgeSet creates edges once: an edge of the same type and ends that is
// already current is not created again.
type edgeSet struct {
	p    *project
	tx   *store.Tx
	ev   store.EventInput
	seen map[string]bool
}

func newEdgeSet(p *project, tx *store.Tx, ev store.EventInput) *edgeSet {
	return &edgeSet{p: p, tx: tx, ev: ev, seen: map[string]bool{}}
}

func (s *edgeSet) add(t store.EdgeType, from, to, anchor string) error {
	if from == "" || to == "" {
		return nil
	}
	key := string(t) + "|" + from + "|" + to
	if s.seen[key] {
		return nil
	}
	s.seen[key] = true
	for _, e := range s.p.edgesOut[from] {
		if e.Type == t && e.ToID == to {
			return nil
		}
	}
	_, err := s.tx.AddEdge(t, from, to, anchor, s.ev)
	return err
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
