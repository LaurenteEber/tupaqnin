package provider

import (
	"context"
	"encoding/json"
	"fmt"
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
}

// Response is one completed call: every raw output item, the joined
// output text, and the reported usage.
type Response struct {
	RawItems []json.RawMessage
	Text     string
	Usage    Usage
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
}

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
	return rec.RecordModelCall(ctx, store.ModelCall{
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
	Name       string            `json:"name"`
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
type Fake struct {
	Recorder CallRecorder

	fixtures []Fixture
	next     int
}

// NewFake returns a fake that replays the fixtures in the given order.
func NewFake(fixtures ...Fixture) *Fake { return &Fake{fixtures: fixtures} }

var _ Provider = (*Fake)(nil)

// Respond emits the next fixture's deltas at its DeltaDelay rhythm
// and returns its recorded items byte-for-byte.
func (f *Fake) Respond(ctx context.Context, req Request, onDelta func(Delta)) (Response, error) {
	if len(f.fixtures) == 0 {
		return Response{}, &ProviderError{Code: "no_fixtures", Type: "fake"}
	}
	fx := f.fixtures[f.next%len(f.fixtures)]
	f.next++
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
	resp := Response{RawItems: fx.Items, Usage: fx.Usage}
	resp.Text = textOfItems(fx.Items)
	if resp.Text == "" {
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
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		}
		if err := json.Unmarshal(raw, &item); err != nil || item.Type != "message" {
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
