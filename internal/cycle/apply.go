package cycle

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"sort"
	"strings"
	"unicode"

	"github.com/LaurenteEber/tupaqnin/internal/checks"
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
	// ClaimOrigin is stored from slice 2 on; its logic is piece 2.11 (R-1).
	ClaimOrigin string `json:"claim_origin"`
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
	// Stretch is the proposed stretch (B-43); rule 0 reads it.
	Stretch               string          `json:"stretch"`
	TriageLine            string          `json:"triage_line"`
	Reply                 string          `json:"reply"`
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
	// ResearchRequest asks for a round of readers (B-57); it is read
	// leniently, so a request without its shape is rejected alone.
	ResearchRequest json.RawMessage `json:"research_request"`
	// NamedReferences are what the step names without a citation
	// (B-69); a round's references point into them.
	NamedReferences json.RawMessage `json:"named_references"`
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
	// Asked: the step proposed a «por aclarar» (B-84); it stays no work.
	Asked bool
	// Rejected are what the step did not apply and the agent must hear:
	// closes rule 7 rejected, each with its message; the rest of the step
	// applied.
	Rejected []string
	// Proposals are the labels of the proposals the step left (B-73, B-77):
	// its texts that were not applied, and go to the person.
	Proposals []string
	// Stretch is the stretch the app settled after rule 0 (B-43);
	// Reclassified says why a direct step became build, nil otherwise.
	Stretch      string
	Reclassified []string
	// Change is the applied change, for the checker, which judges it
	// after the step's transaction (backlog-r2 2.3).
	Change checks.Change
	// Round is the id of the round the step's research_request created
	// (B-57), "" when none; the runner launches it (B-64).
	Round string
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
	return g.ApplyChainStep(ctx, projectID, turnID, output, false)
}

// ApplyChainStep is ApplyStep for a step of the chain (B-17): when the
// chain stops at this step, step_ended carries why (B-24): no_work for
// a step without new text or decisions (a reply alone is not work,
// R-5), step_limit when last is set and the step had work.
func (g *Gate) ApplyChainStep(ctx context.Context, projectID, turnID string, output json.RawMessage, last bool) (ApplyResult, error) {
	return g.applyChainStep(ctx, projectID, turnID, output, stepOptions{last: last})
}

func (g *Gate) applyChainStep(ctx context.Context, projectID, turnID string, output json.RawMessage, opts stepOptions) (ApplyResult, error) {
	last := opts.last
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
	if p.versionErr != nil {
		return ApplyResult{}, p.versionErr
	}
	if len(errs) > 0 {
		return ApplyResult{Errors: errs}, nil
	}
	var rejected []string

	work := len(out.NewText) > 0 || len(out.NewDecisions) > 0
	var asked []json.RawMessage
	_ = json.Unmarshal(out.ProposedOpenQuestions, &asked)
	var stop StopReason
	switch {
	case !work:
		stop = StopNoWork
	case last:
		stop = StopStepLimit
	}
	if opts.quiet {
		stop = ""
	}

	beforeApplyWrite()
	var labels map[string]string
	err = g.db.Write(ctx, projectID, func(tx *store.Tx) error {
		return TrackCriticality(ctx, tx, turnID, func() error {
			var err error
			p.lastStep = last
			if !work {
				if p.earlierRound, err = roundEarlierInTurn(ctx, tx, turnID); err != nil {
					return err
				}
			}
			labels, err = p.apply(tx, turnID, out, plan, stop)
			return err
		})
	})
	var stale errStale
	if errors.As(err, &stale) {
		return ApplyResult{Errors: []string{stale.Error()}}, nil
	}
	var locked errLocked
	if errors.As(err, &locked) {
		return ApplyResult{Errors: []string{locked.Error()}}, nil
	}
	if err != nil {
		return ApplyResult{}, err
	}
	for _, c := range p.rejectedCloses { // rule 7: only that close is rejected
		rejected = append(rejected, c["message"].(string))
	}
	return ApplyResult{Applied: true, Labels: labels, Rejected: rejected, Proposals: p.proposals, Work: work, Asked: len(asked) > 0,
		Stretch: p.stretch, Reclassified: p.reclassified, Change: p.change, Round: p.round}, nil
}

// sortedDecidedElsewhere are the sections with a decision of this step that
// the step did not write, in a stable order.
func sortedDecidedElsewhere(decided map[string]bool, written []string) []string {
	var out []string
	for id, ok := range decided {
		if ok && !slices.Contains(written, id) {
			out = append(out, id)
		}
	}
	slices.Sort(out)
	return out
}

