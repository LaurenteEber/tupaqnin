package cycle_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/LaurenteEber/tupaqnin/internal/cycle"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

func cardsOf(t *testing.T, db *store.DB) map[string]string {
	t.Helper()
	cards, err := db.ListObjects(context.Background(), "p1", store.ObjectCard)
	if err != nil {
		t.Fatalf("cards: %v", err)
	}
	out := map[string]string{} // decision label → card state
	for _, c := range cards {
		var b map[string]any
		_ = json.Unmarshal(c.Body, &b)
		dec, err := db.GetObject(context.Background(), "p1", b["decision_id"].(string))
		if err != nil {
			t.Fatalf("card decision: %v", err)
		}
		out[dec.Label] = c.State
	}
	return out
}

// T-5, estados §2: the agent's decision that covers a figure becomes
// critical in the step's transaction and gets its open card.
func TestT5_AgentDecisionThatBecomesCriticalOpensItsCard(t *testing.T) {
	db := seedStepProject(t)
	applied := applyRecorded(t, db, stepFile(t, "figure-datum-loose"))
	if got := cardsOf(t, db); len(got) != 1 || got[applied.Labels["new-1"]] != "open" {
		t.Fatalf("cards: %v, want one open card for %s", got, applied.Labels["new-1"])
	}
}

func TestT5_NonCriticalDecisionGetsNoCard(t *testing.T) {
	db := seedStepProject(t)
	applyRecorded(t, db, stepFile(t, "supersedes-agent")) // dec-3, used only in sec-1 (role other)
	if got := cardsOf(t, db); len(got) != 0 {
		t.Fatalf("cards: %v, want none", got)
	}
}

// Estados §3: a card whose decision changed before the answer becomes
// obsolete, and the decision that replaces it gets its own card.
func TestE3_CardOfASupersededDecisionBecomesObsolete(t *testing.T) {
	db := seedStepProject(t)
	ctx := context.Background()
	// Make dec-2 critical: sec-1 becomes a conclusion.
	if err := db.Write(ctx, "p1", func(tx *store.Tx) error {
		return cycle.TrackCriticality(ctx, tx, "", func() error {
			sec, _ := tx.GetObject(ctx, "p1", "sec-1")
			var b map[string]any
			_ = json.Unmarshal(sec.Body, &b)
			b["role"] = map[string]any{"value": "conclusion", "state": "confirmed"}
			_, err := tx.UpdateObject(sec.ID, sec.CurrentVersion, sec.State, b,
				store.EventInput{Type: store.EventRoleChanged, Actor: "human"})
			return err
		})
	}); err != nil {
		t.Fatalf("role change: %v", err)
	}
	if got := cardsOf(t, db); got["dec-2"] != "open" {
		t.Fatalf("cards after role change: %v, want dec-2 open", got)
	}
	out := []byte(`{"triage_line":"Construir.","closing_risk":"none","note":"Reemplazo.",
		"new_decisions":[{"id":"new-1","statement":"Comparar por veces.","instead_of":"Comparar por promedio.",
		"would_choose_if":"Hubiera un solo filtro.","assumption":false,"used_in_sections":["sec-1"],"supersedes":"dec-2"}]}`)
	applyRecorded(t, db, out)
	got := cardsOf(t, db)
	if got["dec-2"] != "obsolete" || got["dec-3"] != "open" {
		t.Fatalf("cards: %v, want dec-2 obsolete and dec-3 open", got)
	}
}

// K-6: a decision of a signed section that becomes critical moves the
// section to changed_since_signoff.
func TestK6_DecisionOfASignedSectionThatBecomesCriticalChangesTheSection(t *testing.T) {
	db := seedStepProject(t)
	ctx := context.Background()
	if _, err := db.Writer.ExecContext(ctx, `UPDATE object SET state = 'signed' WHERE project_id = 'p1' AND label = 'sec-1'`); err != nil {
		t.Fatalf("sign: %v", err)
	}
	if err := db.Write(ctx, "p1", func(tx *store.Tx) error {
		return cycle.TrackCriticality(ctx, tx, "", func() error {
			sec, _ := tx.GetObject(ctx, "p1", "sec-1")
			var b map[string]any
			_ = json.Unmarshal(sec.Body, &b)
			b["role"] = map[string]any{"value": "recommendation", "state": "confirmed"}
			_, err := tx.UpdateObject(sec.ID, sec.CurrentVersion, sec.State, b,
				store.EventInput{Type: store.EventRoleChanged, Actor: "human"})
			return err
		})
	}); err != nil {
		t.Fatalf("role change: %v", err)
	}
	if sec, _ := object(t, db, "sec-1"); sec.State != "changed_since_signoff" {
		t.Fatalf("sec-1 state: %s, want changed_since_signoff", sec.State)
	}
}

// The demo's fake recording (P-4 in the browser): the step that fixes
// the comparison rule leaves its decision critical, with its open card.
func TestT5_DemoRecordingOpensACard(t *testing.T) {
	db, runner, _ := newTestRunner(t, loadAppFixture(t, "01b-principal-critical-decision.json"),
		loadAppFixture(t, "04-principal-no-work.json"))
	if err := runner.RunTurn(context.Background(), "p1", "turn-1", "Escribe la regla."); err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	if got := cardsOf(t, db); len(got) != 1 || got["dec-1"] != "open" {
		t.Fatalf("cards: %v, want dec-1 open", got)
	}
}
