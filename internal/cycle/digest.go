package cycle

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/LaurenteEber/tupaqnin/internal/checks"
	"github.com/LaurenteEber/tupaqnin/internal/graph"
	"github.com/LaurenteEber/tupaqnin/internal/provider"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// The digest of a round (backlog-r3 3.4; fichas §2 «Investigar» and §7
// «Principal · digesto»; flujos §2 «del digesto al cierre»; B-63, B-66;
// estados §7). The principal digests what entered, in its live instance
// and at its usual effort; what can be computed (the discarded, the
// unverified, the coverage inventory, the cost) the app puts (B-63).
// The digest is the end of a step: it goes through the checks (2.3),
// and the round closes.

// The digest's instructions (a draft Eber reviews) and its fixed
// output, a copy of salida-digesto.schema.json.
//
//go:embed prompts/digest.en.md
var digestInstructions string

//go:embed schema/digest_output.schema.json
var digestSchema []byte

// The digest's status in the round's body.
const (
	DigestDone          = "done"
	DigestInvalidOutput = "invalid_output" // the fixed shape failed twice
	DigestProviderError = "provider_error" // the model call failed
	DigestProjectCap    = "project_cap"    // the project reached its cap
)

// Kinds of the «por aclarar» this piece opens (estados §4).
const (
	// OpenQuestionSourceConflict: a contradiction between findings of
	// different sources (R-9, B-66).
	OpenQuestionSourceConflict = "source_conflict"
	// OpenQuestionReaderContradiction: a contradiction between readers
	// on the same source that touches a critical decision (flujos §2,
	// estados §4).
	// [EL DISEÑO NO LO DICE]: its kind's name; contradiction, the
	// glossary's word for «contradicción».
	OpenQuestionReaderContradiction = "contradiction"
	// OpenQuestionScope: «dejarlo por aclarar» when a round did not
	// reduce the doubt (flujos §2, estados §4), a scope one (B-68).
	OpenQuestionScope = checks.OpenQuestionScope
)

// The options of a scope «por aclarar» (B-68: complete the review with
// another round, with its cost, or declare the limit in the text), as
// codes the page translates (every on-screen string from i18n).
const (
	ScopeOptionCompleteReview = checks.ScopeOptionCompleteReview
	ScopeOptionDeclareLimit   = checks.ScopeOptionDeclareLimit
)

// DigestOutput is salida-digesto, as the principal wrote it: findings
// by their number in the list the app gave.
type DigestOutput struct {
	Answer              string                `json:"answer"`
	Outline             []string              `json:"outline"`
	Findings            []DigestFindingUse    `json:"findings"`
	ExpectationContrast *string               `json:"expectation_contrast"`
	ReducedDoubt        string                `json:"reduced_doubt"`
	Contradictions      []DigestContradiction `json:"contradictions"`
}

// DigestFindingUse is one finding the outline uses, by number, with the
// outline line it serves (both from 1).
type DigestFindingUse struct {
	Finding     int `json:"finding"`
	OutlineLine int `json:"outline_line"`
}

// DigestContradiction is a pair of findings that contradict, by number,
// and what they clash about. It is shown, never resolved (flujos §2).
type DigestContradiction struct {
	FindingA int    `json:"finding_a"`
	FindingB int    `json:"finding_b"`
	About    string `json:"about"`
}

// DigestedFinding is one finding of the numbered list the principal
// received (entered or «no verificado», fichas §7): its reader and
// number there, kind, the source's own stretch, the note as the reader
// wrote it, and
// the id of its source ("" when the app stored none).
type DigestedFinding struct {
	N            int    `json:"n"`
	Part         int    `json:"part"`
	Finding      int    `json:"finding"`
	Kind         string `json:"kind"`
	Passage      string `json:"passage"`
	Note         string `json:"note,omitempty"`
	SourceID     string `json:"source_id,omitempty"`
	Hit          string `json:"hit,omitempty"`
	Verification string `json:"verification"`
	Access       string `json:"access,omitempty"`
	// ReaderPassage is the reader's wording when Passage, the source's own
	// stretch, differs from it (verifier F1, B-59).
	ReaderPassage string `json:"reader_passage,omitempty"`
}

