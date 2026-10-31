package cycle_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/LaurenteEber/tupaqnin/internal/cycle"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// B-54: with the plan confirmed, a new-n section of the agent is
// rejected with a message for it; its text goes with it, and the rest
// of the step (sec-1's text, the new decision) applies.
func TestB54_NewSectionIsRejectedOnceThePlanIsConfirmed(t *testing.T) {
	db := seedStepProject(t)
	if _, err := cycle.ConfirmPlan(context.Background(), db, "p1"); err != nil {
		t.Fatalf("ConfirmPlan: %v", err)
	}
	before, _ := db.ListObjects(context.Background(), "p1", store.ObjectSection)

	applied := applyRecorded(t, db, json.RawMessage(lockedStep))
	if !applied.Applied {
		t.Fatalf("the rest of the step must apply: %v", applied.Errors)
	}
	if len(applied.Rejected) != 1 || !strings.Contains(applied.Rejected[0], "new-1") ||
		!strings.Contains(applied.Rejected[0], "+ Section") {
		t.Fatalf("rejected: %v, want one message for the agent naming new-1 and «+ Section»", applied.Rejected)
	}
	if _, ok := applied.Labels["new-1"]; ok {
		t.Fatalf("new-1 got a label: %v", applied.Labels)
	}
	after, _ := db.ListObjects(context.Background(), "p1", store.ObjectSection)
	if len(after) != len(before) {
		t.Fatalf("sections: %d, want %d (no new section)", len(after), len(before))
	}
	if _, b := object(t, db, "sec-1"); !strings.Contains(b["text"].(string), "1,5 veces") {
		t.Fatalf("sec-1 must be written: %q", b["text"])
	}
	if _, err := db.GetObject(context.Background(), "p1", applied.Labels["new-2"]); err != nil {
		t.Fatalf("the new decision must apply: %v", err)
	}
	ended := eventsOfType(t, db, store.EventStepEnded)
	notApplied, _ := eventData(t, ended[len(ended)-1])["not_applied"].(map[string]any)
	sections, _ := notApplied["new_sections"].([]any)
	if len(sections) != 1 || sections[0].(map[string]any)["section_id"] != "new-1" ||
		sections[0].(map[string]any)["title"] != "Otra" {
		t.Fatalf("not_applied: %v, want new_sections with new-1", notApplied)
	}
	if texts, _ := notApplied["new_text"].([]any); len(texts) != 1 {
		t.Fatalf("not_applied: %v, want new-1's text", notApplied)
	}
}

// B-54: before the plan is confirmed, the same step creates the section
// as today.
func TestB54_NewSectionIsCreatedBeforeThePlanIsConfirmed(t *testing.T) {
	db := seedStepProject(t)
	applied := applyRecorded(t, db, json.RawMessage(lockedStep))
	if !applied.Applied || len(applied.Rejected) != 0 {
		t.Fatalf("got %+v, want applied with nothing rejected", applied)
	}
	if sec, b := object(t, db, applied.Labels["new-1"]); sec.Type != store.ObjectSection || b["text"] != "Texto de la otra sección." {
		t.Fatalf("new-1: %+v %v", sec, b)
	}
}

// B-53: the understanding's focus is up to 60 words, checked in Go
// (salida-entendi: «validado en Go»); longer, the output is retried.
func TestB53_UnderstandingFocusOverSixtyWordsIsRetried(t *testing.T) {
	db := openTemp(t)
	refs := seedRequest(t, db)
	out := strings.Replace(string(recordedOutput(t, "understanding-valid.json")),
		`"focus": "Lo que concluye la sección."`, `"focus": "`+strings.Repeat("palabra ", 61)+`"`, 1)
	result, err := cycle.NewGate(db).EndUnderstanding(context.Background(), "p1", "turn-1", 1, json.RawMessage(out), refs)
	if err != nil {
		t.Fatalf("EndUnderstanding: %v", err)
	}
	if result.Status != cycle.StatusRetry || !strings.Contains(strings.Join(result.Errors, "\n"), "/sections/0/focus") {
		t.Fatalf("got %v %v, want retry naming /sections/0/focus", result.Status, result.Errors)
	}
}

// R-6: confirming twice records plan_confirmed once.
func TestR6_ConfirmingThePlanTwiceRecordsItOnce(t *testing.T) {
	db := seedStepProject(t)
	for i := 0; i < 2; i++ {
		if _, err := cycle.ConfirmPlan(context.Background(), db, "p1"); err != nil {
			t.Fatalf("ConfirmPlan: %v", err)
		}
	}
	if n := len(eventsOfType(t, db, store.EventPlanConfirmed)); n != 1 {
		t.Fatalf("plan_confirmed: %d, want 1", n)
	}
	ok, err := cycle.PlanConfirmed(context.Background(), db, "p1")
	if err != nil || !ok {
		t.Fatalf("PlanConfirmed: %v %v", ok, err)
	}
}
