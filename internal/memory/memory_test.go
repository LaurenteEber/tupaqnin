package memory_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/LaurenteEber/tupaqnin/internal/memory"
	seeddata "github.com/LaurenteEber/tupaqnin/internal/memory/testdata"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// fakeCount mirrors the fake provider's count (B-5): 1 token per
// 4 characters of the input items.
func fakeCount(_ context.Context, input []json.RawMessage) (int, error) {
	total := 0
	for _, item := range input {
		total += len(item)
	}
	return total / 4, nil
}

func seeded(t *testing.T) (*store.DB, seeddata.Seeded) {
	t.Helper()
	db, err := store.Open(context.Background(), t.TempDir())
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	seeded, err := seeddata.Seed(context.Background(), db, "p1")
	if err != nil {
		t.Fatalf("Seed: %v", err)
	}
	return db, seeded
}

func briefText(b memory.Brief) string {
	var sb strings.Builder
	for _, item := range b.Input {
		sb.Write(item)
		sb.WriteByte('\n')
	}
	return sb.String()
}

func TestB2_HumanTextEntersWorkingStateVerbatim(t *testing.T) {
	db, seeded := seeded(t)
	mem := memory.New(db)

	state, err := mem.WorkingState(context.Background(), "p1")
	if err != nil {
		t.Fatalf("WorkingState: %v", err)
	}
	if state.Essentials.RequestText != seeded.RequestText ||
		state.Essentials.Audience != seeded.Audience {
		t.Fatalf("request not verbatim: %+v", state.Essentials)
	}
	if len(state.Essentials.Criteria) != 2 {
		t.Fatalf("criteria: got %d, want 2", len(state.Essentials.Criteria))
	}
	for i, c := range state.Essentials.Criteria {
		if c.Text != seeded.CriteriaTexts[i] {
			t.Fatalf("criterion %d not verbatim: %q", i, c.Text)
		}
		if len(c.Cases) == 0 {
			t.Fatalf("criterion %d has no understanding cases", i)
		}
	}
	if state.Essentials.PremiseText != seeded.PremiseText {
		t.Fatalf("premise not verbatim: %q", state.Essentials.PremiseText)
	}
	foundUnanswered := false
	for _, m := range state.Essentials.UnansweredMessages {
		if m == seeded.UnansweredMessage {
			foundUnanswered = true
		}
	}
	if !foundUnanswered {
		t.Fatalf("unanswered message missing from essentials: %v", state.Essentials.UnansweredMessages)
	}
	if len(state.Essentials.OpenCards) != 1 {
		t.Fatalf("open cards: got %v, want 1", state.Essentials.OpenCards)
	}
	critical := 0
	for _, d := range state.Essentials.Decisions {
		if d.Critical {
			critical++
		}
	}
	if critical != 2 {
		t.Fatalf("critical decisions in essentials: got %d, want 2", critical)
	}
}

func TestB2_StartupOfSeededProjectIsWithinCapAndHasNoInternalIDs(t *testing.T) {
	db, seeded := seeded(t)
	mem := memory.New(db)

	brief, err := mem.Startup(context.Background(), "p1", memory.StartupNewSession, 1048576, fakeCount)
	if err != nil {
		t.Fatalf("Startup: %v", err)
	}
	if brief.InstanceID == "" {
		t.Fatal("Startup: empty instance id")
	}
	capTokens := 1048576 * 15 / 100
	if brief.Tokens > capTokens || brief.OverCap {
		t.Fatalf("Startup tokens: got %d (over=%v), cap %d", brief.Tokens, brief.OverCap, capTokens)
	}
	text := briefText(brief)
	if !strings.Contains(text, seeded.RequestText) {
		t.Fatal("brief does not carry the request verbatim")
	}
	if !strings.Contains(text, seeded.UnansweredMessage) {
		t.Fatal("brief does not carry the current task (unanswered message)")
	}
	for _, id := range seeded.InternalIDs {
		if id != "" && strings.Contains(text, id) {
			t.Fatalf("brief leaks internal id %q", id)
		}
	}
	events, err := db.EventsSince(context.Background(), "p1", 0)
	if err != nil {
		t.Fatalf("EventsSince: %v", err)
	}
	var startups int
	for _, ev := range events {
		if ev.Type == store.EventStartup {
			startups++
			var data map[string]any
			if err := json.Unmarshal(ev.Data, &data); err != nil {
				t.Fatalf("startup data: %v", err)
			}
			if data["reason"] != "new_session" || data["instance_id"] != brief.InstanceID {
				t.Fatalf("startup event data: got %v", data)
			}
		}
	}
	if startups != 1 {
		t.Fatalf("startup events: got %d, want 1", startups)
	}
}

