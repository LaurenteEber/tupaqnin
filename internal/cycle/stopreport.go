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

	"github.com/LaurenteEber/tupaqnin/internal/graph"
	"github.com/LaurenteEber/tupaqnin/internal/provider"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// The stop report's instructions (a draft Eber reviews) and its fixed
// output, a copy of salida-informe.schema.json (backlog-r2 2.5).
//
//go:embed prompts/stop-report.en.md
var stopReportInstructions string

//go:embed schema/stop_report_output.schema.json
var stopReportSchema []byte

// The stop reasons slice 2 adds to B-24's (B-37, fichas §2 Parada).
const (
	// StopSectionReady: a section the chain wrote since the last stop has
	// text and no pendings and no open changes.
	StopSectionReady StopReason = "section_ready"
	// StopUnseenDecisions: UnseenDecisionsToStop or more decisions of
	// the agent you have not seen (B-39).
	StopUnseenDecisions StopReason = "unseen_decisions"
	// StopHumanOnly: only what you decide is left (open cards and «por
	// aclarar»); the agent has nothing left to write or fix.
	StopHumanOnly StopReason = "human_only"
)

// UnseenDecisionsToStop is how many unseen decisions stop the chain
// (fichas §2, B-39: ≥ 5 [inicial]).
const UnseenDecisionsToStop = 5

// ShouldStop decides, after a step and its checks were applied, whether
// the chain stops for one of the reasons of the stop (fichas §2, B-37):
// unseen decisions, a section without pendings (only among touched,
// the sections the chain wrote since the last stop, and only those with
// text) or only what you decide left. The cuts of B-17 and the limit
// of B-37 are the runner's.
// [EL DISEÑO NO LO DICE]: the backlog's signature has no error; reading
// the graph can fail, so it returns one. The order of the reasons when
// several hold: unseen decisions, section ready, only yours.
//
// With «continúa hasta terminar» on, only what is yours stops it (B-83, fichas
// §2): the cap of the mode and no_work are the runner's.
func (g *Gate) ShouldStop(ctx context.Context, projectID string, touched []string) (StopReason, bool, error) {
	mode, err := graph.CurrentMode(ctx, g.db, projectID)
	if err != nil {
		return "", false, err
	}
	if !mode.On() {
		unseen, err := graph.UnseenDecisions(ctx, g.db, projectID)
		if err != nil {
			return "", false, err
		}
		if len(unseen) >= UnseenDecisionsToStop {
			return StopUnseenDecisions, true, nil
		}
		clear, err := g.clearSection(ctx, projectID, touched)
		if err != nil {
			return "", false, err
		}
		if clear != "" {
			return StopSectionReady, true, nil
		}
	}
	only, err := g.humanOnly(ctx, projectID)
	if err != nil {
		return "", false, err
	}
	if only {
		return StopHumanOnly, true, nil
	}
	return "", false, nil
}

// clearSection is the last touched section that could be signed now
// (graph.SectionClear), or "".
func (g *Gate) clearSection(ctx context.Context, projectID string, touched []string) (string, error) {
	for i := len(touched) - 1; i >= 0; i-- {
		ok, err := graph.SectionClear(ctx, g.db, projectID, touched[i])
		if errors.Is(err, store.ErrNotFound) {
			continue
		}
		if err != nil {
			return "", err
		}
		if ok {
			return touched[i], nil
		}
	}
	return "", nil
}

// humanOnly reports «solo queda lo tuyo»: the project has sections,
// every one has text and no open changes, and every pending left is an
// open card or an open «por aclarar».
// [EL DISEÑO NO LO DICE]: a section without text is still the agent's
// to write, and an open change the agent's to handle (A-1).
func (g *Gate) humanOnly(ctx context.Context, projectID string) (bool, error) {
	sections, err := graph.OrderedSections(ctx, g.db, projectID)
	if err != nil || len(sections) == 0 {
		return false, err
	}
	for _, sec := range sections {
		var b struct {
			Text string `json:"text"`
		}
		_ = json.Unmarshal(sec.Body, &b)
		if strings.TrimSpace(b.Text) == "" || len(graph.OpenChanges(sec)) > 0 {
			return false, nil
		}
		_, pendings, err := graph.SectionStatus(ctx, g.db, projectID, sec.Label)
		if err != nil {
			return false, err
		}
		for _, p := range pendings {
			if p.Kind != graph.PendingOpenCard && p.Kind != graph.PendingOpenQuestion {
				return false, nil
			}
		}
	}
	return true, nil
}

