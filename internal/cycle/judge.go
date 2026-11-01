package cycle

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/LaurenteEber/tupaqnin/internal/checks"
	"github.com/LaurenteEber/tupaqnin/internal/graph"
	"github.com/LaurenteEber/tupaqnin/internal/provider"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// Verifying with the judge (backlog-r3 3.6; fichas §2 «Verificar» and §7;
// bloque-1 §4, the bounded correction cycle; K-3, K-4, B-67). A section
// that closed with a risk, or that you ask to audit, gets one judge round:
// its flaws go to the next step of the principal, the one correction
// batch, less those you discarded before it starts; then one recheck.
// What stays open opens a «por aclarar» per flaw; nothing the judge finds
// in the recheck opens anything. The judge writes nothing.

// The judge's verification instructions (a draft Eber reviews) and its
// fixed output, a copy of salida-juez-verificacion.schema.json.
//
//go:embed prompts/judge-verify.en.md
var judgeVerifyInstructions string

//go:embed schema/judge_verify_output.schema.json
var judgeVerifySchema []byte

var (
	// ErrNotSection is a label that is not a section of the project.
	ErrNotSection = errors.New("cycle: not a section")
	// ErrJudgeRoundOpen: the section's judge round has not ended (one
	// batch and one recheck at a time).
	ErrJudgeRoundOpen = errors.New("cycle: the section has a judge round open")
	// ErrFlawNotFound: the round has no flaw with that number.
	ErrFlawNotFound = errors.New("cycle: flaw not found")
	// ErrFlawTooLate: the batch started, or the round closed (K-4: what
	// you discard counts until the batch starts).
	ErrFlawTooLate = errors.New("cycle: the correction batch already started")
	// ErrFlawDiscardedTwice: that flaw was already discarded.
	ErrFlawDiscardedTwice = errors.New("cycle: the flaw was already discarded")
)

const (
	// The judge's status in its round.
	JudgeDone = "done"
	// JudgeFailed is also the result of its output_retry after the second
	// failure (fichas §7): visible, blocking nothing.
	// [EL DISEÑO NO LO DICE]: the name; not agent_pending, which would
	// block the signoff (B-15), as DefenseFailed.
	JudgeFailed     = "judge_failed"
	JudgeProjectCap = "project_cap"

	// Why the round entered (fichas §8.3: a listed risk, or your audit).
	// [EL DISEÑO NO LO DICE]: the two values' names.
	JudgeReasonRisk  = "risk"
	JudgeReasonAudit = "audit"

	// OpenQuestionJudgeFlaw: what stays open after the recheck (estados
	// §4). [EL DISEÑO NO LO DICE]: its kind's name.
	OpenQuestionJudgeFlaw = "judge_flaw"
	// The two options the app gives it (B-67), as codes the page
	// translates (every on-screen string from i18n).
	FlawOptionRedoPassage = "redo_passage"
	FlawOptionAcceptRisk  = "accept_risk"

	// The verdicts of the recheck (glossary).
	FlawResolved  = "resolved"
	FlawStillOpen = "still_open"
)

// JudgeFlaw is one flaw in its round's body: N is its number in the list
// the app gives, stable through discards; Section its label, as the judge
// named it; Verdict and RecheckPassage are the recheck's, and
// OpenQuestionID the «por aclarar» a still_open opened.
type JudgeFlaw struct {
	N              int    `json:"n"`
	Section        string `json:"section"`
	Passage        string `json:"passage"`
	Risk           string `json:"risk"`
	Scenario       string `json:"scenario"`
	Discarded      bool   `json:"discarded,omitempty"`
	Verdict        string `json:"verdict,omitempty"`
	RecheckPassage string `json:"recheck_passage,omitempty"`
	OpenQuestionID string `json:"open_question_id,omitempty"`
}

// judgeOutput is salida-juez-verificacion as the judge wrote it.
type judgeOutput struct {
	Flaws   []judgeOutFlaw    `json:"flaws"`
	Recheck []judgeOutRecheck `json:"recheck"`
}

type judgeOutFlaw struct {
	Section  string `json:"section"`
	Passage  string `json:"passage"`
	Risk     string `json:"risk"`
	Scenario string `json:"scenario"`
}

type judgeOutRecheck struct {
	Flaw    int     `json:"flaw"`
	Verdict string  `json:"verdict"`
	Passage *string `json:"passage"`
}

var (
	judgeSchemaOnce sync.Once
	judgeSchemaC    *jsonschema.Schema
	judgeSendable   json.RawMessage
	judgeSchemaErr  error
)

func judgeSchemas() (*jsonschema.Schema, json.RawMessage, error) {
	judgeSchemaOnce.Do(func() {
		doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(judgeVerifySchema))
		if err != nil {
			judgeSchemaErr = fmt.Errorf("cycle: parse judge schema: %w", err)
			return
		}
		c := jsonschema.NewCompiler()
		const loc = "https://tupaqnin.invalid/judge_verify_output.schema.json"
		if err := c.AddResource(loc, doc); err != nil {
			judgeSchemaErr = fmt.Errorf("cycle: add judge schema: %w", err)
			return
		}
		if judgeSchemaC, err = c.Compile(loc); err != nil {
			judgeSchemaErr = fmt.Errorf("cycle: compile judge schema: %w", err)
			return
		}
		judgeSendable, judgeSchemaErr = provider.TransportSchema(judgeVerifySchema)
	})
	return judgeSchemaC, judgeSendable, judgeSchemaErr
}

