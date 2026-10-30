package cycle

import (
	"bytes"
	"context"
	"crypto/rand"
	_ "embed"
	"encoding/base32"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/LaurenteEber/tupaqnin/internal/memory"
	"github.com/LaurenteEber/tupaqnin/internal/provider"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

//go:embed prompts/principal.en.md
var principalInstructions string

// liveConversationWindow is how long the principal keeps its
// conversation: its last step was less than 2 h ago (B-7, [inicial]).
const liveConversationWindow = 2 * time.Hour

// TurnPublisher is where a runner publishes the stream events of a
// turn (backlog 0.6). *server.Hub satisfies it.
type TurnPublisher interface {
	BeginTurn(projectID, turnID string)
	PublishDelta(projectID, turnID, text string)
	PublishStep(projectID, turnID string, step json.RawMessage)
	PublishError(projectID, turnID, code string)
}

// ErrTurnRunning reports a message sent while a turn of the same
// project is still running. [EL DISEÑO NO LO DICE] one turn per
// project: the second message is refused (the API answers
// 409 turn_running) instead of running two turns at once or waiting.
var ErrTurnRunning = errors.New("cycle: a turn is already running for this project")

// Runner runs one human turn end to end (backlog 0.10): it records
// the human message, reuses the live principal conversation or
// starts a new instance, calls the principal, closes the step with
// EndStep and streams everything through the Publisher.
type Runner struct {
	// Logger, when set, receives turn failures by code only (T-10):
	// never message or document content.
	Logger *slog.Logger
	// Now overrides the clock (tests). Nil means time.Now.
	Now func() time.Time

	db       *store.DB
	prov     provider.Provider
	count    memory.TokenCounter
	pub      TurnPublisher
	gate     *Gate
	memory   *memory.Memory
	schema   json.RawMessage
	schemaOK bool

	// One turn per project ([EL DISEÑO NO LO DICE], see
	// ErrTurnRunning): running holds the projects with a turn in
	// flight, guarded by mu.
	mu      sync.Mutex
	running map[string]bool
}

// NewRunner wires a runner. count is the token counter already bound
// to the principal's model (B-5).
func NewRunner(db *store.DB, prov provider.Provider, count memory.TokenCounter, pub TurnPublisher) *Runner {
	return &Runner{db: db, prov: prov, count: count, pub: pub,
		gate: NewGate(db), memory: memory.New(db), running: map[string]bool{}}
}

// tryAcquireTurn takes the project's turn slot, or reports false
// when a turn of that project is already running.
func (r *Runner) tryAcquireTurn(projectID string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.running[projectID] {
		return false
	}
	r.running[projectID] = true
	return true
}

func (r *Runner) releaseTurn(projectID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.running, projectID)
}

func (r *Runner) now() time.Time {
	if r.Now != nil {
		return r.Now()
	}
	return time.Now()
}

func newTurnID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("generate turn id: %w", err)
	}
	return strings.ToLower(base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(b[:])), nil
}

// StartTurn registers a turn and runs it in the background,
// returning its id at once (B-1). It is the server's TurnStarter.
// A second turn for the same project is refused with
// ErrTurnRunning while one is in flight.
func (r *Runner) StartTurn(projectID, text string) (string, error) {
	if !r.tryAcquireTurn(projectID) {
		return "", ErrTurnRunning
	}
	turnID, err := newTurnID()
	if err != nil {
		r.releaseTurn(projectID)
		return "", err
	}
	if r.pub != nil {
		r.pub.BeginTurn(projectID, turnID)
	}
	go func() {
		defer r.releaseTurn(projectID)
		if err := r.runTurn(context.Background(), projectID, turnID, text); err != nil && r.Logger != nil {
			r.Logger.Error("turn failed", "error", err.Error())
		}
	}()
	return turnID, nil
}

func userItem(text string) json.RawMessage {
	raw, _ := json.Marshal(map[string]any{
		"type": "message", "role": "user",
		"content": []map[string]string{{"type": "input_text", "text": text}},
	})
	return raw
}

