package graph

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Number is one number found in a text (B-20 b): Raw as it is written,
// Value with its separators normalized (fichas §3, «cifra sin
// decisión»), so a datum copied from a source compares equal.
type Number struct {
	Raw   string
	Value string
}

// numberPattern is a sequence with at least one digit; digit groups may
// be joined by a point, a comma or a no-break or thin space. A power goes with
// its digits: 10⁴, 10^4, 1e4 (B-104).
var numberPattern = regexp.MustCompile("[0-9]+(?:[.,  ][0-9]+)*(?:\\^[-−]?[0-9]+|[⁰¹²³⁴-⁹⁻]+|[eE][-+]?[0-9]+)?")

// lostSuperscript is a power of ten whose exponent came down to the line, as text
// copied from a PDF leaves it: the «104» of «3,14 × 104» (B-104). Exponents from 1 and
// up to two digits, so not «× 100» nor «× 1000», which are numbers.
var lostSuperscript = regexp.MustCompile(`^10([1-9][0-9]?)$`)

// exponent reads the superscript digits and sign of a power.
var exponent = strings.NewReplacer("⁰", "0", "¹", "1", "²", "2", "³", "3", "⁴", "4",
	"⁵", "5", "⁶", "6", "⁷", "7", "⁸", "8", "⁹", "9", "⁻", "-", "−", "-")

// powerForm splits a number with a power: its base digits, the mark, and what follows.
var powerForm = regexp.MustCompile(`^([0-9.]+?)(\^|[eE]|[⁰¹²³⁴-⁹⁻])(.*)$`)

// labelBefore matches the end of a text that makes the next digits part
// of a short label such as sec-3 or new-1 (B-20 b: labels do not count).
var labelBefore = regexp.MustCompile(`[a-z]{2,5}-$`)

// NormalizeNumber normalizes the separators of a number: no-break and
// thin spaces are dropped and a comma becomes a point. A rounded or
// converted number is still different (fichas §3).
func NormalizeNumber(raw string) string {
	return NormalizeFigure(raw)
}

// NormalizeFigure is the one form figures compare in: its separators normalized
// and a power as 10^4, however it is written (B-104): 10⁴, 10^4 and 1e4 are the same
// figure, and 10⁵ another one.
func NormalizeFigure(raw string) string {
	r := strings.NewReplacer(" ", "", " ", "", ",", ".")
	value := r.Replace(raw)
	m := powerForm.FindStringSubmatch(value)
	if m == nil {
		return value
	}
	base, mark, rest := m[1], m[2], m[3]
	switch mark {
	case "^":
		return base + "^" + exponent.Replace(rest)
	case "e", "E":
		power := strings.TrimPrefix(exponent.Replace(rest), "+")
		if base == "1" {
			return "10^" + power
		}
		return base + "×10^" + power
	}
	return base + "^" + exponent.Replace(mark+rest)
}

// figureValue is a number's value where it stands: a lost superscript reads as the
// power it was only after a «×».
func figureValue(text string, start, end int) string {
	raw := text[start:end]
	if m := lostSuperscript.FindStringSubmatch(raw); m != nil &&
		strings.HasSuffix(strings.TrimRightFunc(text[:start], unicode.IsSpace), "×") {
		return "10^" + m[1]
	}
	return NormalizeFigure(raw)
}

// FigureStillIn says the figure of an anchor is still in the text, written as the
// same figure: correcting «3,14 × 104» to «3,14 × 10⁴» did not remove it (B-104).
func FigureStillIn(text, anchor string) bool {
	return strings.Contains(figureText(text), figureText(anchor))
}

// figureText is a text with each number in its one form, for comparing passages.
func figureText(text string) string {
	compared := CompareString(text)
	var b strings.Builder
	last := 0
	for _, loc := range numberSpans(compared) {
		b.WriteString(compared[last:loc[0]])
		b.WriteString(figureValue(compared, loc[0], loc[1]))
		last = loc[1]
	}
	b.WriteString(compared[last:])
	return strings.NewReplacer(" ×", "×", "× ", "×").Replace(b.String())
}

// Numbers returns the numbers of a text, by B-20 (b): a sequence with at
// least one digit counts, a year included; list numbering at the start
// of a line and the digits of a short label do not.
func Numbers(text string) []Number {
	var out []Number
	for _, loc := range numberSpans(text) {
		raw := text[loc[0]:loc[1]]
		out = append(out, Number{Raw: raw, Value: figureValue(text, loc[0], loc[1])})
	}
	return out
}

// numberSpans returns where the numbers of a text are, by Numbers'
// rules.
func numberSpans(text string) [][]int {
	var out [][]int
	for _, loc := range numberPattern.FindAllStringIndex(text, -1) {
		start, end := loc[0], loc[1]
		if labelBefore.MatchString(text[:start]) || codeBefore(text[:start]) {
			continue
		}
		if isListNumber(text, start, end) {
			continue
		}
		out = append(out, loc)
	}
	return out
}

// codeBefore reports whether the digits that follow are attached to a
// letter, as in a study code such as T5 or B4: like a label, it is not a
// number (B-20 b, P-63).
// [EL DISEÑO NO LO DICE]: only a letter right before the digits; one
// right after («20µm») still leaves a number.
func codeBefore(before string) bool {
	r, _ := utf8.DecodeLastRuneInString(before)
	return r != utf8.RuneError && unicode.IsLetter(r)
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
