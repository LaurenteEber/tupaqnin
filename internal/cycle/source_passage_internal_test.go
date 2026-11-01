package cycle

import (
	"testing"

	"github.com/LaurenteEber/tupaqnin/internal/graph"
)

// Backlog-r3 3.5 (verifier F1): from a match under verifyForm back to the
// source's own bytes. verifyForm changes lengths (NFC, spaces, curly quotes
// and dashes of 3 bytes become 1), so the stretch comes from a map of
// offsets, never from the match's position in the normalized text.
func TestB59_LiteralStretchMapsANormalizedMatchBackToTheSourceBytes(t *testing.T) {
	const nfd = "Café y água"
	for name, tc := range map[string]struct{ text, passage, want string }{
		"curly quotes and en dash": {"Antes. Bottled water held “many” particles – mostly small ones.",
			`Bottled water held "many" particles - mostly small`, "Bottled water held “many” particles – mostly small"},
		"case":                               {"Bottled water held many particles.", "bottled water held many", "Bottled water held many"},
		"multi-byte before the match":        {"«Título» — ñandú: el agua tiene más.", "el agua tiene", "el agua tiene"},
		"match ends on a multi-byte rune":    {"Dijo “sí” ayer.", `dijo "sí"`, "Dijo “sí”"},
		"NFD source, NFC passage":            {"Antes " + nfd + " después.", "Café y água", nfd},
		"NFC source, NFD passage":            {"Antes Café y água después.", nfd, "Café y água"},
		"NFD inside curly quotes":            {"Vio “Café” ayer.", `"Café"`, "“Café”"},
		"collapsed spaces keep the source's": {"Bottled  water\nheld many.", "bottled water held", "Bottled  water\nheld"},
		"not in the text":                    {"Otra cosa.", "nada de esto", ""},
		"blank passage":                      {"Otra cosa.", "   ", ""},
	} {
		if form, start, end := verifyMap(tc.text); form != verifyForm(tc.text) || len(start) != len(form) || len(end) != len(form) {
			t.Fatalf("%s: verifyMap %q (%d, %d), verifyForm %q", name, form, len(start), len(end), verifyForm(tc.text))
		}
		got := literalStretch(tc.text, tc.passage)
		if got != tc.want {
			t.Fatalf("%s: %q, want %q", name, got, tc.want)
		}
		if got != "" && len(graph.Compare(tc.text).Find(got)) == 0 {
			t.Fatalf("%s: %q is not literal for the gate in %q", name, got, tc.text)
		}
		if got != "" && verifyForm(got) != verifyForm(tc.passage) {
			t.Fatalf("%s: %q is not the passage's stretch %q", name, got, tc.passage)
		}
	}
}
