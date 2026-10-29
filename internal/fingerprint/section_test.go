package fingerprint_test

import (
	"strings"
	"testing"

	"github.com/LaurenteEber/tupaqnin/internal/fingerprint"
)

func TestS4_SameContentWithOtherFormattingGivesTheSameFingerprint(t *testing.T) {
	a := "# Título\n\nUn *énfasis* y **fuerte**, con [enlace](https://x.org).\n\n- uno\n- dos\n\n| a | b |\n|---|---|\n| 1 | 2 |\n\n```mermaid\ngraph TD\n```\n"
	b := "Título\n======\n\nUn _énfasis_   y __fuerte__,\ncon [enlace](https://x.org).\n\n* uno\n* dos\n\n|a|b|\n|-|-|\n|1|2|\n\n```mermaid\ngraph TD\n```"
	fa, fb := fingerprint.Section(a), fingerprint.Section(b)
	if fa != fb {
		t.Fatalf("same content, other formatting:\n%s\n%s", fa, fb)
	}
	if !strings.HasPrefix(fa, "tq-md/1:") {
		t.Fatalf("scheme: %s", fa)
	}
}

func TestS4_InvisibleContentChangesTheFingerprint(t *testing.T) {
	plain := "La regla da 1,5 veces."
	if fingerprint.Section(plain) == fingerprint.Section("La regla da 1,5​ veces.") {
		t.Fatal("a zero-width space did not change the fingerprint")
	}
}

func TestS4_ContentChangesChangeTheFingerprint(t *testing.T) {
	base := fingerprint.Section("Un *énfasis* aquí.")
	for _, other := range []string{
		"Un **énfasis** aquí.",       // emphasis level
		"Un *énfasis*aquí.",          // a space is content
		"## Un *énfasis* aquí.",      // a heading
		"1. Un *énfasis* aquí.",      // an ordered list
		"Un [*énfasis*](y) aquí.",    // a link
		"Un *énfasis* aquí también.", // text
	} {
		if fingerprint.Section(other) == base {
			t.Errorf("%q has the fingerprint of the base", other)
		}
	}
}
