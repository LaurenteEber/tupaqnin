package provider_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/LaurenteEber/tupaqnin/internal/provider"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

func openTempDB(t *testing.T) *store.DB {
	t.Helper()
	db, err := store.Open(context.Background(), t.TempDir())
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := db.Writer.ExecContext(context.Background(),
		`INSERT INTO project (id, created_at) VALUES ('p1', '2026-10-28T00:00:00.000Z')`); err != nil {
		t.Fatalf("seed project: %v", err)
	}
	return db
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// sseReply is a canned streamed reply: one reasoning item (odd
// spacing on purpose, to prove bytes are kept) and one message item.
const sseReply = "data: {\"type\":\"response.output_text.delta\",\"delta\":\"Hello \"}\n\n" +
	"data: {\"type\":\"response.output_text.delta\",\"delta\":\"world\"}\n\n" +
	"data: {\"type\":\"response.completed\",\"response\":{\"output\":[" +
	`{"type": "reasoning",  "id" : "rs_1", "summary": [], "encrypted_content": "abc"}` + "," +
	`{"type":"message","role":"assistant","content":[{"type":"output_text","text":"Hello world"}]}` +
	`],"usage":{"input_tokens":100,"output_tokens":10}}}` + "\n\n"

func cannedMeta(bodies *[][]byte, status int, contentType, reply string, rec provider.CallRecorder, logger *slog.Logger) *provider.Meta {
	m := provider.NewMeta("test-key", rec, logger)
	m.BaseURL = "https://meta.test/v1"
	m.Client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		b, _ := io.ReadAll(r.Body)
		if bodies != nil {
			*bodies = append(*bodies, b)
		}
		return &http.Response{StatusCode: status, Header: http.Header{"Content-Type": {contentType}},
			Body: io.NopCloser(strings.NewReader(reply)), Request: r}, nil
	})}
	return m
}

func TestT11_RawItemsResentByteForByte(t *testing.T) {
	var bodies [][]byte
	m := cannedMeta(&bodies, http.StatusOK, "text/event-stream", sseReply, nil, nil)
	ctx := context.Background()
	req := provider.Request{ProjectID: "p1", TurnID: "t1", Role: provider.RolePrincipal,
		Model: provider.PrincipalModel, Effort: provider.PrincipalEffort,
		Input: []json.RawMessage{json.RawMessage(`{"role":"user","content":"hi"}`)}}

	var deltas []string
	resp, err := m.Respond(ctx, req, func(d provider.Delta) { deltas = append(deltas, d.Text) })
	if err != nil {
		t.Fatalf("Respond: %v", err)
	}
	if resp.Text != "Hello world" || strings.Join(deltas, "") != "Hello world" {
		t.Fatalf("text/deltas: got %q / %v", resp.Text, deltas)
	}
	if len(resp.RawItems) != 2 {
		t.Fatalf("raw items: got %d, want 2", len(resp.RawItems))
	}

	// Resend the raw items as the next input: they must arrive untouched.
	req.Input = resp.RawItems
	if _, err := m.Respond(ctx, req, nil); err != nil {
		t.Fatalf("Respond (replay): %v", err)
	}
	if len(bodies) != 2 {
		t.Fatalf("requests captured: got %d, want 2", len(bodies))
	}
	want := `{"type": "reasoning",  "id" : "rs_1", "summary": [], "encrypted_content": "abc"}`
	if !strings.Contains(string(bodies[1]), want) {
		t.Fatalf("replayed item not byte-for-byte:\nwant substring: %s\nin: %s", want, bodies[1])
	}

	var sent struct {
		Store   bool     `json:"store"`
		Include []string `json:"include"`
	}
	if err := json.Unmarshal(bodies[0], &sent); err != nil {
		t.Fatalf("decode request: %v", err)
	}
	if sent.Store || len(sent.Include) != 1 || sent.Include[0] != "reasoning.encrypted_content" {
		t.Fatalf("request must send store:false and the encrypted-content include: %+v", sent)
	}
}

func TestT10_ProviderErrorKeepsRawBodyOutOfErrorAndLog(t *testing.T) {
	const marker = "raw-secret-body-marker"
	var logBuf strings.Builder
	logger := slog.New(slog.NewJSONHandler(&logBuf, nil))
	m := cannedMeta(nil, http.StatusBadRequest, "application/json",
		`{"error":{"code":null,"type":"invalid_request_error","param":"text","message":"`+marker+`"}}`, nil, logger)

	_, err := m.Respond(context.Background(), provider.Request{
		ProjectID: "p1", Model: provider.PrincipalModel,
		Input: []json.RawMessage{json.RawMessage(`{"role":"user","content":"hi"}`)},
	}, nil)
	if err == nil {
		t.Fatal("Respond: got no error, want a ProviderError")
	}
	var perr *provider.ProviderError
	if !errors.As(err, &perr) {
		t.Fatalf("Respond error type: got %T, want *ProviderError", err)
	}
	if perr.Code != "http_400" || perr.Type != "invalid_request_error" || perr.Param != "text" {
		t.Fatalf("ProviderError: got %+v, want http_400 (code was null)/invalid_request_error/text", perr)
	}
	if strings.Contains(err.Error(), marker) || strings.Contains(logBuf.String(), marker) {
		t.Fatalf("raw provider body leaked: err=%q log=%q", err, logBuf.String())
	}
}