// validateJudgeShape checks an output against salida-juez-verificacion.
func validateJudgeShape(raw string) ([]string, judgeOutput, error) {
	var out judgeOutput
	sch, _, err := judgeSchemas()
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
			return nil, out, fmt.Errorf("cycle: validate judge output: %w", err)
		}
		return flattenErrors(verr), out, nil
	}
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, out, fmt.Errorf("cycle: parse judge output: %w", err)
	}
	return nil, out, nil
}

// validateFirstCall is the first call's fixed shape: the schema, recheck
// empty, and what the schema cannot say: every flaw on the section in
// question (rule 4) with its passage literal in its current text (T-4).
// [EL DISEÑO NO LO DICE]: a passage that is not literal in the section is
// an error of shape (retried): «pasaje literal» (bloque-1 §4), and only a
// literal one can be underlined.
func validateFirstCall(raw, section, text string) ([]string, judgeOutput, error) {
	errs, out, err := validateJudgeShape(raw)
	if err != nil || len(errs) > 0 {
		return errs, out, err
	}
	if len(out.Recheck) > 0 {
		errs = append(errs, "/recheck: it is empty in the first call")
	}
	compared := graph.Compare(text)
	for i, f := range out.Flaws {
		if f.Section != section {
			errs = append(errs, fmt.Sprintf("/flaws/%d/section: %s is not the section in question, %s", i, f.Section, section))
		}
		if len(compared.Find(f.Passage)) == 0 {
			errs = append(errs, fmt.Sprintf("/flaws/%d/passage: it is not literal in the section's current text", i))
		}
	}
	return errs, out, nil
}

// validateRecheck is the recheck's fixed shape: the schema, one entry for
// each listed flaw, by its number, and on still_open a passage literal in
// the current text. What it brings in flaws is not an error: it opens
// nothing (bloque-1 §4), and the round keeps it.
func validateRecheck(raw, text string, listed []int) ([]string, judgeOutput, error) {
	errs, out, err := validateJudgeShape(raw)
	if err != nil || len(errs) > 0 {
		return errs, out, err
	}
	compared := graph.Compare(text)
	given := map[int]bool{}
	for i, c := range out.Recheck {
		switch {
		case !slices.Contains(listed, c.Flaw):
			errs = append(errs, fmt.Sprintf("/recheck/%d/flaw: %d is not in the list of flaws", i, c.Flaw))
		case given[c.Flaw]:
			errs = append(errs, fmt.Sprintf("/recheck/%d/flaw: %d comes twice", i, c.Flaw))
		}
		given[c.Flaw] = true
		switch {
		case c.Verdict == FlawStillOpen && c.Passage == nil:
			errs = append(errs, fmt.Sprintf("/recheck/%d/passage: still_open needs the passage where it stays", i))
		case c.Verdict == FlawStillOpen && len(compared.Find(*c.Passage)) == 0:
			errs = append(errs, fmt.Sprintf("/recheck/%d/passage: it is not literal in the section's current text", i))
		case c.Verdict == FlawResolved && c.Passage != nil:
			errs = append(errs, fmt.Sprintf("/recheck/%d/passage: resolved has a null passage", i))
		}
	}
	for _, n := range listed {
		if !given[n] {
			errs = append(errs, fmt.Sprintf("/recheck: the flaw %d has no entry", n))
		}
	}
	return errs, out, nil
}

// callJudge makes the judge's call: Spark 1.3 high, a clean context, no
// principal instance (3.2, B-62), the fixed shape and one retry. Each call
// reserves before it starts (B-65, as the digest's). Each output that
// fails, or a failed call, appends output_retry of turn kind judge_verify:
// retry after the first, JudgeFailed after the second. It returns the
// status. No transaction is open while the judge answers.
func (r *Runner) callJudge(ctx context.Context, projectID, turnID, brief string, recheck []int,
	validate func(string) ([]string, judgeOutput, error)) (judgeOutput, string, error) {
	_, schema, err := judgeSchemas()
	if err != nil {
		return judgeOutput{}, "", err
	}
	input := []json.RawMessage{userItem(brief)}
	dearest := 0.0
	for attempt := 1; attempt <= 2; attempt++ {
		over, err := r.digestOverCap(ctx, projectID, dearest)
		if err != nil {
			return judgeOutput{}, "", err
		}
		if over {
			return judgeOutput{}, JudgeProjectCap, nil
		}
		resp, err := r.prov.Respond(ctx, provider.Request{ProjectID: projectID, TurnID: turnID,
			Role: provider.RoleJudge, TurnKind: provider.TurnJudgeVerify,
			Model: provider.JudgeVerifyModel, Effort: provider.JudgeVerifyEffort,
			Instructions: judgeVerifyInstructions, Input: input, Schema: schema, SchemaStrict: true,
			JudgeRecheck: recheck}, nil)
		var errs []string
		var out judgeOutput
		if err != nil {
			if ctx.Err() != nil {
				return judgeOutput{}, "", ctx.Err()
			}
			// The provider's error body is never kept (T-10).
			errs = []string{"provider_error"}
		} else {
			dearest = max(dearest, resp.Usage.CostUSD)
			if errs, out, err = validate(resp.Text); err != nil {
				return judgeOutput{}, "", err
			}
		}
		if len(errs) == 0 {
			return out, JudgeDone, nil
		}
		data := map[string]any{"turn_kind": string(provider.TurnJudgeVerify), "attempt": attempt, "errors": errs,
			"result": "retry"}
		if attempt == 2 {
			data["result"] = JudgeFailed
		}
		if err := r.gate.append(ctx, projectID, store.EventInput{Type: store.EventOutputRetry, Actor: "agent",
			AgentRole:      string(provider.RoleJudge),
			ModelAndEffort: provider.JudgeVerifyModel + " " + provider.JudgeVerifyEffort,
			TurnID:         turnID, Data: data}); err != nil {
			return judgeOutput{}, "", err
		}
		if resp.RawItems != nil {
			input = append(append(append([]json.RawMessage{}, input...), resp.RawItems...), userItem(RetryMessage(errs)))
		}
	}
	return judgeOutput{}, JudgeFailed, nil
}