// RoundConflict is one contradiction of the digest as the app routed it
// (B-66): the two findings by number, what they clash about, whether
// their sources differ, and the «por aclarar» it opened (an id), if
// any.
type RoundConflict struct {
	FindingA       int    `json:"finding_a"`
	FindingB       int    `json:"finding_b"`
	About          string `json:"about"`
	SameSource     bool   `json:"same_source"`
	OpenQuestionID string `json:"open_question_id,omitempty"`
}

// CoverageEntry is one reader's coverage (A-5): what was assigned (the
// project sources of its brief, by id, and the hit-n it saw) against
// what it says it read (sources_read), and its gaps.
type CoverageEntry struct {
	Part        int      `json:"part"`
	Status      string   `json:"status"`
	Reason      string   `json:"reason,omitempty"`
	Sources     []string `json:"sources"`
	SourcesRead []string `json:"sources_read"`
	Hits        []string `json:"hits"`
	HitsRead    []string `json:"hits_read"`
	Gaps        []string `json:"gaps"`
}

// readerFindingNote is a reader finding with its note.
type readerFindingNote struct {
	readerFinding
	Note *string `json:"note"`
}

// readerOutputOf parses a reader's stored output.
func readerOutputOf(rd ReaderResult) (findings []readerFindingNote, sourcesRead []string) {
	var out struct {
		Findings    []readerFindingNote `json:"findings"`
		SourcesRead []string            `json:"sources_read"`
	}
	if rd.Status != ReaderReturned || json.Unmarshal(rd.Output, &out) != nil {
		return nil, nil
	}
	return out.Findings, out.SourcesRead
}

// readerFindingAt is finding n (from 1) of reader part, if it exists.
func readerFindingAt(body RoundBody, part, n int) (readerFindingNote, bool) {
	for _, rd := range body.Readers {
		if rd.Part != part {
			continue
		}
		findings, _ := readerOutputOf(rd)
		if n >= 1 && n <= len(findings) {
			return findings[n-1], true
		}
	}
	return readerFindingNote{}, false
}

func deref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// digestFindings is the numbered list the principal receives (fichas
// §7, salida-digesto): the readers' findings that entered or are «no
// verificado», in the order the app checked them (reader by reader).
// The app's own (the round's references, part 0) have no passage and
// stay in the app's part of the digest.
func digestFindings(body RoundBody) []DigestedFinding {
	out := []DigestedFinding{}
	for _, c := range body.Findings {
		if c.Part == 0 || c.Verification == VerificationDiscarded {
			continue
		}
		f, ok := readerFindingAt(body, c.Part, c.Finding)
		if !ok {
			continue
		}
		reader, readerPassage := strings.TrimSpace(deref(f.Passage)), ""
		passage := findingPassage(c, reader)
		if passage != reader {
			readerPassage = reader
		}
		out = append(out, DigestedFinding{N: len(out) + 1, Part: c.Part, Finding: c.Finding, Kind: f.Kind,
			Passage: passage, Note: deref(f.Note), SourceID: c.SourceID,
			Hit: deref(f.Hit), Verification: c.Verification, Access: c.Access, ReaderPassage: readerPassage})
	}
	return out
}

// findingPassage is the passage everything after the check hands on: the
// source's own stretch, or the reader's when none was recorded (not
// entered, or a round stored before it).
func findingPassage(c FindingCheck, reader string) string {
	if c.SourcePassage != "" {
		return c.SourcePassage
	}
	return reader
}

