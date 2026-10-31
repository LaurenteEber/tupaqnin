package cycle_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/LaurenteEber/tupaqnin/internal/cycle"
	"github.com/LaurenteEber/tupaqnin/internal/provider"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// Piece 2.8 (backlog-r2): changing a criterion creates its new version
// (yours only, estados §6) and queues B-46's probe in the checker (2.3)
// through P-72's queue: against the current text of the sections with a
// decision that applies the criterion; if none, every section with text.

const newCriterion = "Comparar conteos solo de estudios de campo."

// seedCriterionProject is seedCheckerProject plus sec-2 with text and,
// when applied, dec-2 (used in sec-1) applying crit-1: B-46's probe then
// goes against sec-1 only.
func seedCriterionProject(t *testing.T, applied bool) *store.DB {
	t.Helper()
	db := seedCheckerProject(t)
	ctx := context.Background()
	err := db.Write(ctx, "p1", func(tx *store.Tx) error {
		if _, err := tx.CreateObject(store.ObjectSection, "draft", map[string]any{
			"title": "Método", "text": "El método cuenta partículas por litro.",
			"role": map[string]any{"value": "other", "state": "confirmed"}, "open_changes": []any{},
		}, store.EventInput{Type: store.EventStateChanged, Actor: "human"}); err != nil {
			return err
		}
		if !applied {
			return nil
		}
		dec, err := tx.GetObject(ctx, "p1", "dec-2")
		if err != nil {
			return err
		}
		crit, err := tx.GetObject(ctx, "p1", "crit-1")
		if err != nil {
			return err
		}
		_, err = tx.AddEdge(store.EdgeApplies, dec.ID, crit.ID, "", nil, store.EventInput{Type: store.EventEdge, Actor: "agent"})
		return err
	})
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	return db
}

func TestB46_ChangingACriterionCreatesItsNewVersionAndProbesTheAffectedSections(t *testing.T) {
	db := seedCriterionProject(t, true)
	runner, cap := checkerRunner(t, db, checkerFixture(t, "no-findings"))
	ctx := context.Background()

	got, err := runner.ChangeCriterion(ctx, "p1", "crit-1", newCriterion)
	if err != nil {
		t.Fatalf("ChangeCriterion: %v", err)
	}
	crit, body := object(t, db, "crit-1")
	if got.Label != "crit-1" || got.Version != 2 || crit.CurrentVersion != 2 || crit.State != "current" ||
		body["text"] != newCriterion || body["request_id"] == "" {
		t.Fatalf("crit-1: %+v · %d %s %v; want its version 2, current, with the new literal text", got, crit.CurrentVersion, crit.State, body)
	}
	versions, err := db.Versions(ctx, "p1", crit.ID)
	if err != nil || len(versions) != 2 {
		t.Fatalf("versions: %d, %v; want 2 (the old one kept)", len(versions), err)
	}
	var human bool
	for _, ev := range eventsOfType(t, db, store.EventStateChanged) {
		if ev.ObjectID != crit.ID || len(ev.Data) == 0 {
			continue
		}
		d := eventData(t, ev)
		if ev.ObjectID == crit.ID && ev.Actor == "human" && d["field"] == "text" {
			human = true
		}
	}
	if !human {
		t.Fatal("no human state_changed for the criterion's new version")
	}
	if len(cap.checker) != 1 {
		t.Fatalf("checker calls: %d, want the probe at once (no turn running)", len(cap.checker))
	}
	brief := briefOf(t, cap.checker[0])
	for _, s := range []string{"crit-1 (criterion): " + newCriterion, "New passage of section sec-1"} {
		if !strings.Contains(brief, s) {
			t.Fatalf("probe brief lacks %q:\n%s", s, brief)
		}
	}
	if strings.Contains(brief, "sec-2") {
		t.Fatalf("probe brief reaches sec-2, which no decision applying crit-1 uses:\n%s", brief)
	}
	if len(queuedEvents(t, db)) != 1 || uncheckedNow(t, db, "sec-1") {
		t.Fatal("after the probe: want its queued event closed and sec-1 checked")
	}
}

// B-46: with no decision applying the criterion, the probe goes against
// every section with text.
func TestB46_ACriterionNoDecisionAppliesIsProbedAgainstEverySectionWithText(t *testing.T) {
	db := seedCriterionProject(t, false)
	runner, cap := checkerRunner(t, db, checkerFixture(t, "no-findings"))
	if _, err := runner.ChangeCriterion(context.Background(), "p1", "crit-1", newCriterion); err != nil {
		t.Fatalf("ChangeCriterion: %v", err)
	}
	if len(cap.checker) != 1 {
		t.Fatalf("checker calls: %d, want 1", len(cap.checker))
	}
	brief := briefOf(t, cap.checker[0])
	for _, s := range []string{"New passage of section sec-1", "New passage of section sec-2"} {
		if !strings.Contains(brief, s) {
			t.Fatalf("probe brief lacks %q:\n%s", s, brief)
		}
	}
}

