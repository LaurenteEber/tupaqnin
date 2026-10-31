package graph

import "strings"

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
// deterministically: every «<<<BEGIN» becomes «<< <BEGIN» and every
// «<<<END» becomes «<< <END», so the text can never close (or open) a
// block; nothing else changes, and the stored text never does.
func Delimit(kind, text string) string {
	text = NeutralizeMarkers(text)
	if !strings.HasSuffix(text, "\n") {
		text += "\n"
	}
	return markerBegin + " " + kind + ">>>\n" + text + markerEnd + " " + kind + ">>>"
}

// NeutralizeMarkers breaks every marker start inside text (see Delimit).
func NeutralizeMarkers(text string) string {
	text = strings.ReplaceAll(text, markerBegin, "<< <BEGIN")
	return strings.ReplaceAll(text, markerEnd, "<< <END")
}

// HasMarker reports whether text holds the start of a marker, as a
// model that echoed one would write it.
func HasMarker(text string) bool {
	return strings.Contains(text, markerBegin) || strings.Contains(text, markerEnd)
}