// coverageInventory is A-5, by reader: the project sources of its brief
// and the hit-n it saw (its own results and the round's references the
// app found, part 0) against its sources_read, and its gaps' notes. An
// empty reader read nothing.
func coverageInventory(body RoundBody, labeler *graph.Labeler) []CoverageEntry {
	out := []CoverageEntry{}
	for i, part := range body.Parts {
		e := CoverageEntry{Part: i + 1, Status: ReaderPending, Sources: []string{}, SourcesRead: []string{},
			Hits: []string{}, HitsRead: []string{}, Gaps: []string{}}
		var findings []readerFindingNote
		read := map[string]bool{}
		for _, rd := range body.Readers {
			if rd.Part != i+1 {
				continue
			}
			e.Status, e.Reason = rd.Status, rd.Reason
			var sourcesRead []string
			findings, sourcesRead = readerOutputOf(rd)
			for _, l := range sourcesRead {
				if strings.HasPrefix(l, "hit-") {
					read[l] = true
				} else if id, err := labeler.ToID(l); err == nil {
					read[id] = true
				}
			}
		}
		for _, s := range part.Sources {
			e.Sources = append(e.Sources, s.SourceID)
			if read[s.SourceID] {
				e.SourcesRead = append(e.SourcesRead, s.SourceID)
			}
		}
		for _, h := range body.Hits {
			if h.Part != 0 && h.Part != i+1 {
				continue
			}
			e.Hits = append(e.Hits, h.Label)
			if read[h.Label] {
				e.HitsRead = append(e.HitsRead, h.Label)
			}
		}
		for _, f := range findings {
			if f.Kind == "gap" {
				e.Gaps = append(e.Gaps, deref(f.Note))
			}
		}
		out = append(out, e)
	}
	return out
}

// digestMessage is what the principal gets for the digest (fichas §7:
// «retornos de los lectores ya verificados + tu expectativa de la
// ronda»): the question, the doubt, the expectation in literal and the
// verified findings, numbered, each with its kind, its verification and
// its source by label. Every passage, note and title is data, between
// the markers (P-83); labels only, never ids (T-2).
func digestMessage(roundLabel string, body RoundBody, findings []DigestedFinding, labeler *graph.Labeler,
	titles map[string]string) string {
	var b strings.Builder
	line := func(format string, args ...any) { fmt.Fprintf(&b, format+"\n", args...) }
	line("# Digest of round %s", roundLabel)
	line("")
	line("Round question:%s", body.questionLine(body.Question))
	line("Doubt it reduces: %s", body.Doubt)
	if body.Expectation == "" {
		line("Expectation of the human for this round: none (no expectation): expectation_contrast is null.")
	} else {
		line("Expectation of the human for this round (literal): «%s»", body.Expectation)
	}
	line("")
	line("## Verified findings, numbered")
	if len(findings) == 0 {
		line("None: no finding entered. Say so in your answer; findings and contradictions are empty.")
	}
	for _, f := range findings {
		source := "none"
		if f.SourceID != "" {
			if l, err := labeler.ToLabel(f.SourceID); err == nil {
				source = l
			}
		}
		verification := "entered: its passage is literal in its source"
		if f.Verification == VerificationUnverified {
			verification = "not verified: its source could not be read"
		}
		line("%d. Reader %d · %s · %s · source %s", f.N, f.Part, f.Kind, verification, source)
		if t := titles[f.SourceID]; t != "" {
			line("Source title:")
			line("%s", graph.Delimit(graph.DataSearchResult, t))
		}
		line("Passage:")
		line("%s", graph.Delimit(graph.DataSource, f.Passage))
		if f.Note != "" {
			line("Reader's note:")
			line("%s", graph.Delimit(graph.DataSource, f.Note))
		}
	}
	line("")
	line("The app adds the discarded findings, the unverified ones and what was left unread; do not list them.")
	line("Return your fixed output only.")
	return b.String()
}

var (
	digestSchemaOnce sync.Once
	digestSchemaC    *jsonschema.Schema
	digestSchemaErr  error
)