// splitItems splits a stored JSON array of raw items back into its
// items without touching their bytes (§7).
func splitItems(raw json.RawMessage) ([]json.RawMessage, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	if _, err := dec.Token(); err != nil {
		return nil, fmt.Errorf("split items: %w", err)
	}
	var out []json.RawMessage
	for dec.More() {
		var item json.RawMessage
		if err := dec.Decode(&item); err != nil {
			return nil, fmt.Errorf("split items: %w", err)
		}
		out = append(out, item)
	}
	return out, nil
}

func parseStoredTime(s string) time.Time {
	t, _ := time.Parse("2006-01-02T15:04:05.000Z", s)
	return t
}

// ensureInstance returns the principal instance for this turn and,
// when a new one was started, its startup input. A new instance
// starts when there is no live conversation (B-7: last activity
// under 2 h ago) or when the principal's model or effort changed
// since the instance's last call (block 2 §4).
func (r *Runner) ensureInstance(ctx context.Context, projectID string) (instanceID string, startupInput []json.RawMessage, started bool, err error) {
	events, err := r.db.EventsSince(ctx, projectID, 0)
	if err != nil {
		return "", nil, false, err
	}
	var lastActivity time.Time
	for _, ev := range events {
		if ev.Type != store.EventStartup {
			continue
		}
		var data map[string]any
		_ = json.Unmarshal(ev.Data, &data)
		if id, _ := data["instance_id"].(string); id != "" {
			instanceID = id
			lastActivity = parseStoredTime(ev.CreatedAt)
		}
	}
	reason := memory.StartupNewSession
	if instanceID != "" {
		calls, err := r.db.ModelCalls(ctx, projectID, instanceID)
		if err != nil {
			return "", nil, false, err
		}
		if len(calls) > 0 {
			if t := parseStoredTime(calls[len(calls)-1].CreatedAt); t.After(lastActivity) {
				lastActivity = t
			}
			current := provider.PrincipalModel + " " + provider.PrincipalEffort
			if calls[len(calls)-1].ModelAndEffort != current {
				reason = memory.StartupModelChange
			}
		}
	}
	live := instanceID != "" && !lastActivity.IsZero() &&
		r.now().Sub(lastActivity) < liveConversationWindow
	if live && reason == memory.StartupNewSession {
		return instanceID, nil, false, nil
	}
	brief, err := r.memory.Startup(ctx, projectID, reason,
		provider.ContextWindow(provider.PrincipalModel), r.count)
	if err != nil {
		return "", nil, false, err
	}
	return brief.InstanceID, brief.Input, true, nil
}

// conversation rebuilds the live instance's conversation from its
// model_call rows, byte-for-byte (B-7). Each row already stores the
// full input that was sent, so the conversation is the last row
// only: its input plus what it answered. Joining every row would
// repeat the earlier calls once per later call.
func (r *Runner) conversation(ctx context.Context, projectID, instanceID string) ([]json.RawMessage, error) {
	calls, err := r.db.ModelCalls(ctx, projectID, instanceID)
	if err != nil {
		return nil, err
	}
	if len(calls) == 0 {
		return nil, nil
	}
	last := calls[len(calls)-1]
	sent, err := splitItems(last.InputItems)
	if err != nil {
		return nil, err
	}
	answered, err := splitItems(last.RawItems)
	if err != nil {
		return nil, err
	}
	return append(sent, answered...), nil
}

// changeItems renders what changed since the last delivery, in
// literal, and registers each read (block 2 §3).
func (r *Runner) changeItems(ctx context.Context, projectID, instanceID string) ([]json.RawMessage, error) {
	changes, err := r.memory.ChangesSinceDelivery(ctx, projectID)
	if err != nil {
		return nil, err
	}
	var out []json.RawMessage
	for _, change := range changes {
		item, err := r.memory.Read(ctx, projectID, instanceID, change.Label)
		if err != nil {
			return nil, err
		}
		out = append(out, userItem(fmt.Sprintf(
			"Changed since it was delivered to you (%s, version %d):\n%s",
			item.Label, item.Version, item.Text)))
	}
	return out, nil
}

func (r *Runner) transportSchema() (json.RawMessage, error) {
	if r.schemaOK {
		return r.schema, nil
	}
	schema, err := provider.TransportSchema(principalSchema)
	if err != nil {
		return nil, err
	}
	r.schema, r.schemaOK = schema, true
	return schema, nil
}

