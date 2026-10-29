package cycle_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LaurenteEber/tupaqnin/internal/cycle"
	"github.com/LaurenteEber/tupaqnin/internal/provider"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// seedRequest creates the request, two criteria (born at submit,
// P-24) and the premise directly in the store.
func seedRequest(t *testing.T, db *store.DB) []cycle.CriterionRef {
	t.Helper()
	ctx := context.Background()
	var refs []cycle.CriterionRef
	err := db.Write(ctx, "p1", func(tx *store.Tx) error {
		req, err := tx.CreateObject(store.ObjectRequest, "",
			map[string]any{"text": "Escribe el capítulo.", "audience": "Comité"},
			store.EventInput{Type: store.EventStateChanged, Actor: "human"})
		if err != nil {
			return err
		}
		for _, text := range []string{"Solo estudios de campo.", "Comparar conteos, no salud."} {
			crit, err := tx.CreateObject(store.ObjectCriterion, "current",
				map[string]any{"text": text, "request_id": req.ID},
				store.EventInput{Type: store.EventStateChanged, Actor: "human"})
			if err != nil {
				return err
			}
			refs = append(refs, cycle.CriterionRef{Label: crit.Label, ID: crit.ID})
		}
		_, err = tx.CreateObject(store.ObjectPremise, "current",
			map[string]any{"text": "La embotellada trae más.", "request_id": req.ID, "exploratory": false},
			store.EventInput{Type: store.EventStateChanged, Actor: "human"})
		return err
	})
	if err != nil {
		t.Fatalf("seed request: %v", err)
	}
	return refs
}

func TestUnderstandingSchemaCopyIsIdenticalToDesign(t *testing.T) {
	embedded, err := os.ReadFile(filepath.Join("schema", "understanding_output.schema.json"))
	if err != nil {
		t.Fatalf("embedded schema: %v", err)
	}
	design, err := os.ReadFile(filepath.Join("..", "..", "docs", "diseno", "bloque-3", "salida-entendi.schema.json"))
	if err != nil {
		t.Fatalf("design schema: %v", err)
	}
	if string(embedded) != string(design) {
		t.Fatal("embedded understanding schema differs from docs/")
	}
}

func TestB12_UnderstandingCreatesCasesAndSectionsWithProposedRole(t *testing.T) {
	db := openTemp(t)
	refs := seedRequest(t, db)
	ctx := context.Background()

	result, err := cycle.NewGate(db).EndUnderstanding(ctx, "p1", "turn-1", 1,
		recordedOutput(t, "understanding-valid.json"), refs)
	if err != nil {
		t.Fatalf("EndUnderstanding: %v", err)
	}
	if result.Status != cycle.StatusAccepted {
		t.Fatalf("status: got %v (%v)", result.Status, result.Errors)
	}

	cases, err := db.ListObjects(ctx, "p1", store.ObjectUnderstandingCase)
	if err != nil || len(cases) != 5 { // 2 per criterion + 1 material reading
		t.Fatalf("cases: %d, %v; want 5", len(cases), err)
	}
	for _, c := range cases {
		if c.State != "unconfirmed" {
			t.Fatalf("case %s state: got %q, want unconfirmed (nothing waits for the human)", c.Label, c.State)
		}
	}
	sections, err := db.ListObjects(ctx, "p1", store.ObjectSection)
	if err != nil || len(sections) != 2 {
		t.Fatalf("sections: %d, %v; want 2", len(sections), err)
	}
	for _, sec := range sections {
		var b map[string]any
		_ = json.Unmarshal(sec.Body, &b)
		role, _ := b["role"].(map[string]any)
		if sec.State != "draft" || role["state"] != "proposed" || b["title"] == "" {
			t.Fatalf("section %s: state=%q role=%v title=%v", sec.Label, sec.State, role, b["title"])
		}
	}
	ended := eventsOfType(t, db, store.EventStepEnded)
	if len(ended) != 1 || !strings.Contains(string(ended[0].Data), "Escribo el estado del arte") {
		t.Fatalf("understanding end event: %+v", ended)
	}
}

