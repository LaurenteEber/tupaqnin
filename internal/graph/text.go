package graph

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

// Compared is a text prepared for literal comparison (specification §3,
// rule 2; T-4): NFC, with every run of white space collapsed to one
// space. Start and End map each byte of Text back to the byte range of
// Orig it came from, so a match can be applied to the original.
type Compared struct {
	Orig  string // the NFC text
	Text  string
	Start []int
	End   []int
}

// Compare prepares s for literal comparison.
func Compare(s string) Compared {
	orig := norm.NFC.String(s)
	c := Compared{Orig: orig}
	var b strings.Builder
	inSpace := false
	for i := 0; i < len(orig); {
		r, size := utf8.DecodeRuneInString(orig[i:])
		if unicode.IsSpace(r) {
			if inSpace {
				c.End[len(c.End)-1] = i + size
			} else {
				b.WriteByte(' ')
				c.Start = append(c.Start, i)
				c.End = append(c.End, i+size)
				inSpace = true
			}
			i += size
			continue
		}
		inSpace = false
		b.WriteString(orig[i : i+size])
		for k := 0; k < size; k++ {
			c.Start = append(c.Start, i)
			c.End = append(c.End, i+size)
		}
		i += size
	}
	c.Text = b.String()
	return c
}

// CompareString is the compared form of s, without the offset map.
func CompareString(s string) string { return Compare(s).Text }

// Find returns the byte ranges of Orig where needle occurs, counting
// overlapping occurrences. needle is compared in the same form.
func (c Compared) Find(needle string) [][2]int {
	n := CompareString(needle)
	if n == "" {
		return nil
	}
	var out [][2]int
	for from := 0; from <= len(c.Text)-len(n); {
		i := strings.Index(c.Text[from:], n)
		if i < 0 {
			break
		}
		at := from + i
		out = append(out, [2]int{c.Start[at], c.End[at+len(n)-1]})
		_, size := utf8.DecodeRuneInString(c.Text[at:])
		from = at + size
	}
	return out
}
