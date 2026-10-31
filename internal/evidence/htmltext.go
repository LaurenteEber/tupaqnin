package evidence

import (
	"bytes"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// skipped elements never reach the text (B-60): scripts, styles and
// everything that is not the page's readable body.
var skipped = map[atom.Atom]bool{
	atom.Head: true, atom.Script: true, atom.Style: true, atom.Noscript: true,
	atom.Template: true, atom.Iframe: true, atom.Object: true, atom.Embed: true,
	atom.Svg: true, atom.Math: true, atom.Canvas: true,
}

// blocks start and end a line.
var blocks = map[atom.Atom]bool{
	atom.Address: true, atom.Article: true, atom.Aside: true, atom.Blockquote: true, atom.Br: true,
	atom.Caption: true, atom.Dd: true, atom.Div: true, atom.Dl: true, atom.Dt: true, atom.Figcaption: true,
	atom.Figure: true, atom.Footer: true, atom.Form: true, atom.H1: true, atom.H2: true, atom.H3: true,
	atom.H4: true, atom.H5: true, atom.H6: true, atom.Header: true, atom.Hr: true, atom.Li: true,
	atom.Main: true, atom.Nav: true, atom.Ol: true, atom.P: true, atom.Pre: true, atom.Section: true,
	atom.Table: true, atom.Td: true, atom.Th: true, atom.Tr: true, atom.Ul: true,
}

// htmlText turns an HTML page into plain text with golang.org/x/net/html
// (B-60), never with regular expressions: one line per block, spaces
// collapsed, without scripts, styles or comments.
func htmlText(page []byte) string {
	doc, err := html.Parse(bytes.NewReader(page))
	if err != nil {
		return ""
	}
	var b strings.Builder
	var walk func(n *html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && (skipped[n.DataAtom] || skippedName(n.Data)) {
			return
		}
		if n.Type == html.TextNode {
			b.WriteString(n.Data)
			return
		}
		block := n.Type == html.ElementNode && blocks[n.DataAtom]
		if block {
			b.WriteByte('\n')
		}
		for ch := n.FirstChild; ch != nil; ch = ch.NextSibling {
			walk(ch)
		}
		if block {
			b.WriteByte('\n')
		}
	}
	walk(doc)

	var lines []string
	for _, line := range strings.Split(b.String(), "\n") {
		if line = strings.Join(strings.Fields(line), " "); line != "" {
			lines = append(lines, line)
		}
	}
	return strings.Join(lines, "\n")
}

// skippedName also catches script and style elements in foreign
// content (SVG, MathML), whose atom may differ.
func skippedName(name string) bool {
	switch strings.ToLower(name) {
	case "script", "style", "noscript", "template":
		return true
	}
	return false
}