// call makes one principal step call, with its tools (R-3). It returns
// the input of the last call, tool rounds included.
func (r *Runner) call(ctx context.Context, projectID, turnID, instanceID string, input []json.RawMessage, f *fieldFilter) (provider.Response, []json.RawMessage, error) {
	schema, err := r.transportSchema()
	if err != nil {
		return provider.Response{}, nil, err
	}
	return r.respond(ctx, provider.Request{
		ProjectID: projectID, TurnID: turnID, InstanceID: instanceID,
		Role: provider.RolePrincipal, TurnKind: provider.TurnStep, Model: provider.PrincipalModel,
		Effort: provider.PrincipalEffort, Instructions: principalInstructions,
		Input: input, Schema: schema, SchemaStrict: true,
	}, r.filtered(projectID, turnID, f))
}

// FieldPublisher is a TurnPublisher that also streams each piece with
// its field (triage_line, text, note, understood; P-28) and, in a
// devroutes build only, the raw model text as field "raw".
type FieldPublisher interface {
	PublishFieldDelta(projectID, turnID string, s Segment)
}

// filtered streams only what the person reads of the model's raw JSON
// (P-28).
func (r *Runner) filtered(projectID, turnID string, f *fieldFilter) func(provider.Delta) {
	f.reset()
	return func(d provider.Delta) {
		if r.pub == nil {
			return
		}
		fp, ok := r.pub.(FieldPublisher)
		if rawStream && ok {
			fp.PublishFieldDelta(projectID, turnID, Segment{Field: "raw", Text: d.Text})
		}
		for _, s := range f.feed(d.Text) {
			if ok {
				fp.PublishFieldDelta(projectID, turnID, s)
				continue
			}
			text := s.Text
			if s.Start && f.published {
				text = "\n\n" + text
			}
			f.published = true
			r.pub.PublishDelta(projectID, turnID, text)
		}
	}
}

type stepPayload struct {
	Status      StepStatus      `json:"status"`
	Errors      []string        `json:"errors,omitempty"`
	TriageLine  string          `json:"triage_line,omitempty"`
	ClosingRisk string          `json:"closing_risk,omitempty"`
	Note        string          `json:"note,omitempty"`
	Deferred    json.RawMessage `json:"deferred,omitempty"`
	// Reply is the agent's answer in the conversation (R-5).
	Reply string `json:"reply,omitempty"`
	// StopReason says why a chain stopped (P-43), so the page can say it.
	StopReason StopReason `json:"stop_reason,omitempty"`
}

func (r *Runner) publishStep(projectID, turnID string, result StepResult, outputText string) {
	r.publishChainStep(projectID, turnID, result, outputText, "")
}

func (r *Runner) publishChainStep(projectID, turnID string, result StepResult, outputText string, reason StopReason) {
	payload := stepPayload{Status: result.Status, Errors: result.Errors, StopReason: reason}
	if result.Status == StatusAccepted {
		var out struct {
			TriageLine  string          `json:"triage_line"`
			Reply       string          `json:"reply"`
			ClosingRisk string          `json:"closing_risk"`
			Note        string          `json:"note"`
			Deferred    json.RawMessage `json:"deferred"`
		}
		if err := json.Unmarshal([]byte(outputText), &out); err == nil {
			payload.TriageLine = out.TriageLine
			payload.Reply = out.Reply
			payload.ClosingRisk = out.ClosingRisk
			payload.Note = out.Note
			payload.Deferred = out.Deferred
		}
	}
	raw, err := json.Marshal(payload)
	if err != nil || r.pub == nil {
		return
	}
	r.pub.PublishStep(projectID, turnID, raw)
}

// StartUnderstandingTurn registers an understanding turn (1.2) and
// runs it in the background, returning its id at once (B-1).
func (r *Runner) StartUnderstandingTurn(projectID string) (string, error) {
	if !r.tryAcquireTurn(projectID) {
		return "", ErrTurnRunning
	}
	turnID, err := newTurnID()
	if err != nil {
		r.releaseTurn(projectID)
		return "", err
	}
	if r.pub != nil {
		r.pub.BeginTurn(projectID, turnID)
	}
	go func() {
		defer r.releaseTurn(projectID)
		if err := r.runUnderstandingTurn(context.Background(), projectID, turnID); err != nil && r.Logger != nil {
			r.Logger.Error("understanding turn failed", "error", err.Error())
		}
	}()
	return turnID, nil
}

