package server_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/LaurenteEber/tupaqnin/internal/cycle"
	"github.com/LaurenteEber/tupaqnin/internal/graph"
	"github.com/LaurenteEber/tupaqnin/internal/server"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// newAPIAt is newAPI with the clock the return line reads (M-2): the
// test comes back `later` after the moment it asks, without sleeping.
func newAPIAt(t *testing.T, later *time.Duration) (http.Handler, *store.DB) {
	t.Helper()
	db, err := store.Open(context.Background(), t.TempDir())
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	clock := func() time.Time { return time.Now().Add(*later) }
	return withVisitor(t, db, server.NewWithAPI(nil, server.NewHub(), db, &stubStarter{}, server.WithClock(clock))), db
}

// editSection saves a section's text as you (PUT /sections/{label}).
func editSection(t *testing.T, h http.Handler, db *store.DB, projectID, label, text string) {
	t.Helper()
	sec, err := db.GetObject(context.Background(), projectID, label)
	if err != nil {
		t.Fatal(err)
	}
	rec := do(t, h, http.MethodPut, "/api/projects/"+projectID+"/sections/"+label,
		map[string]any{"base_version": sec.CurrentVersion, "text": text})
	if rec.Code != http.StatusOK {
		t.Fatalf("edit %s: %d (%s)", label, rec.Code, rec.Body.String())
	}
}

// agentChanges moves «Conclusiones» to stale and confirms dec-1, as
// the agent: two state changes after your last event.
func agentChanges(t *testing.T, db *store.DB, projectID string) {
	t.Helper()
	ctx := context.Background()
	err := db.Write(ctx, projectID, func(tx *store.Tx) error {
		sec, err := tx.GetObject(ctx, projectID, "sec-2")
		if err != nil {
			return err
		}
		if _, err := tx.UpdateObject(sec.ID, sec.CurrentVersion, "stale", map[string]any{"title": "Conclusiones",
			"text": conclusionsText, "role": map[string]any{"value": "conclusion", "state": "proposed"},
			"open_changes": []any{}}, store.EventInput{Type: store.EventStateChanged, Actor: "agent"}); err != nil {
			return err
		}
		dec, err := tx.GetObject(ctx, projectID, "dec-1")
		if err != nil {
			return err
		}
		var body map[string]any
		_ = json.Unmarshal(dec.Body, &body)
		_, err = tx.UpdateObject(dec.ID, dec.CurrentVersion, "confirmed", body,
			store.EventInput{Type: store.EventStateChanged, Actor: "agent"})
		return err
	})
	if err != nil {
		t.Fatalf("agent changes: %v", err)
	}
}

// M-2 (spec §9, B-48 a): back after 4 days, the frame brings the return
// line: the last section and your last act, the state changes since
// your last event, and how many pendings are yours.
func TestM2_ReturnLineAfterFourDaysBringsItsThreeNumbers(t *testing.T) {
	later := time.Duration(0)
	h, db := newAPIAt(t, &later)
	projectID := seedCase5Frame(t, h, db)
	editSection(t, h, db, projectID, "sec-1", synthesisText+"\n\nUna línea tuya.")
	agentChanges(t, db, projectID)

	later = 4 * 24 * time.Hour
	f := getFrame(t, h, projectID)
	line := f.ReturnLine
	if line == nil {
		t.Fatalf("no return line after 4 days")
	}
	if line.Days != 4 || line.Section != "sec-1" || line.Act != "edit" {
		t.Fatalf("where you left off: %+v, want 4 days, sec-1, edit", line)
	}
	if line.Changes != 2 || len(line.ChangedSections) != 1 || line.ChangedSections[0] != "sec-2" {
		t.Fatalf("what changed: %d %v, want 2 and [sec-2]", line.Changes, line.ChangedSections)
	}
	// Yours: the open card and the unconfirmed cases (one line).
	if line.Todo != 2 || f.Todo.Count != 2 {
		t.Fatalf("what is yours: line %d, todo count %d, want 2 (%+v)", line.Todo, f.Todo.Count, f.Todo)
	}
}

// M-2: one hour later there is no line; the frame opens as usual.
func TestM2_NoReturnLineOneHourLater(t *testing.T) {
	later := time.Duration(0)
	h, db := newAPIAt(t, &later)
	projectID := seedCase5Frame(t, h, db)
	editSection(t, h, db, projectID, "sec-1", synthesisText+"\n\nUna línea tuya.")
	agentChanges(t, db, projectID)

	later = time.Hour
	f := getFrame(t, h, projectID)
	if f.ReturnLine != nil {
		t.Fatalf("return line one hour later: %+v", f.ReturnLine)
	}
	if len(f.Sections) != 2 || f.Todo.Count != 2 {
		t.Fatalf("frame without the line changed: %+v", f)
	}
}

