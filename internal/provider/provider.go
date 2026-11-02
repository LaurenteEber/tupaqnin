package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// AgentRole is who a call speaks for (glossary §5).
type AgentRole string

const (
	RolePrincipal  AgentRole = "principal"
	RoleReader     AgentRole = "reader"
	RoleJudge      AgentRole = "judge"
	RoleChecker    AgentRole = "checker"
	RoleContraster AgentRole = "contraster"
)

// TurnKind is the kind of turn a call belongs to: the understanding
// (1.2), an agent step, a signoff with Wonder (B-13) or the checker's
// judgment of a change (backlog-r2 2.3), the principal's stop report
// (backlog-r2 2.5) or the judge's defense preparation at the stop
// (backlog-r2 2.6). The fake picks its recording by it (P-29); Meta
// ignores it.
type TurnKind string

const (
	TurnUnderstanding TurnKind = "understanding"
	TurnStep          TurnKind = "step"
	TurnSignoff       TurnKind = "signoff"
	TurnChecker       TurnKind = "checker"
	TurnStopReport    TurnKind = "stop_report"
	TurnJudgeDefense  TurnKind = "judge_defense"
	// TurnReader is one reader of a round (backlog-r3 3.2): its calls
	// carry the round's turn id and no principal instance (B-62).
	TurnReader TurnKind = "reader"
	// TurnDigest is the principal's digest of a round (backlog-r3 3.4):
	// in its live instance, at its usual effort, with salida-digesto.
	TurnDigest TurnKind = "digest"
	// TurnJudgeVerify is the judge's verification of a section and its one
	// recheck (backlog-r3 3.6): no principal instance, with
	// salida-juez-verificacion.
	TurnJudgeVerify TurnKind = "judge_verify"
)

// Provider is one model backend. onDelta may be nil; when set, it is
// called with each text delta as it arrives.
type Provider interface {
	Respond(ctx context.Context, req Request, onDelta func(Delta)) (Response, error)
}

// Request is one model call. Input items are raw: items received from
// a previous call are resent byte-for-byte (§7).
type Request struct {
	ProjectID    string
	TurnID       string
	InstanceID   string
	Role         AgentRole
	TurnKind     TurnKind
	Model        string
	Effort       string
	Instructions string
	Input        []json.RawMessage
	Schema       json.RawMessage
	// SchemaStrict asks Meta to validate Schema against its strict
	// subset (Meta structured-output docs: with strict, every object
	// must list all of its properties in required, among other
	// rules). The principal's schema uses optional properties by
	// design, so it is sent without strict and validated in Go
	// instead (B-2; finding of micro-test 0.8, param="schema").
	SchemaStrict bool
	// Tools are the function tools the model may call (R-3), a JSON
	// array of Responses API tool definitions; nil sends none.
	Tools json.RawMessage
	// ProbeLabels are the labels of a checker call's probes, in order,
	// so the fake can answer a checker call it has no recording for
	// (see Fake.Respond); Meta ignores it, like TurnKind.
	ProbeLabels []string
	// ReportDecisions are the decisions a stop report call must sketch
	// (B-39), so the fake can answer a stop report call it has no
	// recording for (see Fake.Respond); Meta ignores it.
	ReportDecisions []ReportDecision
	// DefenseSection is the label of the section a defense call prepares
	// (backlog-r2 2.6), so the fake can answer a judge call it has no
	// recording for (see Fake.Respond); Meta ignores it.
	DefenseSection string
	// Part is the round part a reader call answers, from 1 (backlog-r3
	// 3.2), so the fake can pick a recording by part; 0 for every other
	// call. Meta ignores it.
	Part int
	// DigestExpectation says whether a digest call's round has an
	// expectation (K-5), so the fake can answer a digest call it has no
	// recording for (see Fake.Respond); Meta ignores it.
	DigestExpectation bool
	// JudgeRecheck are the numbers of the flaws a judge's recheck answers
	// (backlog-r3 3.6), so the fake can answer a recheck it has no
	// recording for (see Fake.Respond); nil on the first call. Meta ignores
	// it.
	JudgeRecheck []int
}