// roundBodyOf reads a stored round's body.
func roundBodyOf(round store.Object) (RoundBody, error) {
	var body RoundBody
	if err := json.Unmarshal(round.Body, &body); err != nil {
		return RoundBody{}, fmt.Errorf("cycle: round body: %w", err)
	}
	return body, nil
}

// sectionOf reads a section by label, or ErrNotSection.
func sectionOf(ctx context.Context, q store.Querier, projectID, label string) (store.Object, error) {
	sec, err := q.GetObject(ctx, projectID, label)
	if errors.Is(err, store.ErrNotFound) || (err == nil && sec.Type != store.ObjectSection) {
		return store.Object{}, ErrNotSection
	}
	return sec, err
}

// judgeRoundsOf are the project's judge rounds, in creation order.
func judgeRoundsOf(ctx context.Context, q store.Querier, projectID string) ([]store.Object, []RoundBody, error) {
	rounds, err := q.ListObjects(ctx, projectID, store.ObjectRound)
	if err != nil {
		return nil, nil, err
	}
	var objs []store.Object
	var bodies []RoundBody
	for _, round := range rounds {
		body, err := roundBodyOf(round)
		if err != nil {
			return nil, nil, err
		}
		if body.Kind == "judge" {
			objs, bodies = append(objs, round), append(bodies, body)
		}
	}
	return objs, bodies, nil
}

// ShouldVerify is B-67 (a): a step left the section without the agent's
// pendings and, since its last verification, some step closed with a
// listed risk. A step's closing_risk counts for the sections its new_text
// touched; «desde su última verificación» counts from the version the last
// judge round of the section keeps in its body; a round that could not
// verify does not count. A section whose round has not ended needs no
// other. It runs before ShouldStop (2.5).
func (g *Gate) ShouldVerify(ctx context.Context, projectID, section string) (bool, error) {
	risks, err := g.risksToVerify(ctx, projectID, section)
	return len(risks) > 0, err
}

// risksToVerify are the risks ShouldVerify reads, in order and once each.
// [EL DISEÑO NO LO DICE]: «sin pendientes del agente» is the rule of
// humanOnly: text, no open changes, and every pending an open card or an
// open «por aclarar».
func (g *Gate) risksToVerify(ctx context.Context, projectID, section string) ([]string, error) {
	sec, err := sectionOf(ctx, g.db, projectID, section)
	if errors.Is(err, ErrNotSection) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(bodyStr(bodyOf(sec), "text")) == "" || len(graph.OpenChanges(sec)) > 0 {
		return nil, nil
	}
	_, pendings, err := graph.SectionStatus(ctx, g.db, projectID, sec.Label)
	if err != nil {
		return nil, err
	}
	for _, p := range pendings {
		if p.Kind != graph.PendingOpenCard && p.Kind != graph.PendingOpenQuestion {
			return nil, nil
		}
	}
	rounds, bodies, err := judgeRoundsOf(ctx, g.db, projectID)
	if err != nil {
		return nil, err
	}
	since := 0
	for i, body := range bodies {
		if body.SectionID != sec.ID || body.JudgeStatus != JudgeDone {
			continue
		}
		if rounds[i].State == "open" {
			return nil, nil
		}
		since = max(since, body.SectionVersion)
	}
	events, err := g.db.EventsSince(ctx, projectID, 0)
	if err != nil {
		return nil, err
	}
	var risks []string
	version := 0
	for _, ev := range events {
		if ev.ObjectID == sec.ID && ev.NewVersion > 0 {
			version = ev.NewVersion
		}
		if ev.Type != store.EventStepEnded || version <= since {
			continue
		}
		var data struct {
			ClosingRisk string   `json:"closing_risk"`
			Written     []string `json:"written_sections"`
		}
		_ = json.Unmarshal(ev.Data, &data)
		if data.ClosingRisk == "" || data.ClosingRisk == "none" || !slices.Contains(data.Written, sec.Label) ||
			slices.Contains(risks, data.ClosingRisk) {
			continue
		}
		risks = append(risks, data.ClosingRisk)
	}
	return risks, nil
}

