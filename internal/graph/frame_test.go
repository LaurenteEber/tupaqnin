package graph

import (
	"strings"
	"testing"
)

// M-4: a section's first line is the first sentence of its text, up to
// 200 characters [inicial].
func TestM4_FirstLineIsTheFirstSentenceUpTo200Characters(t *testing.T) {
	long := strings.Repeat("palabra ", 40) + "fin."
	tests := []struct {
		name, text, want string
	}{
		{"first sentence of the first block", "Primera oración. Segunda.\n\nOtro párrafo.", "Primera oración."},
		{"a decimal is not a sentence end", "La diferencia es de 1.5 veces. Luego.", "La diferencia es de 1.5 veces."},
		{"a heading is not the first sentence", "## Síntesis del capítulo\n\nTexto del párrafo. Otro.", "Texto del párrafo."},
		{"question mark ends it", "¿Entra en la comparación? No.", "¿Entra en la comparación?"},
		{"no end mark: the whole block", "Sin punto final", "Sin punto final"},
		{"empty text", "", ""},
		{"capped at 200 characters", long, strings.TrimSpace(string([]rune(long)[:200]))},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := firstSentence(tt.text); got != tt.want {
				t.Fatalf("firstSentence(%q) = %q, want %q", tt.text, got, tt.want)
			}
		})
	}
}

// §9: the icon is the first reason it meets, in the order figure ·
// conclusion or recommendation · several sections · marked by you.
func TestM1_IconFollowsTheReasonOrder(t *testing.T) {
	tests := []struct {
		reasons []Reason
		want    Reason
	}{
		{[]Reason{ReasonHumanMarked, ReasonTwoOrMoreSections, ReasonCoversFigure}, ReasonCoversFigure},
		{[]Reason{ReasonHumanMarked, ReasonTwoOrMoreSections, ReasonConclusionSection}, ReasonConclusionSection},
		{[]Reason{ReasonHumanMarked, ReasonTwoOrMoreSections}, ReasonTwoOrMoreSections},
		{[]Reason{ReasonHumanMarked}, ReasonHumanMarked},
	}
	for _, tt := range tests {
		if got := icon(tt.reasons); got != tt.want {
			t.Fatalf("icon(%v) = %s, want %s", tt.reasons, got, tt.want)
		}
	}
}
