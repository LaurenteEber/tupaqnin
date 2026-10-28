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
