package provider

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
)

// DefaultBaseURL is Meta's Model API (docs: base_url
// https://api.meta.ai/v1, POST /v1/responses).
const DefaultBaseURL = "https://api.meta.ai/v1"

// Meta is a client for Meta's Responses API. It always sends
// store: false with include: ["reasoning.encrypted_content"] (§7),
// streams the response, and records each call (when a recorder is
// set) with its raw input and output items, byte-for-byte.
type Meta struct {
	APIKey   string
	BaseURL  string
	Client   *http.Client
	Recorder CallRecorder
	Logger   *slog.Logger
}

// NewMeta returns a client with the default base URL and HTTP client.
func NewMeta(apiKey string, recorder CallRecorder, logger *slog.Logger) *Meta {
	return &Meta{APIKey: apiKey, BaseURL: DefaultBaseURL, Client: http.DefaultClient,
		Recorder: recorder, Logger: logger}
}

var _ Provider = (*Meta)(nil)

type responsesRequest struct {
	Model        string          `json:"model"`
	Instructions string          `json:"instructions,omitempty"`
	Stream       bool            `json:"stream"`
	Store        bool            `json:"store"`
	Include      []string        `json:"include"`
	Reasoning    *reasoningParam `json:"reasoning,omitempty"`
	Text         *textParam      `json:"text,omitempty"`
}

// marshalWithInput marshals base (an object without an input field)
// and splices the raw input items in untouched: encoding/json
// compacts json.RawMessage, which would break byte-for-byte replay.
func marshalWithInput(base any, input []json.RawMessage) ([]byte, error) {
	b, err := json.Marshal(base)
	if err != nil {
		return nil, err
	}
	out := make([]byte, 0, len(b)+len(input)*32)
	out = append(out, b[:len(b)-1]...) // drop the closing brace
	out = append(out, `,"input":`...)
	out = append(out, joinRawItems(input)...)
	out = append(out, '}')
	return out, nil
}

type reasoningParam struct {
	Effort string `json:"effort"`
}

// textParam is the Responses API structured-output parameter
// (docs: text.format). [EL DISEÑO NO LO DICE] in the design files:
// the name "principal_output"; the micro-test 0.8 verifies the real
// shape against Meta.
type textParam struct {
	Format formatParam `json:"format"`
}

type formatParam struct {
	Type   string          `json:"type"`
	Name   string          `json:"name"`
	Strict bool            `json:"strict"`
	Schema json.RawMessage `json:"schema"`
}

type rawResponse struct {
	Output []json.RawMessage `json:"output"`
	Usage  struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
	Error *struct {
		Code string `json:"code"`
		Type string `json:"type"`
	} `json:"error"`
}

