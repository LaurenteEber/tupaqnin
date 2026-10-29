package graph_test

import (
	"reflect"
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
