package graph

import (
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
	src := []byte(markdown)
	doc := blockParser.Parse(text.NewReader(src))
	var out []string
	for n := doc.FirstChild(); n != nil; n = n.NextSibling() {
		if list, ok := n.(*ast.List); ok {
			for item := list.FirstChild(); item != nil; item = item.NextSibling() {
				out = append(out, span(item, src))
			}
			continue
		}
		out = append(out, span(n, src))
	}
	return out
}

// span is the raw source a block covers: from its first to its last
// segment, its own lines and its descendants'.
func span(n ast.Node, src []byte) string {
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
		return ""
	}
	return string(src[start:stop])
}