// evaluatedSection is the section the stop report evaluates (backlog-r2
// 2.5): the last touched one that could be signed now, else the last
// touched with text, else none.
// [EL DISEÑO NO LO DICE]: which section a stop for another reason than
// section_ready evaluates.
func (g *Gate) evaluatedSection(ctx context.Context, projectID string, touched []string) (string, error) {
	clear, err := g.clearSection(ctx, projectID, touched)
	if err != nil || clear != "" {
		return clear, err
	}
	for i := len(touched) - 1; i >= 0; i-- {
		sec, err := g.db.GetObject(ctx, projectID, touched[i])
		if errors.Is(err, store.ErrNotFound) {
			continue
		}
		if err != nil {
			return "", err
		}
		var b struct {
			Text string `json:"text"`
		}
		_ = json.Unmarshal(sec.Body, &b)
		if strings.TrimSpace(b.Text) != "" {
			return sec.Label, nil
		}
	}
	return "", nil
}

// ReportUnseen is the principal's sketch of one unseen decision (D6).
type ReportUnseen struct {
	Decision string   `json:"decision"`
	Gist     string   `json:"gist"`
	Sections []string `json:"sections"`
}

// StopReport is the stop report as the stop_report event stores it,
// complete (backlog-r2 2.5): why the chain stopped, the section it
// evaluates and whether the app proposes signing it, what the app
// computes from the graph (B-39, B-41) and what the principal wrote.
type StopReport struct {
	StopReason StopReason `json:"stop_reason"`
	// Cap says which of the demo's caps stopped the chain (B-89: link, day
	// or total), with stop_reason demo_cap.
	Cap string `json:"cap,omitempty"`
	// Cause says why the mode finished, with stop_reason mode_finished (B-83).
	Cause string `json:"cause,omitempty"`
	// Section is the evaluated section's label, "" when the chain wrote
	// none; ProposeSignoff is true only when it could be signed now
	// (graph.SectionClear), whatever the principal wrote.
	Section        string `json:"section"`
	ProposeSignoff bool   `json:"propose_signoff"`
	// Decisions is the app's list of unseen decisions (B-39); the report
	// shows them, and opening it marks each seen (B-40).
	Decisions   []graph.UnseenDecision `json:"unseen_decisions"`
	Verified    []graph.ReportCitation `json:"verified"`
	NotVerified []graph.ReportCitation `json:"not_verified"`
	Open        []graph.ReportOpenItem `json:"open"`
	// AgentPendings are the agent's open pendings (P-111).
	AgentPendings []graph.ReportPending `json:"agent_pendings,omitempty"`
	// The principal's part (salida-informe.schema.json). PrincipalFailed
	// is set when its output failed twice: the report keeps the app's
	// part only.
	Answer          string         `json:"answer"`
	Done            []string       `json:"done"`
	Next            string         `json:"next"`
	Unseen          []ReportUnseen `json:"unseen"`
	PrincipalFailed bool           `json:"principal_failed,omitempty"`
	// JudgeNotVerified are the sections whose last judge round could not
	// verify (backlog-r3 3.6, fichas §7: visible, blocking nothing).
	JudgeNotVerified []string `json:"judge_not_verified,omitempty"`
	// Decided and AgentChanges are the stretch since the previous report (B-103):
	// what was decided and why, and what the agent changed on its own. The app
	// reads them from the history, with no model. A report stored before B-103
	// has neither.
	Decided      []graph.ReportDecided     `json:"decided,omitempty"`
	AgentChanges []graph.ReportAgentChange `json:"agent_changes,omitempty"`
}

// StoredStopReport is the last stop report with its place in the
// history and whether you opened it (B-40: a stop_report_seen after it).
type StoredStopReport struct {
	StopReport
	Seq       int
	CreatedAt string
	Seen      bool
}

// LatestStopReport reads the project's last stop report and whether you
// opened it; false when there is none.
func LatestStopReport(ctx context.Context, q store.Querier, projectID string) (StoredStopReport, bool, error) {
	events, err := q.EventsSince(ctx, projectID, 0)
	if err != nil {
		return StoredStopReport{}, false, err
	}
	return latestStopReport(events)
}