// Respond streams one call, forwarding each output_text delta, and
// returns the completed response's raw output items untouched.
func (m *Meta) Respond(ctx context.Context, req Request, onDelta func(Delta)) (Response, error) {
	body := responsesRequest{
		Model: req.Model, Instructions: req.Instructions,
		Stream: true, Store: false, Include: []string{"reasoning.encrypted_content"},
	}
	if req.Effort != "" {
		body.Reasoning = &reasoningParam{Effort: req.Effort}
	}
	if len(req.Schema) > 0 {
		body.Text = &textParam{Format: formatParam{
			Type: "json_schema", Name: "principal_output", Strict: true, Schema: req.Schema}}
	}
	payload, err := marshalWithInput(body, req.Input)
	if err != nil {
		return Response{}, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		m.baseURL()+"/responses", bytes.NewReader(payload))
	if err != nil {
		return Response{}, fmt.Errorf("build request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+m.APIKey)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")

	httpResp, err := m.client().Do(httpReq)
	if err != nil {
		return Response{}, fmt.Errorf("call provider: %w", err)
	}
	defer func() { _ = httpResp.Body.Close() }()

	if httpResp.StatusCode != http.StatusOK {
		return Response{}, m.httpError(httpResp)
	}

	resp, err := readResponseStream(httpResp.Body, onDelta)
	if err != nil {
		return Response{}, err
	}
	fillUsage(req.Model, &resp.Usage)
	if m.Logger != nil {
		// T-10: turn, role, model, tokens and cost only; never content.
		m.Logger.Info("model_call", "turn_id", req.TurnID, "role", string(req.Role),
			"model", req.Model, "effort", req.Effort,
			"input_tokens", resp.Usage.InputTokens, "output_tokens", resp.Usage.OutputTokens,
			"cost_usd", resp.Usage.CostUSD)
	}
	if err := recordCall(ctx, m.Recorder, req, resp); err != nil {
		return Response{}, err
	}
	return resp, nil
}

func (m *Meta) baseURL() string {
	if m.BaseURL != "" {
		return strings.TrimSuffix(m.BaseURL, "/")
	}
	return DefaultBaseURL
}

func (m *Meta) client() *http.Client {
	if m.Client != nil {
		return m.Client
	}
	return http.DefaultClient
}

// httpError turns a non-200 reply into a ProviderError with only its
// code and type. The raw body is read to find them and then dropped:
// it is never returned or logged (T-10).
func (m *Meta) httpError(httpResp *http.Response) error {
	raw, _ := io.ReadAll(io.LimitReader(httpResp.Body, 1<<20))
	perr := &ProviderError{Code: fmt.Sprintf("http_%d", httpResp.StatusCode), Type: "http_error"}
	var parsed struct {
		Error *struct {
			Code string `json:"code"`
			Type string `json:"type"`
		} `json:"error"`
	}
	if err := json.Unmarshal(raw, &parsed); err == nil && parsed.Error != nil {
		if parsed.Error.Code != "" {
			perr.Code = parsed.Error.Code
		}
		if parsed.Error.Type != "" {
			perr.Type = parsed.Error.Type
		}
	}
	if m.Logger != nil {
		m.Logger.Error("provider error", "code", perr.Code, "type", perr.Type,
			"status", httpResp.StatusCode)
	}
	return perr
}

// streamEvent is one SSE event of the Responses stream (docs:
// response.output_text.delta, response.completed, response.failed).
type streamEvent struct {
	Type     string          `json:"type"`
	Delta    string          `json:"delta"`
	Item     json.RawMessage `json:"item"`
	Response *rawResponse    `json:"response"`
}

func readResponseStream(r io.Reader, onDelta func(Delta)) (Response, error) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64<<10), 10<<20)

	var completed *rawResponse
	var doneItems []json.RawMessage
	var text strings.Builder

	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "" || data == "[DONE]" {
			continue
		}
		var ev streamEvent
		if err := json.Unmarshal([]byte(data), &ev); err != nil {
			return Response{}, fmt.Errorf("decode stream event: %w", err)
		}
		switch ev.Type {
		case "response.output_text.delta":
			text.WriteString(ev.Delta)
			if onDelta != nil {
				onDelta(Delta{Text: ev.Delta})
			}
		case "response.output_item.done":
			if len(ev.Item) > 0 {
				doneItems = append(doneItems, ev.Item)
			}
		case "response.failed":
			if ev.Response != nil && ev.Response.Error != nil {
				return Response{}, &ProviderError{
					Code: ev.Response.Error.Code, Type: ev.Response.Error.Type}
			}
			return Response{}, &ProviderError{Code: "response_failed", Type: "stream"}
		case "response.completed":
			completed = ev.Response
		}
	}
	if err := scanner.Err(); err != nil {
		return Response{}, fmt.Errorf("read stream: %w", err)
	}
	if completed == nil {
		return Response{}, &ProviderError{Code: "stream_ended", Type: "stream"}
	}
	if completed.Error != nil {
		return Response{}, &ProviderError{Code: completed.Error.Code, Type: completed.Error.Type}
	}
	items := completed.Output
	if len(items) == 0 {
		items = doneItems
	}
	resp := Response{RawItems: items}
	resp.Text = textOfItems(items)
	if resp.Text == "" {
		resp.Text = text.String()
	}
	resp.Usage.InputTokens = completed.Usage.InputTokens
	resp.Usage.OutputTokens = completed.Usage.OutputTokens
	return resp, nil
}

// CountTokens counts input tokens with Meta's endpoint (B-5):
// POST /v1/responses/input_tokens with the model and the input.
func (m *Meta) CountTokens(ctx context.Context, model string, input []json.RawMessage) (int, error) {
	payload, err := marshalWithInput(struct {
		Model string `json:"model"`
	}{Model: model}, input)
	if err != nil {
		return 0, fmt.Errorf("marshal count request: %w", err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		m.baseURL()+"/responses/input_tokens", bytes.NewReader(payload))
	if err != nil {
		return 0, fmt.Errorf("build request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+m.APIKey)
	httpReq.Header.Set("Content-Type", "application/json")

	httpResp, err := m.client().Do(httpReq)
	if err != nil {
		return 0, fmt.Errorf("count tokens: %w", err)
	}
	defer func() { _ = httpResp.Body.Close() }()
	if httpResp.StatusCode != http.StatusOK {
		return 0, m.httpError(httpResp)
	}
	var out struct {
		InputTokens int `json:"input_tokens"`
	}
	if err := json.NewDecoder(httpResp.Body).Decode(&out); err != nil {
		return 0, fmt.Errorf("decode count: %w", err)
	}
	return out.InputTokens, nil
}
