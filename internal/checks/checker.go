package checks

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/LaurenteEber/tupaqnin/internal/graph"
	"github.com/LaurenteEber/tupaqnin/internal/provider"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// Passage is one new text of a change, with the id of its section.
type Passage struct {
	Section string
	Text    string
}

// Probe is one closed question the app builds from the graph (bloque-2
// §5, B2-7): what is protected, by label and in literal, against the
// change's new passages. The checker answers contradicts · no · doubt.
type Probe struct {
	Protected string `json:"protected"`
	// Kind is the protected object's type: request, understanding_case,
	// criterion, premise or decision.
	Kind string `json:"kind"`
	Text string `json:"text"`
	// Criterion and Counts: an understanding case's criterion, by label,
	// and whether the case counts.
	Criterion string `json:"criterion,omitempty"`
	Counts    bool   `json:"counts,omitempty"`
	// State: a decision's state (human, confirmed, agent…).
	State string `json:"state,omitempty"`
}

// Probes builds the probes of a change from the graph (fichas §3
// «Contradice lo confirmado o tu premisa», bloque-2 §5): always the
// request, the confirmed understanding with its cases, the criteria,
// the premise, every decision of yours and every current critical
// decision, linked to the change or not; besides, the confirmed
// decisions and those of a signed section that are linked to what the
// change touched (used_in one of its sections). Each protected object
// once, in that order.
// [EL DISEÑO NO LO DICE]: the backlog's signature has no reader; q is
// added, as in RunRules.
func Probes(ctx context.Context, q store.Querier, projectID string, change Change) ([]Probe, error) {
	var out []Probe
	seen := map[string]bool{}
	add := func(p Probe) {
		if !seen[p.Protected] {
			seen[p.Protected] = true
			out = append(out, p)
		}
	}
	list := func(t store.ObjectType) ([]store.Object, error) { return q.ListObjects(ctx, projectID, t) }

	requests, err := list(store.ObjectRequest)
	if err != nil {
		return nil, err
	}
	for _, r := range requests {
		add(Probe{Protected: r.Label, Kind: string(r.Type), Text: bodyText(r, "text")})
	}
	criteria, err := list(store.ObjectCriterion)
	if err != nil {
		return nil, err
	}
	criterionLabel := map[string]string{}
	for _, c := range criteria {
		criterionLabel[c.ID] = c.Label
	}
	cases, err := list(store.ObjectUnderstandingCase)
	if err != nil {
		return nil, err
	}
	for _, c := range cases {
		// The confirmed understanding: its cases you confirmed or
		// corrected (estados §6).
		if c.State != "confirmed" && c.State != "corrected" {
			continue
		}
		var b struct {
			Counts      bool   `json:"counts"`
			CriterionID string `json:"criterion_id"`
		}
		_ = json.Unmarshal(c.Body, &b)
		add(Probe{Protected: c.Label, Kind: string(c.Type), Text: bodyText(c, "text"),
			Criterion: criterionLabel[b.CriterionID], Counts: b.Counts})
	}
	for _, c := range criteria {
		add(Probe{Protected: c.Label, Kind: string(c.Type), Text: bodyText(c, "text")})
	}
	premises, err := list(store.ObjectPremise)
	if err != nil {
		return nil, err
	}
	for _, p := range premises {
		add(Probe{Protected: p.Label, Kind: string(p.Type), Text: bodyText(p, "text")})
	}

	decisions, err := list(store.ObjectDecision)
	if err != nil {
		return nil, err
	}
	critical, err := graph.CriticalAll(ctx, q, projectID)
	if err != nil {
		return nil, err
	}
	touched := map[string]bool{}
	for _, id := range change.Sections {
		touched[id] = true
	}
	sections, err := list(store.ObjectSection)
	if err != nil {
		return nil, err
	}
	signed := map[string]bool{}
	for _, s := range sections {
		if s.State == "signed" || s.State == "changed_since_signoff" {
			signed[s.ID] = true
		}
	}
	for _, d := range decisions {
		if d.State == "superseded" || d.State == "revoked" {
			continue // only current decisions are protected
		}
		edges, err := q.EdgesFrom(ctx, projectID, d.ID)
		if err != nil {
			return nil, err
		}
		linked, inSigned := false, false
		for _, e := range edges {
			if e.Type != store.EdgeUsedIn {
				continue
			}
			linked = linked || touched[e.ToID]
			inSigned = inSigned || signed[e.ToID]
		}
		if d.State == "human" || len(critical[d.ID]) > 0 ||
			(linked && (d.State == "confirmed" || inSigned)) {
			add(Probe{Protected: d.Label, Kind: string(d.Type), Text: bodyText(d, "statement"), State: d.State})
		}
	}
	return out, nil
}