func TestT11_EachMetaCallWritesAModelCallRow(t *testing.T) {
	db := openTempDB(t)
	var bodies [][]byte
	m := cannedMeta(&bodies, http.StatusOK, "text/event-stream", sseReply, db, nil)

	resp, err := m.Respond(context.Background(), provider.Request{
		ProjectID: "p1", TurnID: "turn-1", InstanceID: "inst-1", Role: provider.RolePrincipal,
		Model: provider.PrincipalModel, Effort: provider.PrincipalEffort,
		Input: []json.RawMessage{json.RawMessage(`{"role":"user","content":"hi"}`)},
	}, nil)
	if err != nil {
		t.Fatalf("Respond: %v", err)
	}

	var role, modelEffort, inputItems, rawItems string
	var inTok, outTok, window int
	var cost float64
	err = db.Reader.QueryRowContext(context.Background(),
		`SELECT role, model_and_effort, input_items, raw_items,
			input_tokens, output_tokens, context_window, cost
		 FROM model_call WHERE project_id = 'p1'`).
		Scan(&role, &modelEffort, &inputItems, &rawItems, &inTok, &outTok, &window, &cost)
	if err != nil {
		t.Fatalf("read model_call: %v", err)
	}
	if role != "principal" || modelEffort != "muse-spark-1.3 medium" {
		t.Fatalf("model_call role/model: got %q / %q", role, modelEffort)
	}
	if inTok != 100 || outTok != 10 || window != 1_048_576 {
		t.Fatalf("model_call usage: got %d/%d window %d", inTok, outTok, window)
	}
	wantRaw := "[" + string(resp.RawItems[0]) + "," + string(resp.RawItems[1]) + "]"
	if rawItems != wantRaw {
		t.Fatalf("model_call raw_items:\ngot  %s\nwant %s", rawItems, wantRaw)
	}
	if !strings.Contains(inputItems, `"hi"`) {
		t.Fatalf("model_call input_items does not hold what was sent: %s", inputItems)
	}
}

func TestB2_SchemaStrictFlagReachesTheRequest(t *testing.T) {
	cases := []struct {
		name   string
		strict bool
		want   string
	}{
		{"without strict by default", false, `"strict":false`},
		{"with strict when asked", true, `"strict":true`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var bodies [][]byte
			m := cannedMeta(&bodies, http.StatusOK, "text/event-stream", sseReply, nil, nil)
			_, err := m.Respond(context.Background(), provider.Request{
				ProjectID: "p1", Model: provider.PrincipalModel,
				Input:        []json.RawMessage{json.RawMessage(`{"role":"user","content":"hi"}`)},
				Schema:       json.RawMessage(`{"type":"object"}`),
				SchemaStrict: tc.strict,
			}, nil)
			if err != nil {
				t.Fatalf("Respond: %v", err)
			}
			if len(bodies) != 1 || !strings.Contains(string(bodies[0]), tc.want) {
				t.Fatalf("request does not carry %s: %s", tc.want, bodies)
			}
		})
	}
}

func TestP8_MetaCallHasAFiveMinuteCap(t *testing.T) {
	m := provider.NewMeta("test-key", nil, nil)
	if m.Client == nil {
		t.Fatal("NewMeta: got a nil HTTP client, want one with a timeout")
	}
	if m.Client.Timeout != 5*time.Minute {
		t.Fatalf("Meta call timeout: got %v, want 5m (P-8)", m.Client.Timeout)
	}
}

func TestB5_MetaCountTokensUsesTheCountingEndpoint(t *testing.T) {
	m := provider.NewMeta("test-key", nil, nil)
	m.BaseURL = "https://meta.test/v1"
	m.Client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/v1/responses/input_tokens" {
			t.Errorf("count path: got %q", r.URL.Path)
		}
		return &http.Response{StatusCode: http.StatusOK,
			Header:  http.Header{"Content-Type": {"application/json"}},
			Body:    io.NopCloser(strings.NewReader(`{"object":"response.input_tokens","input_tokens":42}`)),
			Request: r}, nil
	})}

	n, err := m.CountTokens(context.Background(), provider.PrincipalModel,
		[]json.RawMessage{json.RawMessage(`{"role":"user","content":"hi"}`)})
	if err != nil {
		t.Fatalf("CountTokens: %v", err)
	}
	if n != 42 {
		t.Fatalf("CountTokens: got %d, want 42", n)
	}
}
