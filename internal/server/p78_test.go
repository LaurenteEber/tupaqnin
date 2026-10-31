package server_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/LaurenteEber/tupaqnin/internal/graph"
	"github.com/LaurenteEber/tupaqnin/internal/server"
)

// listText is a section with a list: the history counts one block per
// item (B-10), so «- dos con DEC‑1» is block 3.
const (
	listItem = "- dos con DEC‑1"
	listText = "Intro.\n\n- uno\n" + listItem + "\n- tres\n\nFinal.\n"
)

func sectionView(t *testing.T, h http.Handler, base, label string) server.SectionView {
	t.Helper()
	rec := do(t, h, http.MethodGet, base+"/sections", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET sections: %d %s", rec.Code, rec.Body.String())
	}
	for _, v := range decode[[]server.SectionView](t, rec) {
		if v.Label == label {
			return v
		}
	}
	t.Fatalf("no %s in the sections", label)
	return server.SectionView{}
}

func internalCodesWarning(t *testing.T, h http.Handler, base string) graph.Warning {
	t.Helper()
	for _, got := range warningsOf(t, h, base, "sec-1") {
		if got.Kind == graph.WarningAgentPending && got.Pending == graph.PendingInternalCodes {
			return got
		}
	}
	t.Fatal("no internal codes warning")
	return graph.Warning{}
}

// P-78: inside a list, a warning sits in its item's block, and its
// offsets count from the start of that item's paragraph in the editor
// (its line, marker included), not from the whole list.
func TestP78_WarningInAListItemCountsFromItsItem(t *testing.T) {
	h, db, _ := newAPI(t)
	projectID := newProject(t, h, nil)
	base := "/api/projects/" + projectID
	seedSectionWithCode(t, db, projectID, listText)

	w := internalCodesWarning(t, h, base)
	if w.Block != 3 {
		t.Fatalf("block %d, want 3 (the item «dos»)", w.Block)
	}
	if cut := utf16Slice(listItem, w.Start, w.End); cut != "DEC‑1" {
		t.Fatalf("offsets %d..%d cut %q of the item, want «DEC‑1»", w.Start, w.End, cut)
	}
}

// P-78: Go gives the editor its paragraphs, one per block of the history
// (one per list item): paragraph n holds block n, and the paragraphs
// with their separators are the text, byte for byte.
func TestP78_EditorParagraphsAreTheHistoryBlocksOnePerListItem(t *testing.T) {
	h, db, _ := newAPI(t)
	projectID := newProject(t, h, nil)
	base := "/api/projects/" + projectID
	seedSectionWithCode(t, db, projectID, listText)

	view := sectionView(t, h, base, "sec-1")
	blocks := graph.Blocks(listText)
	if len(view.Paragraphs) != len(blocks) || len(blocks) != 5 {
		t.Fatalf("paragraphs %+v, want one per block %q", view.Paragraphs, blocks)
	}
	var joined strings.Builder
	for i, p := range view.Paragraphs {
		if !strings.Contains(p.Text, blocks[i]) {
			t.Fatalf("paragraph %d %q does not hold block %d %q", i+1, p.Text, i+1, blocks[i])
		}
		joined.WriteString(p.Text + p.Separator)
	}
	if joined.String() != listText {
		t.Fatalf("paragraphs put together: %q, want the text %q", joined.String(), listText)
	}
	if view.Paragraphs[2].Text != listItem || view.Paragraphs[2].Separator != "\n" {
		t.Fatalf("paragraph 3: %+v, want the item %q followed by one line break", view.Paragraphs[2], listItem)
	}
}

// P-78: the previous numbering still holds for a section without lists:
// one paragraph per blank-line paragraph, a heading keeping its «## ».
func TestP78_ASectionWithoutListsNumbersAsBefore(t *testing.T) {
	h, db, _ := newAPI(t)
	projectID := newProject(t, h, nil)
	base := "/api/projects/" + projectID
	text := "Primer párrafo.\n\n## Según la DEC‑1, sube\n\nÚltimo,\ncon dos líneas."
	seedSectionWithCode(t, db, projectID, text)

	view := sectionView(t, h, base, "sec-1")
	want := strings.Split(text, "\n\n")
	if len(view.Paragraphs) != len(want) {
		t.Fatalf("paragraphs %+v, want %q", view.Paragraphs, want)
	}
	for i, p := range view.Paragraphs {
		sep := "\n\n"
		if i == len(want)-1 {
			sep = ""
		}
		if p.Text != want[i] || p.Separator != sep {
			t.Fatalf("paragraph %d: %+v, want %q then %q", i+1, p, want[i], sep)
		}
	}
	if w := internalCodesWarning(t, h, base); w.Block != 2 || utf16Slice(want[1], w.Start, w.End) != "DEC‑1" {
		t.Fatalf("warning %+v, want «DEC‑1» in block 2", w)
	}
}