// validateDigest checks a digest against salida-digesto and what the
// schema cannot say: every finding number is in the list the app gave
// (1 to n), every outline line exists, a contradiction names two
// different findings, and the contrast is there exactly when the round
// has an expectation (K-5). "path: message" per error; nil when it has
// the fixed shape.
func validateDigest(text string, n int, expectation bool) ([]string, DigestOutput) {
	errs, out := validateDigestSchema(text)
	if len(errs) > 0 {
		return errs, out
	}
	fail := func(format string, args ...any) { errs = append(errs, fmt.Sprintf(format, args...)) }
	for i, f := range out.Findings {
		if f.Finding > n {
			fail("/findings/%d/finding: %d is not in the list of findings (1 to %d)", i, f.Finding, n)
		}
		if f.OutlineLine > len(out.Outline) {
			fail("/findings/%d/outline_line: %d is not a line of the outline (1 to %d)", i, f.OutlineLine, len(out.Outline))
		}
	}
	for i, c := range out.Contradictions {
		if c.FindingA > n {
			fail("/contradictions/%d/finding_a: %d is not in the list of findings (1 to %d)", i, c.FindingA, n)
		}
		if c.FindingB > n {
			fail("/contradictions/%d/finding_b: %d is not in the list of findings (1 to %d)", i, c.FindingB, n)
		}
		if c.FindingA == c.FindingB {
			fail("/contradictions/%d: a contradiction names two different findings", i)
		}
	}
	switch {
	case expectation && out.ExpectationContrast == nil:
		fail("/expectation_contrast: the round has an expectation; say what matched and what did not")
	case !expectation && out.ExpectationContrast != nil:
		fail("/expectation_contrast: the round has no expectation; it is null")
	}
	return errs, out
}

// validateDigestSchema checks a digest against salida-digesto only.
func validateDigestSchema(text string) ([]string, DigestOutput) {
	var out DigestOutput
	digestSchemaOnce.Do(func() {
		doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(digestSchema))
		if err != nil {
			digestSchemaErr = err
			return
		}
		c := jsonschema.NewCompiler()
		const loc = "https://tupaqnin.invalid/digest_output.schema.json"
		if err := c.AddResource(loc, doc); err != nil {
			digestSchemaErr = err
			return
		}
		digestSchemaC, digestSchemaErr = c.Compile(loc)
	})
	if digestSchemaErr != nil {
		return []string{"/: digest schema unavailable"}, out
	}
	inst, err := jsonschema.UnmarshalJSON(strings.NewReader(text))
	if err != nil {
		return []string{"/: output is not valid JSON"}, out
	}
	if err := digestSchemaC.Validate(inst); err != nil {
		var verr *jsonschema.ValidationError
		if errors.As(err, &verr) {
			return flattenErrors(verr), out
		}
		return []string{"/: " + err.Error()}, out
	}
	if err := json.Unmarshal([]byte(text), &out); err != nil {
		return []string{"/: output is not valid JSON"}, out
	}
	return nil, out
}

// ValidateDigest checks a digest output as the app does for a round
// with n numbered findings, with or without an expectation; the
// micro-test of the digest records with it.
func ValidateDigest(text string, n int, expectation bool) ([]string, DigestOutput) {
	return validateDigest(text, n, expectation)
}

