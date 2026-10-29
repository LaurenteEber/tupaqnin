//go:build !devroutes

package cycle_test

import (
	"context"
	"strings"
	"testing"
)

// P-28: the stream carries the parsed triage line, text and note,
// never the raw JSON.
func TestP28_StreamShowsTheParsedTextNeverTheRawJSON(t *testing.T) {
	_, runner, _, pub := newCaptureRunner(t,
		loadAppFixture(t, "01-principal-valid.json"), loadAppFixture(t, "04-principal-no-work.json"))
	if err := runner.RunTurn(context.Background(), "p1", "turn-1", "Escribe."); err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	streamed := strings.Join(pub.deltas, "")
	for _, want := range []string{
		"Construir: redacto la primera sección del pedido.",
		"# Alcance\n\nPrimera versión del alcance.",
		"Intenté una primera sección mínima",
	} {
		if !strings.Contains(streamed, want) {
			t.Fatalf("stream misses %q:\n%s", want, streamed)
		}
	}
	for _, raw := range []string{"{", `"triage_line"`, "section_id", "Confirmar el destinatario"} {
		if strings.Contains(streamed, raw) {
			t.Fatalf("stream carries raw JSON %q:\n%s", raw, streamed)
		}
	}
}

// P-28: each streamed piece carries its field, so the page shows the
// triage line, the text and the note apart; the raw stream exists only
// in a devroutes build.
func TestP28_StreamedPiecesCarryTheirFieldAndNoRawInTheDemoBuild(t *testing.T) {
	_, runner, _, pub := newCaptureRunner(t,
		loadAppFixture(t, "01-principal-valid.json"), loadAppFixture(t, "04-principal-no-work.json"))
	if err := runner.RunTurn(context.Background(), "p1", "turn-1", "Escribe."); err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	var starts []string
	byField := map[string]string{}
	for _, s := range pub.segments {
		if s.Field == "raw" {
			t.Fatalf("raw segment in the demo build: %q", s.Text)
		}
		if s.Start {
			starts = append(starts, s.Field)
		}
		byField[s.Field] += s.Text
	}
	want := []string{"triage_line", "text", "note", "triage_line", "note"}
	if strings.Join(starts, ",") != strings.Join(want, ",") {
		t.Fatalf("value starts: %v, want %v", starts, want)
	}
	if !strings.HasPrefix(byField["triage_line"], "Construir: redacto la primera sección del pedido.") ||
		byField["text"] != "# Alcance\n\nPrimera versión del alcance." {
		t.Fatalf("fields: %q", byField)
	}
}