func bodyText(obj store.Object, key string) string {
	var b map[string]any
	_ = json.Unmarshal(obj.Body, &b)
	s, _ := b[key].(string)
	return s
}

// CheckerOutput is the checker's fixed output (salida-comprobador.
// schema.json, fichas §7).
type CheckerOutput struct {
	Probes     []ProbeResult `json:"probes"`
	Candidates []Candidate   `json:"candidates"`
}

// ProbeResult is the checker's verdict on one probe.
type ProbeResult struct {
	Protected string  `json:"protected"`
	Verdict   string  `json:"verdict"`
	Passage   *string `json:"passage"`
}

// Verdicts of a probe (glossary §10, probe_result).
const (
	VerdictContradicts = "contradicts"
	VerdictNo          = "no"
	VerdictDoubt       = "doubt"
)

// Candidate is one thing the checker found, with its literal passages;
// the app routes it (flujos §1).
type Candidate struct {
	Type            string            `json:"type"`
	Section         string            `json:"section"`
	NewPassage      string            `json:"new_passage"`
	ClashingPassage *string           `json:"clashing_passage"`
	Protected       *string           `json:"protected"`
	Citation        *string           `json:"citation"`
	Question        *string           `json:"question"`
	Options         []CandidateOption `json:"options"`
}

// CandidateOption is one option of the «por aclarar» the checker drafts
// (B-44, estados §4).
type CandidateOption struct {
	Option      string `json:"option"`
	Contrast    string `json:"contrast"`
	Consequence string `json:"consequence"`
	Risk        string `json:"risk"`
}

func str(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// Checker calls the checker model on a change (backlog-r2 2.3): the
// instructions and the full output schema come from the caller, which
// embeds them (internal/cycle).
type Checker struct {
	Provider     provider.Provider
	Instructions string
	Schema       []byte

	once     sync.Once
	compiled *jsonschema.Schema
	sendable json.RawMessage
	err      error
}

// NewChecker returns a checker over prov.
func NewChecker(prov provider.Provider, instructions string, schema []byte) *Checker {
	return &Checker{Provider: prov, Instructions: instructions, Schema: schema}
}

func (c *Checker) schemas() (*jsonschema.Schema, json.RawMessage, error) {
	c.once.Do(func() {
		doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(c.Schema))
		if err != nil {
			c.err = fmt.Errorf("checks: parse checker schema: %w", err)
			return
		}
		comp := jsonschema.NewCompiler()
		const loc = "https://tupaqnin.invalid/checker_output.schema.json"
		if err := comp.AddResource(loc, doc); err != nil {
			c.err = fmt.Errorf("checks: add checker schema: %w", err)
			return
		}
		if c.compiled, err = comp.Compile(loc); err != nil {
			c.err = fmt.Errorf("checks: compile checker schema: %w", err)
			return
		}
		c.sendable, c.err = provider.TransportSchema(c.Schema)
	})
	return c.compiled, c.sendable, c.err
}