func TestB2_StartupTrimsInTheDesignOrderWhenOverCap(t *testing.T) {
	db, seeded := seeded(t)
	mem := memory.New(db)

	// A small window makes the seeded brief pass the 15% cap.
	brief, err := mem.Startup(context.Background(), "p1", memory.StartupNewSession, 2000, fakeCount)
	if err != nil {
		t.Fatalf("Startup: %v", err)
	}
	text := briefText(brief)
	// Core and current task are never trimmed.
	if !strings.Contains(text, seeded.RequestText) {
		t.Fatal("core was trimmed: request missing")
	}
	if !strings.Contains(text, seeded.UnansweredMessage) {
		t.Fatal("current task was trimmed: unanswered message missing")
	}
	// Trimming order (§3): index down to sections only, then changes
	// to one line each, then the oldest answered messages.
	if strings.Contains(text, seeded.AnsweredMessage) {
		t.Fatal("answered messages were not trimmed (oldest first)")
	}
	for _, label := range seeded.SectionLabels {
		if !strings.Contains(text, label) {
			t.Fatalf("section %s missing from the trimmed index", label)
		}
	}
}

func TestB2_ModelChangeStartsANewInstanceCarryingOnlyNoteAndDeferred(t *testing.T) {
	db, seeded := seeded(t)
	mem := memory.New(db)
	ctx := context.Background()

	first, err := mem.Startup(ctx, "p1", memory.StartupNewSession, 1048576, fakeCount)
	if err != nil {
		t.Fatalf("Startup new_session: %v", err)
	}
	err = db.Write(ctx, "p1", func(tx *store.Tx) error {
		_, err := tx.AppendEvent(store.EventInput{
			Type: store.EventStepEnded, Actor: "agent", AgentRole: "principal", TurnID: "turn-1",
			Data: map[string]any{
				"triage_line": "Build: drafted.", "closing_risk": "none",
				"note":     "del agente: tried the short draft first",
				"deferred": []any{map[string]any{"what": seeded.DeferredWhat, "why": "needs the audience"}},
			},
		})
		return err
	})
	if err != nil {
		t.Fatalf("append step_ended: %v", err)
	}

	second, err := mem.Startup(ctx, "p1", memory.StartupModelChange, 1048576, fakeCount)
	if err != nil {
		t.Fatalf("Startup model_change: %v", err)
	}
	if second.InstanceID == "" || second.InstanceID == first.InstanceID {
		t.Fatalf("model change must create a new instance: %q vs %q", second.InstanceID, first.InstanceID)
	}
	text := briefText(second)
	if !strings.Contains(text, "del agente: tried the short draft first") {
		t.Fatal("previous note did not pass to the new instance")
	}
	if !strings.Contains(text, seeded.DeferredWhat) {
		t.Fatal("previous deferred item did not pass to the new instance")
	}
	var reasons []string
	events, err := db.EventsSince(ctx, "p1", 0)
	if err != nil {
		t.Fatalf("EventsSince: %v", err)
	}
	for _, ev := range events {
		if ev.Type == store.EventStartup {
			var data map[string]any
			if err := json.Unmarshal(ev.Data, &data); err != nil {
				t.Fatalf("startup data: %v", err)
			}
			reasons = append(reasons, fmt.Sprint(data["reason"]))
		}
	}
	if len(reasons) != 2 || reasons[0] != "new_session" || reasons[1] != "model_change" {
		t.Fatalf("startup reasons: got %v", reasons)
	}
}

// T-6: the agent note enters the startup as its first 80 words,
// marked as truncated; it is never rejected for length.
func TestT6_AgentNoteEntersStartupAsFirst80WordsMarkedTruncated(t *testing.T) {
	db, _ := seeded(t)
	mem := memory.New(db)
	ctx := context.Background()

	words := make([]string, 100)
	for i := range words {
		words[i] = fmt.Sprintf("word%03d", i+1)
	}
	note := strings.Join(words, " ")
	err := db.Write(ctx, "p1", func(tx *store.Tx) error {
		_, err := tx.AppendEvent(store.EventInput{
			Type: store.EventStepEnded, Actor: "agent", AgentRole: "principal", TurnID: "turn-1",
			Data: map[string]any{"triage_line": "Build.", "closing_risk": "none", "note": note, "deferred": []any{}},
		})
		return err
	})
	if err != nil {
		t.Fatalf("append step_ended: %v", err)
	}

	brief, err := mem.Startup(ctx, "p1", memory.StartupNewSession, 1048576, fakeCount)
	if err != nil {
		t.Fatalf("Startup: %v", err)
	}
	text := briefText(brief)
	if !strings.Contains(text, "word080") {
		t.Fatal("brief misses the 80th word of the note")
	}
	if strings.Contains(text, "word081") {
		t.Fatal("brief carries more than the first 80 words of the note")
	}
	if !strings.Contains(text, "truncated") {
		t.Fatal("brief does not mark the note as truncated")
	}
	// The full note stays in the working state (in flight).
	state, err := mem.WorkingState(ctx, "p1")
	if err != nil {
		t.Fatalf("WorkingState: %v", err)
	}
	if state.InFlight.AgentNote != note {
		t.Fatal("working state does not keep the complete note")
	}
}

