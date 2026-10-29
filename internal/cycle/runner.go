package cycle

import (
	"bytes"
	"context"
	"crypto/rand"
	_ "embed"
	"encoding/base32"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
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
}

// NewRunner wires a runner. count is the token counter already bound
// to the principal's model (B-5).
func NewRunner(db *store.DB, prov provider.Provider, count memory.TokenCounter, pub TurnPublisher) *Runner {
	return &Runner{db: db, prov: prov, count: count, pub: pub,
		gate: NewGate(db), memory: memory.New(db)}
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
func (r *Runner) StartTurn(projectID, text string) (string, error) {
	turnID, err := newTurnID()
	if err != nil {
		return "", err
	}
	if r.pub != nil {
		r.pub.BeginTurn(projectID, turnID)
	}
	go func() {
		if err := r.RunTurn(context.Background(), projectID, turnID, text); err != nil && r.Logger != nil {
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
// model_call rows: what was sent and what it answered, in order,
// byte-for-byte (B-7).
func (r *Runner) conversation(ctx context.Context, projectID, instanceID string) ([]json.RawMessage, error) {
	calls, err := r.db.ModelCalls(ctx, projectID, instanceID)
	if err != nil {
		return nil, err
	}
	var out []json.RawMessage
	for _, call := range calls {
		sent, err := splitItems(call.InputItems)
		if err != nil {
			return nil, err
		}
		answered, err := splitItems(call.RawItems)
		if err != nil {
			return nil, err
		}
		out = append(out, sent...)
		out = append(out, answered...)
	}
	return out, nil
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

func (r *Runner) call(ctx context.Context, projectID, turnID, instanceID string, input []json.RawMessage) (provider.Response, error) {
	schema, err := r.transportSchema()
	if err != nil {
		return provider.Response{}, err
	}
	return r.prov.Respond(ctx, provider.Request{
		ProjectID: projectID, TurnID: turnID, InstanceID: instanceID,
		Role: provider.RolePrincipal, Model: provider.PrincipalModel,
		Effort: provider.PrincipalEffort, Instructions: principalInstructions,
		Input: input, Schema: schema, SchemaStrict: true,
	}, func(d provider.Delta) {
		if r.pub != nil {
			r.pub.PublishDelta(projectID, turnID, d.Text)
		}
	})
}

type stepPayload struct {
	Status      StepStatus      `json:"status"`
	Errors      []string        `json:"errors,omitempty"`
	TriageLine  string          `json:"triage_line,omitempty"`
	ClosingRisk string          `json:"closing_risk,omitempty"`
	Note        string          `json:"note,omitempty"`
	Deferred    json.RawMessage `json:"deferred,omitempty"`
}

func (r *Runner) publishStep(projectID, turnID string, result StepResult, outputText string) {
	payload := stepPayload{Status: result.Status, Errors: result.Errors}
	if result.Status == StatusAccepted {
		var out struct {
			TriageLine  string          `json:"triage_line"`
			ClosingRisk string          `json:"closing_risk"`
			Note        string          `json:"note"`
			Deferred    json.RawMessage `json:"deferred"`
		}
		if err := json.Unmarshal([]byte(outputText), &out); err == nil {
			payload.TriageLine = out.TriageLine
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

// RunTurn runs one turn synchronously. StartTurn wraps it in a
// goroutine; tests call it directly.
func (r *Runner) RunTurn(ctx context.Context, projectID, turnID, text string) error {
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

	instanceID, startupInput, started, err := r.ensureInstance(ctx, projectID)
	if err != nil {
		r.fail(projectID, turnID, "turn_failed")
		return err
	}

	var input []json.RawMessage
	if started {
		// The startup already carries this message as the current
		// task (block 2 §3), so it is not appended again.
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

	resp, err := r.call(ctx, projectID, turnID, instanceID, input)
	if err != nil {
		r.fail(projectID, turnID, "provider_error")
		return nil
	}
	result, err := r.gate.EndStep(ctx, projectID, turnID, 1, json.RawMessage(resp.Text))
	if err != nil {
		r.fail(projectID, turnID, "turn_failed")
		return err
	}
	if result.Status == StatusRetry {
		retryInput := append(append([]json.RawMessage{}, input...), resp.RawItems...)
		retryInput = append(retryInput, userItem(RetryMessage(result.Errors)))
		resp2, err := r.call(ctx, projectID, turnID, instanceID, retryInput)
		if err != nil {
			r.fail(projectID, turnID, "provider_error")
			return nil
		}
		result, err = r.gate.EndStep(ctx, projectID, turnID, 2, json.RawMessage(resp2.Text))
		if err != nil {
			r.fail(projectID, turnID, "turn_failed")
			return err
		}
		resp = resp2
	}
	r.publishStep(projectID, turnID, result, resp.Text)
	return nil
}

func (r *Runner) fail(projectID, turnID, code string) {
	if r.pub != nil {
		r.pub.PublishError(projectID, turnID, code)
	}
}