// roundStepStop is the stop_reason of a step whose only work is its round
// (P-99, B-24): none while the chain goes on after the digest; the reason
// runChain stops for otherwise, so the step that ended the chain says why.
func (p *project) roundStepStop() StopReason {
	switch {
	case p.earlierRound:
		return StopNoWork
	case p.lastStep:
		return StopStepLimit
	}
	return ""
}

// roundEarlierInTurn says whether an earlier step of the turn asked for a
// round: the same count as runChain's, read from the history inside the
// step's transaction.
func roundEarlierInTurn(ctx context.Context, tx *store.Tx, turnID string) (bool, error) {
	events, err := tx.EventsSince(ctx, tx.ProjectID(), 0)
	if err != nil {
		return false, err
	}
	for _, ev := range events {
		if ev.Type != store.EventStepEnded || ev.TurnID != turnID {
			continue
		}
		var data struct {
			Round string `json:"round"`
		}
		if json.Unmarshal(ev.Data, &data) == nil && data.Round != "" {
			return true, nil
		}
	}
	return false, nil
}

// beforeApplyWrite runs between ApplyStep's read and its write; tests
// use it to change the project in between.
var beforeApplyWrite = func() {}

// project is what ApplyStep reads before the write: the label
// translation (only in graph, rule 7 of AGENTS) and current objects.
type project struct {
	// stretch and reclassified are what rule 0 settled when applying.
	stretch      string
	reclassified []string
	// round is the round the step's research_request created (B-57).
	round string
	// lastStep is the chain's last allowed step, and earlierRound says an
	// earlier step of this turn already asked for a round (P-99, B-24).
	lastStep, earlierRound bool
	// rejectedCloses are the handled_changes rule 7 rejected, each with
	// its message for the agent (specification §3).
	rejectedCloses []map[string]any
	labels         *graph.Labeler
	byID           map[string]store.Object
	edgesOut       map[string][]store.Edge
	sources        []store.Object
	// inSignoff are the sections with a signoff in progress (B-14).
	inSignoff map[string]bool
	// planConfirmed: you answered the plan, so a new section of the
	// agent is a proposal (B-77).
	planConfirmed bool
	// yourEdits is, by section, the last version your edits made: a step on
	// an older one is a proposal (B-73 a).
	yourEdits map[string]int
	// proposals are the labels of the proposals this step left.
	proposals []string
	// newCitations and change are what the checker judges after the step
	// (backlog-r2 2.3): the citations the step created and its change.
	newCitations []string
	change       checks.Change
	// versionText reads the text of a stored version of an object, for
	// P-84; versionErr keeps the first error it met.
	versionText func(id string, version int) (string, bool)
	versionErr  error
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
	read, err := g.db.ReadObjectIDs(ctx, projectID)
	if err != nil {
		return nil, err
	}
	// B-62: an external (web) source also counts as read when the
	// principal received it in a digest (via = digest).
	digested, err := g.db.DeliveredObjectIDs(ctx, projectID, store.ViaDigest)
	if err != nil {
		return nil, err
	}
	planConfirmed, err := PlanConfirmed(ctx, g.db, projectID)
	if err != nil {
		return nil, err
	}
	events, err := g.db.EventsSince(ctx, projectID, 0)
	if err != nil {
		return nil, err
	}
	yourEdits := map[string]int{}
	for _, ev := range events {
		if ev.Type == store.EventEdit && ev.Actor == "human" && ev.NewVersion > yourEdits[ev.ObjectID] {
			yourEdits[ev.ObjectID] = ev.NewVersion
		}
	}
	p := &project{labels: labeler, byID: make(map[string]store.Object, len(objects)), yourEdits: yourEdits,
		edgesOut: make(map[string][]store.Edge), inSignoff: inSignoff, planConfirmed: planConfirmed}
	p.versionText = func(id string, version int) (string, bool) {
		versions, err := g.db.Versions(ctx, projectID, id)
		if err != nil {
			if p.versionErr == nil {
				p.versionErr = err
			}
			return "", false
		}
		for _, v := range versions {
			if v.Version == version {
				var b map[string]any
				_ = json.Unmarshal(v.Body, &b)
				return bodyStr(b, "text"), true
			}
		}
		return "", false
	}
	for _, obj := range objects {
		p.byID[obj.ID] = obj
		// R-3: only a source the principal read can hold a figure or a
		// citation (specification §3, «Precisiones del 30-oct»).
		if obj.Type == store.ObjectSource && (read[obj.ID] || digested[obj.ID] && bodyStr(bodyOf(obj), "kind") == graph.SourceKindWeb) {
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

// wasRead reports whether the principal read the source (R-3).
func (p *project) wasRead(id string) bool {
	for _, src := range p.sources {
		if src.ID == id {
			return true
		}
	}
	return false
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
	origin     string // claim_origin (R-1)
	operation  string
}

// sectionPlan is the text a step writes in one section.
type sectionPlan struct {
	ref         string // label, or new-n for a section born in this output
	baseVersion int
	// readVersion and readText are the section's version and text when
	// the step was validated; the write is conditioned on them (P-84).
	readVersion int
	readText    string
	base        string // NFC base text
	edits       []edit
	result      string
	// why is set when the text is not applied and goes to a proposal (B-73,
	// B-77): newRole for a new section, sectionID for an existing one, and
	// the figures and citations whose anchor is only in the proposed text.
	why       string
	sectionID string
	newRole   *proposedRole
	figures   []figureLink
	citations []citationItem
}

type plan struct {
	sections []*sectionPlan // in the order of their first new_text item
	declared map[string]string
	newRoles map[string]proposedRole
	// proposals are the sections whose texts go to a proposal (B-73, B-77),
	// in the order of their first new_text item.
	proposals []*sectionPlan
	// proposedSections are the new sections with the plan confirmed (B-77).
	proposedSections map[string]proposedRole
	// skipFigures and skipCitations are the links not applied now; held are
	// those that go with a proposal, the rest into a section that takes no
	// text from the agent (P-52).
	skipFigures, heldFigures     map[int]bool
	skipCitations, heldCitations map[int]bool
}

// isProposed says whether a section (label or new-n) goes to a proposal.
func (pl *plan) isProposed(ref string) bool {
	for _, sp := range pl.proposals {
		if sp.ref == ref {
			return true
		}
	}
	return false
}

// proposedAnchor is the proposal whose result holds the anchor, or nil.
func (pl *plan) proposedAnchor(anchor string) *sectionPlan {
	for _, sp := range pl.proposals {
		if len(graph.Compare(sp.result).Find(anchor)) > 0 {
			return sp
		}
	}
	return nil
}

// errLocked is a section whose signoff started, or that was signed,
// between the step's validation and its write (P-89, B-14): the whole
// step is rejected, since its transaction rolls back, and retried once.
// [EL DISEÑO NO LO DICE]: the whole step, not only that text, is
// rejected at the write; the retry validates again and then that text is
// a proposal (B-73).
type errLocked struct {
	label   string
	signing bool
}

func (e errLocked) Error() string {
	if e.signing {
		return fmt.Sprintf("%s started being signed by the person after you read it: your step was not applied. "+
			"Send your step again", e.label)
	}
	return fmt.Sprintf("%s was signed by the person after you read it: your step was not applied. "+
		"Send your step again", e.label)
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
		proposedSections: map[string]proposedRole{}, skipFigures: map[int]bool{}, heldFigures: map[int]bool{},
		skipCitations: map[int]bool{}, heldCitations: map[int]bool{}}

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
		// B-77: with the plan confirmed, the section is a proposal. Its text
		// and the decisions used in it go with it; a depends_on naming it is
		// skipped, and the rest applies.
		// [EL DISEÑO NO LO DICE]: what happens to a depends_on naming a
		// proposed section; like B-54, it is not created.
		if p.planConfirmed {
			pl.proposedSections[r.SectionID] = r
			continue
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
		// P-83: a data marker echoed from a brief or a tool result is
		// never document text (it would also read as raw HTML).
		if graph.HasMarker(item.Text) {
			fail(path+"/text", "the text carries a data marker (<<<BEGIN or <<<END); the markers only delimit what you were given and are not part of any text")
		} else if rawHTML.MatchString(item.Text) {
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
				if role, ok := pl.proposedSections[item.SectionID]; ok { // B-77
					sp.why, sp.newRole = graph.ProposalFromPlan, &role
				}
			} else {
				obj, ok := ref(path+"/section_id", item.SectionID, section)
				if !ok {
					continue
				}
				// B-73: what you edited, what is signed or in «cambió», and what
				// is being signed get a proposal; its anchors are looked for in
				// the version the agent read (T-4 (2)). B-11 stays for the rest:
				// it looks at the text only (P-84, T-4, K-17): a version that
				// changed the title, focus, position or role but kept the text
				// of the version read does not make it stale.
				current := bodyStr(bodyOf(obj), "text")
				sp.sectionID = obj.ID
				sp.why = p.proposalWhy(obj, item.BaseVersion)
				if obj.CurrentVersion != item.BaseVersion {
					text, ok := p.versionText(obj.ID, item.BaseVersion)
					if !ok || sp.why == "" && text != current {
						fail(path+"/base_version", "%s", errStale{label: item.SectionID}.Error())
						continue
					}
					if sp.why != "" {
						current = text
					}
				}
				sp.readVersion, sp.readText = obj.CurrentVersion, current
				sp.base = graph.Compare(current).Orig
			}
			bySection[item.SectionID] = sp
			if sp.why != "" {
				pl.proposals = append(pl.proposals, sp)
			} else {
				pl.sections = append(pl.sections, sp)
			}
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
		e := edit{index: i, text: item.Text, anchor: item.Anchor, origin: item.ClaimOrigin, operation: item.Operation}
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
	for _, sp := range pl.proposals {
		if msg := overlap(sp); msg != "" {
			fail("/new_text", "%s", msg)
		}
		sp.result = applyEdits(sp.base, sp.edits)
	}
	// B-77: a new section declared without text is a proposal too.
	for _, r := range out.ProposedRoles {
		if role, ok := pl.proposedSections[r.SectionID]; ok && !pl.isProposed(r.SectionID) {
			pl.proposals = append(pl.proposals, &sectionPlan{ref: r.SectionID, why: graph.ProposalFromPlan, newRole: &role})
		}
	}

	// Figure and citation anchors are found in the text that results
	// from the step, in any section (P-52: the schema's «pasaje literal
	// de la cifra en el texto que resulta del paso»; B-15 closes «cifra
	// sin decisión» «al vincularla»), so a step can link what an earlier
	// step wrote. They name no section: each must appear once in the
	// project. An anchor found only in a proposed text goes with its
	// proposal and is applied at accepting (B-73).
	after := p.textsAfter(pl)
	written := func(path, anchor string) (skip bool, proposal *sectionPlan) {
		n, locked := 0, false
		for key, text := range after {
			found := len(graph.Compare(text).Find(anchor))
			n += found
			locked = locked || found > 0 && p.locked(key)
		}
		if n == 0 || n == 1 && locked {
			if sp := pl.proposedAnchor(anchor); sp != nil {
				return true, sp
			}
		}
		// B-14: linking into a section in signoff or signed touches it;
		// the link is not applied, like a text there.
		// [EL DISEÑO NO LO DICE]: P-52 does not say it; the smallest
		// choice follows B-14.
		if n == 1 && locked {
			return true, nil
		}
		if n != 1 {
			fail(path, "the anchor appears %d times in the document after this step; give a longer anchor that appears once", n)
		}
		return false, nil
	}
	for i, f := range out.FigureLinks {
		skip, sp := written(fmt.Sprintf("/figure_links/%d/figure_anchor", i), f.FigureAnchor)
		pl.skipFigures[i] = skip
		if sp != nil {
			pl.heldFigures[i] = true
			sp.figures = append(sp.figures, f)
		}
		if len(graph.Numbers(f.FigureAnchor)) == 0 {
			fail(fmt.Sprintf("/figure_links/%d/figure_anchor", i), "the anchor of a figure must contain its number")
		}
	}
	for i, c := range out.Citations {
		skip, sp := written(fmt.Sprintf("/citations/%d/claim_anchor", i), c.ClaimAnchor)
		pl.skipCitations[i] = skip
		if sp != nil {
			pl.heldCitations[i] = true
			sp.citations = append(sp.citations, c)
		}
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

// proposalWhy says why a step's text on this section goes to a proposal
// (B-73 a to c), or "". «Cambió por tu edición» is an edit event of yours
// with a new version above the step's base version.
func (p *project) proposalWhy(obj store.Object, baseVersion int) string {
	switch {
	case p.inSignoff[obj.ID]:
		return graph.ProposalFromSignoff
	case obj.State == "signed":
		return graph.ProposalFromSigned
	case obj.State == "changed_since_signoff":
		return graph.ProposalFromChanged
	case p.yourEdits[obj.ID] > baseVersion:
		return graph.ProposalFromYourEdit
	}
	return ""
}

// locked reports whether a section (by id) takes no text of the agent's
// steps, which go to a proposal (B-14, P-40, B-73).
func (p *project) locked(id string) bool {
	obj, ok := p.byID[id]
	return ok && (p.inSignoff[id] || obj.State == "signed" || obj.State == "changed_since_signoff")
}

// textsAfter is the text of every section after the step: what the
// step writes, or the current text of a section it does not write.
func (p *project) textsAfter(pl *plan) map[string]string {
	out := map[string]string{}
	for _, sp := range pl.sections {
		key := sp.ref
		if obj, ok := p.object(sp.ref); ok && !isNew(sp.ref) {
			key = obj.ID
		}
		out[key] = sp.result
	}
	for id, obj := range p.byID {
		if _, done := out[id]; !done && obj.Type == store.ObjectSection {
			out[id] = bodyStr(bodyOf(obj), "text")
		}
	}
	return out
}

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
		if e.start == e.end {
			out = insertBlock(out[:e.start], e.text, out[e.end:])
			continue
		}
		out = out[:e.start] + e.text + out[e.end:]
	}
	return out
}

// insertBlock puts an inserted text (insert_after) after its anchor.
// A text that would be glued to what surrounds it («…equipos.Se
// consideran…») is its own paragraph, with a blank line on each side
// (P-46); a text that brings its own space or line break is inserted
// as written.
// [EL DISEÑO NO LO DICE]: T-4 says only that the text goes after the
// anchor; an anchor inside a paragraph splits it. Only the spaces that
// start after are dropped: edits apply back to front, so before keeps
// the offsets of the edits still to apply.
func insertBlock(before, text, after string) string {
	space := func(r byte) bool { return r == ' ' || r == '\n' || r == '\t' }
	gluedBefore := before != "" && text != "" && !space(before[len(before)-1]) && !space(text[0])
	gluedAfter := before == "" && after != "" && text != "" && !space(text[len(text)-1]) && !space(after[0])
	if !gluedBefore && !gluedAfter {
		return before + text + after
	}
	text = strings.Trim(text, "\n")
	if before != "" {
		text = "\n\n" + text
	}
	after = strings.TrimLeft(after, " \t")
	if after != "" && !strings.HasPrefix(after, "\n\n") {
		if strings.HasPrefix(after, "\n") {
			text += "\n"
		} else {
			text += "\n\n"
		}
	}
	return before + text + after
}

// apply writes the validated step. Section text first, then the new
// objects, so each new-n has its label before any edge is created
// (specification §3, rule 4), then the edges and step_ended.
func (p *project) apply(tx *store.Tx, turnID string, out stepOutput, pl *plan, stop StopReason) (map[string]string, error) {
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
	// P-89: the signoffs in progress read again in the transaction.
	inSignoff, err := graph.SectionsInSignoff(context.Background(), tx, tx.ProjectID())
	if err != nil {
		return nil, err
	}

	for _, sp := range pl.sections {
		// The step's texts and anchors go in its event, so the history by
		// block finds the step (B-10).
		// [EL DISEÑO NO LO DICE]: the edit event records base and new
		// version only; texts and anchors are added to its data.
		texts, anchors, origins := []string{}, []string{}, []string{}
		for _, e := range sp.edits {
			texts = append(texts, e.text)
			origins = append(origins, e.origin)
			if e.anchor != "" {
				anchors = append(anchors, e.anchor)
			}
		}
		stepEvent := func(t store.EventType) store.EventInput {
			ev := agent(t)
			// claim_origins goes by text, in the order of texts (R-1;
			// logic in piece 2.11).
			ev.Data = map[string]any{"texts": texts, "anchors": anchors, "claim_origins": origins}
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
		read, _ := p.object(sp.ref)
		// P-84: a version written after the read that kept the text and
		// the state (title, focus, position, role) does not make the step
		// stale; the write keeps those details and is conditioned on the
		// version read here, in the transaction.
		obj, err := tx.Object(read.ID)
		if err != nil {
			return nil, err
		}
		// P-89 (B-14): a signoff that started, or a signature, after the
		// validation rejects the step here, in the write's transaction.
		if inSignoff[obj.ID] || obj.State == "signed" || obj.State == "changed_since_signoff" {
			return nil, errLocked{label: sp.ref, signing: inSignoff[obj.ID]}
		}
		if obj.CurrentVersion != sp.readVersion &&
			(obj.State != read.State || bodyStr(bodyOf(obj), "text") != sp.readText) {
			return nil, errStale{label: sp.ref} // B-11: changed after the step was read
		}
		b := bodyOf(obj)
		b["text"] = sp.result
		// [EL DISEÑO NO LO DICE]: the event of the agent's text change is
		// not named; edit with actor agent, like the human's (estados §8).
		_, err = tx.UpdateObject(obj.ID, obj.CurrentVersion, obj.State, b, stepEvent(store.EventEdit))
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
		// A principal's new section goes last (K-17).
		position, err := graph.NextSectionPosition(context.Background(), tx, tx.ProjectID())
		if err != nil {
			return nil, err
		}
		obj, err := tx.CreateObject(store.ObjectSection, "draft", map[string]any{
			"title": role.Title, "position": position, "text": "",
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
	var changed []string // decisions whose content this step changed (rule 4)
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
			changed = append(changed, old.ID)
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
	// The anchor is in a section the step wrote or, already, in any
	// other (P-52).
	inSection := func(anchor string) string {
		for _, sec := range sectionIDs {
			if len(graph.Compare(resultText[sec]).Find(anchor)) > 0 {
				return sec
			}
		}
		for id, obj := range p.byID {
			if _, done := resultText[id]; !done && obj.Type == store.ObjectSection &&
				len(graph.Compare(bodyStr(bodyOf(obj), "text")).Find(anchor)) > 0 {
				return id
			}
		}
		return ""
	}
	linkedValues := map[string]bool{}
	for i, f := range out.FigureLinks {
		if pl.skipFigures[i] {
			continue
		}
		if err := linkFigure(tx, edges, inSection(f.FigureAnchor), f.FigureAnchor, resolve(f.DecisionID), agent(store.EventStateChanged)); err != nil {
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
		src, _ := p.object(c.SourceID)
		id, err := p.linkCitation(tx, edges, inSection(c.ClaimAnchor), c.ClaimAnchor, c.Passage, src, agent(store.EventStateChanged))
		if err != nil {
			return nil, err
		}
		p.newCitations = append(p.newCitations, id)
	}

	// Data (A-3): a number of the new text with no figure link that
	// appears literal, separators normalized, in a source used in the
	// step or in the human's material is linked to that source.
	// [EL DISEÑO NO LO DICE]: «tu material» is read as the project's
	// sources, all uploaded by the human in slice 1.
	if err := p.linkData(tx, out, pl, ids, linkedValues, edges, agent(store.EventStateChanged)); err != nil {
		return nil, err
	}

	// The principal's proposed open questions open, with their blocks
	// edges (backlog-r2 2.4; what B-17 kept as not applied). The
	// principal declared it, so it is not «no lo declaró el agente».
	// [EL DISEÑO NO LO DICE]: a depends_on naming a new section that went
	// to a proposal (B-77) is skipped; the question still opens on the rest.
	var proposed []struct {
		ID        string           `json:"id"`
		Question  string           `json:"question"`
		Options   []map[string]any `json:"options"`
		DependsOn []string         `json:"depends_on"`
		Variants  []string         `json:"variants"`
	}
	_ = json.Unmarshal(out.ProposedOpenQuestions, &proposed)
	for _, q := range proposed {
		var dependsOn []string
		for _, dep := range q.DependsOn {
			if id := resolve(dep); id != "" {
				dependsOn = append(dependsOn, id)
			}
		}
		oq, err := checks.Open(context.Background(), tx, checks.OpenQuestion{Question: q.Question, Options: q.Options,
			DependsOn: dependsOn, Variants: q.Variants,
			Extra: map[string]any{"origin": "principal", "undeclared_by_agent": false},
			Event: agent(store.EventStateChanged)})
		if err != nil {
			return nil, err
		}
		ids[q.ID], labels[q.ID] = oq.ID, oq.Label
	}

	// B-73, B-74: what goes to a proposal, in the step's transaction; the
	// «actualizado» closes of those sections go with it and are not judged
	// now.
	handledNow, err := p.createProposals(tx, turnID, out, pl, ids, labels)
	if err != nil {
		return nil, err
	}

	notApplied := map[string]any{}
	// Rule 7 of the gate (specification §3): each close is checked in
	// this transaction; one that does not comply stays here, rejected
	// alone, with its message for the agent.
	rejectedCloses, err := applyHandledChanges(context.Background(), tx, turnID, handledNow, written)
	if err != nil {
		return nil, err
	}
	if rejectedCloses != nil {
		notApplied["handled_changes"] = rejectedCloses
	}
	p.rejectedCloses = rejectedCloses
	if rolesNotApplied != nil {
		notApplied["proposed_roles"] = rolesNotApplied // P-32
	}
	// Links into a section that takes no text of the agent (P-52); those that
	// went with a proposal are not «not applied» (B-73).
	{
		var figures, citations []any
		for i, f := range out.FigureLinks {
			if pl.skipFigures[i] && !pl.heldFigures[i] {
				figures = append(figures, map[string]any{"figure_anchor": f.FigureAnchor, "decision_id": f.DecisionID})
			}
		}
		for i, c := range out.Citations {
			if pl.skipCitations[i] && !pl.heldCitations[i] {
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
	// The sections the step wrote and the decisions it declared, in the
	// step's history (the pending they closed was retired by P-65).
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
	if len(p.proposals) > 0 {
		data["proposals"] = p.proposals
	}
	// The app's rules 0 to 5, in this transaction, before step_ended,
	// so the reclassification (rule 0) and «escrito sin decisión» (rule
	// 2) are written in it (backlog-r2 2.1). No rule stops the chain.
	if err := p.runRules(tx, turnID, out, pl, ids, sectionIDs, changed, data); err != nil {
		return nil, err
	}
	// B-57: the round request, after rule 0 settled the stretch.
	if p.round, err = p.applyResearchRequest(tx, turnID, out, data); err != nil {
		return nil, err
	}
	if longDeferred != nil {
		data["long_deferred"] = longDeferred
	}
	if out.Reply != "" { // R-5: the reply goes to the conversation
		data["reply"] = out.Reply
	}
	// B-69: what the step names without a citation stays in its step_ended
	// (graph.Candidates): candidates, never sources.
	var named []namedReference
	if err := json.Unmarshal(out.NamedReferences, &named); err == nil && len(named) > 0 {
		data["named_references"] = named
	}
	if stop == StopNoWork && p.round != "" {
		stop = p.roundStepStop()
	}
	if stop != "" { // B-24
		data["stop_reason"] = string(stop)
	}
	ev := agent(store.EventStepEnded)
	ev.Data = data
	if _, err := tx.AppendEvent(ev); err != nil {
		return nil, err
	}
	return labels, nil
}

// linkFigure creates a figure with its covers edge, and the decision's
// used_in to the section where the figure is.
func linkFigure(tx *store.Tx, edges *edgeSet, sectionID, anchor, decisionID string, ev store.EventInput) error {
	// [EL DISEÑO NO LO DICE]: a figure's value is the first number of
	// its anchor.
	value := graph.Numbers(anchor)[0].Raw
	fig, err := tx.CreateObject(store.ObjectFigure, "",
		map[string]any{"section_id": sectionID, "anchor": anchor, "value": value}, ev)
	if err != nil {
		return err
	}
	if err := edges.add(store.EdgeCovers, fig.ID, decisionID, anchor); err != nil {
		return err
	}
	return edges.add(store.EdgeUsedIn, decisionID, sectionID, "")
}

// linkCitation creates a citation, with its comes_from edge when the
// passage is literal in a source the principal read. Otherwise no
// comes_from: the pending «fuente que no sostiene» (B-15), computed by
// graph.SourcesNotSupporting. It returns the citation's id.
func (p *project) linkCitation(tx *store.Tx, edges *edgeSet, sectionID, anchor, passage string, src store.Object, ev store.EventInput) (string, error) {
	cit, err := tx.CreateObject(store.ObjectCitation, "",
		map[string]any{"section_id": sectionID, "anchor": anchor, "passage": passage}, ev)
	if err != nil {
		return "", err
	}
	if p.wasRead(src.ID) && len(graph.Compare(bodyStr(bodyOf(src), "text")).Find(passage)) > 0 {
		if err := edges.add(store.EdgeComesFrom, cit.ID, src.ID, ""); err != nil {
			return "", err
		}
	}
	return cit.ID, nil
}

func (p *project) linkData(tx *store.Tx, out stepOutput, pl *plan, ids map[string]string,
	linked map[string]bool, edges *edgeSet, ev store.EventInput) error {
	used := map[string]bool{}
	var order []store.Object
	for _, s := range out.UsedSources {
		if src, ok := p.object(s); ok && !used[src.ID] && p.wasRead(src.ID) {
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
		if pl.isProposed(item.SectionID) { // its data come at accepting (B-73)
			continue
		}
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
	_, err := s.tx.AddEdge(t, from, to, anchor, usedData(s.tx, t, from, to), s.ev)
	return err
}

// usedData is what a using edge stores of what it used (B-45): the
// decision of used_in, the figure of covers, the source of uses; nil
// for every other edge.
func usedData(tx *store.Tx, t store.EdgeType, from, to string) any {
	id := ""
	switch t {
	case store.EdgeUsedIn, store.EdgeCovers:
		id = from
	case store.EdgeUses:
		id = to
	default:
		return nil
	}
	obj, err := tx.Object(id)
	if err != nil {
		return nil
	}
	return graph.UsageOf(obj)
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// runRules builds the change of an applied step, runs the app's rules
// on it and writes into step_ended's data what rules 0 and 2 leave
// there: the stretch the app settled, with the proposed one and why
// when rule 0 reclassified it (B-43), the sections left with «escrito
// sin decisión» (B-42) and those where the step declared a decision,
// which closes it.
func (p *project) runRules(tx *store.Tx, turnID string, out stepOutput, pl *plan, ids map[string]string,
	sectionIDs, changed []string, data map[string]any) error {
	resolve := func(value string) string {
		if isNew(value) {
			return ids[value]
		}
		obj, _ := p.object(value)
		return obj.ID
	}
	change := checks.Change{Kind: checks.AgentStep, Sections: sectionIDs, Stretch: out.Stretch,
		Decided: map[string]bool{}, Changed: changed, Actor: "agent", TurnID: turnID}
	for _, sp := range pl.sections {
		for _, e := range sp.edits {
			change.Texts = append(change.Texts, e.text)
			change.Passages = append(change.Passages, checks.Passage{Section: resolve(sp.ref), Text: e.text})
		}
	}
	change.NewCitations = p.newCitations
	// What the step's proposed open questions depend on, by label: what
	// the principal declared, for «no lo declaró el agente» (B2-7).
	var openQuestions []struct {
		DependsOn []string `json:"depends_on"`
	}
	_ = json.Unmarshal(out.ProposedOpenQuestions, &openQuestions)
	for _, q := range openQuestions {
		for _, dep := range q.DependsOn {
			if isNew(dep) {
				if obj, err := tx.Object(ids[dep]); err == nil {
					change.Declared = append(change.Declared, obj.Label)
				}
				continue
			}
			change.Declared = append(change.Declared, dep)
		}
	}
	for i := range out.FigureLinks {
		if !pl.skipFigures[i] {
			change.Figures = true
		}
	}
	for i := range out.Citations {
		if !pl.skipCitations[i] {
			change.Citations = true
		}
	}
	for _, d := range out.NewDecisions {
		if len(d.Applies) > 0 {
			change.Criteria = true
		}
		for _, sec := range d.UsedInSections {
			change.Decided[resolve(sec)] = true
		}
	}
	if len(out.UsedDecisions) > 0 { // used_in goes to every section the step wrote
		for _, sec := range sectionIDs {
			change.Decided[sec] = true
		}
	}
	findings, err := checks.RunRules(context.Background(), tx, tx.ProjectID(), change)
	if err != nil {
		return err
	}
	p.change = change
	data["stretch"] = out.Stretch
	p.stretch = out.Stretch
	withoutDecision, decided, warned := []string{}, []string{}, []any{}
	for _, f := range findings {
		switch f.Kind {
		case checks.FindingUsageChange:
			warned = append(warned, map[string]any{"change": f.Change, "section": f.Section, "about": f.About})
		case checks.FindingReclassified:
			data["stretch"], data["proposed_stretch"], data["reclassified"] = f.About, out.Stretch, f.Reasons
			p.stretch, p.reclassified = f.About, f.Reasons
		case graph.PendingTextWithoutDecision:
			withoutDecision = append(withoutDecision, f.Section)
		}
	}
	for _, id := range sectionIDs {
		if change.Decided[id] {
			if sec, err := tx.GetObject(context.Background(), tx.ProjectID(), id); err == nil {
				decided = append(decided, sec.Label)
			}
		}
	}
	// P-111, B-42: a decision declared later, used in a section this step
	// does not write, closes that section's «escrito sin decisión» too.
	for _, id := range sortedDecidedElsewhere(change.Decided, sectionIDs) {
		if sec, err := tx.GetObject(context.Background(), tx.ProjectID(), id); err == nil {
			decided = append(decided, sec.Label)
		}
	}
	data["text_without_decision"], data["decided_sections"] = withoutDecision, decided
	if len(warned) > 0 {
		// [EL DISEÑO NO LO DICE]: step_ended does not list the changes
		// rule 4 entered; they are added as usage_changes.
		data["usage_changes"] = warned
	}
	return nil
}
