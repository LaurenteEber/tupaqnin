package cycle

import (
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// AssentList is, per language, what does not count as a «porque»
// (B-20 (a), T-1). The on-screen catalog carries the same list under
// assent_list in web/src/i18n/en.json and es.json; a test keeps both
// equal.
var AssentList = map[string][]string{
	"es": {"ok", "okay", "sí", "si", "dale", "listo", "de acuerdo", "bien", "vale", "va"},
	"en": {"ok", "okay", "yes", "yep", "sure", "fine", "agreed", "k", "lgtm"},
}

// normalizeReason compares without case, without accents and without
// the signs at the end (B-20 (a)).
// [EL DISEÑO NO LO DICE]: only the final signs are named; the opening
// ones (¡, ¿) are dropped too, so «¡ok!» is an assent like «ok!».
func normalizeReason(s string) string {
	var b strings.Builder
	for _, r := range norm.NFD.String(strings.ToLower(s)) {
		if !unicode.Is(unicode.Mn, r) {
			b.WriteRune(r)
		}
	}
	return strings.TrimFunc(b.String(), func(r rune) bool {
		return unicode.IsSpace(r) || unicode.IsPunct(r) || unicode.IsSymbol(r)
	})
}

// ValidReason reports whether a «porque» counts (fichas §3, «Tarjeta
// válida»): not empty, not only signs or emojis, and not an assent.
// [EL DISEÑO NO LO DICE]: the answer does not carry the person's
// language; the assents of both languages are refused.
func ValidReason(reason string) bool {
	n := normalizeReason(reason)
	hasWord := false
	for _, r := range n {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			hasWord = true
			break
		}
	}
	if !hasWord {
		return false
	}
	for _, list := range AssentList {
		for _, assent := range list {
			if n == normalizeReason(assent) {
				return false
			}
		}
	}
	return true
}
