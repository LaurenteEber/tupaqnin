package memory_test

import (
	"context"
	"strings"
	"testing"

	"github.com/LaurenteEber/tupaqnin/internal/memory"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

const toolSource = "# Muestreo de grifos\n\nSe tomaron dos muestras por hogar.\n\n" +
	"El agua embotellada tuvo 1,5 veces más partículas por litro que la de grifo.\n\nLa diferencia se mantuvo."

func seedSource(t *testing.T) (*store.DB, store.Object) {
	t.Helper()
	db, err := store.Open(context.Background(), t.TempDir())
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := db.Writer.ExecContext(context.Background(),
		`INSERT INTO project (id, created_at) VALUES ('p1', '2026-10-30T00:00:00.000Z')`); err != nil {
		t.Fatalf("seed project: %v", err)
	}
	var src store.Object
	err = db.Write(context.Background(), "p1", func(tx *store.Tx) error {
		var err error
		src, err = tx.CreateObject(store.ObjectSource, "", map[string]any{"kind": "material",
			"filename": "grifos.md", "media_type": "text/markdown", "text": toolSource},
			store.EventInput{Type: store.EventStateChanged, Actor: "human"})
		return err
	})
	if err != nil {
		t.Fatalf("seed source: %v", err)
	}
	return db, src
}

func reads(t *testing.T, db *store.DB) int {
	t.Helper()
	ids, err := db.ReadObjectIDs(context.Background(), "p1")
	if err != nil {
		t.Fatalf("ReadObjectIDs: %v", err)
	}
	return len(ids)
}

// R-3: read_source returns the span literally, by numbered paragraph,
// and records the read; an unknown source records nothing.
func TestR3_ReadSourceReturnsTheSpanLiterallyAndRecordsTheRead(t *testing.T) {
	db, src := seedSource(t)
	mem := memory.New(db)
	out, err := mem.ReadSource(context.Background(), "p1", "i1", "src-9", "")
	if err != nil || !strings.Contains(out, "No source src-9. The sources are: "+src.Label) || reads(t, db) != 0 {
		t.Fatalf("unknown source: %q, %v, reads %d", out, err, reads(t, db))
	}
	out, err = mem.ReadSource(context.Background(), "p1", "i1", src.Label, "3")
	if err != nil {
		t.Fatalf("ReadSource: %v", err)
	}
	if out != src.Label+", paragraphs 3-3 of 4:\n[3] El agua embotellada tuvo 1,5 veces más partículas por litro que la de grifo.\n" {
		t.Fatalf("span 3: %q", out)
	}
	if reads(t, db) != 1 {
		t.Fatal("the read was not recorded (via = read)")
	}
	// «null» as a string, as Spark sent it in the micro-test, is the
	// whole source.
	if out, _ = mem.ReadSource(context.Background(), "p1", "i1", src.Label, "null"); !strings.Contains(out, "paragraphs 1-4 of 4") {
		t.Fatalf("span null: %q", out)
	}
}

// R-3: search_sources finds the paragraph that holds the text, ignoring
// letter case, and records the read.
func TestR3_SearchSourcesFindsTheParagraphAndRecordsTheRead(t *testing.T) {
	db, src := seedSource(t)
	mem := memory.New(db)
	out, err := mem.SearchSources(context.Background(), "p1", "i1", "1,5 VECES")
	if err != nil || out != src.Label+" [3] El agua embotellada tuvo 1,5 veces más partículas por litro que la de grifo.\n" {
		t.Fatalf("search: %q, %v", out, err)
	}
	if reads(t, db) != 1 {
		t.Fatal("the search was not recorded as a read")
	}
	if out, _ := mem.SearchSources(context.Background(), "p1", "i1", "pozo"); out != "No paragraph of the sources holds that text." {
		t.Fatalf("no match: %q", out)
	}
}

// R-3: the startup's index shows a card of each source (title, type,
// length, first lines), not its text.
func TestR3_IndexShowsACardOfEachSource(t *testing.T) {
	db, src := seedSource(t)
	brief, err := memory.New(db).Startup(context.Background(), "p1", memory.StartupNewSession, 1048576, fakeCount)
	if err != nil {
		t.Fatalf("Startup: %v", err)
	}
	text := briefText(brief)
	want := src.Label + " · v1 ·  · grifos.md · material, text/markdown · 4 paragraphs, 28 words · first lines: Muestreo de grifos / Se tomaron dos muestras por hogar."
	if !strings.Contains(text, want) {
		t.Fatalf("index card missing:\n%s", text)
	}
	if strings.Contains(text, "1,5 veces") {
		t.Fatal("the index carries the source's body")
	}
}
