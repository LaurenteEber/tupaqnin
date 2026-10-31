package memory_test

import (
	"context"
	"strings"
	"testing"

	"github.com/LaurenteEber/tupaqnin/internal/memory"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// B-40: the stop report is unseen until a stop_report_seen after it; the
// memory reads that event, not a field of the stop_report (history only
// grows). A newer report is unseen again.
func TestB40_StopReportIsUnseenUntilItsStopReportSeenEvent(t *testing.T) {
	db, _ := seeded(t)
	mem := memory.New(db)
	ctx := context.Background()
	unseen := func() bool {
		t.Helper()
		st, err := mem.WorkingState(ctx, "p1")
		if err != nil {
			t.Fatalf("WorkingState: %v", err)
		}
		return st.Essentials.StopReportUnseen
	}
	if unseen() {
		t.Fatal("unseen without any stop report")
	}
	report := store.EventInput{Type: store.EventStopReport, Actor: "agent", AgentRole: "principal",
		Data: map[string]any{"stop_reason": "no_work", "seen": true}}
	appendEvent(t, db, report)
	if !unseen() {
		t.Fatal("a stop report without stop_report_seen counts as seen")
	}
	appendEvent(t, db, store.EventInput{Type: store.EventStopReportSeen, Actor: "human"})
	if unseen() {
		t.Fatal("still unseen after stop_report_seen")
	}
	appendEvent(t, db, report)
	if !unseen() {
		t.Fatal("a newer stop report counts as seen")
	}
}

// P-18 (bloque-2 §3, item 5): the startup carries your answered messages
// since the last stop, not those before it.
func TestP18_StartupCarriesAnsweredMessagesSinceTheLastStop(t *testing.T) {
	db, seeded := seeded(t)
	mem := memory.New(db)
	ctx := context.Background()
	appendEvent(t, db, store.EventInput{Type: store.EventStopReport, Actor: "agent", AgentRole: "principal",
		Data: map[string]any{"stop_reason": "no_work"}})
	after := "¿Y la cuenca alta?"
	appendEvent(t, db, store.EventInput{Type: store.EventHumanMessage, Actor: "human",
		Data: map[string]any{"text": after, "response": "También entra."}})
	brief, err := mem.Startup(ctx, "p1", memory.StartupNewSession, 1048576, fakeCount)
	if err != nil {
		t.Fatalf("Startup: %v", err)
	}
	text := briefText(brief)
	if strings.Contains(text, seeded.AnsweredMessage) {
		t.Fatal("an answered message from before the last stop is in the startup")
	}
	if !strings.Contains(text, after) {
		t.Fatal("the answered message since the last stop is missing")
	}
}
