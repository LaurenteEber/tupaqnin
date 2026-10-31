package graph

import (
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/text"
)

var blockParser = goldmark.New(goldmark.WithExtensions(extension.GFM)).Parser()

// Blocks splits a section's Markdown into the blocks its history is kept
// by (B-10): each top-level block in goldmark's order, a list counting
// as its items (a nested list is part of its item). Block n is
// Blocks(text)[n-1]: the raw Markdown it spans.
func Blocks(markdown string) []string {
	var out []string
	for _, r := range blockRanges(markdown) {
		out = append(out, markdown[r[0]:r[1]])
	}
	return out
}

// blockRanges are the byte ranges of markdown that Blocks returns, in
// the same order (an empty block is an empty range).
func blockRanges(markdown string) [][2]int {
	var out [][2]int
	for _, n := range blockNodes(markdown) {
		out = append(out, span(n))
	}
	return out
}

// blockNodes are the nodes of Blocks: each top-level block, a list as
// its items.
func blockNodes(markdown string) []ast.Node {
	doc := blockParser.Parse(text.NewReader([]byte(markdown)))
	var out []ast.Node
	for n := doc.FirstChild(); n != nil; n = n.NextSibling() {
		if list, ok := n.(*ast.List); ok {
			for item := list.FirstChild(); item != nil; item = item.NextSibling() {
				out = append(out, item)
			}
			continue
		}
		out = append(out, n)
	}
	return out
}

// Paragraph is one paragraph of the page's editor (P-78): paragraph n
// is block n of the history (B-10), so a list shows one paragraph per
// item. Text is the raw Markdown from the block's first line, its list
// marker or «## » included; Separator is the blank space up to the next
// paragraph (after the last one, up to the end). The paragraphs and
// their separators, put together, are the text byte for byte.
// [EL DISEÑO NO LO DICE]: the editor's paragraphs; Go gives them so the
// page counts blocks as the history does (rule 5).
type Paragraph struct {
	Text      string `json:"text"`
	Separator string `json:"separator"`
}

// Paragraphs splits a section's Markdown into the editor's paragraphs,
// one per block of Blocks. A text without blocks is one paragraph.
func Paragraphs(markdown string) []Paragraph {
	starts := paragraphStarts(markdown)
	if len(starts) == 0 {
		end := len(strings.TrimRight(markdown, " \t\r\n"))
		return []Paragraph{{Text: markdown[:end], Separator: markdown[end:]}}
	}
	out := make([]Paragraph, len(starts))
	for i, start := range starts {
		next := len(markdown)
		if i+1 < len(starts) {
			next = starts[i+1]
		}
		end := start + len(strings.TrimRight(markdown[start:next], " \t\r\n"))
		out[i] = Paragraph{Text: markdown[start:end], Separator: markdown[end:next]}
	}
	return out
}

// paragraphStarts is where each editor paragraph begins, one per block:
// the first one at the start of the text, the others at the start of
// the line where their block begins (before a list marker, a «## » or a
// quote's «> »; a fenced code block's opening fence). A block without
// lines of its own (a thematic break) begins at the first non-blank
// line after the previous one.
func paragraphStarts(markdown string) []int {
	nodes := blockNodes(markdown)
	starts := make([]int, len(nodes))
	prevEnd := 0
	for i, n := range nodes {
		r := span(n)
		s := lineStart(markdown, r[0])
		if fencedFirst(n) && s > 0 {
			s = lineStart(markdown, s-1)
		}
		if i > 0 && (r == [2]int{0, 0} || s <= starts[i-1]) {
			s = nextLine(markdown, max(prevEnd, starts[i-1]))
		}
		if i == 0 {
			s = 0
		}
		starts[i] = s
		prevEnd = max(r[1], s)
	}
	return starts
}

// fencedFirst reports whether the first leaf block of n is a fenced
// code block, whose opening fence is a line before its content.
func fencedFirst(n ast.Node) bool {
	for ; n != nil; n = n.FirstChild() {
		if _, ok := n.(*ast.FencedCodeBlock); ok {
			return true
		}
		if n.Type() != ast.TypeBlock {
			return false
		}
	}
	return false
}

// lineStart is the start of the line that holds byte at.
func lineStart(s string, at int) int {
	return strings.LastIndexByte(s[:at], '\n') + 1
}

// nextLine is the start of the first non-blank line after the line that
// holds byte at (the end of s if there is none).
func nextLine(s string, at int) int {
	i := strings.IndexByte(s[at:], '\n')
	if i < 0 {
		return len(s)
	}
	pos := at + i + 1
	for pos < len(s) {
		j := strings.IndexByte(s[pos:], '\n')
		if j < 0 || strings.TrimSpace(s[pos:pos+j]) != "" {
			return pos
		}
		pos += j + 1
	}
	return len(s)
}

// span is the byte range of the raw source a block covers: from its
// first to its last segment, its own lines and its descendants'.
func span(n ast.Node) [2]int {
	start, stop := -1, -1
	var walk func(ast.Node)
	walk = func(n ast.Node) {
		if n.Type() == ast.TypeBlock {
			lines := n.Lines()
			for i := 0; i < lines.Len(); i++ {
				seg := lines.At(i)
				if start < 0 || seg.Start < start {
					start = seg.Start
				}
				if seg.Stop > stop {
					stop = seg.Stop
				}
			}
		}
		if t, ok := n.(*ast.Text); ok {
			if start < 0 || t.Segment.Start < start {
				start = t.Segment.Start
			}
			if t.Segment.Stop > stop {
				stop = t.Segment.Stop
			}
		}
		for c := n.FirstChild(); c != nil; c = c.NextSibling() {
			walk(c)
		}
	}
	walk(n)
	if start < 0 {
		return [2]int{0, 0}
	}
	return [2]int{start, stop}
}
