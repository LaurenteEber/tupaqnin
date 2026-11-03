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
// be joined by a point, a comma or a no-break or thin space. A power's exponent
// joins it afterwards (powerEnd).
var numberPattern = regexp.MustCompile("[0-9]+(?:[.,  ][0-9]+)*")

// lostSuperscript is a power of ten whose exponent came down to the line, as text
// copied from a PDF leaves it: the «104» of «3,14 × 104» (B-104). Exponents from 1 and
// up to two digits; scientificBefore is what must come before it, so «1920 × 1080»
// and «2 × 105» stay multiplications.
var (
	lostSuperscript  = regexp.MustCompile(`^10([1-9][0-9]?)$`)
	scientificBefore = regexp.MustCompile(`(?:^|[^0-9.,])[1-9][.,][0-9]+\s*×\s*$`)
)

// exponent reads the superscript digits and sign of a power.
var exponent = strings.NewReplacer("⁰", "0", "¹", "1", "²", "2", "³", "3", "⁴", "4",
	"⁵", "5", "⁶", "6", "⁷", "7", "⁸", "8", "⁹", "9", "⁻", "-", "−", "-")

// The ways a power is written (B-104): 10⁴ and 10^4 on a 10, and 1e4 on a 1.
var (
	tenPower   = regexp.MustCompile(`^(?:\^[-−]?[0-9]+|[⁰¹²³⁴-⁹⁻]+)`)
	onePower   = regexp.MustCompile(`^[eE][-+]?[0-9]{1,2}`)
	powerValue = regexp.MustCompile(`^(?:10(\^?)(.+)|1[eE]\+?(.+))$`)
)

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
	m := powerValue.FindStringSubmatch(value)
	switch {
	case m == nil:
		return value
	case m[3] != "":
		return "10^" + m[3]
	case m[1] == "" && !tenPower.MatchString(m[2]):
		return value // digits on a 10, as in 104: no power
	}
	return "10^" + exponent.Replace(m[2])
}

// LostPower is the power a number would be with its superscript lost, or "": a
// figure stored as «104» covers the «10⁴» you corrected it to.
func LostPower(value string) string {
	if m := lostSuperscript.FindStringSubmatch(value); m != nil {
		return "10^" + m[1]
	}
	return ""
}

// figureValue is a number's value where it stands: a lost superscript reads as the
// power it was only after a number in scientific form and a «×».
func figureValue(text string, start, end int) string {
	raw := text[start:end]
	if power := LostPower(raw); power != "" && scientificBefore.MatchString(text[:start]) {
		return power
	}
	return NormalizeFigure(raw)
}

// spanIsFigure says the number at text[start:end] is this figure, as it stands or
// as it is written: a stored «104» is still the «104» of «3,14 × 104».
func spanIsFigure(text string, start, end int, want string) bool {
	return figureValue(text, start, end) == want || NormalizeFigure(text[start:end]) == want
}

// FigureStillIn says the figure of an anchor is still in the text, written as the
// same figure: correcting «3,14 × 104» to «3,14 × 10⁴» did not remove it (B-104).
// Digits added around it make another figure: «13,14 × 10⁴» did.
func FigureStillIn(text, anchor string) bool {
	hay, needle := figureText(text), figureText(anchor)
	if needle == "" {
		return false
	}
	for from := 0; from <= len(hay); {
		i := strings.Index(hay[from:], needle)
		if i < 0 {
			return false
		}
		if wholeFigure(hay, from+i, from+i+len(needle)) {
			return true
		}
		from += i + 1
	}
	return false
}

// wholeFigure says what is at text[start:end] does not sit inside a longer number.
func wholeFigure(text string, start, end int) bool {
	before, _ := utf8.DecodeLastRuneInString(text[:start])
	after, size := utf8.DecodeRuneInString(text[end:])
	first, _ := utf8.DecodeRuneInString(text[start:end])
	last, _ := utf8.DecodeLastRuneInString(text[start:end])
	if unicode.IsDigit(first) && (unicode.IsDigit(before) || before == '^' || ((before == '.' || before == ',') && digitBefore(text[:start]))) {
		return false
	}
	if unicode.IsDigit(last) && (unicode.IsDigit(after) || after == '^' || ((after == '.' || after == ',') && digitAfter(text[end+size:]))) {
		return false
	}
	return true
}

func digitBefore(text string) bool {
	r, _ := utf8.DecodeLastRuneInString(strings.TrimRight(text, ".,"))
	return unicode.IsDigit(r)
}

func digitAfter(text string) bool {
	r, _ := utf8.DecodeRuneInString(text)
	return unicode.IsDigit(r)
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
	return strings.ReplaceAll(strings.ReplaceAll(b.String(), " ×", "×"), "× ", "×")
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
	taken := 0 // the exponent of a power is not a number of its own
	for _, loc := range numberPattern.FindAllStringIndex(text, -1) {
		start, end := loc[0], loc[1]
		if start < taken {
			continue
		}
		if labelBefore.MatchString(text[:start]) || codeBefore(text[:start]) {
			continue
		}
		if isListNumber(text, start, end) {
			continue
		}
		taken = powerEnd(text, start, end)
		out = append(out, []int{start, taken})
	}
	return out
}

// powerEnd is where a number ends with its power, if it has one: a 10 with its
// exponent, or a 1 with e and at most two digits. Only as a word of its own, so the
// digits of a hash, a code or a colour stay what they were (B-104).
func powerEnd(text string, start, end int) int {
	var power string
	rest := text[end:]
	switch text[start:end] {
	case "10":
		power = tenPower.FindString(rest)
	case "1":
		before, _ := utf8.DecodeLastRuneInString(text[:start])
		if !strings.ContainsRune("-_#:./", before) {
			power = onePower.FindString(rest)
		}
	}
	if power == "" || !endsPower(rest[len(power):], text[start:end] == "1") {
		return end
	}
	return end + len(power)
}

// endsPower: what follows a power leaves it whole: no letter or digit, nor a decimal
// part; after a 1e4, nor a hyphen or underscore, as in a code.
func endsPower(after string, code bool) bool {
	r, size := utf8.DecodeRuneInString(after)
	if unicode.IsLetter(r) || unicode.IsDigit(r) {
		return false
	}
	if (r == '.' || r == ',') && digitAfter(after[size:]) {
		return false
	}
	return !code || (r != '-' && r != '_')
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