// createJudgeRound creates the section's judge round, open (estados §7):
// the app's when a risk brought it, yours when you asked for the audit.
func (r *Runner) createJudgeRound(ctx context.Context, projectID string, sec store.Object, reason string, risks []string,
	actor, turnID string) (store.Object, error) {
	if risks == nil {
		risks = []string{}
	}
	body := RoundBody{Kind: "judge", DelegationReason: reason, Parts: []RoundPart{}, References: []namedReference{},
		TurnID: turnID, SectionID: sec.ID, SectionVersion: sec.CurrentVersion, Risks: risks}
	var round store.Object
	err := r.db.Write(ctx, projectID, func(tx *store.Tx) error {
		var err error
		round, err = tx.CreateObject(store.ObjectRound, "open", body, store.EventInput{
			Type: store.EventStateChanged, Actor: actor, TurnID: turnID,
			Data: map[string]any{"to": "open", "kind": "judge", "reason": reason, "section": sec.Label, "risks": risks},
		})
		return err
	})
	return round, err
}

// markRunning shows the round running on its card while the judge answers.
func (r *Runner) markRunning(roundID string) (done func()) {
	r.mu.Lock()
	if r.rounds == nil {
		r.rounds = map[string]*roundRun{}
	}
	r.rounds[roundID] = newRoundRun(0)
	r.mu.Unlock()
	return func() {
		r.mu.Lock()
		delete(r.rounds, roundID)
		r.mu.Unlock()
	}
}

// judgeRound makes the round's first judge call and writes what it found
// in one write: the flaws, with the round left open for its batch; or,
// with no flaws or when the judge could not answer (fichas §7, as P-75 and
// the defense), closed. It returns the round's flaws. A cancelled call
// closes it as not verified. The caller holds the project's turn slot.
func (r *Runner) judgeRound(ctx context.Context, projectID string, round store.Object) (int, error) {
	defer r.markRunning(round.ID)()
	body, err := roundBodyOf(round)
	if err != nil {
		return 0, err
	}
	sec, err := r.db.GetObject(ctx, projectID, body.SectionID)
	if err != nil {
		return 0, err
	}
	text := bodyStr(bodyOf(sec), "text")
	brief, err := r.judgeBrief(ctx, projectID, sec, body, nil)
	if err != nil {
		return 0, err
	}
	out, status, callErr := r.callJudge(ctx, projectID, body.TurnID, brief, nil, func(raw string) ([]string, judgeOutput, error) {
		return validateFirstCall(raw, sec.Label, text)
	})
	writeCtx := ctx
	if callErr != nil {
		if ctx.Err() == nil {
			return 0, callErr
		}
		// [EL DISEÑO NO LO DICE]: a judge cut by the app's shutdown leaves
		// its round closed as not verified, not open forever.
		status, writeCtx = JudgeFailed, context.Background()
	}
	body.JudgeStatus, body.SectionVersion = status, sec.CurrentVersion
	if status == JudgeDone {
		for i, f := range out.Flaws {
			body.Flaws = append(body.Flaws, JudgeFlaw{N: i + 1, Section: f.Section, Passage: f.Passage, Risk: f.Risk,
				Scenario: f.Scenario})
		}
	}
	state := "closed"
	if len(body.Flaws) > 0 {
		state = "open"
	}
	err = r.db.Write(writeCtx, projectID, func(tx *store.Tx) error {
		_, err := tx.UpdateObject(round.ID, round.CurrentVersion, state, body, store.EventInput{
			Type: store.EventStateChanged, Actor: "app", TurnID: body.TurnID,
			Data: map[string]any{"to": state, "judge": status, "flaws": len(body.Flaws)},
		})
		return err
	})
	if err != nil {
		return 0, err
	}
	return len(body.Flaws), callErr
}

// RunJudgeVerify creates a judge round on a section and makes its first
// call, synchronously; the micro-test of the judge records with it. The
// caller holds the project's turn slot. It returns the round's label.
func (r *Runner) RunJudgeVerify(ctx context.Context, projectID, section, reason string, risks []string) (string, error) {
	sec, err := sectionOf(ctx, r.db, projectID, section)
	if err != nil {
		return "", err
	}
	turnID, err := newTurnID()
	if err != nil {
		return "", err
	}
	round, err := r.createJudgeRound(ctx, projectID, sec, reason, risks, "app", turnID)
	if err != nil {
		return "", err
	}
	_, err = r.judgeRound(ctx, projectID, round)
	return round.Label, err
}

