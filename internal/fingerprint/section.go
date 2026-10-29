package fingerprint

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/gowebpki/jcs"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	extast "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/text"
	"golang.org/x/text/unicode/norm"
)

// SectionScheme names the section fingerprint (specification §4).
const SectionScheme = "tq-md/1"

var markdown = goldmark.New(goldmark.WithExtensions(extension.GFM))

// padding is a run of spaces or tabs inside text; one space keeps the
// content (a word boundary), more is formatting.
var padding = regexp.MustCompile(`[ \t]{2,}`)

// node is the canonical form of one Markdown node: its kind, its text in
// NFC, its level, its link destination, its code language and literal
// content, and its children. List markers, the emphasis character,
// padding and escapes leave no trace.
type node struct {
	Kind     string  `json:"k"`
	Text     string  `json:"t,omitempty"`
	Level    int     `json:"l,omitempty"`
	Dest     string  `json:"d,omitempty"`
	Lang     string  `json:"g,omitempty"`
	Ordered  bool    `json:"o,omitempty"`
	Children []*node `json:"c,omitempty"`
}

// Section returns the section fingerprint tq-md/1 (specification §4):
// SHA-256 of the canonical JSON (JCS) of the goldmark tree with GFM. Two
// texts with the same content and different formatting give the same
// fingerprint; invisible content changes it. It replaces the body
// comparison of B-3 for sections.
// [EL DISEÑO NO LO DICE]: the shape of the fingerprint string; it is the
// scheme, a colon and the hex digest, so a later scheme can coexist.
func Section(markdownText string) string {
	src := []byte(norm.NFC.String(strings.ReplaceAll(markdownText, "\r\n", "\n")))
	doc := markdown.Parser().Parse(text.NewReader(src))
	tree := canonical(doc, src)
	raw, err := json.Marshal(tree)
	if err != nil {
		panic(fmt.Sprintf("fingerprint: marshal tree: %v", err)) // only strings, ints and bools
	}
	canon, err := jcs.Transform(raw)
	if err != nil {
		panic(fmt.Sprintf("fingerprint: canonical json: %v", err))
	}
	sum := sha256.Sum256(canon)
	return SectionScheme + ":" + hex.EncodeToString(sum[:])
}

func canonical(n ast.Node, src []byte) *node {
	out := &node{Kind: n.Kind().String()}
	switch v := n.(type) {
	case *ast.Heading:
		out.Level = v.Level
	case *ast.Emphasis:
		out.Level = v.Level // 1 or 2, whatever the character
	case *ast.List:
		out.Ordered = v.IsOrdered()
	case *ast.Link:
		out.Dest = string(v.Destination)
	case *ast.Image:
		out.Dest = string(v.Destination)
	case *ast.AutoLink:
		out.Dest = string(v.URL(src))
	case *ast.FencedCodeBlock:
		out.Lang = string(v.Language(src)) // mermaid included
		out.Text = linesOf(v.Lines(), src)
		return out
	case *ast.CodeBlock:
		out.Text = linesOf(v.Lines(), src)
		return out
	case *ast.HTMLBlock:
		out.Text = linesOf(v.Lines(), src)
		return out
	case *extast.TableCell:
		out.Level = int(v.Alignment)
	case *extast.TaskCheckBox:
		if v.IsChecked {
			out.Level = 1
		}
	case *ast.Text:
		out.Text = string(v.Segment.Value(src))
		if v.HardLineBreak() {
			out.Level = 2
		} else if v.SoftLineBreak() {
			out.Text += " " // a wrapped line is padding, not content
		}
		return out
	case *ast.String:
		out.Text = string(v.Value)
		return out
	case *ast.CodeSpan:
		var b strings.Builder
		for c := v.FirstChild(); c != nil; c = c.NextSibling() {
			if t, ok := c.(*ast.Text); ok {
				b.Write(t.Segment.Value(src))
			}
		}
		out.Text = b.String()
		return out
	}
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		child := canonical(c, src)
		// Adjacent plain texts are one text: escapes and line wraps split
		// them without changing the content.
		if last := lastChild(out); last != nil && child.Kind == "Text" && last.Kind == "Text" && last.Level == 0 {
			last.Text += child.Text
			last.Level = child.Level
			continue
		}
		out.Children = append(out.Children, child)
	}
	for _, c := range out.Children {
		if c.Kind == "Text" {
			c.Text = padding.ReplaceAllString(c.Text, " ") // padding, not content
		}
	}
	if last := lastChild(out); last != nil && last.Kind == "Text" {
		last.Text = strings.TrimRight(last.Text, " ") // the wrap at a block's end
	}
	return out
}

func lastChild(n *node) *node {
	if len(n.Children) == 0 {
		return nil
	}
	return n.Children[len(n.Children)-1]
}

func linesOf(lines *text.Segments, src []byte) string {
	var b strings.Builder
	for i := 0; i < lines.Len(); i++ {
		seg := lines.At(i)
		b.Write(seg.Value(src))
	}
	return b.String()
}
