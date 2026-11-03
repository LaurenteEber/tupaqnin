package checks_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LaurenteEber/tupaqnin/internal/checks"
	"github.com/LaurenteEber/tupaqnin/internal/graph"
	"github.com/LaurenteEber/tupaqnin/internal/provider"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// B-108: besides its section, the checker receives the current decisions of the
// other sections. The recording named handmade-* is hand-made: Eber records a
// real one, and measures the cost with Meta.

const (
	rosaStatement = "Priorizar sectorización y gestión de presiones antes de renovar redes."
	rosaReason    = "El personal propio ya conoce las válvulas."
	rosaRecoText  = "Se recomienda empezar por sectorizar, sin tocar la red."
	rosaNewText   = "Hay que renovar las redes primero."
)

// rosaProject is Rosa's document: sec-1 «Diagnóstico», sec-2 «Recomendación»
// and dec-1, the agent's, used only in the recommendation. The change is a new
// passage of the diagnosis.
func rosaProject(t *testing.T) (*store.DB, checks.Change) {
	t.Helper()
	ctx := context.Background()
	db, err := store.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := db.Writer.ExecContext(ctx, `INSERT INTO project (id, created_at) VALUES ('p1', '2026-11-03T00:00:00.000Z')`); err != nil {
		t.Fatalf("seed project: %v", err)
	}
	var change checks.Change
	err = db.Write(ctx, "p1", func(tx *store.Tx) error {
		human := store.EventInput{Type: store.EventStateChanged, Actor: "human"}
		agent := store.EventInput{Type: store.EventStateChanged, Actor: "agent"}
		if _, err := tx.CreateObject(store.ObjectRequest, "", map[string]any{"text": "Escribe la nota."}, human); err != nil {
			return err
		}
		diagnosis, err := tx.CreateObject(store.ObjectSection, "draft", map[string]any{"title": "Diagnóstico", "text": rosaNewText}, agent)
		if err != nil {
			return err
		}
		reco, err := tx.CreateObject(store.ObjectSection, "draft", map[string]any{"title": "Recomendación", "text": rosaRecoText}, agent)
		if err != nil {
			return err
		}
		dec, err := tx.CreateObject(store.ObjectDecision, "agent", map[string]any{"statement": rosaStatement, "origin": "agent"}, agent)
		if err != nil {
			return err
		}
		// Its «porque» lives on its answered card, as in the app.
		if _, err := tx.CreateObject(store.ObjectCard, "answered", map[string]any{"decision_id": dec.ID, "chosen_option": "keep", "reason": rosaReason}, human); err != nil {
			return err
		}
		if _, err := tx.AddEdge(store.EdgeUsedIn, dec.ID, reco.ID, "", nil, store.EventInput{Type: store.EventEdge, Actor: "agent"}); err != nil {
			return err
		}
		change = checks.Change{Kind: checks.AgentStep, Sections: []string{diagnosis.ID},
			Passages: []checks.Passage{{Section: diagnosis.ID, Text: rosaNewText}}}
		return nil
	})
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	return db, change
}

func protectedLabels(t *testing.T, db *store.DB, change checks.Change) []string {
	t.Helper()
	probes, err := checks.Probes(context.Background(), db, "p1", change)
	if err != nil {
		t.Fatalf("Probes: %v", err)
	}
	labels := make([]string, 0, len(probes))
	for _, p := range probes {
		labels = append(labels, p.Protected)
	}
	return labels
}

func TestB108_TheCheckerGetsTheDecisionsOfOtherSectionsNotTheirText(t *testing.T) {
	db, change := rosaProject(t)
	ctx := context.Background()
	probes, err := checks.Probes(ctx, db, "p1", change)
	if err != nil {
		t.Fatalf("Probes: %v", err)
	}
	if got := strings.Join(protectedLabels(t, db, change), " "); got != "req-1 dec-1" {
		t.Fatalf("probes: %s", got)
	}
	brief, err := checks.Brief(ctx, db, "p1", change, probes)
	if err != nil {
		t.Fatalf("Brief: %v", err)
	}
	for _, want := range []string{rosaStatement, rosaReason, "«Recomendación»"} {
		if !strings.Contains(brief, want) {
			t.Fatalf("the brief lacks %q:\n%s", want, brief)
		}
	}
	if strings.Contains(brief, rosaRecoText) {
		t.Fatalf("the brief carries the other section's text:\n%s", brief)
	}
}

