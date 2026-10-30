package cycle_test

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LaurenteEber/tupaqnin/internal/graph"
	"github.com/LaurenteEber/tupaqnin/internal/provider"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// The toy material of the tools micro-test (make microtest-tools): the
// figure is only in the body of src-1, never in its first line.
var toolSources = []string{
	"Muestreo de agua de grifo en 40 hogares de Lima (2025).\n\nSe tomaron dos muestras por hogar, en la mañana y en la noche.\n\n" +
		"Las partículas se contaron con espectroscopía infrarroja.\n\nEl agua embotellada tuvo 1,5 veces más partículas por litro que la de grifo.\n\n" +
		"La diferencia se mantuvo en los dos horarios.",
	"Revisión de métodos de conteo de microplásticos (2024).\n\nLos métodos ópticos subestiman las partículas menores de 20 µm.\n\n" +
		"Ningún estudio revisado compara agua embotellada con agua de grifo.",
}

// seedToolsProject creates the empty section sec-1 «Exposición» and the
// two sources src-1 and src-2 of the micro-test.
func seedToolsProject(t *testing.T, db *store.DB) {
	t.Helper()
	err := db.Write(context.Background(), "p1", func(tx *store.Tx) error {
		if _, err := tx.CreateObject(store.ObjectSection, "draft", map[string]any{"title": "Exposición", "text": "",
			"role": map[string]any{"value": "other", "state": "proposed"}, "open_changes": []any{}},
			store.EventInput{Type: store.EventStateChanged, Actor: "agent"}); err != nil {
			return err
		}
		for i, text := range toolSources {
			if _, err := tx.CreateObject(store.ObjectSource, "", map[string]any{"kind": "material",
				"filename": []string{"grifos.md", "metodos.md"}[i], "media_type": "text/markdown", "text": text},
				store.EventInput{Type: store.EventStateChanged, Actor: "human"}); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("seed tools project: %v", err)
	}
}

// toolsFixture loads one recorded call of the micro-test. The second
// call wrote against base_version 0 because the micro-test's toy
// section had no version; here sec-1 exists at version 1.
func toolsFixture(t *testing.T, name string) provider.Fixture {
	t.Helper()
	fx, err := provider.LoadFixture(filepath.Join("..", "provider", "testdata", "tools", name))
	if err != nil {
		t.Fatalf("LoadFixture %s: %v", name, err)
	}
	fx.TurnKind = provider.TurnStep
	for i, item := range fx.Items {
		fx.Items[i] = json.RawMessage(strings.ReplaceAll(string(item), `\"base_version\": 0`, `\"base_version\": 1`))
	}
	return fx
}

func readSourceLabels(t *testing.T, db *store.DB) map[string]bool {
	t.Helper()
	rows, err := db.Reader.QueryContext(context.Background(),
		`SELECT o.label FROM delivery d JOIN object o ON o.project_id = d.project_id AND o.id = d.object_id
		 WHERE d.project_id = 'p1' AND d.via = 'read'`)
	if err != nil {
		t.Fatalf("read deliveries: %v", err)
	}
	defer func() { _ = rows.Close() }()
	out := map[string]bool{}
	for rows.Next() {
		var label string
		if err := rows.Scan(&label); err != nil {
			t.Fatalf("scan: %v", err)
		}
		out[label] = true
	}
	return out
}

// R-3, with the recorded outputs of the micro-test: the principal gets
// the two tools, reads both sources with read_source, each read is
// recorded (via = read), and its citation of src-1 is applied with the
// passage it read.
func TestR3_ToolsReadTheMaterialBeforeCitingWithRecordedOutputs(t *testing.T) {
	db, runner, cap, _ := newCaptureRunner(t, toolsFixture(t, "tools-call-1.json"),
		toolsFixture(t, "tools-call-2.json"), loadAppFixture(t, "04-principal-no-work.json"))
	seedToolsProject(t, db)
	if err := runner.RunTurn(context.Background(), "p1", "turn-1", "Escribe la exposición con la cifra."); err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	if len(cap.sent) < 2 {
		t.Fatalf("provider calls: %d, want the tool call and its answer", len(cap.sent))
	}
	var tools []map[string]any
	if err := json.Unmarshal(cap.tools[0], &tools); err != nil || len(tools) != 2 ||
		tools[0]["name"] != "read_source" || tools[1]["name"] != "search_sources" || tools[0]["strict"] != true {
		t.Fatalf("tools sent: %s", cap.tools[0])
	}
	// The second call carries both tool outputs, the figure among them.
	var outputs []string
	for _, item := range cap.sent[1] {
		var out struct {
			Type   string `json:"type"`
			Output string `json:"output"`
		}
		if json.Unmarshal(item, &out) == nil && out.Type == "function_call_output" {
			outputs = append(outputs, out.Output)
		}
	}
	if len(outputs) != 2 || !strings.Contains(outputs[0], "[4] El agua embotellada tuvo 1,5 veces más partículas por litro que la de grifo.") {
		t.Fatalf("tool outputs sent back: %q", outputs)
	}
	if read := readSourceLabels(t, db); !read["src-1"] || !read["src-2"] {
		t.Fatalf("sources read: %v, want src-1 and src-2", read)
	}
	sec, err := db.GetObject(context.Background(), "p1", "sec-1")
	if err != nil || !strings.Contains(bodyOfSection(t, sec), "1,5 veces") {
		t.Fatalf("sec-1 text: %v", err)
	}
	_, pendings, err := graph.SectionStatus(context.Background(), db, "p1", "sec-1")
	if err != nil {
		t.Fatalf("SectionStatus: %v", err)
	}
	for _, p := range pendings {
		if p.Kind == graph.PendingSourceNotSupporting || p.Kind == graph.PendingFigureWithoutDecision {
			t.Fatalf("pending after reading the source: %+v", p)
		}
	}
}

func bodyOfSection(t *testing.T, obj store.Object) string {
	t.Helper()
	var b struct {
		Text string `json:"text"`
	}
	_ = json.Unmarshal(obj.Body, &b)
	return b.Text
}

// R-3 (specification §3, «Precisiones del 30-oct»): a number written
// from a source the principal never read is not linked to it and
// leaves «cifra sin decisión»; once the source was read (via = read),
// the same number is linked.
func TestR3_FigureFromASourceNotReadLeavesFigureWithoutDecision(t *testing.T) {
	step := `{"triage_line":"Escribo la exposición.","closing_risk":"none","note":"Escribí la cifra.",` +
		`"new_text":[{"section_id":"sec-1","base_version":1,"anchor":"","operation":"insert_after",` +
		`"text":"El agua embotellada tuvo 1,5 veces más partículas."}]}`
	for _, read := range []bool{false, true} {
		db, runner, _, _ := newCaptureRunner(t, textFixture(t, step), loadAppFixture(t, "04-principal-no-work.json"))
		seedToolsProject(t, db)
		if read {
			src, err := db.GetObject(context.Background(), "p1", "src-1")
			if err != nil {
				t.Fatalf("GetObject: %v", err)
			}
			if err := db.RecordDelivery(context.Background(), store.Delivery{ProjectID: "p1", InstanceID: "i0",
				Via: store.ViaRead, ObjectID: src.ID, Version: src.CurrentVersion}); err != nil {
				t.Fatalf("RecordDelivery: %v", err)
			}
		}
		if err := runner.RunTurn(context.Background(), "p1", "turn-1", "Escribe."); err != nil {
			t.Fatalf("RunTurn: %v", err)
		}
		_, pendings, err := graph.SectionStatus(context.Background(), db, "p1", "sec-1")
		if err != nil {
			t.Fatalf("SectionStatus: %v", err)
		}
		without := false
		for _, p := range pendings {
			if p.Kind == graph.PendingFigureWithoutDecision && p.About == "1,5" {
				without = true
			}
		}
		if without == read {
			t.Fatalf("source read=%v: «cifra sin decisión» for 1,5 = %v, pendings %+v", read, without, pendings)
		}
	}
}