// seedReport appends a stop report that shows dec-1 (B-39).
func seedReport(t *testing.T, db *store.DB, projectID string) {
	t.Helper()
	report := cycle.StopReport{StopReason: cycle.StopUnseenDecisions, Section: "sec-1",
		Answer: "Paré: hay decisiones que no viste.",
		Decisions: []graph.UnseenDecision{{Label: "dec-1", Statement: "Comparar solo con el mismo filtro.",
			Sections: []string{"sec-1", "sec-2"}}}}
	if err := db.Write(context.Background(), projectID, func(tx *store.Tx) error {
		_, err := tx.AppendEvent(store.EventInput{Type: store.EventStopReport, Actor: "agent", AgentRole: "principal", Data: report})
		return err
	}); err != nil {
		t.Fatalf("seed report: %v", err)
	}
}

// B-48 (b): the short return opens with the unseen report and, under
// it, what the app detected since that report, your edits included;
// an edit before the report is not there.
func TestB48_YourEditAfterTheLastReportIsDetectedInTheShortReturn(t *testing.T) {
	later := time.Duration(0)
	h, db := newAPIAt(t, &later)
	projectID := seedCase5Frame(t, h, db)
	editSection(t, h, db, projectID, "sec-1", synthesisText+"\n\nAntes del informe.")
	seedReport(t, db, projectID)
	editSection(t, h, db, projectID, "sec-2", conclusionsText+"\n\nDespués del informe.")

	later = time.Hour
	view := decode[server.StopReportView](t, do(t, h, http.MethodGet, "/api/projects/"+projectID+"/stop-report", nil))
	if !view.OpensConversation || view.Seen {
		t.Fatalf("short return with an unseen report must open the conversation with it: %+v", view)
	}
	var yours []server.DetectedView
	for _, d := range view.Detected {
		if d.Kind == "edit" {
			yours = append(yours, d)
		}
	}
	if len(yours) != 1 || yours[0].Actor != "human" || yours[0].Section == nil ||
		yours[0].Section.Label != "sec-2" || yours[0].Section.Title != "Conclusiones" || yours[0].Count != 1 {
		t.Fatalf("detected edits: %+v, want only yours on «Conclusiones»", view.Detected)
	}

	// After 3 days the return line takes the lead: the report is a click
	// away in «Lo que te toca» and does not open the conversation.
	later = 4 * 24 * time.Hour
	view = decode[server.StopReportView](t, do(t, h, http.MethodGet, "/api/projects/"+projectID+"/stop-report", nil))
	if view.OpensConversation || len(view.Detected) == 0 {
		t.Fatalf("long return: %+v", view)
	}
}

// B3-9 (the exact «sin ver»): a critical decision with a decision_seen
// from any way (frame, card, history, report) carries no dot and leaves
// the filter.
func TestB3_9_CriticalDecisionSeenByAnyWayHasNoDot(t *testing.T) {
	ways := map[string]func(t *testing.T, h http.Handler, db *store.DB, base string){
		"frame": func(t *testing.T, h http.Handler, _ *store.DB, base string) {
			if rec := do(t, h, http.MethodPost, base+"/frame/decisions/dec-1/seen", nil); rec.Code != http.StatusNoContent {
				t.Fatalf("frame seen: %d", rec.Code)
			}
		},
		"card": func(t *testing.T, h http.Handler, _ *store.DB, base string) {
			if rec := do(t, h, http.MethodPost, base+"/cards/card-1/answer",
				map[string]any{"option": "keep", "reason": "Si no, comparo filtros y no aguas."}); rec.Code != http.StatusOK {
				t.Fatalf("card: %d (%s)", rec.Code, rec.Body.String())
			}
		},
		"history": func(t *testing.T, h http.Handler, _ *store.DB, base string) {
			if rec := do(t, h, http.MethodGet, base+"/decisions/dec-1/history", nil); rec.Code != http.StatusOK {
				t.Fatalf("history: %d", rec.Code)
			}
		},
		"report": func(t *testing.T, h http.Handler, db *store.DB, base string) {
			seedReport(t, db, base[len("/api/projects/"):])
			if rec := do(t, h, http.MethodPost, base+"/stop-report/seen", nil); rec.Code != http.StatusNoContent {
				t.Fatalf("report seen: %d", rec.Code)
			}
		},
	}
	for way, see := range ways {
		t.Run(way, func(t *testing.T) {
			h, db, _ := newAPI(t)
			projectID := seedCase5Frame(t, h, db)
			base := "/api/projects/" + projectID
			f := getFrame(t, h, projectID)
			if f.Sections[0].Decisions[0].Seen || len(f.Unseen) != 1 || f.Unseen[0].Decision.Label != "dec-1" {
				t.Fatalf("before: dot %v, filter %+v", f.Sections[0].Decisions[0].Seen, f.Unseen)
			}
			see(t, h, db, base)
			f = getFrame(t, h, projectID)
			for _, s := range f.Sections {
				for _, d := range s.Decisions {
					if d.Label == "dec-1" && !d.Seen {
						t.Fatalf("dec-1 under %s still has its dot after %s", s.Title, way)
					}
				}
			}
			if len(f.Unseen) != 0 {
				t.Fatalf("filter after %s: %+v", way, f.Unseen)
			}
		})
	}
}