// verifyTouched is B-67 (a) in the chain, before ShouldStop: each touched
// section, and each whose last judge round could not verify (the stop
// report reset what is touched), gets its judge round if ShouldVerify asks
// for it. failed holds the sections whose judge could not verify in this
// chain: the next chain is their «siguiente oportunidad». It reports
// whether a round has flaws waiting for their batch.
// [EL DISEÑO NO LO DICE]: «siguiente oportunidad» is the next chain, not
// the next step: otherwise a judge that keeps failing is called twice per
// step, up to 12 steps. A chain whose step did no work counts: the section
// may be the last one, and nothing else would verify it. A section that
// already could not verify and is still over the project's cap gets no
// other round (no call would go out).
func (r *Runner) verifyTouched(ctx context.Context, projectID, publishTurn string, failed map[string]bool) (bool, error) {
	touched, err := graph.TouchedSinceStop(ctx, r.db, projectID)
	if err != nil {
		return false, err
	}
	notVerified, err := JudgeNotVerified(ctx, r.db, projectID)
	if err != nil {
		return false, err
	}
	for _, label := range notVerified {
		if !slices.Contains(touched, label) {
			touched = append(touched, label)
		}
	}
	over, err := r.digestOverCap(ctx, projectID, 0)
	if err != nil {
		return false, err
	}
	waiting := false
	for _, label := range touched {
		if failed[label] || (over && slices.Contains(notVerified, label)) {
			continue
		}
		risks, err := r.gate.risksToVerify(ctx, projectID, label)
		if err != nil {
			return false, err
		}
		if len(risks) == 0 {
			continue
		}
		sec, err := r.db.GetObject(ctx, projectID, label)
		if err != nil {
			return false, err
		}
		turnID, err := newTurnID()
		if err != nil {
			return false, err
		}
		round, err := r.createJudgeRound(ctx, projectID, sec, JudgeReasonRisk, risks, "app", turnID)
		if err != nil {
			return false, err
		}
		r.publishRound(projectID, publishTurn, round.Label)
		n, err := r.judgeRound(ctx, projectID, round)
		if err != nil {
			return false, err
		}
		waiting = waiting || n > 0
		if done, err := r.db.GetObject(ctx, projectID, round.ID); err == nil {
			if b, err := roundBodyOf(done); err == nil && b.JudgeStatus != JudgeDone {
				failed[label] = true
			}
		}
	}
	return waiting, nil
}

// publishRound tells the page a round started, so its card shows; on a
// judge round the card carries the triage line «Verifico con el juez:
// riesgo …» (K-3), which does not wait for your yes.
func (r *Runner) publishRound(projectID, turnID, label string) {
	if fp, ok := r.pub.(FieldPublisher); ok && turnID != "" {
		fp.PublishFieldDelta(projectID, turnID, Segment{Field: "round", Text: label, Start: true})
	}
}

// JudgeBatchMessage is the fixed closing of a batch step without a message
// of the person, in place of ContinueMessage.
const JudgeBatchMessage = "Correct what still fails in this step; new text waits for the next one."

// startJudgeBatch starts the correction batch (K-4, B-67): the next step
// of the principal carries in its task each open judge round's flaws you
// did not discard. From here on a discard no longer counts; the write and
// a discard's are serialized. A step that does not apply gets them again:
// the batch stays until its step applied. It returns the task's text, ""
// when no round waits.
// [EL DISEÑO NO LO DICE]: the batch's text; a round left waiting by a cut
// of the chain has its batch in the next chain.
func (r *Runner) startJudgeBatch(ctx context.Context, projectID string) (string, error) {
	rounds, bodies, err := judgeRoundsOf(ctx, r.db, projectID)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	for i, round := range rounds {
		if round.State != "open" || bodies[i].JudgeStatus != JudgeDone {
			continue
		}
		var body RoundBody
		var sectionLabel string
		live := false
		err := r.db.Write(ctx, projectID, func(tx *store.Tx) error {
			fresh, err := tx.Object(round.ID)
			if err != nil {
				return err
			}
			if body, err = roundBodyOf(fresh); err != nil {
				return err
			}
			if fresh.State != "open" {
				return nil
			}
			live = true
			sec, err := tx.Object(body.SectionID)
			if err != nil {
				return err
			}
			sectionLabel = sec.Label
			if body.BatchStarted {
				return nil
			}
			body.BatchStarted = true
			_, err = tx.UpdateObject(fresh.ID, fresh.CurrentVersion, "open", body, store.EventInput{
				Type: store.EventStateChanged, Actor: "app", TurnID: body.TurnID,
				Data: map[string]any{"to": "open", "batch_started": true},
			})
			return err
		})
		if err != nil {
			return "", err
		}
		if !live {
			continue
		}
		fmt.Fprintf(&b, "The judge found flaws in %s. Correct them now: this is the only correction batch, and the judge "+
			"rechecks once. Its suggestions only inform.\n", sectionLabel)
		for _, f := range body.Flaws {
			if f.Discarded {
				continue
			}
			fmt.Fprintf(&b, "Flaw %d · risk %s. Passage:\n%s\nScenario:\n%s\n", f.N, f.Risk,
				graph.Delimit(graph.DataSection, f.Passage), graph.Delimit(graph.DataPassage, f.Scenario))
		}
	}
	return strings.TrimSpace(b.String()), nil
}

// recheckJudgeRounds makes the one recheck of each round whose batch
// started, after the batch step applied.
func (r *Runner) recheckJudgeRounds(ctx context.Context, projectID string) error {
	rounds, bodies, err := judgeRoundsOf(ctx, r.db, projectID)
	if err != nil {
		return err
	}
	for i, round := range rounds {
		if round.State == "open" && bodies[i].BatchStarted {
			if err := r.RunJudgeRecheck(ctx, projectID, round.Label); err != nil {
				return err
			}
		}
	}
	return nil
}