// Validate checks a checker output against its fixed shape: the schema,
// and what the schema cannot say (fichas §7, «forma fija»): one verdict
// for each probe the app built, each a probe of this call; a probe that
// contradicts or doubts has its candidate (contradiction or doubtful
// term on the same protected label), which carries the question and
// the options (B-44); a candidate names a section of the change, a
// contradiction one of the probes, a doubtful term a criterion and a
// source that does not support a citation. Errors name labels only.
// [EL DISEÑO NO LO DICE]: the order of the probes is not checked, only
// that each comes once.
func (c *Checker) Validate(ctx context.Context, q store.Querier, projectID string, change Change, probes []Probe, raw string) ([]string, CheckerOutput, error) {
	var out CheckerOutput
	sch, _, err := c.schemas()
	if err != nil {
		return nil, out, err
	}
	inst, err := jsonschema.UnmarshalJSON(strings.NewReader(raw))
	if err != nil {
		return []string{"/: output is not valid JSON"}, out, nil
	}
	if err := sch.Validate(inst); err != nil {
		var verr *jsonschema.ValidationError
		if !errors.As(err, &verr) {
			return nil, out, fmt.Errorf("checks: validate checker output: %w", err)
		}
		return validationErrors(verr), out, nil
	}
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, out, fmt.Errorf("checks: parse checker output: %w", err)
	}

	var errs []string
	fail := func(format string, args ...any) { errs = append(errs, fmt.Sprintf(format, args...)) }
	built := map[string]bool{}
	for _, p := range probes {
		built[p.Protected] = true
	}
	answered := map[string]bool{}
	for i, r := range out.Probes {
		switch {
		case !built[r.Protected]:
			fail("/probes/%d/protected: %s is not one of the probes", i, r.Protected)
		case answered[r.Protected]:
			fail("/probes/%d/protected: %s is answered twice", i, r.Protected)
		}
		answered[r.Protected] = true
	}
	for _, p := range probes {
		if !answered[p.Protected] {
			fail("/probes: the probe %s has no verdict", p.Protected)
		}
	}
	touched := map[string]bool{}
	for _, id := range change.Sections {
		if sec, err := q.GetObject(ctx, projectID, id); err == nil {
			touched[sec.Label] = true
		}
	}
	routed := map[string]bool{}
	for i, cand := range out.Candidates {
		path := fmt.Sprintf("/candidates/%d", i)
		if !touched[cand.Section] {
			fail("%s/section: %s is not a section of this change", path, cand.Section)
		}
		switch cand.Type {
		case graph.CandidateContradiction, graph.CandidateDoubtfulTerm:
			prot := str(cand.Protected)
			if cand.Type == graph.CandidateContradiction && !built[prot] {
				fail("%s/protected: a contradiction names one of the probes", path)
			}
			if cand.Type == graph.CandidateDoubtfulTerm && !isType(ctx, q, projectID, prot, store.ObjectCriterion) {
				fail("%s/protected: a doubtful term names a criterion", path)
			}
			if cand.Question == nil || len(cand.Options) < 2 {
				fail("%s: a %s carries its question and two options or more", path, cand.Type)
			}
			routed[prot] = true
		case graph.CandidateSourceNotSupporting:
			if !isType(ctx, q, projectID, str(cand.Citation), store.ObjectCitation) {
				fail("%s/citation: a source that does not support names its citation", path)
			}
		}
	}
	for i, r := range out.Probes {
		if (r.Verdict == VerdictContradicts || r.Verdict == VerdictDoubt) && !routed[r.Protected] {
			fail("/probes/%d: %s is %s but no contradiction or doubtful_term candidate names it", i, r.Protected, r.Verdict)
		}
	}
	return errs, out, nil
}

func isType(ctx context.Context, q store.Querier, projectID, label string, t store.ObjectType) bool {
	if label == "" {
		return false
	}
	obj, err := q.GetObject(ctx, projectID, label)
	return err == nil && obj.Type == t && obj.Label == label
}

