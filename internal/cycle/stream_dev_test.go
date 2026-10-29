//go:build devroutes

package cycle_test

import (
	"context"
	"strings"
	"testing"
)

// P-28: a devroutes build also streams the model's raw text, as field
// "raw", next to the parsed fields.
func TestP28_DevBuildAlsoStreamsTheRawText(t *testing.T) {
	_, runner, _, pub := newCaptureRunner(t,
		loadAppFixture(t, "01-principal-valid.json"), loadAppFixture(t, "04-principal-no-work.json"))
	if err := runner.RunTurn(context.Background(), "p1", "turn-1", "Escribe."); err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	var raw, text string
	for _, s := range pub.segments {
		switch s.Field {
		case "raw":
			raw += s.Text
		case "text":
			text += s.Text
		}
	}
	if !strings.Contains(raw, `"triage_line"`) || text == "" {
		t.Fatalf("raw %q, text %q: want both", raw, text)
	}
}