func TestB12_UnderstandingWithoutOneCriterionIsRejectedAndRetried(t *testing.T) {
	db := openTemp(t)
	refs := seedRequest(t, db)
	ctx := context.Background()
	gate := cycle.NewGate(db)

	result, err := gate.EndUnderstanding(ctx, "p1", "turn-1", 1,
		recordedOutput(t, "understanding-missing-criterion.json"), refs)
	if err != nil {
		t.Fatalf("EndUnderstanding: %v", err)
	}
	if result.Status != cycle.StatusRetry || len(result.Errors) == 0 {
		t.Fatalf("first attempt: got %v, want retry with errors", result.Status)
	}
	cases, _ := db.ListObjects(ctx, "p1", store.ObjectUnderstandingCase)
	if len(cases) != 0 {
		t.Fatalf("cases after rejected output: got %d, want 0", len(cases))
	}

	// The retry with the complete output is accepted.
	result, err = gate.EndUnderstanding(ctx, "p1", "turn-1", 2,
		recordedOutput(t, "understanding-valid.json"), refs)
	if err != nil || result.Status != cycle.StatusAccepted {
		t.Fatalf("retry: got %v, %v", result.Status, err)
	}
	if got := eventsOfType(t, db, store.EventOutputRetry); len(got) != 1 {
		t.Fatalf("output_retry events: got %d, want 1", len(got))
	}
}

func TestB12_UnderstandingRejectsNewNInCriterion(t *testing.T) {
	db := openTemp(t)
	refs := seedRequest(t, db)
	out := recordedOutput(t, "understanding-valid.json")
	out = json.RawMessage(strings.Replace(string(out), `"crit-1"`, `"new-1"`, 1))
	result, err := cycle.NewGate(db).EndUnderstanding(context.Background(), "p1", "turn-1", 1, out, refs)
	if err != nil {
		t.Fatalf("EndUnderstanding: %v", err)
	}
	if result.Status != cycle.StatusRetry {
		t.Fatalf("status: got %v, want retry (rule 4 rejects new-n in criterion)", result.Status)
	}
}

func TestB12_UnderstandingTurnEndToEndWithRecordedOutput(t *testing.T) {
	fx, err := provider.LoadFixture(filepath.Join("..", "provider", "testdata", "understanding-valid.json"))
	if err != nil {
		t.Fatalf("LoadFixture: %v", err)
	}
	fx.TurnKind = provider.TurnUnderstanding
	db, runner, pub := newTestRunner(t, fx, loadAppFixture(t, "04-principal-no-work.json"))
	seedRequest(t, db)
	// The runner starts its own instance via startup, which reads
	// the request from the seeded project "p1".
	if err := runner.RunUnderstandingTurn(context.Background(), "p1", "turn-u"); err != nil {
		t.Fatalf("RunUnderstandingTurn: %v", err)
	}
	cases, _ := db.ListObjects(context.Background(), "p1", store.ObjectUnderstandingCase)
	if len(cases) != 5 {
		t.Fatalf("cases after turn: got %d, want 5", len(cases))
	}
	// The understanding's step, then the chain's (B-17), which starts on
	// its own as the turn the understanding's step names.
	step := pub.stepAt(t, 0, 2)
	if step["status"] != "accepted" || step["understood"] == "" || step["next_turn_id"] == "" {
		t.Fatalf("published step: %v", step)
	}
	calls, err := db.ModelCalls(context.Background(), "p1", "")
	_ = calls
	_ = err
	// The call went at effort high (fichas §1).
	rows, err := db.Reader.QueryContext(context.Background(),
		`SELECT DISTINCT model_and_effort FROM model_call WHERE project_id = 'p1' AND turn_id = 'turn-u'`)
	if err != nil {
		t.Fatalf("model_call: %v", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var me string
		if err := rows.Scan(&me); err != nil {
			t.Fatalf("scan: %v", err)
		}
		if !strings.HasSuffix(me, " high") {
			t.Fatalf("understanding effort: got %q, want suffix ' high'", me)
		}
	}
}