func latestStopReport(events []store.Event) (StoredStopReport, bool, error) {
	var out StoredStopReport
	found := false
	for _, ev := range events {
		switch ev.Type {
		case store.EventStopReport:
			out = StoredStopReport{Seq: ev.Seq, CreatedAt: ev.CreatedAt}
			if err := json.Unmarshal(ev.Data, &out.StopReport); err != nil {
				return StoredStopReport{}, false, fmt.Errorf("cycle: parse stop report: %w", err)
			}
			found = true
		case store.EventStopReportSeen:
			if found {
				out.Seen = true
			}
		}
	}
	return out, found, nil
}

// ErrNoStopReport is a stop report route on a project that has none.
var ErrNoStopReport = errors.New("cycle: the project has no stop report")

// MarkStopReportSeen records that you opened the last stop report in
// the panel (B-40): stop_report_seen, and decision_seen (report) for
// each decision the report shows, in one transaction. A report already
// opened records nothing; folded does not count (the page calls this on
// opening it).
// [EL DISEÑO NO LO DICE]: opening it again records nothing more.
func (g *Gate) MarkStopReportSeen(ctx context.Context, projectID string) error {
	return g.db.Write(ctx, projectID, func(tx *store.Tx) error {
		events, err := tx.EventsSince(ctx, projectID, 0)
		if err != nil {
			return err
		}
		report, ok, err := latestStopReport(events)
		if err != nil {
			return err
		}
		if !ok {
			return ErrNoStopReport
		}
		if report.Seen {
			return nil
		}
		if _, err := tx.AppendEvent(store.EventInput{Type: store.EventStopReportSeen, Actor: "human",
			Data: map[string]any{"stop_report_seq": report.Seq}}); err != nil {
			return err
		}
		for _, d := range report.Decisions {
			dec, err := tx.GetObject(ctx, projectID, d.Label)
			if errors.Is(err, store.ErrNotFound) {
				continue
			}
			if err != nil {
				return err
			}
			if _, err := tx.AppendEvent(store.EventInput{Type: store.EventDecisionSeen, Actor: "human",
				ObjectID: dec.ID, Data: map[string]any{"label": dec.Label, "seen_via": "report"}}); err != nil {
				return err
			}
		}
		return nil
	})
}

var (
	reportSchemaOnce sync.Once
	reportSchema     *jsonschema.Schema
	reportSendable   json.RawMessage
	reportSchemaErr  error
)

func stopReportSchemas() (*jsonschema.Schema, json.RawMessage, error) {
	reportSchemaOnce.Do(func() {
		doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(stopReportSchema))
		if err != nil {
			reportSchemaErr = fmt.Errorf("cycle: parse stop report schema: %w", err)
			return
		}
		c := jsonschema.NewCompiler()
		const loc = "https://tupaqnin.invalid/stop_report_output.schema.json"
		if err := c.AddResource(loc, doc); err != nil {
			reportSchemaErr = fmt.Errorf("cycle: add stop report schema: %w", err)
			return
		}
		if reportSchema, err = c.Compile(loc); err != nil {
			reportSchemaErr = fmt.Errorf("cycle: compile stop report schema: %w", err)
			return
		}
		reportSendable, reportSchemaErr = provider.TransportSchema(stopReportSchema)
	})
	return reportSchema, reportSendable, reportSchemaErr
}

type reportOutput struct {
	Answer string         `json:"answer"`
	Done   []string       `json:"done"`
	Next   string         `json:"next"`
	Unseen []ReportUnseen `json:"unseen"`
}