func validationErrors(verr *jsonschema.ValidationError) []string {
	var out []string
	var walk func(u *jsonschema.OutputUnit)
	walk = func(u *jsonschema.OutputUnit) {
		if len(u.Errors) == 0 {
			if u.Error != nil {
				path := u.InstanceLocation
				if path == "" {
					path = "/"
				}
				out = append(out, path+": "+u.Error.String())
			}
			return
		}
		for i := range u.Errors {
			walk(&u.Errors[i])
		}
	}
	walk(verr.BasicOutput())
	if len(out) == 0 {
		out = append(out, "/: "+verr.Error())
	}
	return out
}

// Brief is the checker's reduced brief (fichas §7, bloque-1.md §4): the
// probes in literal, the new passage of each section the change wrote
// and, for each new citation whose passage is literal in its source,
// the pair {claim, the source's literal passage, source label and
// fingerprint} (A-2). Labels only (T-2).
func Brief(ctx context.Context, q store.Querier, projectID string, change Change, probes []Probe) (string, error) {
	var b strings.Builder
	line := func(format string, args ...any) { fmt.Fprintf(&b, format+"\n", args...) }
	line("# Checker brief")
	line("")
	line("## Probes (one entry each in \"probes\", by label)")
	for _, p := range probes {
		switch {
		case p.Kind == string(store.ObjectUnderstandingCase):
			counts := "does not count"
			if p.Counts {
				counts = "counts"
			}
			line("- %s (understanding case of %s, %s): %s", p.Protected, p.Criterion, counts, p.Text)
		case p.State != "":
			line("- %s (%s, %s): %s", p.Protected, p.Kind, p.State, p.Text)
		default:
			line("- %s (%s): %s", p.Protected, p.Kind, p.Text)
		}
	}
	line("")
	bySection := map[string][]string{}
	var order []string
	for _, p := range change.Passages {
		if _, ok := bySection[p.Section]; !ok {
			order = append(order, p.Section)
		}
		bySection[p.Section] = append(bySection[p.Section], p.Text)
	}
	for _, id := range order {
		sec, err := q.GetObject(ctx, projectID, id)
		if err != nil {
			return "", err
		}
		line("## New passage of section %s «%s»", sec.Label, bodyText(sec, "title"))
		for _, text := range bySection[id] {
			line("<<<")
			line("%s", text)
			line(">>>")
		}
		line("")
	}
	var pairs []string
	for _, id := range change.NewCitations {
		cit, err := q.GetObject(ctx, projectID, id)
		if err != nil {
			return "", err
		}
		edges, err := q.EdgesFrom(ctx, projectID, cit.ID)
		if err != nil {
			return "", err
		}
		for _, e := range edges {
			if e.Type != store.EdgeComesFrom {
				continue
			}
			src, err := q.GetObject(ctx, projectID, e.ToID)
			if err != nil {
				return "", err
			}
			sec, err := q.GetObject(ctx, projectID, bodyText(cit, "section_id"))
			if err != nil {
				return "", err
			}
			pairs = append(pairs, fmt.Sprintf("- %s in %s: claim «%s» · source %s (fingerprint %s): «%s»",
				cit.Label, sec.Label, bodyText(cit, "anchor"), src.Label, bodyText(src, "fingerprint"), bodyText(cit, "passage")))
		}
	}
	if len(pairs) > 0 {
		line("## New citations (claim and the source's literal passage)")
		for _, p := range pairs {
			line("%s", p)
		}
	}
	return b.String(), nil
}

func userItem(text string) json.RawMessage {
	raw, _ := json.Marshal(map[string]any{
		"type": "message", "role": "user",
		"content": []map[string]string{{"type": "input_text", "text": text}},
	})
	return raw
}

// retryMessage is what the checker gets on its retry (fichas §7), as
// the principal's (cycle.RetryMessage).
func retryMessage(errs []string) string {
	return "Your output did not match the required schema: " + strings.Join(errs, "; ") +
		". Return the complete output again."
}