// T-6 (specification §3): a deferred item over 30 words is not
// trimmed; it is registered as such in flight (a signal for B2-6).
func TestT6_LongDeferredItemIsRegisteredInFlightNotTrimmed(t *testing.T) {
	db, _ := seeded(t)
	mem := memory.New(db)
	ctx := context.Background()

	longWhat := strings.TrimSpace(strings.Repeat("pendiente ", 20))
	longWhy := strings.TrimSpace(strings.Repeat("porque ", 15)) // 35 words in the item
	err := db.Write(ctx, "p1", func(tx *store.Tx) error {
		_, err := tx.AppendEvent(store.EventInput{
			Type: store.EventStepEnded, Actor: "agent", AgentRole: "principal", TurnID: "turn-1",
			Data: map[string]any{
				"triage_line": "Build.", "closing_risk": "none", "note": "short note",
				"deferred": []any{
					map[string]any{"what": "short one", "why": "later"},
					map[string]any{"what": longWhat, "why": longWhy},
				},
			},
		})
		return err
	})
	if err != nil {
		t.Fatalf("append step_ended: %v", err)
	}

	state, err := mem.WorkingState(ctx, "p1")
	if err != nil {
		t.Fatalf("WorkingState: %v", err)
	}
	if len(state.InFlight.Deferred) != 2 {
		t.Fatalf("deferred in flight: got %d items, want 2", len(state.InFlight.Deferred))
	}
	short, long := state.InFlight.Deferred[0], state.InFlight.Deferred[1]
	if short.Long {
		t.Fatal("short deferred item marked long")
	}
	if !long.Long {
		t.Fatal("deferred item over 30 words was not registered as long")
	}
	if long.What != longWhat || long.Why != longWhy {
		t.Fatal("long deferred item was trimmed in flight")
	}
}

func TestB2_ReadRegistersDeliveryAndClearsTheChange(t *testing.T) {
	db, seeded := seeded(t)
	mem := memory.New(db)
	ctx := context.Background()

	brief, err := mem.Startup(ctx, "p1", memory.StartupNewSession, 1048576, fakeCount)
	if err != nil {
		t.Fatalf("Startup: %v", err)
	}
	changes, err := mem.ChangesSinceDelivery(ctx, "p1")
	if err != nil {
		t.Fatalf("ChangesSinceDelivery: %v", err)
	}
	if len(changes) != 0 {
		t.Fatalf("changes right after startup: got %v, want none", changes)
	}

	// The human edits the first section: a new version the principal
	// has not been delivered.
	sec, err := db.GetObject(ctx, "p1", seeded.SectionLabels[0])
	if err != nil {
		t.Fatalf("GetObject: %v", err)
	}
	err = db.Write(ctx, "p1", func(tx *store.Tx) error {
		_, err := tx.UpdateObject(sec.ID, sec.CurrentVersion, "draft",
			map[string]any{"text": "# Alcance\n\nTexto editado por la persona.", "role": map[string]any{"value": "other", "state": "proposed"}, "open_changes": []any{}},
			store.EventInput{Type: store.EventEdit, Actor: "human"})
		return err
	})
	if err != nil {
		t.Fatalf("UpdateObject: %v", err)
	}
	changes, err = mem.ChangesSinceDelivery(ctx, "p1")
	if err != nil {
		t.Fatalf("ChangesSinceDelivery after edit: %v", err)
	}
	if len(changes) != 1 || changes[0].Label != seeded.SectionLabels[0] || changes[0].Version != 2 {
		t.Fatalf("changes after edit: got %+v", changes)
	}

	item, err := mem.Read(ctx, "p1", brief.InstanceID, seeded.SectionLabels[0])
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if item.Version != 2 || !strings.Contains(item.Text, "Texto editado por la persona.") {
		t.Fatalf("Read item: got %+v", item)
	}
	changes, err = mem.ChangesSinceDelivery(ctx, "p1")
	if err != nil {
		t.Fatalf("ChangesSinceDelivery after read: %v", err)
	}
	if len(changes) != 0 {
		t.Fatalf("changes after read: got %v, want none", changes)
	}
	if _, err := mem.Read(ctx, "p1", brief.InstanceID, "sec-99"); err == nil {
		t.Fatal("Read of an unknown label: want an error")
	}
}