// ReportDecision is one decision the app lists for the stop report: its
// label and the labels of the sections that use it (B-39).
type ReportDecision struct {
	Label    string
	Sections []string
}

// Response is one completed call: every raw output item, the joined
// output text, the function calls the model made (R-3), and the
// reported usage.
type Response struct {
	RawItems  []json.RawMessage
	Text      string
	ToolCalls []ToolCall
	Usage     Usage
}

// ToolCall is one function_call output item (R-3): the model asks the
// app to run a tool and answer with ToolOutputItem.
type ToolCall struct {
	CallID    string `json:"call_id"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// toolCallsOf reads the function_call items of an output.
func toolCallsOf(items []json.RawMessage) []ToolCall {
	var out []ToolCall
	for _, item := range items {
		var fc struct {
			Type string `json:"type"`
			ToolCall
		}
		if json.Unmarshal(item, &fc) == nil && fc.Type == "function_call" {
			out = append(out, fc.ToolCall)
		}
	}
	return out
}

// ToolOutputItem is the input item that answers one tool call.
func ToolOutputItem(callID, output string) json.RawMessage {
	b, _ := json.Marshal(map[string]string{"type": "function_call_output", "call_id": callID, "output": output})
	return b
}

// Usage is the use a call reports (§1.4).
type Usage struct {
	InputTokens   int     `json:"input_tokens"`
	OutputTokens  int     `json:"output_tokens"`
	ContextWindow int     `json:"context_window"`
	CostUSD       float64 `json:"cost_usd"`
}

// Delta is one streamed piece of output text.
type Delta struct {
	Text string
}

// ProviderError is an error from the provider, carrying only its
// code, type and the request field it names (param, per Meta's error
// docs; code may be null there). The raw error body, including its
// message, is never kept or logged (T-10).
type ProviderError struct {
	Code  string
	Type  string
	Param string
	// HTTPStatus is the provider's HTTP status, 0 when the error did not
	// come from an HTTP reply. B-20 (c) reads it, never the body's code.
	HTTPStatus int
	// contextFull is what the error's body said about the window (B-86),
	// read before the body is dropped; see Is.
	contextFull bool
}

// ErrContextFull is the provider saying a call does not fit in the model's
// context window (B-86). A *ProviderError answers it through errors.Is; the
// message it was recognised from is not kept (T-10).
var ErrContextFull = errors.New("provider: context window full")

// Is makes errors.Is(err, ErrContextFull) answer for a full window.
func (e *ProviderError) Is(target error) bool { return target == ErrContextFull && e.contextFull }

func (e *ProviderError) Error() string {
	return fmt.Sprintf("provider error: code=%q type=%q param=%q", e.Code, e.Type, e.Param)
}

// CallRecorder stores one model_call row per call (§1.4).
// *store.DB is the CallRecorder.
type CallRecorder interface {
	RecordModelCall(ctx context.Context, c store.ModelCall) error
}

// joinRawItems joins raw items into a JSON array without touching
// their bytes: json.Marshal would compact them, and items must be
// resent and stored byte-for-byte (§7, B-5 of the history design).
func joinRawItems(items []json.RawMessage) json.RawMessage {
	out := json.RawMessage("[")
	for i, item := range items {
		if i > 0 {
			out = append(out, ',')
		}
		out = append(out, item...)
	}
	return append(out, ']')
}

// recordCall writes the model_call row for a finished call, if a
// recorder is set. Items are stored exactly as sent and received.
func recordCall(ctx context.Context, rec CallRecorder, req Request, resp Response) error {
	if rec == nil {
		return nil
	}
	inputItems := joinRawItems(req.Input)
	rawItems := joinRawItems(resp.RawItems)
	modelAndEffort := req.Model
	if req.Effort != "" {
		modelAndEffort += " " + req.Effort
	}
	// The call is over and its cost known: an aborted request still pays it
	// (B3-22 row 2).
	return rec.RecordModelCall(context.WithoutCancel(ctx), store.ModelCall{
		ProjectID: req.ProjectID, InstanceID: req.InstanceID, TurnID: req.TurnID,
		Role: string(req.Role), ModelAndEffort: modelAndEffort,
		InputTokens: resp.Usage.InputTokens, OutputTokens: resp.Usage.OutputTokens,
		ContextWindow: resp.Usage.ContextWindow, InputItems: inputItems,
		RawItems: rawItems, CostUSD: resp.Usage.CostUSD,
	})
}

// fillUsage completes window and cost when the backend did not.
func fillUsage(model string, u *Usage) {
	if u.ContextWindow == 0 {
		u.ContextWindow = ContextWindow(model)
	}
	if u.CostUSD == 0 {
		u.CostUSD = CostUSD(model, u.InputTokens, u.OutputTokens)
	}
}

// Fixture is one recorded call the fake provider replays (T-11).
type Fixture struct {
	Name string `json:"name"`
	// TurnKind is the kind of turn the recording answers (P-29);
	// empty serves any kind.
	TurnKind TurnKind `json:"turn_kind,omitempty"`
	// Part is the round part a reader recording answers (backlog-r3
	// 3.2); 0 serves any part.
	Part       int               `json:"part,omitempty"`
	Deltas     []string          `json:"deltas"`
	Items      []json.RawMessage `json:"items"`
	Usage      Usage             `json:"usage"`
	DeltaDelay time.Duration     `json:"delta_delay"`
}

// LoadFixture reads one recorded fixture from a JSON file.
func LoadFixture(path string) (Fixture, error) {
	return loadFixtureFile(path)
}

// Fake replays fixtures in order, one per call, and starts again at
// the first when they run out (backlog 0.5, for the app fixtures).
// A call of a given turn kind only gets the fixtures of that kind
// (or without one), each kind with its own order (P-29).
type Fake struct {
	Recorder CallRecorder

	fixtures []Fixture
	// mu guards next: the readers of a round call the fake in parallel
	// (backlog-r3 3.2).
	mu   sync.Mutex
	next map[pickKey]int
}

// pickKey is one replay order: by turn kind and, for a reader, by part.
type pickKey struct {
	kind TurnKind
	part int
}

// NewFake returns a fake that replays the fixtures in the given order.
func NewFake(fixtures ...Fixture) *Fake {
	return &Fake{fixtures: fixtures, next: make(map[pickKey]int)}
}

// pick returns the next fixture for the turn kind, or false when no
// fixture serves it. A checker call only gets checker recordings: a
// recording without a kind predates the checker and is the principal's.
// A stop report call, a judge's defense call and a reader call likewise
// only get recordings of their own kind. A reader call gets the
// recordings of its part (or without one), each part with its own order
// (backlog-r3 3.2).
func (f *Fake) pick(kind TurnKind, part int) (Fixture, bool) {
	var matching []Fixture
	for _, fx := range f.fixtures {
		if (kind == TurnChecker || kind == TurnStopReport || kind == TurnJudgeDefense || kind == TurnReader ||
			kind == TurnDigest || kind == TurnJudgeVerify) && fx.TurnKind != kind {
			continue
		}
		if kind == TurnReader && fx.Part != 0 && fx.Part != part {
			continue
		}
		if kind == "" || fx.TurnKind == "" || fx.TurnKind == kind {
			matching = append(matching, fx)
		}
	}
	if len(matching) == 0 {
		return Fixture{}, false
	}
	key := pickKey{kind: kind}
	if kind == TurnReader {
		key.part = part
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	fx := matching[f.next[key]%len(matching)]
	f.next[key]++
	return fx, true
}

var _ Provider = (*Fake)(nil)

// Respond emits the next fixture's deltas at its DeltaDelay rhythm
// and returns its recorded items byte-for-byte.
func (f *Fake) Respond(ctx context.Context, req Request, onDelta func(Delta)) (Response, error) {
	fx, ok := f.pick(req.TurnKind, req.Part)
	if !ok && req.TurnKind == TurnChecker {
		fx, ok = noFindingsFixture(req.ProbeLabels), true
	}
	if !ok && req.TurnKind == TurnStopReport {
		fx, ok = plainReportFixture(req.ReportDecisions), true
	}
	if !ok && req.TurnKind == TurnJudgeDefense {
		fx, ok = plainDefenseFixture(req.DefenseSection), true
	}
	if !ok && req.TurnKind == TurnDigest {
		fx, ok = plainDigestFixture(req.DigestExpectation), true
	}
	if !ok && req.TurnKind == TurnJudgeVerify {
		fx, ok = plainJudgeVerifyFixture(req.JudgeRecheck), true
	}
	if !ok {
		return Response{}, &ProviderError{Code: "no_fixtures", Type: "fake"}
	}
	for _, d := range fx.Deltas {
		if fx.DeltaDelay > 0 {
			timer := time.NewTimer(fx.DeltaDelay)
			select {
			case <-ctx.Done():
				timer.Stop()
				return Response{}, ctx.Err()
			case <-timer.C:
			}
		} else if err := ctx.Err(); err != nil {
			return Response{}, err
		}
		if onDelta != nil {
			onDelta(Delta{Text: d})
		}
	}
	resp := Response{RawItems: fx.Items, Usage: fx.Usage, ToolCalls: toolCallsOf(fx.Items)}
	resp.Text = textOfItems(fx.Items)
	if resp.Text == "" && len(resp.ToolCalls) == 0 {
		for _, d := range fx.Deltas {
			resp.Text += d
		}
	}
	fillUsage(req.Model, &resp.Usage)
	if err := recordCall(ctx, f.Recorder, req, resp); err != nil {
		return Response{}, err
	}
	return resp, nil
}

// noFindingsFixture is the fake's answer to a checker call without a
// checker recording: every probe «no» and no candidates, so a fake
// without checker recordings (most tests, TQ_PROVIDER=fake) runs the
// checker without finding anything.
// [EL DISEÑO NO LO DICE]: what the fake answers without a recording;
// the signoff errors instead, but an error here would leave every
// section «sin comprobar» (B-50).
func noFindingsFixture(labels []string) Fixture {
	probes := make([]map[string]any, 0, len(labels))
	for _, l := range labels {
		probes = append(probes, map[string]any{"protected": l, "verdict": "no", "passage": nil})
	}
	text, _ := json.Marshal(map[string]any{"probes": probes, "candidates": []any{}})
	item, _ := json.Marshal(map[string]any{"type": "message", "role": "assistant",
		"content": []map[string]string{{"type": "output_text", "text": string(text)}}})
	return Fixture{Name: "checker-no-findings", TurnKind: TurnChecker, Items: []json.RawMessage{item}}
}

// plainReportFixture is the fake's answer to a stop report call without
// a stop report recording: a short report with one line for each
// decision the app listed, so a fake without stop report recordings
// (most tests, TQ_PROVIDER=fake) still stops with a report.
// [EL DISEÑO NO LO DICE]: what the fake answers without a recording; as
// with the checker, an error would leave every stop without its report.
func plainReportFixture(decisions []ReportDecision) Fixture {
	unseen := make([]map[string]any, 0, len(decisions))
	for _, d := range decisions {
		unseen = append(unseen, map[string]any{"decision": d.Label,
			"gist": "Decision " + d.Label + ".", "sections": d.Sections})
	}
	text, _ := json.Marshal(map[string]any{"answer": "The agent stopped; the report is below.",
		"done": []string{}, "next": "Continue with the plan.", "unseen": unseen})
	item, _ := json.Marshal(map[string]any{"type": "message", "role": "assistant",
		"content": []map[string]string{{"type": "output_text", "text": string(text)}}})
	return Fixture{Name: "stop-report-plain", TurnKind: TurnStopReport, Items: []json.RawMessage{item}}
}

// plainDefenseFixture is the fake's answer to a judge's defense call
// without a judge recording: three plain questions on the section and
// nothing uncovered, so a fake without judge recordings (most tests,
// TQ_PROVIDER=fake) still prepares the defense.
// [EL DISEÑO NO LO DICE]: what the fake answers without a recording; as
// with the stop report, an error would leave every defense failed.
func plainDefenseFixture(section string) Fixture {
	questions := make([]map[string]any, 0, 3)
	for _, q := range []string{"What is this section's main claim based on?",
		"What would change the conclusion?", "What does the work leave out?"} {
		questions = append(questions, map[string]any{"question": q, "section": section})
	}
	text, _ := json.Marshal(map[string]any{"audience_questions": questions, "not_covered": []any{}})
	item, _ := json.Marshal(map[string]any{"type": "message", "role": "assistant",
		"content": []map[string]string{{"type": "output_text", "text": string(text)}}})
	return Fixture{Name: "judge-defense-plain", TurnKind: TurnJudgeDefense, Items: []json.RawMessage{item}}
}

// plainDigestFixture is the fake's answer to a digest call without a
// digest recording: a one-line outline that uses no finding, the doubt
// reduced only in part, and the contrast only when the round has an
// expectation (K-5), so a fake without digest recordings (most tests,
// TQ_PROVIDER=fake) still closes its rounds.
// [EL DISEÑO NO LO DICE]: what the fake answers without a recording; as
// with the stop report, an error would leave every round open.
func plainDigestFixture(expectation bool) Fixture {
	var contrast any
	if expectation {
		contrast = "The findings do not settle the expectation."
	}
	text, _ := json.Marshal(map[string]any{"answer": "The readers' findings are listed below.",
		"outline": []string{"What the readers found."}, "findings": []any{}, "expectation_contrast": contrast,
		"reduced_doubt": "partly", "contradictions": []any{}})
	item, _ := json.Marshal(map[string]any{"type": "message", "role": "assistant",
		"content": []map[string]string{{"type": "output_text", "text": string(text)}}})
	return Fixture{Name: "digest-plain", TurnKind: TurnDigest, Items: []json.RawMessage{item}}
}

// plainJudgeVerifyFixture is the fake's answer to a judge's verification
// call without a recording: no flaws on the first call and, on a recheck,
// every flaw resolved, so a fake without judge recordings (most tests,
// TQ_PROVIDER=fake) still closes its judge rounds.
// [EL DISEÑO NO LO DICE]: what the fake answers without a recording; as
// with the digest, an error would make every judge round fail.
func plainJudgeVerifyFixture(recheck []int) Fixture {
	entries := make([]map[string]any, 0, len(recheck))
	for _, n := range recheck {
		entries = append(entries, map[string]any{"flaw": n, "verdict": "resolved", "passage": nil})
	}
	text, _ := json.Marshal(map[string]any{"flaws": []any{}, "recheck": entries})
	item, _ := json.Marshal(map[string]any{"type": "message", "role": "assistant",
		"content": []map[string]string{{"type": "output_text", "text": string(text)}}})
	return Fixture{Name: "judge-verify-plain", TurnKind: TurnJudgeVerify, Items: []json.RawMessage{item}}
}

// CountTokens counts 1 token per 4 characters of input (B-5).
func (f *Fake) CountTokens(_ context.Context, _ string, input []json.RawMessage) (int, error) {
	chars := 0
	for _, item := range input {
		chars += len(item)
	}
	return (chars + 3) / 4, nil
}

// textOfItems joins the output_text of every message item.
func textOfItems(items []json.RawMessage) string {
	var text string
	for _, raw := range items {
		var item struct {
			Type    string `json:"type"`
			Phase   string `json:"phase"`
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		}
		// A commentary message comes beside tool calls (R-3, recorded in
		// the tools micro-test): it is not the fixed output.
		if err := json.Unmarshal(raw, &item); err != nil || item.Type != "message" || item.Phase == "commentary" {
			continue
		}
		for _, part := range item.Content {
			if part.Type == "output_text" {
				text += part.Text
			}
		}
	}
	return text
}