// CallChecker makes the checker's one call for a change (fichas §7: Spark
// 1.3 low, a clean context, the fixed shape and one retry). Each output
// that fails the shape, or a failed call, appends output_retry of turn
// kind checker: result retry after the first, unchecked after the
// second, which leaves «sin comprobar» on the change's sections (B-50).
// ok is false then. No transaction is open while the model answers
// (specification §2).
func (c *Checker) CallChecker(ctx context.Context, db *store.DB, projectID, turnID string, change Change, probes []Probe) (CheckerOutput, bool, error) {
	_, schema, err := c.schemas()
	if err != nil {
		return CheckerOutput{}, false, err
	}
	brief, err := Brief(ctx, db, projectID, change, probes)
	if err != nil {
		return CheckerOutput{}, false, err
	}
	labels := make([]string, 0, len(probes))
	for _, p := range probes {
		labels = append(labels, p.Protected)
	}
	sections, err := sectionLabels(ctx, db, projectID, change)
	if err != nil {
		return CheckerOutput{}, false, err
	}
	input := []json.RawMessage{userItem(brief)}
	for attempt := 1; attempt <= 2; attempt++ {
		resp, err := c.Provider.Respond(ctx, provider.Request{
			ProjectID: projectID, TurnID: turnID,
			Role: provider.RoleChecker, TurnKind: provider.TurnChecker,
			Model: provider.CheckerModel, Effort: provider.CheckerEffort,
			Instructions: c.Instructions, Input: input, Schema: schema, SchemaStrict: true,
			ProbeLabels: labels,
		}, nil)
		var errs []string
		var out CheckerOutput
		if err != nil {
			if ctx.Err() != nil {
				return CheckerOutput{}, false, ctx.Err()
			}
			// The provider's error body is never kept (T-10): only that
			// the call failed.
			errs = []string{"provider_error"}
		} else if errs, out, err = c.Validate(ctx, db, projectID, change, probes, resp.Text); err != nil {
			return CheckerOutput{}, false, err
		}
		if len(errs) == 0 {
			return out, true, nil
		}
		data := map[string]any{"turn_kind": "checker", "attempt": attempt, "errors": errs,
			"result": "retry", "sections": sections}
		if attempt == 2 {
			data["result"] = "unchecked"
		}
		err = db.Write(ctx, projectID, func(tx *store.Tx) error {
			_, err := tx.AppendEvent(store.EventInput{Type: store.EventOutputRetry, Actor: "agent",
				AgentRole: string(provider.RoleChecker), ModelAndEffort: provider.CheckerModel + " " + provider.CheckerEffort,
				TurnID: turnID, Data: data})
			return err
		})
		if err != nil {
			return CheckerOutput{}, false, err
		}
		if resp.RawItems != nil {
			input = append(append(input, resp.RawItems...), userItem(retryMessage(errs)))
		}
	}
	return CheckerOutput{}, false, nil
}

func sectionLabels(ctx context.Context, q store.Querier, projectID string, change Change) ([]string, error) {
	out := []string{}
	for _, id := range change.Sections {
		sec, err := q.GetObject(ctx, projectID, id)
		if err != nil {
			return nil, err
		}
		out = append(out, sec.Label)
	}
	return out, nil
}

// HasWork reports whether a change brings something for the checker to
// judge: a new passage or a new citation.
func (ch Change) HasWork() bool { return len(ch.Passages) > 0 || len(ch.NewCitations) > 0 }

// Check runs the checker on a change, outside its transaction (backlog-r2
// 2.3, specification §2): the probes, one call with its retry, and the
// judgment applied in a transaction of its own (Apply).
func (c *Checker) Check(ctx context.Context, db *store.DB, projectID, turnID string, change Change) error {
	probes, err := Probes(ctx, db, projectID, change)
	if err != nil {
		return err
	}
	out, ok, err := c.CallChecker(ctx, db, projectID, turnID, change, probes)
	if err != nil || !ok {
		return err
	}
	return Apply(ctx, db, projectID, turnID, change, out)
}