// addCriticalDecision creates a decision of the agent used in the given
// sections (all conclusions, so it is critical, T-5).
func addCriticalDecision(t *testing.T, db *store.DB, projectID, statement string, sections ...string) {
	t.Helper()
	ctx := context.Background()
	agent := store.EventInput{Type: store.EventStateChanged, Actor: "agent"}
	err := db.Write(ctx, projectID, func(tx *store.Tx) error {
		return cycle.TrackCriticality(ctx, tx, "", func() error {
			dec, err := tx.CreateObject(store.ObjectDecision, "agent", map[string]any{
				"statement": statement, "instead_of": "Otra cosa.", "origin": "agent"}, agent)
			if err != nil {
				return err
			}
			for _, label := range sections {
				sec, err := tx.GetObject(ctx, projectID, label)
				if err != nil {
					return err
				}
				if _, err := tx.AddEdge(store.EdgeUsedIn, dec.ID, sec.ID, "", nil, store.EventInput{Type: store.EventEdge, Actor: "agent"}); err != nil {
					return err
				}
			}
			return nil
		})
	})
	if err != nil {
		t.Fatalf("add decision: %v", err)
	}
}

// B3-9 / spec §9: the filter «solo lo que no viste» puts the decisions
// with more used_in edges first and, at equal number, follows the
// document order.
func TestB3_9_UnseenFilterOrdersByAffectedSections(t *testing.T) {
	h, db, _ := newAPI(t)
	projectID := seedCase5Frame(t, h, db) // dec-1: «Síntesis» and «Conclusiones»
	rec := do(t, h, http.MethodPost, "/api/projects/"+projectID+"/sections",
		map[string]any{"title": "Método", "role": "conclusion"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("new section: %d (%s)", rec.Code, rec.Body.String())
	}
	addCriticalDecision(t, db, projectID, "Solo en la síntesis.", "sec-1")               // dec-2: 1 section, first
	addCriticalDecision(t, db, projectID, "En conclusiones y método.", "sec-2", "sec-3") // dec-3: 2 sections, later

	f := getFrame(t, h, projectID)
	var got []string
	for _, u := range f.Unseen {
		got = append(got, u.Decision.Label)
	}
	want := []string{"dec-1", "dec-3", "dec-2"}
	if len(got) != len(want) {
		t.Fatalf("filter: %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("filter: %v, want %v", got, want)
		}
	}
	if f.Unseen[1].Section != "sec-2" || f.Unseen[2].Section != "sec-1" {
		t.Fatalf("each item under its first section: %+v", f.Unseen)
	}
}

// Spec §9: ≤ 300 ms with the case, now with the return line and the
// filter computed.
func TestM2_FrameWithReturnLineOfCase5Within300ms(t *testing.T) {
	later := time.Duration(0)
	h, db := newAPIAt(t, &later)
	projectID := seedCase5Frame(t, h, db)
	editSection(t, h, db, projectID, "sec-1", synthesisText+"\n\nUna línea tuya.")
	later = 10 * 24 * time.Hour
	getFrame(t, h, projectID) // warm the connection
	start := time.Now()
	f := getFrame(t, h, projectID)
	if took := time.Since(start); took > 300*time.Millisecond {
		t.Fatalf("frame took %s, want ≤ 300 ms (§9)", took)
	}
	if f.ReturnLine == nil {
		t.Fatalf("no return line after 10 days")
	}
}