// The same text writes nothing and probes nothing; an empty text or a
// label that is not a criterion is refused with nothing written.
func TestB46_CriterionChangeRefusesEmptyAndUnknownAndIgnoresTheSameText(t *testing.T) {
	db := seedCriterionProject(t, true)
	runner, cap := checkerRunner(t, db)
	ctx := context.Background()
	before := len(eventsOfType(t, db, store.EventStateChanged))
	if _, err := runner.ChangeCriterion(ctx, "p1", "crit-1", "  "); !errors.Is(err, cycle.ErrCriterionTextRequired) {
		t.Fatalf("empty: %v, want ErrCriterionTextRequired", err)
	}
	if _, err := runner.ChangeCriterion(ctx, "p1", "sec-1", newCriterion); !errors.Is(err, cycle.ErrCriterionNotFound) {
		t.Fatalf("not a criterion: %v, want ErrCriterionNotFound", err)
	}
	got, err := runner.ChangeCriterion(ctx, "p1", "crit-1", "Comparar conteos.")
	if err != nil || got.Version != 1 {
		t.Fatalf("same text: %+v, %v; want version 1 kept", got, err)
	}
	if after := len(eventsOfType(t, db, store.EventStateChanged)); after != before || len(queuedEvents(t, db)) != 0 || len(cap.checker) != 0 {
		t.Fatalf("events %d→%d, queued %d, checker %d; want nothing written or sent", before, after, len(queuedEvents(t, db)), len(cap.checker))
	}
}

// P-72 for a criterion (B-46, L16): changed while a turn of the project
// runs, its probe is queued in the same transaction as the new version,
// the affected sections are «sin comprobar» meanwhile, and it runs once
// when the turn ends.
func TestP72_NewCriterionDuringATurnIsQueuedAndRunsWhenTheTurnEnds(t *testing.T) {
	db := seedCriterionProject(t, true)
	fake := provider.NewFake(loadAppFixture(t, "04-principal-no-work.json"), checkerFixture(t, "no-findings"))
	fake.Recorder = db
	ctx := context.Background()
	var runner *cycle.Runner
	var cap *captureProvider
	fired := false
	inner := &fireDuringPrincipal{inner: fake, fire: func() {
		fired = true
		if _, err := runner.ChangeCriterion(ctx, "p1", "crit-1", newCriterion); err != nil {
			t.Errorf("change during the turn: %v", err)
			return
		}
		if n := len(cap.checker); n != 0 {
			t.Errorf("checker calls during the turn: %d, want 0 (the probe is queued, not sent)", n)
		}
		if q := queuedEvents(t, db); len(q) != 1 || q[0]["protected"] != "crit-1" ||
			mustJSON(t, q[0]["queued_sections"]) != `["sec-1"]` {
			t.Errorf("queued probe events: %v, want one for crit-1 over sec-1", q)
		}
		if !uncheckedNow(t, db, "sec-1") {
			t.Errorf("sec-1 while the probe is queued: want unchecked (B-50)")
		}
		if uncheckedNow(t, db, "sec-2") {
			t.Errorf("sec-2 while the probe is queued: unchecked, but no decision applying crit-1 uses it")
		}
	}}
	cap = &captureProvider{inner: inner}
	count := func(ctx context.Context, input []json.RawMessage) (int, error) {
		return fake.CountTokens(ctx, provider.PrincipalModel, input)
	}
	runner = cycle.NewRunner(db, cap, count, &fakePublisher{})

	if err := runner.RunTurn(ctx, "p1", "turn-1", "Sigue."); err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	if !fired {
		t.Fatal("the principal was never called")
	}
	if n := len(cap.checker); n != 1 {
		t.Fatalf("checker calls: %d, want the queued probe once when the turn ends", n)
	}
	if brief := briefOf(t, cap.checker[0]); !strings.Contains(brief, "crit-1 (criterion): "+newCriterion) {
		t.Fatalf("probe brief lacks the new criterion:\n%s", brief)
	}
	if uncheckedNow(t, db, "sec-1") {
		t.Fatal("sec-1 after the probe ran: still unchecked")
	}
}