// callDigest makes the digest's call in the principal's instance, with
// its fixed shape and one retry (fichas §7). Each output that fails the
// shape appends output_retry of turn kind digest: retry after the
// first, agent_pending after the second (fichas §7: «el del principal,
// como pendiente del agente»). It returns the output and the digest's
// status. No transaction is open while the model answers.
func (r *Runner) callDigest(ctx context.Context, projectID, turnID, instanceID string, input []json.RawMessage,
	n int, expectation bool) (out DigestOutput, status string, sent bool, err error) {
	schema, err := provider.TransportSchema(digestSchema)
	if err != nil {
		return DigestOutput{}, "", false, err
	}
	dearest := 0.0
	req := provider.Request{ProjectID: projectID, TurnID: turnID, InstanceID: instanceID,
		Role: provider.RolePrincipal, TurnKind: provider.TurnDigest, Model: provider.PrincipalModel,
		Effort: provider.PrincipalEffort, Instructions: digestInstructions, Input: input,
		Schema: schema, SchemaStrict: true, DigestExpectation: expectation}
	for attempt := 1; attempt <= 2; attempt++ {
		// B-65: the call reserves before it starts, the retry too.
		over, err := r.digestOverCap(ctx, projectID, dearest)
		if err != nil {
			return DigestOutput{}, "", sent, err
		}
		if over {
			return DigestOutput{}, DigestProjectCap, sent, nil
		}
		resp, err := r.prov.Respond(ctx, req, nil)
		if err != nil {
			if ctx.Err() != nil {
				return DigestOutput{}, "", sent, ctx.Err()
			}
			// The provider's error body is never kept (T-10).
			return DigestOutput{}, DigestProviderError, sent, nil
		}
		sent = true
		dearest = max(dearest, resp.Usage.CostUSD)
		errs, out := validateDigest(resp.Text, n, expectation)
		if len(errs) == 0 {
			return out, DigestDone, true, nil
		}
		data := map[string]any{"turn_kind": string(provider.TurnDigest), "attempt": attempt, "errors": errs,
			"result": "retry"}
		if attempt == 2 {
			data["result"] = "agent_pending"
		}
		if err := r.gate.append(ctx, projectID, store.EventInput{Type: store.EventOutputRetry, Actor: "agent",
			AgentRole: string(provider.RolePrincipal), ModelAndEffort: provider.PrincipalModel + " " + provider.PrincipalEffort,
			TurnID: turnID, Data: data}); err != nil {
			return DigestOutput{}, "", sent, err
		}
		req.Input = append(append(append([]json.RawMessage{}, req.Input...), resp.RawItems...),
			userItem(RetryMessage(errs)))
	}
	return DigestOutput{}, DigestInvalidOutput, sent, nil
}

// digestOverCap is B-65 for the digest: what the project spent plus the
// reservation of one call (the maximum of one call: readerCallReserveUSD,
// or the dearest digest call of this round if it cost more) passes the
// project's cap.
// [EL DISEÑO NO LO DICE]: a digest call's maximum; the readers' reserve.
func (r *Runner) digestOverCap(ctx context.Context, projectID string, dearest float64) (bool, error) {
	spent, err := r.db.ProjectSpend(ctx, projectID)
	if err != nil {
		return false, err
	}
	const eps = 1e-9
	return spent+max(readerCallReserveUSD, dearest) > r.projectCap()+eps, nil
}

// digestInput is the principal's input for the digest: its live
// conversation, what changed since it was delivered and the digest's
// message; or, when no conversation is live, its startup and the
// message (block 2 §3, as the understanding turn).
func (r *Runner) digestInput(ctx context.Context, projectID, message string) (string, []json.RawMessage, error) {
	instanceID, startupInput, started, err := r.ensureInstance(ctx, projectID)
	if err != nil {
		return "", nil, err
	}
	if started {
		return instanceID, append(startupInput, userItem(message)), nil
	}
	conversation, err := r.conversation(ctx, projectID, instanceID)
	if err != nil {
		return "", nil, err
	}
	changes, err := r.changeItems(ctx, projectID, instanceID)
	if err != nil {
		return "", nil, err
	}
	input := append(conversation, changes...)
	return instanceID, append(input, userItem(message)), nil
}

