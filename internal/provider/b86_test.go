package provider_test

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/LaurenteEber/tupaqnin/internal/provider"
)

// metaAnswering is a Meta client over a server that always answers status
// and body, with its log in logs.
func metaAnswering(t *testing.T, status int, body string, logs *bytes.Buffer) *provider.Meta {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return &provider.Meta{APIKey: "k", BaseURL: srv.URL, Client: srv.Client(),
		Logger: slog.New(slog.NewJSONHandler(logs, nil))}
}

// B-86: a 400 whose code, param or message contains «context» is the window
// full, recognised before the body is dropped; what the message said stays
// out of the error and of the log (T-10).
func TestB86_A400WithContextIsRecognisedAsAFullWindow(t *testing.T) {
	bodies := map[string]string{
		"code":    `{"error":{"code":"context_length_exceeded","type":"invalid_request_error"}}`,
		"param":   `{"error":{"code":"bad","type":"invalid_request_error","param":"context_size"}}`,
		"message": `{"error":{"code":"bad","type":"invalid_request_error","message":"Input exceeds the CONTEXT window: DOC-SECRET"}}`,
	}
	for name, body := range bodies {
		var logs bytes.Buffer
		_, err := metaAnswering(t, 400, body, &logs).Respond(context.Background(), provider.Request{Model: provider.PrincipalModel}, nil)
		if !errors.Is(err, provider.ErrContextFull) {
			t.Fatalf("%s: %v, want ErrContextFull", name, err)
		}
		if provider.Unavailable(err) {
			t.Fatalf("%s: a full window is not an unavailable model", name)
		}
		if strings.Contains(err.Error(), "DOC-SECRET") || strings.Contains(logs.String(), "DOC-SECRET") {
			t.Fatalf("%s: the message escaped: %v · %s", name, err, logs.String())
		}
	}
}

// B-86: only a 400 that says «context»: another 400, or a 500 that says it,
// stays the error it was.
func TestB86_OtherProviderErrorsAreNotAFullWindow(t *testing.T) {
	cases := []struct {
		status int
		body   string
	}{
		{400, `{"error":{"code":"bad","type":"invalid_request_error","message":"schema is wrong"}}`},
		{500, `{"error":{"code":"server","type":"server_error","message":"context store down"}}`},
	}
	for _, c := range cases {
		var logs bytes.Buffer
		_, err := metaAnswering(t, c.status, c.body, &logs).Respond(context.Background(), provider.Request{Model: provider.PrincipalModel}, nil)
		var perr *provider.ProviderError
		if !errors.As(err, &perr) || errors.Is(err, provider.ErrContextFull) {
			t.Fatalf("%d: %v", c.status, err)
		}
	}
}

// B-86, bloque-2 §4: ≤ 40 % free is ≥ 60 % used, a constant per model; a model
// the file does not know never crosses it.
func TestB86_TheThresholdIs60PercentOfTheWindowPerModel(t *testing.T) {
	cases := []struct {
		model  string
		tokens int
		want   bool
	}{
		{provider.PrincipalModel, 5999, false},
		{provider.PrincipalModel, 6000, true},
		{"unknown-model", 9000, false},
	}
	for _, c := range cases {
		if got := provider.ContextThresholdReached(c.model, c.tokens, 10000); got != c.want {
			t.Fatalf("%s at %d: %v", c.model, c.tokens, got)
		}
	}
	if provider.ContextThresholdReached(provider.PrincipalModel, 10, 0) {
		t.Fatal("no window, no threshold")
	}
}
