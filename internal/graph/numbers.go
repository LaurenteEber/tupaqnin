package graph

import (
	"regexp"
	"strings"
)

// Number is one number found in a text (B-20 b): Raw as it is written,
// Value with its separators normalized (fichas §3, «cifra sin
// decisión»), so a datum copied from a source compares equal.
type Number struct {
	Raw   string
	Value string
}

// numberPattern is a sequence with at least one digit; digit groups may
// be joined by a point, a comma or a no-break or thin space.
var numberPattern = regexp.MustCompile("[0-9]+(?:[.,  ][0-9]+)*")

// labelBefore matches the end of a text that makes the next digits part
// of a short label such as sec-3 or new-1 (B-20 b: labels do not count).
var labelBefore = regexp.MustCompile(`[a-z]{2,5}-$`)

// NormalizeNumber normalizes the separators of a number: no-break and
// thin spaces are dropped and a comma becomes a point. A rounded or
// converted number is still different (fichas §3).
func NormalizeNumber(raw string) string {
	r := strings.NewReplacer(" ", "", " ", "", ",", ".")
	return r.Replace(raw)
}

// Numbers returns the numbers of a text, by B-20 (b): a sequence with at
// least one digit counts, a year included; list numbering at the start
// of a line and the digits of a short label do not.
func Numbers(text string) []Number {
	var out []Number
	for _, loc := range numberPattern.FindAllStringIndex(text, -1) {
		start, end := loc[0], loc[1]
		if labelBefore.MatchString(text[:start]) {
			continue
		}
		if isListNumber(text, start, end) {
			continue
		}
		raw := text[start:end]
		out = append(out, Number{Raw: raw, Value: NormalizeNumber(raw)})
	}
	return out
}

// isListNumber reports whether the number at text[start:end] opens a
// Markdown ordered list item: only spaces before it on its line, then
// "." or ")" and a space or the end of the line.
func isListNumber(text string, start, end int) bool {
	lineStart := strings.LastIndexByte(text[:start], '\n') + 1
	if strings.TrimLeft(text[lineStart:start], " \t") != "" {
		return false
	}
	rest := text[end:]
	if len(rest) == 0 || (rest[0] != '.' && rest[0] != ')') {
		return false
	}
	return len(rest) == 1 || rest[1] == ' ' || rest[1] == '\t' || rest[1] == '\n'
}

// NumberValues returns the set of normalized values of the numbers of a
// text.
func NumberValues(text string) map[string]bool {
	out := make(map[string]bool)
	for _, n := range Numbers(text) {
		out[n.Value] = true
	}
	return out
}