// Digest runs the digest of a round whose findings the app verified
// (backlog-r3 3.4): the inventory (A-5), the principal's digest call
// with the numbered findings, the expectation and the question, and in
// one write conditioned on the round's version read: the digest and the
// inventory in the round's body, reduced_doubt, a delivery via = digest
// of each source the digest shows (B-62), the «por aclarar» of each
// contradiction between different sources (B-66), or between readers on
// the same source that touches a critical decision, and the round
// closed (estados §7). Then the digest goes through the checks (2.3):
// its change carries as new passages those of the findings it uses,
// each with its kind and its source. A round already closed is left as
// it is. The caller holds the project's turn slot.
// [EL DISEÑO NO LO DICE]: a digest that fails its shape twice, or whose
// call fails, still closes the round, without reduced_doubt; the app's
// part of the digest (discarded, unverified, inventory) stays.
func (r *Runner) Digest(ctx context.Context, projectID, roundLabel string) error {
	round, err := r.db.GetObject(ctx, projectID, roundLabel)
	if errors.Is(err, store.ErrNotFound) || (err == nil && round.Type != store.ObjectRound) {
		return ErrNotRound
	}
	if err != nil {
		return err
	}
	if round.State != "open" {
		return nil
	}
	var body RoundBody
	if err := json.Unmarshal(round.Body, &body); err != nil {
		return fmt.Errorf("cycle: round body: %w", err)
	}
	labeler, err := graph.NewLabeler(ctx, r.db, projectID)
	if err != nil {
		return err
	}
	findings := digestFindings(body)
	body.DigestFindings = findings
	body.CoverageInventory = coverageInventory(body, labeler)

	titles := map[string]string{}
	for _, f := range findings {
		if f.SourceID == "" {
			continue
		}
		if src, err := r.db.GetObject(ctx, projectID, f.SourceID); err == nil {
			titles[f.SourceID] = graph.SourceTitle(src)
		}
	}

	var out DigestOutput
	status, instanceID, sent := "", "", false
	// [EL DISEÑO NO LO DICE]: the project cap reserves before each digest
	// call (B-65, as the readers'); the round cap cuts the readers, never
	// the digest, so a round gets its digest while the project has budget.
	over, err := r.digestOverCap(ctx, projectID, 0)
	if err != nil {
		return err
	}
	if over {
		status = DigestProjectCap
	} else {
		var input []json.RawMessage
		instanceID, input, err = r.digestInput(ctx, projectID, digestMessage(round.Label, body, findings, labeler, titles))
		if err != nil {
			return err
		}
		if out, status, sent, err = r.callDigest(ctx, projectID, body.TurnID, instanceID, input, len(findings),
			body.Expectation != ""); err != nil {
			return err
		}
	}
	body.DigestStatus = status
	if status == DigestDone {
		d := out
		body.Digest = &d
		body.ReducedDoubt = out.ReducedDoubt
	}

	err = r.db.Write(ctx, projectID, func(tx *store.Tx) error {
		if sent {
			// B-62: what the principal received in the digest.
			seen := map[string]bool{}
			for _, f := range findings {
				if f.SourceID == "" || seen[f.SourceID] {
					continue
				}
				seen[f.SourceID] = true
				src, err := tx.Object(f.SourceID)
				if err != nil {
					return err
				}
				if err := tx.RecordDelivery(store.Delivery{InstanceID: instanceID, Via: store.ViaDigest,
					ObjectID: src.ID, Version: src.CurrentVersion}); err != nil {
					return err
				}
			}
		}
		var listed []DigestContradiction
		if status == DigestDone {
			listed = out.Contradictions
		}
		conflicts, err := openConflicts(ctx, tx, projectID, round.ID, body, findings, listed)
		if err != nil {
			return err
		}
		ignored, err := openReaderContradictions(ctx, tx, projectID, round.ID, body, findings, listed)
		if err != nil {
			return err
		}
		body.Conflicts = append(conflicts, ignored...)
		data := map[string]any{"to": "closed", "digest": status}
		if body.ReducedDoubt != "" {
			data["reduced_doubt"] = body.ReducedDoubt
		}
		_, err = tx.UpdateObject(round.ID, round.CurrentVersion, "closed", body, store.EventInput{
			Type: store.EventStateChanged, Actor: "app", TurnID: body.TurnID, Data: data,
		})
		return err
	})
	if err != nil {
		return err
	}
	if status != DigestDone {
		return nil
	}
	// Flujos §1 and §2: the digest is the end of a step and goes through
	// the checks, outside its transaction, in one checker call.
	change := digestChange(round.ID, body.TurnID, findings, out)
	if !change.HasWork() {
		return nil
	}
	return r.checker.Check(ctx, r.db, projectID, body.TurnID, change)
}

