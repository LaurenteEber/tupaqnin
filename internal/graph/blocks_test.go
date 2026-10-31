package graph_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/LaurenteEber/tupaqnin/internal/graph"
)

func TestB10_BlocksAreParagraphsListItemsAndTablesInOrder(t *testing.T) {
	md := "# Alcance\n\nPrimer párrafo.\n\n- uno\n  - anidado\n- dos\n\n| a | b |\n|---|---|\n| 1 | 2 |\n\nÚltimo."
	got := graph.Blocks(md)
	want := []string{"Alcance", "Primer párrafo.", "uno\n  - anidado", "dos", "| a | b |\n|---|---|\n| 1 | 2 |", "Último."}
	if len(got) != len(want) {
		t.Fatalf("blocks: %q", got)
	}
	for i := range want {
		if i == 4 {
			continue // a table's span is checked below
		}
		if got[i] != want[i] {
			t.Fatalf("block %d: %q, want %q (all %q)", i+1, got[i], want[i], got)
		}
	}
	if !reflect.DeepEqual(len(graph.Compare(got[4]).Find("1")), 1) {
		t.Fatalf("table block: %q", got[4])
	}
}

// P-78: the editor's paragraphs are one per block, each holding its
// block from its first line, and put together with their separators
// they are the text, for lists, headings, fences, quotes and breaks.
func TestP78_ParagraphsAreOnePerBlockAndRebuildTheText(t *testing.T) {
	for _, md := range []string{
		"# Alcance\n\nPrimer párrafo.\n\n- uno\n  - anidado\n- dos\n\n| a | b |\n|---|---|\n| 1 | 2 |\n\nÚltimo.",
		"## Título\nTexto pegado.\n1. uno\n2. dos\n\n\n> cita\n> sigue\n\n---\n\n```mermaid\ngraph TD\n```\nFin.\n\n",
		"",
		"Solo uno.",
	} {
		paras := graph.Paragraphs(md)
		blocks := graph.Blocks(md)
		if len(blocks) > 0 && len(paras) != len(blocks) {
			t.Fatalf("%q: %d paragraphs, %d blocks", md, len(paras), len(blocks))
		}
		var joined string
		for i, p := range paras {
			if i < len(blocks) && !strings.Contains(p.Text, blocks[i]) {
				t.Fatalf("%q: paragraph %d %q lacks block %q", md, i+1, p.Text, blocks[i])
			}
			if strings.TrimSpace(p.Separator) != "" {
				t.Fatalf("%q: separator %q is not blank", md, p.Separator)
			}
			joined += p.Text + p.Separator
		}
		if joined != md {
			t.Fatalf("rebuilt %q, want %q", joined, md)
		}
	}
	got := graph.Paragraphs("## Título\nTexto pegado.\n1. uno\n2. dos\n\n```go\nx\n```")
	want := []string{"## Título", "Texto pegado.", "1. uno", "2. dos", "```go\nx\n```"}
	for i, w := range want {
		if i >= len(got) || got[i].Text != w {
			t.Fatalf("paragraphs %+v, want texts %q", got, want)
		}
	}
}
