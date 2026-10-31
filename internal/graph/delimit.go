package graph

import (
	"strings"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

// P-83 (prompt-injection defense): text from a source, a tool result or
// the document reaches a model between fixed markers, one marker per
// line, so the model can tell data from instructions:
//
//	<<<BEGIN source>>>
//	…the text…
//	<<<END source>>>
//
// The markers are never part of the text: the model copies passages
// from between them, and nothing stored or compared carries them.

// Kinds of delimited text.
const (
	DataSource      = "source"      // a source's text, in a tool result or a brief
	DataPassage     = "passage"     // a new passage of the document, for the checker
	DataSection     = "section"     // a section's current text, for the judge and Wonder
	DataExplanation = "explanation" // the person's explanation, for Wonder
)

const (
	markerBegin = "<<<BEGIN"
	markerEnd   = "<<<END"
)

// Delimit wraps text between the begin and end markers of its kind.
// [EL DISEÑO NO LO DICE]: the markers' shape and how a text that holds
// one is handled. A marker inside the text is neutralized
// deterministically (P-88): every run of three «<» is broken with a
// space, so the text can never close (or open) a block; nothing else
// changes, and the stored text never does.
func Delimit(kind, text string) string {
	text = NeutralizeMarkers(text)
	if !strings.HasSuffix(text, "\n") {
		text += "\n"
	}
	return markerBegin + " " + kind + ">>>\n" + text + markerEnd + " " + kind + ">>>"
}

// NeutralizeMarkers breaks every «<<<» inside text, as NFKC reads it
// (P-88): a rune whose NFKC form is «<» (the full-width «＜», the small
// form «﹤») counts as «<», and a space goes before every third one in
// a row, in the original text. Every other rune is copied as it is, so
// a passage between the markers is still literal; a text without three
// in a row comes back unchanged. «<» has no case, so the word after it
// does not matter.
func NeutralizeMarkers(text string) string {
	var b strings.Builder
	copied, run := 0, 0
	for i, r := range text {
		if !isLessThan(r) {
			run = 0
			continue
		}
		run++
		if run == 3 {
			b.WriteString(text[copied:i])
			b.WriteByte(' ')
			copied, run = i, 1
		}
	}
	if copied == 0 {
		return text
	}
	b.WriteString(text[copied:])
	return b.String()
}

// HasMarker reports whether text holds the start of a marker, as a
// model that echoed one would write it: «<<<BEGIN» or «<<<END» after
// NFKC normalization, in any case (P-88).
func HasMarker(text string) bool {
	folded := strings.ToLower(norm.NFKC.String(text))
	return strings.Contains(folded, "<<<begin") || strings.Contains(folded, "<<<end")
}

// isLessThan reports whether r is «<» once NFKC-normalized.
func isLessThan(r rune) bool {
	if r < utf8.RuneSelf {
		return r == '<'
	}
	return norm.NFKC.String(string(r)) == "<"
}