// digestChange is the digest's change for the checks (2.3): as new
// passages, those of the findings the digest uses, each once, with its
// kind, its source and the reader's note; the probes are the usual ones.
// Its section is the round.
func digestChange(roundID, turnID string, findings []DigestedFinding, out DigestOutput) checks.Change {
	change := checks.Change{Kind: checks.AgentStep, Actor: "agent", TurnID: turnID}
	used := map[int]bool{}
	for _, u := range out.Findings {
		if used[u.Finding] || u.Finding < 1 || u.Finding > len(findings) {
			continue
		}
		used[u.Finding] = true
		f := findings[u.Finding-1]
		change.Passages = append(change.Passages, checks.Passage{Section: roundID, Text: f.Passage, Kind: f.Kind,
			Source: f.SourceID, Note: f.Note})
	}
	if len(change.Passages) > 0 {
		change.Sections = []string{roundID}
	}
	return change
}

// openConflicts routes the digest's contradictions (R-9, B-66, flujos
// §2): between findings of different sources, a «por aclarar»
// source_conflict with the two passages as its options, blocking the
// sections that use either source; between readers on the same source,
// only when it touches a critical decision (a section that uses the
// source has a critical decision used in it), blocking those sections.
// Otherwise it is only shown in the digest. Each opens through the one
// creation path of 2.4 (checks.Open).
// [EL DISEÑO NO LO DICE]: a finding with no stored source (a reference
// the registry did not answer) counts as a different source; «toca algo
// crítico» is read as a critical decision used in a section that uses
// the source; each option's contrast is the passage's source title.
func openConflicts(ctx context.Context, tx *store.Tx, projectID, roundID string, body RoundBody, findings []DigestedFinding,
	contradictions []DigestContradiction) ([]RoundConflict, error) {
	out := []RoundConflict{}
	var critical map[string][]graph.Reason
	for _, c := range contradictions {
		if c.FindingA < 1 || c.FindingA > len(findings) || c.FindingB < 1 || c.FindingB > len(findings) {
			continue
		}
		fa, fb := findings[c.FindingA-1], findings[c.FindingB-1]
		rc := RoundConflict{FindingA: c.FindingA, FindingB: c.FindingB, About: c.About,
			SameSource: fa.SourceID != "" && fa.SourceID == fb.SourceID}
		kind := OpenQuestionSourceConflict
		var sections []string
		var err error
		if rc.SameSource {
			kind = OpenQuestionReaderContradiction
			if critical == nil {
				if critical, err = graph.CriticalAll(ctx, tx, projectID); err != nil {
					return nil, err
				}
			}
			if sections, err = sectionsWithCritical(ctx, tx, projectID, fa.SourceID, critical); err != nil {
				return nil, err
			}
			if len(sections) == 0 {
				out = append(out, rc)
				continue
			}
		} else {
			var sources []string
			for _, id := range []string{fa.SourceID, fb.SourceID} {
				if id != "" {
					sources = append(sources, id)
				}
			}
			if sections, err = checks.SectionsUsing(ctx, tx, projectID, sources); err != nil {
				return nil, err
			}
		}
		option := func(f DigestedFinding) (map[string]any, error) {
			contrast := ""
			if f.SourceID != "" {
				src, err := tx.Object(f.SourceID)
				if err != nil {
					return nil, err
				}
				contrast = graph.SourceTitle(src)
			}
			return map[string]any{"option": f.Passage, "contrast": contrast, "consequence": "", "risk": "",
				"source_id": f.SourceID}, nil
		}
		oa, err := option(fa)
		if err != nil {
			return nil, err
		}
		ob, err := option(fb)
		if err != nil {
			return nil, err
		}
		oq, err := checks.Open(ctx, tx, checks.OpenQuestion{Question: c.About,
			Options: []map[string]any{oa, ob}, DependsOn: sections,
			Extra: map[string]any{"origin": "digest", "kind": kind, "round_id": roundID,
				"new_passage": fa.Passage, "clashing_passage": fb.Passage, "undeclared_by_agent": false},
			Event: store.EventInput{Type: store.EventStateChanged, Actor: "app", TurnID: body.TurnID,
				Data: map[string]any{"via": "digest", "kind": kind}}})
		if err != nil {
			return nil, err
		}
		rc.OpenQuestionID = oq.ID
		out = append(out, rc)
	}
	return out, nil
}