func appendEvent(t *testing.T, db *store.DB, ev store.EventInput) {
	t.Helper()
	err := db.Write(context.Background(), "p1", func(tx *store.Tx) error {
		_, err := tx.AppendEvent(ev)
		return err
	})
	if err != nil {
		t.Fatalf("append %s: %v", ev.Type, err)
	}
}

// (1) Every unanswered human message enters the core verbatim,
// not only the first one.
func TestB2_CoreRendersAllUnansweredMessagesVerbatim(t *testing.T) {
	db, seeded := seeded(t)
	mem := memory.New(db)
	ctx := context.Background()

	const second = "Segunda pregunta sin respuesta: ¿cuánto cuesta mantenerlo?"
	appendEvent(t, db, store.EventInput{
		Type: store.EventHumanMessage, Actor: "human",
		Data: map[string]any{"text": second},
	})

	brief, err := mem.Startup(ctx, "p1", memory.StartupNewSession, 1048576, fakeCount)
	if err != nil {
		t.Fatalf("Startup: %v", err)
	}
	if len(brief.Input) == 0 {
		t.Fatal("brief has no input items")
	}
	core := string(brief.Input[0]) // the first item is the core (§3 order)
	for _, want := range []string{seeded.UnansweredMessage, second} {
		if !strings.Contains(core, want) {
			t.Fatalf("core does not render unanswered message verbatim: %q", want)
		}
	}
}

// (2) The compact form of a decision carries its reason (B2-1:
// statement · reason · instead of · edges). The reason is the
// «porque» recorded in the history for that decision.
func TestB2_DecisionCompactFormCarriesTheReason(t *testing.T) {
	db, seeded := seeded(t)
	mem := memory.New(db)
	ctx := context.Background()

	dec, err := db.GetObject(ctx, "p1", seeded.DecisionLabels[0])
	if err != nil {
		t.Fatalf("GetObject: %v", err)
	}
	appendEvent(t, db, store.EventInput{
		Type: store.EventStateChanged, Actor: "human", ObjectID: dec.ID,
		Reason: "Porque la junta lo pidió así, con el caudal de marzo.",
	})

	state, err := mem.WorkingState(ctx, "p1")
	if err != nil {
		t.Fatalf("WorkingState: %v", err)
	}
	var found *memory.Decision
	for i, d := range state.Essentials.Decisions {
		if d.Label == seeded.DecisionLabels[0] {
			found = &state.Essentials.Decisions[i]
		}
	}
	if found == nil {
		t.Fatalf("decision %s not in essentials", seeded.DecisionLabels[0])
	}
	if found.Reason != "Porque la junta lo pidió así, con el caudal de marzo." {
		t.Fatalf("decision reason: got %q", found.Reason)
	}
	brief, err := mem.Startup(ctx, "p1", memory.StartupNewSession, 1048576, fakeCount)
	if err != nil {
		t.Fatalf("Startup: %v", err)
	}
	if text := briefText(brief); !strings.Contains(text, found.Reason) {
		t.Fatal("brief compact form does not carry the decision reason")
	}
}