// validateReport checks the principal's report against its fixed shape:
// the schema, and what it cannot say (fichas §7): one unseen entry for
// each decision of the app's list, each once, none outside it, and only
// sections of the project (rule 4: never a new-n). Errors name labels.
func validateReport(ctx context.Context, q store.Querier, projectID, raw string, decisions []graph.UnseenDecision) ([]string, reportOutput, error) {
	var out reportOutput
	sch, _, err := stopReportSchemas()
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
			return nil, out, fmt.Errorf("cycle: validate stop report: %w", err)
		}
		return flattenErrors(verr), out, nil
	}
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, out, fmt.Errorf("cycle: parse stop report: %w", err)
	}
	var errs []string
	listed := map[string]bool{}
	for _, d := range decisions {
		listed[d.Label] = true
	}
	given := map[string]bool{}
	for i, u := range out.Unseen {
		switch {
		case !listed[u.Decision]:
			errs = append(errs, fmt.Sprintf("/unseen/%d/decision: %s is not one of the decisions the app listed", i, u.Decision))
		case given[u.Decision]:
			errs = append(errs, fmt.Sprintf("/unseen/%d/decision: %s comes twice", i, u.Decision))
		}
		given[u.Decision] = true
		for j, s := range u.Sections {
			sec, err := q.GetObject(ctx, projectID, s)
			if errors.Is(err, store.ErrNotFound) || (err == nil && sec.Type != store.ObjectSection) {
				errs = append(errs, fmt.Sprintf("/unseen/%d/sections/%d: %s is not a section of the project", i, j, s))
				continue
			}
			if err != nil {
				return nil, out, err
			}
		}
	}
	for _, d := range decisions {
		if !given[d.Label] {
			errs = append(errs, fmt.Sprintf("/unseen: the decision %s has no entry", d.Label))
		}
	}
	return errs, out, nil
}

// stopReasonLine says, for the principal, why the chain stopped.
func stopReasonLine(reason StopReason, unseen int) string {
	switch reason {
	case StopNoWork:
		return "your last step brought no new text and no new decisions"
	case StopDecisionApplied:
		return "you applied the person's decisions in one step"
	case StopCorrectionApplied:
		return "you did, in one step, the correction the person asked for"
	case StopCorrectionStillOpen:
		return "you took one step on the correction the person asked for, and it is still open"
	case StopAgentPending:
		return "your last step was left pending after its retry"
	case StopStepLimit:
		return fmt.Sprintf("the chain reached its limit of %d steps", MaxChainSteps)
	case StopSectionReady:
		return "a section you wrote has no pendings left"
	case StopUnseenDecisions:
		return fmt.Sprintf("the person has not seen %d decisions you made", unseen)
	case StopHumanOnly:
		return "only what the person decides is left"
	case StopModeCap:
		return fmt.Sprintf("the run-to-completion mode reached its limit of %d steps", ModeMaxSteps)
	}
	return string(reason)
}

// reportRequest is the app's message asking for the stop report, with
// the list of unseen decisions it computed (B-39), labels only.
func reportRequest(reason StopReason, section, title string, clear bool, decisions []graph.UnseenDecision) string {
	var b strings.Builder
	fmt.Fprintf(&b, "The app stopped your chain: %s. Write the stop report now.\n", stopReasonLine(reason, len(decisions)))
	if section != "" {
		fmt.Fprintf(&b, "Section in question: %s «%s». ", section, title)
		if clear {
			b.WriteString("It has no pendings and no open changes: the app proposes signing it.\n")
		} else {
			b.WriteString("It still has pendings or open changes: the app does not propose signing it.\n")
		}
	}
	if len(decisions) == 0 {
		b.WriteString("Decisions the person has not seen: none; unseen is an empty list.\n")
	} else {
		b.WriteString("Decisions the person has not seen (one unseen entry for each, in this order):\n")
		for _, d := range decisions {
			fmt.Fprintf(&b, "- %s: %s (instead of: %s); used in: %s\n",
				d.Label, d.Statement, d.InsteadOf, strings.Join(d.Sections, ", "))
		}
	}
	b.WriteString("The app adds what is verified and what is not, and the open questions and cards; do not write them.")
	return b.String()
}

// writeStopReport ends a chain with its stop report (backlog-r2 2.5):
// one call to the principal, in its conversation, with the report's
// schema and the unseen decisions the app computed (B-39), one retry
// (fichas §7), then the stop_report event with the app's part (B-41).
// No transaction is open while the principal answers (specification §2).
// [EL DISEÑO NO LO DICE]: the call goes in the principal's conversation
// (same instance, kept in model_call), with its tools and no streaming;
// after two failures the report keeps the app's part (PrincipalFailed)
// and leaves no agent pending (P-75 (c)).
//
// When the call does not fit in a cap (B-89) the report keeps the app's part
// with that stop, and the refusal is returned.
func (r *Runner) writeStopReport(ctx context.Context, projectID, turnID string, reason StopReason) (*provider.CapError, error) {
	return r.stopReport(ctx, projectID, turnID, reason, nil, nil)
}