// RunUnderstandingTurn runs one understanding turn synchronously.
func (r *Runner) RunUnderstandingTurn(ctx context.Context, projectID, turnID string) error {
	if !r.tryAcquireTurn(projectID) {
		return ErrTurnRunning
	}
	defer r.releaseTurn(projectID)
	return r.runUnderstandingTurn(ctx, projectID, turnID)
}

func (r *Runner) criteriaRefs(ctx context.Context, projectID string) ([]CriterionRef, error) {
	objects, err := r.db.ListObjects(ctx, projectID, store.ObjectCriterion)
	if err != nil {
		return nil, err
	}
	out := make([]CriterionRef, 0, len(objects))
	for _, o := range objects {
		out = append(out, CriterionRef{Label: o.Label, ID: o.ID})
	}
	return out, nil
}

func (r *Runner) requestText(ctx context.Context, projectID string) (string, error) {
	objects, err := r.db.ListObjects(ctx, projectID, store.ObjectRequest)
	if err != nil || len(objects) == 0 {
		return "", err
	}
	var b struct {
		Text string `json:"text"`
	}
	_ = json.Unmarshal(objects[0].Body, &b)
	return b.Text, nil
}

func (r *Runner) callUnderstanding(ctx context.Context, projectID, turnID, instanceID string, input []json.RawMessage, f *fieldFilter) (provider.Response, []json.RawMessage, error) {
	schema, err := provider.TransportSchema(understandingSchema)
	if err != nil {
		return provider.Response{}, nil, err
	}
	return r.respond(ctx, provider.Request{
		ProjectID: projectID, TurnID: turnID, InstanceID: instanceID,
		Role: provider.RolePrincipal, TurnKind: provider.TurnUnderstanding, Model: provider.PrincipalModel,
		Effort: provider.EffortHigh, Instructions: understandingInstructions,
		Input: input, Schema: schema, SchemaStrict: true,
	}, r.filtered(projectID, turnID, f))
}

// runUnderstandingTurn is the «Entendí» turn (backlog 1.2): startup
// (0.9), one call at effort high with the understanding schema, and
// EndUnderstanding applying it; one retry, then agent_pending
// (fichas §7). The chain of steps (B-17) arrives when a principal
// step can be applied (1.3); here the turn ends with the
// understanding, and nothing waits for the human to confirm.
func (r *Runner) runUnderstandingTurn(ctx context.Context, projectID, turnID string) error {
	text, err := r.requestText(ctx, projectID)
	if err != nil {
		r.fail(projectID, turnID, "turn_failed")
		return err
	}
	err = r.db.Write(ctx, projectID, func(tx *store.Tx) error {
		_, err := tx.AppendEvent(store.EventInput{
			Type: store.EventHumanMessage, Actor: "human", TurnID: turnID,
			Data: map[string]any{"text": text},
		})
		return err
	})
	if err != nil {
		return err
	}

	instanceID, startupInput, started, err := r.ensureInstance(ctx, projectID)
	if err != nil {
		r.fail(projectID, turnID, "turn_failed")
		return err
	}
	var input []json.RawMessage
	if started {
		input = startupInput
	} else {
		conversation, err := r.conversation(ctx, projectID, instanceID)
		if err != nil {
			r.fail(projectID, turnID, "turn_failed")
			return err
		}
		changes, err := r.changeItems(ctx, projectID, instanceID)
		if err != nil {
			r.fail(projectID, turnID, "turn_failed")
			return err
		}
		input = append(conversation, changes...)
		input = append(input, userItem(text))
	}

	criteria, err := r.criteriaRefs(ctx, projectID)
	if err != nil {
		r.fail(projectID, turnID, "turn_failed")
		return err
	}

	filter := newFieldFilter(understandingStreamKeys)
	resp, input, err := r.callUnderstanding(ctx, projectID, turnID, instanceID, input, filter)
	if err != nil {
		r.fail(projectID, turnID, "provider_error")
		return nil
	}
	result, err := r.gate.EndUnderstanding(ctx, projectID, turnID, 1, json.RawMessage(resp.Text), criteria)
	if err != nil {
		r.fail(projectID, turnID, "turn_failed")
		return err
	}
	if result.Status == StatusRetry {
		retryInput := append(append([]json.RawMessage{}, input...), resp.RawItems...)
		retryInput = append(retryInput, userItem(RetryMessage(result.Errors)))
		resp2, _, err := r.callUnderstanding(ctx, projectID, turnID, instanceID, retryInput, filter)
		if err != nil {
			r.fail(projectID, turnID, "provider_error")
			return nil
		}
		result, err = r.gate.EndUnderstanding(ctx, projectID, turnID, 2, json.RawMessage(resp2.Text), criteria)
		if err != nil {
			r.fail(projectID, turnID, "turn_failed")
			return err
		}
		resp = resp2
	}
	_ = resp
	// B-17: the chain starts on its own when the understanding ends,
	// without waiting for the cases to be confirmed. It runs as the next
	// turn, whose id the understanding's step carries so the page can
	// follow it.
	// [EL DISEÑO NO LO DICE]: how the page learns of the chain; the
	// smallest choice is next_turn_id in the understanding's step.
	var chainID string
	if result.Status == StatusAccepted {
		if chainID, err = newTurnID(); err != nil {
			return err
		}
		if r.pub != nil {
			r.pub.BeginTurn(projectID, chainID)
		}
	}
	if r.pub != nil {
		step := map[string]any{"status": result.Status, "understood": result.Output.Understood}
		if chainID != "" {
			step["next_turn_id"] = chainID
		}
		if raw, err := json.Marshal(step); err == nil {
			r.pub.PublishStep(projectID, turnID, raw)
		}
	}
	if chainID == "" {
		return nil
	}
	return r.runChain(ctx, projectID, chainID, "", false)
}