// (3) The startup renders what WorkingState already computes:
// mode message, current step, open changes per section, premise
// exploratory, open audience questions, signoff paused and an
// unseen stop report.
func TestB2_StartupRendersWhatWorkingStateComputes(t *testing.T) {
	db, seeded := seeded(t)
	mem := memory.New(db)
	ctx := context.Background()

	appendEvent(t, db, store.EventInput{
		Type: store.EventModeSet, Actor: "human",
		Data: map[string]any{"mode": "run_to_completion", "message": "Sigue hasta terminar, sin parar."},
	})
	appendEvent(t, db, store.EventInput{Type: store.EventSignoffPaused, Actor: "app"})
	appendEvent(t, db, store.EventInput{
		Type: store.EventStopReport, Actor: "agent", AgentRole: "principal",
		Data: map[string]any{"seen": false},
	})
	appendEvent(t, db, store.EventInput{
		Type: store.EventStepEnded, Actor: "agent", AgentRole: "principal", TurnID: "turn-9",
		Data: map[string]any{"triage_line": "Build.", "closing_risk": "none", "note": "nota", "deferred": []any{}},
	})

	err := db.Write(ctx, "p1", func(tx *store.Tx) error {
		_, err := tx.CreateObject(store.ObjectAudienceQuestion, "open",
			map[string]any{"question": "¿Cuánto cuesta regar de noche?", "section_id": ""},
			store.EventInput{Type: store.EventStateChanged, Actor: "human"})
		return err
	})
	if err != nil {
		t.Fatalf("create audience question: %v", err)
	}
	prem, err := db.GetObject(ctx, "p1", "prem-1")
	if err != nil {
		t.Fatalf("GetObject prem-1: %v", err)
	}
	err = db.Write(ctx, "p1", func(tx *store.Tx) error {
		_, err := tx.UpdateObject(prem.ID, prem.CurrentVersion, "current",
			map[string]any{"text": seeded.PremiseText, "request_id": "", "exploratory": true},
			store.EventInput{Type: store.EventStateChanged, Actor: "human"})
		return err
	})
	if err != nil {
		t.Fatalf("update premise: %v", err)
	}
	sec, err := db.GetObject(ctx, "p1", seeded.SectionLabels[0])
	if err != nil {
		t.Fatalf("GetObject section: %v", err)
	}
	err = db.Write(ctx, "p1", func(tx *store.Tx) error {
		_, err := tx.UpdateObject(sec.ID, sec.CurrentVersion, "draft",
			map[string]any{
				"text":         "# Alcance\n\nPrimera línea.",
				"role":         map[string]any{"value": "conclusion", "state": "proposed"},
				"open_changes": []any{map[string]any{"label": "chg-1"}},
			},
			store.EventInput{Type: store.EventEdit, Actor: "human"})
		return err
	})
	if err != nil {
		t.Fatalf("update section: %v", err)
	}

	brief, err := mem.Startup(ctx, "p1", memory.StartupNewSession, 1048576, fakeCount)
	if err != nil {
		t.Fatalf("Startup: %v", err)
	}
	core := string(brief.Input[0])
	lower := strings.ToLower(core)
	for _, want := range []string{
		"Sigue hasta terminar, sin parar.", // mode message, verbatim
		"turn-9",                           // current step
		"chg-1",                            // open changes per section
		"aq-1",                             // open audience question
	} {
		if !strings.Contains(core, want) {
			t.Fatalf("core does not render %q", want)
		}
	}
	for _, want := range []string{"exploratory", "paused", "unseen"} {
		if !strings.Contains(lower, want) {
			t.Fatalf("core does not render %q", want)
		}
	}
}

// (4) Startup records the deliveries with the versions used to
// build the brief, in the same transaction as the startup event.
func TestB2_StartupDeliveriesMatchTheBriefVersions(t *testing.T) {
	db, seeded := seeded(t)
	mem := memory.New(db)
	ctx := context.Background()

	sec, err := db.GetObject(ctx, "p1", seeded.SectionLabels[0])
	if err != nil {
		t.Fatalf("GetObject: %v", err)
	}
	err = db.Write(ctx, "p1", func(tx *store.Tx) error {
		_, err := tx.UpdateObject(sec.ID, sec.CurrentVersion, "draft",
			map[string]any{"text": "# Alcance\n\nVersión dos.", "role": map[string]any{"value": "other", "state": "proposed"}, "open_changes": []any{}},
			store.EventInput{Type: store.EventEdit, Actor: "human"})
		return err
	})
	if err != nil {
		t.Fatalf("UpdateObject: %v", err)
	}

	brief, err := mem.Startup(ctx, "p1", memory.StartupNewSession, 1048576, fakeCount)
	if err != nil {
		t.Fatalf("Startup: %v", err)
	}
	if brief.InstanceID == "" {
		t.Fatal("empty instance")
	}
	delivered, err := db.LatestDeliveries(ctx, "p1")
	if err != nil {
		t.Fatalf("LatestDeliveries: %v", err)
	}
	if delivered[sec.ID] != 2 {
		t.Fatalf("delivered version of the edited section: got %d, want 2 (the version in the brief)", delivered[sec.ID])
	}
	changes, err := mem.ChangesSinceDelivery(ctx, "p1")
	if err != nil {
		t.Fatalf("ChangesSinceDelivery: %v", err)
	}
	if len(changes) != 0 {
		t.Fatalf("changes after startup: got %v, want none", changes)
	}
}