// sectionsWithCritical are the sections that use source and have a
// critical decision used in them.
func sectionsWithCritical(ctx context.Context, tx *store.Tx, projectID, source string,
	critical map[string][]graph.Reason) ([]string, error) {
	using, err := checks.SectionsUsing(ctx, tx, projectID, []string{source})
	if err != nil {
		return nil, err
	}
	out := []string{}
	for _, sec := range using {
		edges, err := tx.EdgesTo(ctx, projectID, sec)
		if err != nil {
			return nil, err
		}
		for _, e := range edges {
			if e.Type == store.EdgeUsedIn && len(critical[e.FromID]) > 0 {
				out = append(out, sec)
				break
			}
		}
	}
	return out, nil
}

// openReaderContradictions routes the reader findings of kind
// contradiction the digest does not list in its contradictions (flujos
// §2: «el principal no las resuelve en silencio … si tocan algo
// crítico, a por aclarar»): the card always shows them (RoundView), and
// one that touches a critical decision (a section that uses its source
// has a critical decision used in it) opens a «por aclarar» blocking
// those sections. It applies also when the digest failed. Each is kept
// as a RoundConflict with FindingB 0.
// [EL DISEÑO NO LO DICE]: a single reader's contradiction is not
// «entre lectores»; it follows the same rule as one (shown always, «por
// aclarar» only when critical). Its question is the reader's note (or
// the passage), its one option the passage, with its source's title as
// contrast.
func openReaderContradictions(ctx context.Context, tx *store.Tx, projectID, roundID string, body RoundBody,
	findings []DigestedFinding, listed []DigestContradiction) ([]RoundConflict, error) {
	inListed := map[int]bool{}
	for _, c := range listed {
		inListed[c.FindingA], inListed[c.FindingB] = true, true
	}
	out := []RoundConflict{}
	var critical map[string][]graph.Reason
	for _, f := range findings {
		if f.Kind != "contradiction" || inListed[f.N] {
			continue
		}
		rc := RoundConflict{FindingA: f.N, About: f.Note, SameSource: true}
		if f.SourceID == "" {
			out = append(out, rc)
			continue
		}
		if critical == nil {
			var err error
			if critical, err = graph.CriticalAll(ctx, tx, projectID); err != nil {
				return nil, err
			}
		}
		sections, err := sectionsWithCritical(ctx, tx, projectID, f.SourceID, critical)
		if err != nil {
			return nil, err
		}
		if len(sections) == 0 {
			out = append(out, rc)
			continue
		}
		src, err := tx.Object(f.SourceID)
		if err != nil {
			return nil, err
		}
		question := f.Note
		if strings.TrimSpace(question) == "" {
			question = f.Passage
		}
		oq, err := checks.Open(ctx, tx, checks.OpenQuestion{Question: question,
			Options: []map[string]any{{"option": f.Passage, "contrast": graph.SourceTitle(src), "consequence": "",
				"risk": "", "source_id": f.SourceID}},
			DependsOn: sections,
			Extra: map[string]any{"origin": "digest", "kind": OpenQuestionReaderContradiction, "round_id": roundID,
				"new_passage": f.Passage, "undeclared_by_agent": false},
			Event: store.EventInput{Type: store.EventStateChanged, Actor: "app", TurnID: body.TurnID,
				Data: map[string]any{"via": "digest", "kind": OpenQuestionReaderContradiction}}})
		if err != nil {
			return nil, err
		}
		rc.OpenQuestionID = oq.ID
		out = append(out, rc)
	}
	return out, nil
}