// stopReport writes the report. A cap already stopping the turn, or end.noCall,
// keep the app's part with no call. With end, the report is the mode's last: its
// reason says mode_finished and the cause why, and the mode goes back to normal
// in the same transaction (B-83, rule 6).
// [EL DISEÑO NO LO DICE]: the backlog says «stop_reason: mode_finished en
// step_ended», but the cause is known after the step's step_ended was written
// (no_work, human_only, the checker), so it lives where B-37's 31-oct note puts
// the reasons known after it: stop_report, and the stream's step_ended. In the
// mode, the history's step_ended carries no stop_reason.
func (r *Runner) stopReport(ctx context.Context, projectID, turnID string, reason StopReason, capErr *provider.CapError, end *modeEnd) (*provider.CapError, error) {
	touched, err := graph.TouchedSinceStop(ctx, r.db, projectID)
	if err != nil {
		return nil, err
	}
	section, err := r.gate.evaluatedSection(ctx, projectID, touched)
	if err != nil {
		return nil, err
	}
	decisions, err := graph.UnseenDecisions(ctx, r.db, projectID)
	if err != nil {
		return nil, err
	}
	var out reportOutput
	ok := false
	if capErr == nil && (end == nil || !end.noCall) {
		if out, ok, capErr, err = r.askReport(ctx, projectID, turnID, reason, section, decisions); err != nil {
			return nil, err
		}
		if capErr != nil {
			reason = StopReason(capErr.Reason)
		}
	}

	var report StopReport
	err = r.db.Write(ctx, projectID, func(tx *store.Tx) error {
		var err error
		report = StopReport{StopReason: reason, Section: section, Decisions: decisions,
			Done: []string{}, Unseen: []ReportUnseen{}}
		if capErr != nil {
			report.Cap = capErr.Cap
		}
		if end != nil {
			report.StopReason, report.Cause = StopModeFinished, string(reason)
		}
		if ok {
			report.Answer, report.Next = out.Answer, out.Next
			if out.Done != nil {
				report.Done = out.Done
			}
			if out.Unseen != nil {
				report.Unseen = out.Unseen
			}
		} else {
			report.PrincipalFailed = true
		}
		// What has evidence value is computed here, in the report's
		// transaction, never taken from the model (B-41, bloque-3 §2).
		if section != "" {
			if report.ProposeSignoff, err = graph.SectionClear(ctx, tx, projectID, section); err != nil {
				return err
			}
		}
		var sections []string
		for _, label := range touched {
			if sec, err := tx.GetObject(ctx, projectID, label); err == nil && sec.Type == store.ObjectSection {
				sections = append(sections, label)
			}
		}
		if report.Verified, report.NotVerified, err = graph.ReportCitations(ctx, tx, projectID, sections); err != nil {
			return err
		}
		if report.Open, err = graph.ReportOpenItems(ctx, tx, projectID); err != nil {
			return err
		}
		if report.AgentPendings, err = graph.ReportAgentPendings(ctx, tx, projectID); err != nil {
			return err
		}
		if report.JudgeNotVerified, err = JudgeNotVerified(ctx, tx, projectID); err != nil {
			return err
		}
		if report.Decided, err = graph.ReportDecisions(ctx, tx, projectID); err != nil {
			return err
		}
		if report.AgentChanges, err = graph.ReportAgentChanges(ctx, tx, projectID); err != nil {
			return err
		}
		if _, err = tx.AppendEvent(store.EventInput{Type: store.EventStopReport, Actor: "agent",
			AgentRole: "principal", TurnID: turnID, Data: report}); err != nil {
			return err
		}
		if end == nil {
			return nil
		}
		return appendModeEnd(ctx, tx, projectID, turnID, report)
	})
	if err != nil {
		return nil, err
	}
	// The defense, at the stop that proposes signing (backlog-r2 2.6),
	// after the report and outside its transaction. At a cap it waits. The
	// mode's last stop prepares every section's (finishMode).
	if end == nil && capErr == nil && report.ProposeSignoff && report.Section != "" {
		return nil, r.prepareDefense(ctx, projectID, turnID, report.Section)
	}
	return capErr, nil
}

