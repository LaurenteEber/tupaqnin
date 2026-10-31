package cycle_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LaurenteEber/tupaqnin/internal/cycle"
)

func TestB20a_AssentsSignsAndEmptyAreNotAReason(t *testing.T) {
	for _, reason := range []string{"ok", "sí", "yes", "sure", "", "   ", "OK.", "¡Sí!", "Si", "de acuerdo...", "LGTM", "!!!", "👍", "👍👍 ."} {
		if cycle.ValidReason(reason) {
			t.Errorf("ValidReason(%q) = true, want false", reason)
		}
	}
	for _, reason := range []string{"Porque el comité pidió conteos.", "sí, porque compara igual", "ok porque 60 veces engaña", "7"} {
		if !cycle.ValidReason(reason) {
			t.Errorf("ValidReason(%q) = false, want true", reason)
		}
	}
}

// The Go list and the on-screen catalog (assent_list) are the same.
func TestB20a_AssentListMatchesTheCatalog(t *testing.T) {
	for lang, want := range cycle.AssentList {
		raw, err := os.ReadFile(filepath.Join("..", "..", "web", "src", "i18n", lang+".json"))
		if err != nil {
			t.Fatalf("read catalog %s: %v", lang, err)
		}
		var catalog map[string]string
		if err := json.Unmarshal(raw, &catalog); err != nil {
			t.Fatalf("catalog %s: %v", lang, err)
		}
		if got := catalog["assent_list"]; got != strings.Join(want, ", ") {
			t.Fatalf("%s assent_list: catalog %q, Go %q", lang, got, strings.Join(want, ", "))
		}
	}
}

// P-66, B-20 (a) refined on 31-oct (T-1): a «porque» made only of
// assents, joined or separated by punctuation, is not valid; one that
// adds anything else is.
func TestP66_OnlyAssentsJoinedByPunctuationAreNotAReason(t *testing.T) {
	for _, reason := range []string{"ok, dale", "sí, sí", "ok dale", "Sí. De acuerdo!", "ok,dale", "yes; sure", "listo, de acuerdo, bien"} {
		if cycle.ValidReason(reason) {
			t.Errorf("ValidReason(%q) = true, want false", reason)
		}
	}
	for _, reason := range []string{"12 años, ok", "ok, dale, porque compara igual", "de acuerdo con el comité", "sí, 7"} {
		if !cycle.ValidReason(reason) {
			t.Errorf("ValidReason(%q) = false, want true", reason)
		}
	}
}
