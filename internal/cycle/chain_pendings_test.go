package cycle_test

import (
	"context"
	"strings"
	"testing"

	"github.com/LaurenteEber/tupaqnin/internal/cycle"
)

// P-54, flujos §1: what the checks find «entra al siguiente triaje».
// The chained step's message carries the computed pendings (P-51)
// before the fixed message.
func TestP54_ChainedStepCarriesTheComputedPendings(t *testing.T) {
	_, runner, cap, _ := newCaptureRunner(t,
		stepFixture("loose-figure", `{"triage_line": "Construir: escribo.", "closing_risk": "none", "note": "Paso.",
			"new_text": [{"section_id": "new-1", "base_version": 0, "anchor": "", "operation": "insert_after", "text": "Quedan 7 casos comparables."}],
			"proposed_roles": [{"section_id": "new-1", "role": "other", "title": "Síntesis"}]}`),
		loadAppFixture(t, "04-principal-no-work.json"))
	if err := runner.RunTurn(context.Background(), "p1", "turn-1", "Escribe la síntesis."); err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	if len(cap.sent) != 2 {
		t.Fatalf("provider calls: %d, want 2", len(cap.sent))
	}
	got := lastUserText(t, cap.sent[1])
	for _, want := range []string{"sec-1 «Síntesis» · figure_without_decision: 7", "Quedan 7 casos comparables."} {
		if !strings.Contains(got, want) {
			t.Fatalf("chained message %q misses %q", got, want)
		}
	}
	if !strings.HasSuffix(got, cycle.ContinueMessage) {
		t.Fatalf("chained message %q does not end with the fixed message", got)
	}
}