// RunJudgeRecheck is the one recheck (bloque-1 §4): a call with the flaws
// not discarded and the corrected text; each still_open opens a «por
// aclarar» with B-67's two options, blocking the section; a new finding
// opens nothing and stays in the body; the round closes keeping the
// version the judge read, so the batch's own risk brings no other round. A
// round already closed is left as it is: there is never a second one. A
// recheck that fails twice closes the round as not verified (fichas §7):
// its flaws stay visible, they open nothing, and it does not count as the
// last verification. The caller holds the project's turn slot.
func (r *Runner) RunJudgeRecheck(ctx context.Context, projectID, roundLabel string) error {
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
	defer r.markRunning(round.ID)()
	body, err := roundBodyOf(round)
	if err != nil {
		return err
	}
	sec, err := r.db.GetObject(ctx, projectID, body.SectionID)
	if err != nil {
		return err
	}
	text := bodyStr(bodyOf(sec), "text")
	var keep []JudgeFlaw
	var listed []int
	for _, f := range body.Flaws {
		if !f.Discarded {
			keep = append(keep, f)
			listed = append(listed, f.N)
		}
	}
	brief, err := r.judgeBrief(ctx, projectID, sec, body, keep)
	if err != nil {
		return err
	}
	out, status, callErr := r.callJudge(ctx, projectID, body.TurnID, brief, listed, func(raw string) ([]string, judgeOutput, error) {
		return validateRecheck(raw, text, listed)
	})
	writeCtx := ctx
	if callErr != nil {
		if ctx.Err() == nil {
			return callErr
		}
		status, writeCtx = JudgeFailed, context.Background()
	}
	body.JudgeStatus, body.SectionVersion = status, sec.CurrentVersion
	err = r.db.Write(writeCtx, projectID, func(tx *store.Tx) error {
		if status == JudgeDone {
			for _, c := range out.Recheck {
				for i := range body.Flaws {
					f := &body.Flaws[i]
					if f.N != c.Flaw {
						continue
					}
					f.Verdict = c.Verdict
					if c.Verdict != FlawStillOpen {
						continue
					}
					f.RecheckPassage = *c.Passage
					oq, err := openFlawQuestion(ctx, tx, round.ID, sec.ID, body.TurnID, *f)
					if err != nil {
						return err
					}
					f.OpenQuestionID = oq.ID
				}
			}
			for i, f := range out.Flaws {
				body.LateFlaws = append(body.LateFlaws, JudgeFlaw{N: i + 1, Section: f.Section, Passage: f.Passage,
					Risk: f.Risk, Scenario: f.Scenario})
			}
		}
		_, err := tx.UpdateObject(round.ID, round.CurrentVersion, "closed", body, store.EventInput{
			Type: store.EventStateChanged, Actor: "app", TurnID: body.TurnID,
			Data: map[string]any{"to": "closed", "judge": status, "recheck": true},
		})
		return err
	})
	if err != nil {
		return err
	}
	return callErr
}

// openFlawQuestion opens the «por aclarar» of a flaw that stays open
// (B-67), through the one creation path of 2.4 (checks.Open): the judge's
// scenario as its question, the passage where it stays, and the app's two
// options as codes.
// [EL DISEÑO NO LO DICE]: its question is the scenario, literal; a resolved
// one reaches the principal in what changed (memory), with your «porque»
// as the indication.
func openFlawQuestion(ctx context.Context, tx *store.Tx, roundID, sectionID, turnID string, f JudgeFlaw) (store.Object, error) {
	return checks.Open(ctx, tx, checks.OpenQuestion{Question: f.Scenario,
		Options: []map[string]any{
			{"option": "", "code": FlawOptionRedoPassage, "contrast": "", "consequence": "", "risk": ""},
			{"option": "", "code": FlawOptionAcceptRisk, "contrast": "", "consequence": "", "risk": ""}},
		DependsOn: []string{sectionID},
		Extra: map[string]any{"origin": "judge", "kind": OpenQuestionJudgeFlaw, "round_id": roundID,
			"new_passage": f.RecheckPassage, "flaw_risk": f.Risk, "undeclared_by_agent": false},
		Event: store.EventInput{Type: store.EventStateChanged, Actor: "app", TurnID: turnID,
			Data: map[string]any{"via": "judge", "kind": OpenQuestionJudgeFlaw}}})
}

// DiscardFlaw is «No aplica» on a flaw (B-67, K-4): flaw_discarded, in one
// write conditioned on the round's version; only before the batch starts.
// With no flaw left, the round closes: no batch and no recheck.
// [EL DISEÑO NO LO DICE]: closing it then; it asks no «porque».
func (r *Runner) DiscardFlaw(ctx context.Context, projectID, roundLabel string, n int) error {
	return r.db.Write(ctx, projectID, func(tx *store.Tx) error {
		round, err := tx.GetObject(ctx, projectID, roundLabel)
		if errors.Is(err, store.ErrNotFound) || (err == nil && round.Type != store.ObjectRound) {
			return ErrNotRound
		}
		if err != nil {
			return err
		}
		body, err := roundBodyOf(round)
		if err != nil {
			return err
		}
		if body.Kind != "judge" {
			return ErrNotRound
		}
		found, left := false, 0
		for i := range body.Flaws {
			f := &body.Flaws[i]
			if f.N == n {
				found = true
				if f.Discarded {
					return firstOf(tooLate(round, body), ErrFlawDiscardedTwice)
				}
				if err := tooLate(round, body); err != nil {
					return err
				}
				f.Discarded = true
			}
			if !f.Discarded {
				left++
			}
		}
		if !found {
			return ErrFlawNotFound
		}
		updated, err := tx.UpdateObject(round.ID, round.CurrentVersion, round.State, body, store.EventInput{
			Type: store.EventFlawDiscarded, Actor: "human", TurnID: body.TurnID,
			Data: map[string]any{"round": round.Label, "flaw": n},
		})
		if err != nil || left > 0 {
			return err
		}
		_, err = tx.UpdateObject(round.ID, updated.CurrentVersion, "closed", body, store.EventInput{
			Type: store.EventStateChanged, Actor: "human", TurnID: body.TurnID,
			Data: map[string]any{"to": "closed", "all_flaws_discarded": true},
		})
		return err
	})
}