// appendModeEnd is mode_set back to normal, by the app, with why (B-83). If you
// turned the mode off meanwhile, your mode_set stands.
func appendModeEnd(ctx context.Context, tx *store.Tx, projectID, turnID string, report StopReport) error {
	mode, err := graph.CurrentMode(ctx, tx, projectID)
	if err != nil || !mode.On() {
		return err
	}
	data := map[string]any{"mode": graph.ModeNormal, "cause": report.Cause}
	if report.Cap != "" {
		data["cap"] = report.Cap
	}
	_, err = tx.AppendEvent(store.EventInput{Type: store.EventModeSet, Actor: "app", TurnID: turnID, Data: data})
	return err
}

// askReport asks the principal for its part of the report, with one retry.
// ok is false after two failures; a call the gate refused is no failure: it
// ends the asking with no output_retry and returns the refusal.
func (r *Runner) askReport(ctx context.Context, projectID, turnID string, reason StopReason, section string,
	decisions []graph.UnseenDecision) (out reportOutput, ok bool, capErr *provider.CapError, err error) {
	title, clear := "", false
	if section != "" {
		if sec, err := r.db.GetObject(ctx, projectID, section); err == nil {
			var b struct {
				Title string `json:"title"`
			}
			_ = json.Unmarshal(sec.Body, &b)
			title = b.Title
		}
		if clear, err = graph.SectionClear(ctx, r.db, projectID, section); err != nil {
			return out, false, nil, err
		}
	}
	_, schema, err := stopReportSchemas()
	if err != nil {
		return out, false, nil, err
	}
	if capErr, err = r.overCap(ctx, projectID, provider.RolePrincipal, provider.TurnStopReport); err != nil || capErr != nil {
		return out, false, capErr, err
	}
	instanceID, startupInput, started, err := r.ensureInstance(ctx, projectID)
	if err != nil {
		return out, false, nil, err
	}
	input := startupInput
	if !started {
		if input, err = r.conversation(ctx, projectID, instanceID); err != nil {
			return out, false, nil, err
		}
	}
	input = append(input, userItem(reportRequest(reason, section, title, clear, decisions)))
	refs := make([]provider.ReportDecision, 0, len(decisions))
	for _, d := range decisions {
		refs = append(refs, provider.ReportDecision{Label: d.Label, Sections: d.Sections})
	}

	for attempt := 1; attempt <= 2 && !ok; attempt++ {
		resp, sent, err := r.respond(ctx, provider.Request{
			ProjectID: projectID, TurnID: turnID, InstanceID: instanceID,
			Role: provider.RolePrincipal, TurnKind: provider.TurnStopReport, Model: provider.PrincipalModel,
			Effort: provider.PrincipalEffort, Instructions: stopReportInstructions,
			Input: input, Schema: schema, SchemaStrict: true, ReportDecisions: refs,
		}, nil)
		if refused, isCap := asCap(err); isCap {
			return out, false, refused, nil
		}
		var errs []string
		if err != nil {
			if ctx.Err() != nil {
				return out, false, nil, ctx.Err()
			}
			// The provider's error body is never kept (T-10).
			errs = []string{"provider_error"}
		} else if errs, out, err = validateReport(ctx, r.db, projectID, resp.Text, decisions); err != nil {
			return out, false, nil, err
		}
		if len(errs) == 0 {
			ok = true
			break
		}
		// P-75 (c): the report's retry leaves no agent pending (estados
		// §1 ties those to a section; the report writes in none): after
		// two failures the stop_report keeps the app's part and says it
		// is incomplete (PrincipalFailed).
		data := map[string]any{"turn_kind": "stop_report", "attempt": attempt, "errors": errs}
		if err := r.gate.append(ctx, projectID, store.EventInput{Type: store.EventOutputRetry,
			Actor: "agent", AgentRole: "principal", TurnID: turnID, Data: data}); err != nil {
			return out, false, nil, err
		}
		if sent != nil {
			input = append(append(append([]json.RawMessage{}, sent...), resp.RawItems...), userItem(RetryMessage(errs)))
		}
	}
	return out, ok, nil, nil
}
