package cycle

import (
	"strings"
	"testing"
)

// P-87 (U-6): «Confirmar» does not condition the start; the chain goes
// on by itself (B-17). The understanding's instructions say the work
// continues whether the human confirms, corrects or does neither, and
// forbid telling the human it continues only if they confirm or
// correct.
func TestP87_UnderstandingSaysTheWorkContinuesEitherWay(t *testing.T) {
	prompt := strings.Join(strings.Fields(understandingInstructions), " ")
	for _, want := range []string{
		"the work continues whether the human confirms it, corrects it or does neither",
		"never write that the work continues only if the human confirms or corrects",
	} {
		if !strings.Contains(prompt, want) {
			t.Errorf("the understanding prompt does not say %q", want)
		}
	}
	if strings.Contains(prompt, "confirms it once, or corrects it, and") {
		t.Errorf("the understanding prompt still ties the work to a confirmation or a correction")
	}
}