func tooLate(round store.Object, body RoundBody) error {
	if round.State != "open" || body.BatchStarted {
		return ErrFlawTooLate
	}
	return nil
}

func firstOf(errs ...error) error {
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}

// AuditSection is «Auditar sección» (B-67 b): your act calls the judge
// always, whatever the section's risks, and it runs in the background in
// the project's turn slot (P-20): ErrTurnRunning while one runs. With
// flaws, the chain continues with their batch and its recheck. It returns
// the round's label and the turn to follow it on.
// [EL DISEÑO NO LO DICE]: refused on a section without text (nothing to
// judge) and while its judge round has not ended; with no flaws the turn
// ends with no chain and no report (the card says it).
func (r *Runner) AuditSection(projectID, section string) (label, turnID string, err error) {
	ctx := context.Background()
	sec, err := sectionOf(ctx, r.db, projectID, section)
	if err != nil {
		return "", "", err
	}
	if !r.tryAcquireTurn(projectID) {
		return "", "", ErrTurnRunning
	}
	release := true
	defer func() {
		if release {
			r.releaseTurn(projectID)
		}
	}()
	if sec, err = r.db.GetObject(ctx, projectID, sec.ID); err != nil {
		return "", "", err
	}
	if strings.TrimSpace(bodyStr(bodyOf(sec), "text")) == "" {
		return "", "", ErrSectionEmpty
	}
	rounds, bodies, err := judgeRoundsOf(ctx, r.db, projectID)
	if err != nil {
		return "", "", err
	}
	for i, round := range rounds {
		if round.State == "open" && bodies[i].SectionID == sec.ID && bodies[i].JudgeStatus == JudgeDone {
			return "", "", ErrJudgeRoundOpen
		}
	}
	if turnID, err = newTurnID(); err != nil {
		return "", "", err
	}
	round, err := r.createJudgeRound(ctx, projectID, sec, JudgeReasonAudit, nil, "human", turnID)
	if err != nil {
		return "", "", err
	}
	if r.pub != nil {
		r.pub.BeginTurn(projectID, turnID)
	}
	release = false
	r.roundTurns.Add(1)
	go func() {
		defer r.roundTurns.Done()
		defer r.endTurn(context.Background(), projectID, turnID)
		n, err := r.judgeRound(r.ctx, projectID, round)
		if err != nil {
			if r.Logger != nil {
				r.Logger.Error("audit failed", "error", err.Error())
			}
			r.fail(projectID, turnID, "turn_failed")
			return
		}
		if n > 0 {
			// runChain publishes its step, and its own failure.
			if err := r.runChain(r.ctx, projectID, turnID, "", false); err != nil && r.Logger != nil {
				r.Logger.Error("audit failed", "error", err.Error())
			}
			return
		}
		if r.pub != nil {
			if raw, err := json.Marshal(map[string]any{"status": StatusAccepted, "round": round.Label}); err == nil {
				r.pub.PublishStep(projectID, turnID, raw)
			}
		}
	}()
	return round.Label, turnID, nil
}