// Apply writes a valid judgment in its own transaction, against the
// current text (B-50): one probe event per probe, with what is
// protected, the verdict, the passage and whether the principal had
// declared it; one probe event with kind candidate per candidate, which
// is discarded (data.discarded) when its new passage is no longer
// literal in its section. Routing (flujos §1): a contradiction or a
// doubtful term the principal had not declared opens a «por aclarar»
// marked undeclared_by_agent (B2-7); process_narrative,
// written_without_decision and source_not_supporting stay in their
// event as the agent's pendings, which graph.SectionStatus computes.
// The checker creates no edges and decides nothing.
func Apply(ctx context.Context, db *store.DB, projectID, turnID string, change Change, out CheckerOutput) error {
	return db.Write(ctx, projectID, func(tx *store.Tx) error {
		sections, err := sectionLabels(ctx, tx, projectID, change)
		if err != nil {
			return err
		}
		// The principal declared it when one of its proposed open questions
		// depends on what is protected or on the candidate's section
		// (depends_on admits only sections and decisions).
		// [EL DISEÑO NO LO DICE]: how «lo había propuesto» is matched.
		declaredLabel := map[string]bool{}
		for _, l := range change.Declared {
			declaredLabel[l] = true
		}
		sectionOf := map[string]string{} // protected label → section of its candidate
		for _, cand := range out.Candidates {
			if p := str(cand.Protected); p != "" && sectionOf[p] == "" {
				sectionOf[p] = cand.Section
			}
		}
		declared := func(protected, section string) bool {
			return declaredLabel[protected] || declaredLabel[section]
		}
		event := func(objectID string, data map[string]any) error {
			data["sections"] = sections
			_, err := tx.AppendEvent(store.EventInput{Type: store.EventProbe, Actor: "agent",
				AgentRole: string(provider.RoleChecker), ModelAndEffort: provider.CheckerModel + " " + provider.CheckerEffort,
				TurnID: turnID, ObjectID: objectID, Data: data})
			return err
		}
		for _, r := range out.Probes {
			prot, err := tx.GetObject(ctx, projectID, r.Protected)
			if err != nil {
				return err
			}
			if err := event(prot.ID, map[string]any{"kind": graph.ProbeKindProbe, "protected": r.Protected,
				"verdict": r.Verdict, "passage": r.Passage, "declared": declared(r.Protected, sectionOf[r.Protected])}); err != nil {
				return err
			}
		}
		for _, cand := range out.Candidates {
			sec, err := tx.GetObject(ctx, projectID, cand.Section)
			if err != nil {
				return err
			}
			wasDeclared := declared(str(cand.Protected), cand.Section)
			data := map[string]any{"kind": graph.ProbeKindCandidate, "type": cand.Type, "section": cand.Section,
				"new_passage": cand.NewPassage, "clashing_passage": cand.ClashingPassage, "protected": cand.Protected,
				"citation": cand.Citation, "question": cand.Question, "options": cand.Options, "declared": wasDeclared}
			present := len(graph.Compare(bodyText(sec, "text")).Find(cand.NewPassage)) > 0
			switch {
			case !present:
				data["discarded"] = true // B-50: the text changed while the checker answered
			case cand.Type == graph.CandidateExceedsReviewed:
				// A-5 needs the coverage inventory (slice 3).
				data["not_applied"] = true
			case (cand.Type == graph.CandidateContradiction || cand.Type == graph.CandidateDoubtfulTerm) && !wasDeclared:
				label, err := openQuestion(ctx, tx, projectID, turnID, change, sec, cand)
				if err != nil {
					return err
				}
				data["open_question"] = label
			}
			if err := event(sec.ID, data); err != nil {
				return err
			}
		}
		return nil
	})
}