// RunTurn runs one turn synchronously. StartTurn wraps it in a
// goroutine; tests call it directly. It takes the project's turn
// slot, like StartTurn.
func (r *Runner) RunTurn(ctx context.Context, projectID, turnID, text string) error {
	if !r.tryAcquireTurn(projectID) {
		return ErrTurnRunning
	}
	defer r.releaseTurn(projectID)
	return r.runTurn(ctx, projectID, turnID, text)
}

func (r *Runner) runTurn(ctx context.Context, projectID, turnID, text string) error {
	err := r.db.Write(ctx, projectID, func(tx *store.Tx) error {
		_, err := tx.AppendEvent(store.EventInput{
			Type: store.EventHumanMessage, Actor: "human", TurnID: turnID,
			Data: map[string]any{"text": text},
		})
		return err
	})
	if err != nil {
		return err
	}
	return r.runChain(ctx, projectID, turnID, text, true)
}

// MaxChainSteps is how many steps the agent chains after the confirmed
// understanding and after each message of the person (B-17 [inicial]).
const MaxChainSteps = 5

// ContinueMessage is the fixed app message of a chained step without a
// message of the person: in English, with nothing of the person's
// (B-17). It tells the agent to write and not to wait for the person
// to confirm cases or roles (P-43, fichas §2: nothing blocks).
const ContinueMessage = "Write the next section of the plan; do not wait for confirmations."

// StopReason is why a chain of steps stopped (B-17), computed here so
// the page can say it (P-43).
type StopReason string

const (
	// StopNoWork: the last step brought no new text and no new decisions.
	StopNoWork StopReason = "no_work"
	// StopAgentPending: the last step was left pending for the agent.
	StopAgentPending StopReason = "agent_pending"
	// StopStepLimit: the chain reached MaxChainSteps.
	StopStepLimit StopReason = "step_limit"
)