// judgeBrief is the judge's brief, assembled by the app with the template
// of bloque-1 §4, in literal: request, question, doubt and reason, criteria
// with their cases, premise, current decisions, scope, sources with what
// for, evidence rules and the fixed return, plus the claims with risk: the
// section's current text and its citations with the passage of their
// source (fichas §7); on the recheck, the flaws not discarded. The judged
// text goes between the markers (P-83); labels only, never ids (T-2).
// [EL DISEÑO NO LO DICE]: the question, the doubt and the scope of a judge
// round (the principal does not ask for it): fixed text of the app.
func (r *Runner) judgeBrief(ctx context.Context, projectID string, sec store.Object, body RoundBody, recheck []JudgeFlaw) (string, error) {
	request, err := r.requestText(ctx, projectID)
	if err != nil {
		return "", err
	}
	labeler, err := graph.NewLabeler(ctx, r.db, projectID)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	line := func(format string, args ...any) { fmt.Fprintf(&b, format+"\n", args...) }
	line("# Judge brief: section %s", sec.Label)
	line("")
	line("## 1. Request of the human (literal)")
	line("«%s»", request)
	line("")
	line("## 2. Question, doubt and why it is delegated")
	line("Question: what in this section fails in front of whoever receives the document?")
	if body.DelegationReason == JudgeReasonAudit {
		line("Doubt it reduces: whether the section holds; the person asked for an audit.")
		line("Why it is delegated: the person asked for an audit.")
	} else {
		line("Doubt it reduces: whether the claims with risk hold.")
		line("Why it is delegated: a step closed with a risk of the list: %s.", strings.Join(body.Risks, ", "))
	}
	line("")
	line("## 3. Criteria, premise and decisions (literal)")
	if err := r.writeBriefContext(ctx, projectID, line); err != nil {
		return "", err
	}
	line("")
	line("## 4. Scope")
	line("Only section %s, in its current text. Do not judge the rest of the document.", sec.Label)
	line("")
	line("## 5. Sources")
	edges, err := r.db.EdgesFrom(ctx, projectID, sec.ID)
	if err != nil {
		return "", err
	}
	sources := 0
	for _, e := range edges {
		if e.Type != store.EdgeUses {
			continue
		}
		src, err := r.db.GetObject(ctx, projectID, e.ToID)
		if err != nil || src.Type != store.ObjectSource {
			continue
		}
		sources++
		line("%s, the section uses it. Title:", src.Label)
		line("%s", graph.Delimit(graph.DataSource, graph.SourceTitle(src)))
	}
	if sources == 0 {
		line("None: the section uses no source.")
	}
	line("")
	line("## 6. Evidence rules")
	line("Quote literally, with the source. Write [inferencia] when you reason without a text. Do not fill gaps. Never quote what you did not see.")
	line("")
	line("## 7. Return")
	line("Your fixed output only: flaw, literal passage, the risk it touches, scenario in steps. Suggestions only inform.")
	line("")
	line("## Claims with risk")
	line("Current text of section %s:", sec.Label)
	line("%s", graph.Delimit(graph.DataSection, bodyStr(bodyOf(sec), "text")))
	citations, err := r.db.ListObjects(ctx, projectID, store.ObjectCitation)
	if err != nil {
		return "", err
	}
	for _, c := range citations {
		cb := bodyOf(c)
		if bodyStr(cb, "section_id") != sec.ID {
			continue
		}
		source := "none: the app did not find its passage in a source"
		citEdges, err := r.db.EdgesFrom(ctx, projectID, c.ID)
		if err != nil {
			return "", err
		}
		for _, e := range citEdges {
			if e.Type == store.EdgeComesFrom {
				if l, err := labeler.ToLabel(e.ToID); err == nil {
					source = l
				}
			}
		}
		line("Citation, source %s. Claim:", source)
		line("%s", graph.Delimit(graph.DataPassage, bodyStr(cb, "anchor")))
		line("Passage of its source:")
		line("%s", graph.Delimit(graph.DataSource, bodyStr(cb, "passage")))
	}
	if recheck != nil {
		line("")
		line("## Flaws to recheck (the correction batch already ran; the text above is the corrected one)")
		for _, f := range recheck {
			line("%d. Risk %s. Passage:", f.N, f.Risk)
			line("%s", graph.Delimit(graph.DataPassage, f.Passage))
			line("Scenario:")
			line("%s", graph.Delimit(graph.DataPassage, f.Scenario))
		}
	}
	return b.String(), nil
}

// JudgeNotVerified are the sections whose last judge round could not
// verify, for the stop report (fichas §7).
func JudgeNotVerified(ctx context.Context, q store.Querier, projectID string) ([]string, error) {
	rounds, bodies, err := judgeRoundsOf(ctx, q, projectID)
	if err != nil {
		return nil, err
	}
	last := map[string]string{}
	var order []string
	for i := range rounds {
		id := bodies[i].SectionID
		if _, seen := last[id]; !seen {
			order = append(order, id)
		}
		last[id] = bodies[i].JudgeStatus
	}
	var out []string
	for _, id := range order {
		if last[id] == JudgeFailed || last[id] == JudgeProjectCap {
			sec, err := q.GetObject(ctx, projectID, id)
			if err != nil {
				return nil, err
			}
			out = append(out, sec.Label)
		}
	}
	return out, nil
}

// FlawView is one flaw as the card shows it: its number, the passage
// literal (rule 8), the risk, the scenario, what you did and what the
// recheck said, and the «por aclarar» it opened, by label.
type FlawView struct {
	N              int    `json:"n"`
	Passage        string `json:"passage"`
	Risk           string `json:"risk"`
	Scenario       string `json:"scenario"`
	Discarded      bool   `json:"discarded,omitempty"`
	Verdict        string `json:"verdict,omitempty"`
	RecheckPassage string `json:"recheck_passage,omitempty"`
	OpenQuestion   string `json:"open_question,omitempty"`
}

// judgeView fills a judge round's part of the card.
func (r *Runner) judgeView(ctx context.Context, projectID string, body RoundBody, labeler *graph.Labeler, v *RoundView) error {
	sec, err := r.db.GetObject(ctx, projectID, body.SectionID)
	if err != nil {
		return err
	}
	v.Section, v.SectionTitle = sec.Label, bodyStr(bodyOf(sec), "title")
	v.Reason, v.Risks, v.JudgeStatus, v.BatchStarted = body.DelegationReason, body.Risks, body.JudgeStatus, body.BatchStarted
	view := func(f JudgeFlaw) FlawView {
		fv := FlawView{N: f.N, Passage: f.Passage, Risk: f.Risk, Scenario: f.Scenario, Discarded: f.Discarded,
			Verdict: f.Verdict, RecheckPassage: f.RecheckPassage}
		if f.OpenQuestionID != "" {
			if l, err := labeler.ToLabel(f.OpenQuestionID); err == nil {
				fv.OpenQuestion = l
			}
		}
		return fv
	}
	for _, f := range body.Flaws {
		v.Flaws = append(v.Flaws, view(f))
	}
	for _, f := range body.LateFlaws {
		v.LateFlaws = append(v.LateFlaws, view(f))
	}
	return nil
}