// openQuestion opens the «por aclarar» of a contradiction or a doubtful
// term (B-44: the checker drafted its question and options). It depends
// on the section and, when there is one, on the protected decision
// (backlog-r2 2.4). undeclared_by_agent marks «no lo declaró el agente»
// on an agent step (B2-7).
// [EL DISEÑO NO LO DICE]: piece 2.3 only creates the open open_question;
// its blocks edges, the premise's contradicted state, its pending and
// its card are piece 2.4's. On your edit there is no agent to have
// declared it, so the mark is false.
func openQuestion(ctx context.Context, tx *store.Tx, projectID, turnID string, change Change, sec store.Object, cand Candidate) (string, error) {
	dependsOn := []string{sec.ID}
	var protectedID string
	if prot := str(cand.Protected); prot != "" {
		obj, err := tx.GetObject(ctx, projectID, prot)
		if err != nil {
			return "", err
		}
		protectedID = obj.ID
		if obj.Type == store.ObjectDecision {
			dependsOn = append(dependsOn, obj.ID)
		}
	}
	options := make([]map[string]any, 0, len(cand.Options))
	for _, o := range cand.Options {
		options = append(options, map[string]any{"option": o.Option, "contrast": o.Contrast,
			"consequence": o.Consequence, "risk": o.Risk})
	}
	oq, err := tx.CreateObject(store.ObjectOpenQuestion, "open", map[string]any{
		"question": str(cand.Question), "options": options, "depends_on": dependsOn, "variants": []any{},
		"origin": "checker", "kind": cand.Type, "protected_id": protectedID, "section_id": sec.ID,
		"new_passage": cand.NewPassage, "clashing_passage": str(cand.ClashingPassage),
		"undeclared_by_agent": change.Kind == AgentStep,
	}, store.EventInput{Type: store.EventStateChanged, Actor: "agent", AgentRole: string(provider.RoleChecker), TurnID: turnID,
		Data: map[string]any{"via": "checker"}})
	if err != nil {
		return "", err
	}
	return oq.Label, nil
}

// YourEditsToCheck is the batch of your edits the checker has not judged
// yet (B-36): each section with an edit of yours after the last pass of
// the checker over it (a probe event, or the output_retry that left it
// «sin comprobar», whose sections include it). Its passage is the
// section's current text. The batch runs when you send a message.
// [EL DISEÑO NO LO DICE]: B-36's other two moments (when you ask to
// sign, and after 60 s without changes) are not built in piece 2.3.
func YourEditsToCheck(ctx context.Context, q store.Querier, projectID string) (Change, error) {
	change := Change{Kind: YourEdit, Actor: "human"}
	events, err := q.EventsSince(ctx, projectID, 0)
	if err != nil {
		return change, err
	}
	edited := map[string]int{} // section id → seq of your last edit
	judged := map[string]int{} // section label → seq of the last pass
	for _, ev := range events {
		var data map[string]any
		_ = json.Unmarshal(ev.Data, &data)
		switch {
		case ev.Type == store.EventEdit && ev.Actor == "human":
			edited[ev.ObjectID] = ev.Seq
		case ev.Type == store.EventProbe,
			ev.Type == store.EventOutputRetry && data["turn_kind"] == "checker" && data["result"] == "unchecked":
			list, _ := data["sections"].([]any)
			for _, l := range list {
				if s, ok := l.(string); ok {
					judged[s] = ev.Seq
				}
			}
		}
	}
	sections, err := graph.OrderedSections(ctx, q, projectID)
	if err != nil {
		return change, err
	}
	for _, sec := range sections {
		seq, ok := edited[sec.ID]
		if !ok || seq <= judged[sec.Label] {
			continue
		}
		text := bodyText(sec, "text")
		if strings.TrimSpace(text) == "" {
			continue
		}
		change.Sections = append(change.Sections, sec.ID)
		change.Passages = append(change.Passages, Passage{Section: sec.ID, Text: text})
	}
	return change, nil
}