// runChain runs up to MaxChainSteps steps in one turn (B-17). The first
// step gets the person's message when there is one; the others get
// ContinueMessage. The chain stops at a step without new text or new
// decisions, at a step left pending for the agent, or at the limit.
func (r *Runner) runChain(ctx context.Context, projectID, turnID, text string, human bool) error {
	filter := newFieldFilter(stepStreamKeys)
	var last StepResult
	var lastText string
	reason := StopStepLimit
	var notice string // B-14: what the last step could not write, for the agent
	for n := 1; n <= MaxChainSteps; n++ {
		message := ContinueMessage
		if n == 1 && human {
			message = text
		} else {
			// What the checks find enters the next triage (flujos §1,
			// P-54): the computed pendings go before the fixed message.
			pendings, err := r.memory.AgentPendings(ctx, projectID)
			if err != nil {
				r.fail(projectID, turnID, "turn_failed")
				return err
			}
			if len(pendings) > 0 {
				message = "Pendings the app computed (resolve them before new text):\n- " +
					strings.Join(pendings, "\n- ") + "\n\n" + message
			}
		}
		if notice != "" {
			message = notice + "\n\n" + message
		}
		instanceID, startupInput, started, err := r.ensureInstance(ctx, projectID)
		if err != nil {
			r.fail(projectID, turnID, "turn_failed")
			return err
		}
		var input []json.RawMessage
		switch {
		case started && n == 1 && human:
			// The startup already carries this message as the current
			// task (block 2 §3), so it is not appended again.
			input = startupInput
		case started:
			input = append(startupInput, userItem(message))
		default:
			conversation, err := r.conversation(ctx, projectID, instanceID)
			if err != nil {
				r.fail(projectID, turnID, "turn_failed")
				return err
			}
			changes, err := r.changeItems(ctx, projectID, instanceID)
			if err != nil {
				r.fail(projectID, turnID, "turn_failed")
				return err
			}
			input = append(conversation, changes...)
			input = append(input, userItem(message))
		}
		result, applied, outText, err := r.runStep(ctx, projectID, turnID, instanceID, input, filter, n == MaxChainSteps)
		if errors.Is(err, errProvider) {
			r.fail(projectID, turnID, "provider_error")
			return nil
		}
		if err != nil {
			r.fail(projectID, turnID, "turn_failed")
			return err
		}
		last, lastText = result, outText
		notice = ""
		if len(applied.Rejected) > 0 {
			notice = strings.Join(applied.Rejected, "\n")
		}
		if result.Status != StatusAccepted {
			reason = StopAgentPending
			break
		}
		if !applied.Work {
			reason = StopNoWork
			break
		}
	}
	r.publishChainStep(projectID, turnID, last, lastText, reason)
	return nil
}

var errProvider = errors.New("cycle: provider call failed")

// runStep runs one step: the call, EndStep (the fixed shape) and
// ApplyStep (rules 2 to 6). A rejection by either counts as the retry
// (specification §3); the retry carries what changed since it was
// delivered, so a section that changed reaches the agent with its
// current text (B-11).
func (r *Runner) runStep(ctx context.Context, projectID, turnID, instanceID string, input []json.RawMessage, f *fieldFilter, last bool) (StepResult, ApplyResult, string, error) {
	var result StepResult
	var applied ApplyResult
	for attempt := 1; attempt <= 2; attempt++ {
		resp, sent, err := r.call(ctx, projectID, turnID, instanceID, input, f)
		input = sent
		if err != nil {
			return StepResult{}, ApplyResult{}, "", errProvider
		}
		result, err = r.gate.EndStep(ctx, projectID, turnID, attempt, json.RawMessage(resp.Text))
		if err != nil {
			return StepResult{}, ApplyResult{}, "", err
		}
		message := RetryMessage(result.Errors)
		if result.Status == StatusAccepted {
			applied, err = r.gate.ApplyChainStep(ctx, projectID, turnID, json.RawMessage(resp.Text), last)
			if err != nil {
				return StepResult{}, ApplyResult{}, "", err
			}
			if applied.Applied {
				return result, applied, resp.Text, nil
			}
			if result, err = r.gate.Reject(ctx, projectID, turnID, attempt, applied.Errors); err != nil {
				return StepResult{}, ApplyResult{}, "", err
			}
			message = RejectionMessage(applied.Errors)
		}
		if result.Status != StatusRetry {
			return result, applied, resp.Text, nil
		}
		changes, err := r.changeItems(ctx, projectID, instanceID)
		if err != nil {
			return StepResult{}, ApplyResult{}, "", err
		}
		input = append(append(append([]json.RawMessage{}, input...), resp.RawItems...), changes...)
		input = append(input, userItem(message))
	}
	return result, applied, "", nil
}

func (r *Runner) fail(projectID, turnID, code string) {
	if r.pub != nil {
		r.pub.PublishError(projectID, turnID, code)
	}
}
