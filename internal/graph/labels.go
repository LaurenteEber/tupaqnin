package graph

import (
	"regexp"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// confusables maps look-alike letters and digits to the label alphabet
// (specification §3, rule 4: UTS #39 confusables).
// [EL DISEÑO NO LO DICE]: UTS #39 needs its data table, which no
// dependency of §7 carries; this hand table covers the Cyrillic, Greek
// and Latin look-alikes of [a-z0-9] that a label can contain.
var confusables = map[rune]rune{
	'а': 'a', 'с': 'c', 'ԁ': 'd', 'е': 'e', 'һ': 'h', 'і': 'i', 'ј': 'j',
	'к': 'k', 'м': 'm', 'о': 'o', 'р': 'p', 'ԛ': 'q', 'ѕ': 's', 'т': 't',
	'х': 'x', 'у': 'y', 'ԝ': 'w', 'в': 'b', 'н': 'h', 'ɡ': 'g', 'ӏ': 'l',
	'α': 'a', 'ε': 'e', 'ι': 'i', 'κ': 'k', 'ν': 'v', 'ο': 'o', 'ρ': 'p',
	'τ': 't', 'υ': 'u', 'χ': 'x', 'ω': 'w', 'ℓ': 'l', 'ı': 'i',
}

// NormalizeLabel prepares a text for label comparison (specification
// §3, rule 4): NFKC, without format characters, look-alikes mapped to
// the label alphabet, every dash to "-", lower case.
func NormalizeLabel(s string) string {
	var b strings.Builder
	for _, r := range norm.NFKC.String(s) {
		switch {
		case unicode.Is(unicode.Cf, r):
			continue
		case unicode.Is(unicode.Pd, r) || r == '−':
			b.WriteRune('-')
		default:
			if c, ok := confusables[unicode.ToLower(r)]; ok {
				r = c
			}
			b.WriteRune(unicode.ToLower(r))
		}
	}
	return b.String()
}

var provisionalInText = regexp.MustCompile(`(^|[^a-z0-9])(new-[1-9][0-9]*)([^0-9]|$)`)

// ProvisionalIDsIn returns the provisional ids (new-n, T-3) a text
// contains, compared after NormalizeLabel. A new-n is never valid inside
// the text (specification §3, rule 4).
func ProvisionalIDsIn(text string) []string {
	var out []string
	for _, m := range provisionalInText.FindAllStringSubmatch(NormalizeLabel(text), -1) {
		out = append(out, m[2])
	}
	return out
}