func TestB108_OnlyCurrentDecisionsOfOtherSectionsAreAdded(t *testing.T) {
	ctx := context.Background()
	t.Run("a decision used only in the changed section", func(t *testing.T) {
		db, change := rosaProject(t)
		// The same project, with the change now on the recommendation.
		reco, err := db.GetObject(ctx, "p1", "sec-2")
		if err != nil {
			t.Fatal(err)
		}
		change.Sections = []string{reco.ID}
		change.Passages = []checks.Passage{{Section: reco.ID, Text: rosaRecoText}}
		if got := strings.Join(protectedLabels(t, db, change), " "); got != "req-1" {
			t.Fatalf("probes: %s", got)
		}
	})
	t.Run("one change writes both sections", func(t *testing.T) {
		db, change := rosaProject(t)
		reco, err := db.GetObject(ctx, "p1", "sec-2")
		if err != nil {
			t.Fatal(err)
		}
		change.Sections = append(change.Sections, reco.ID)
		change.Passages = append(change.Passages, checks.Passage{Section: reco.ID, Text: rosaRecoText})
		if got := strings.Join(protectedLabels(t, db, change), " "); got != "req-1 dec-1" {
			t.Fatalf("probes: %s", got)
		}
	})
	t.Run("a decision of another section no longer current", func(t *testing.T) {
		db, change := rosaProject(t)
		dec, err := db.GetObject(ctx, "p1", "dec-1")
		if err != nil {
			t.Fatal(err)
		}
		var body map[string]any
		if err := json.Unmarshal(dec.Body, &body); err != nil {
			t.Fatal(err)
		}
		err = db.Write(ctx, "p1", func(tx *store.Tx) error {
			_, err := tx.UpdateObject(dec.ID, dec.CurrentVersion, "superseded", body,
				store.EventInput{Type: store.EventStateChanged, Actor: "app"})
			return err
		})
		if err != nil {
			t.Fatal(err)
		}
		if got := strings.Join(protectedLabels(t, db, change), " "); got != "req-1" {
			t.Fatalf("probes: %s", got)
		}
	})
	t.Run("a change with no section of the document", func(t *testing.T) {
		db, change := rosaProject(t)
		change.Sections = []string{"a-round"}
		change.Passages = []checks.Passage{{Section: "a-round", Text: "Un hallazgo."}}
		if got := strings.Join(protectedLabels(t, db, change), " "); got != "req-1" {
			t.Fatalf("probes: %s", got)
		}
	})
}

// Rosa's case (hallazgo 13): the diagnosis against the recommendation leaves a
// «por aclarar» that blocks both sections. The recommendation is blocked through
// the decision, with no edge of its own, so an answer's step stays on the
// diagnosis (B-100); the server names it, TestB108_TheCardNamesBothSections.
func TestB108_RosasDiagnosisAgainstTheRecommendationLeavesAQuestionNamingBoth(t *testing.T) {
	db, change := rosaProject(t)
	ctx := context.Background()
	schema, err := os.ReadFile(filepath.Join("..", "cycle", "schema", "checker_output.schema.json"))
	if err != nil {
		t.Fatalf("read schema: %v", err)
	}
	fx, err := provider.LoadFixture(filepath.Join("..", "provider", "testdata", "checker", "handmade-rosa-cross-section.json"))
	if err != nil {
		t.Fatalf("LoadFixture: %v", err)
	}
	c := checks.NewChecker(provider.NewFake(fx), "", schema)
	if err := c.Check(ctx, db, "p1", "turn-1", change); err != nil {
		t.Fatalf("Check: %v", err)
	}
	questions, err := db.ListObjects(ctx, "p1", store.ObjectOpenQuestion)
	if err != nil || len(questions) != 1 || questions[0].State != "open" {
		t.Fatalf("open questions: %+v, %v", questions, err)
	}
	edges, err := db.EdgesFrom(ctx, "p1", questions[0].ID)
	if err != nil {
		t.Fatalf("EdgesFrom: %v", err)
	}
	blocks := map[string]bool{}
	for _, e := range edges {
		if e.Type != store.EdgeBlocks {
			continue
		}
		obj, err := db.GetObject(ctx, "p1", e.ToID)
		if err != nil {
			t.Fatalf("GetObject: %v", err)
		}
		blocks[obj.Label] = true
	}
	if !blocks["sec-1"] || !blocks["dec-1"] || blocks["sec-2"] || len(blocks) != 2 {
		t.Fatalf("direct blocks: %v, want sec-1 and dec-1", blocks)
	}
	for _, label := range []string{"sec-1", "sec-2"} {
		sec, err := db.GetObject(ctx, "p1", label)
		if err != nil {
			t.Fatalf("GetObject: %v", err)
		}
		blocking, err := graph.OpenQuestionsBlocking(ctx, db, "p1", sec.ID)
		if err != nil || len(blocking) != 1 {
			t.Fatalf("%s blocked by %v, %v", label, blocking, err)
		}
	}
	events, err := db.EventsSince(ctx, "p1", 0)
	if err != nil {
		t.Fatalf("EventsSince: %v", err)
	}
	verdict := ""
	for _, ev := range events {
		var data map[string]any
		_ = json.Unmarshal(ev.Data, &data)
		if ev.Type == store.EventProbe && data["protected"] == "dec-1" && data["kind"] == graph.ProbeKindProbe {
			verdict, _ = data["verdict"].(string)
		}
	}
	if verdict != checks.VerdictContradicts {
		t.Fatalf("the probe's event says %q", verdict)
	}
}
