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

func openTemp(t *testing.T) *store.DB {
	t.Helper()
	db, err := store.Open(context.Background(), t.TempDir())
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := db.Writer.ExecContext(context.Background(),
		`INSERT INTO project (id, created_at) VALUES ('p1', '2026-10-28T00:00:00.000Z')`); err != nil {
		t.Fatalf("seed project: %v", err)
	}
	return db
}

// recordedOutput replays one recorded provider fixture with the fake
// and returns the principal output text it carries (backlog 0.7:
// tests use recorded outputs, T-11).
func recordedOutput(t *testing.T, name string) json.RawMessage {
	t.Helper()
	path := filepath.Join("..", "provider", "testdata", name)
	fixture, err := provider.LoadFixture(path)
	if err != nil {
		t.Fatalf("LoadFixture %s: %v", name, err)
	}
	resp, err := provider.NewFake(fixture).Respond(context.Background(), provider.Request{}, nil)
	if err != nil {
		t.Fatalf("fake Respond %s: %v", name, err)
	}
	return json.RawMessage(resp.Text)
}

func eventsOfType(t *testing.T, db *store.DB, typ store.EventType) []store.Event {
	t.Helper()
	events, err := db.EventsSince(context.Background(), "p1", 0)
	if err != nil {
		t.Fatalf("EventsSince: %v", err)
	}
	var out []store.Event
	for _, ev := range events {
		if ev.Type == typ {
			out = append(out, ev)
		}
	}
	return out
}

func eventData(t *testing.T, ev store.Event) map[string]any {
	t.Helper()
	var data map[string]any
	if err := json.Unmarshal(ev.Data, &data); err != nil {
		t.Fatalf("event %s data: %v", ev.Type, err)
	}
	return data
}

func TestB2_EmbeddedSchemaIsIdenticalToTheDesignSchema(t *testing.T) {
	embedded, err := os.ReadFile(filepath.Join("schema", "principal_output.schema.json"))
	if err != nil {
		t.Fatalf("read embedded schema: %v", err)
	}
	design, err := os.ReadFile(filepath.Join("..", "..", "docs", "diseno", "bloque-3", "salida-principal.schema.json"))
	if err != nil {
		t.Fatalf("read design schema: %v", err)
	}
	if string(embedded) != string(design) {
		t.Fatal("embedded schema differs from docs/diseno/bloque-3/salida-principal.schema.json")
	}
}

// EndStep checks only the shape and writes nothing; ApplyStep writes
// step_ended with the step (backlog-r1 1.3).
func TestB2_ValidRecordedOutputEndsTheStep(t *testing.T) {
	db := openTemp(t)
	gate := cycle.NewGate(db)
	output := recordedOutput(t, filepath.Join("app", "01-principal-valid.json"))

	res, err := gate.EndStep(context.Background(), "p1", "turn-1", 1, output)
	if err != nil {
		t.Fatalf("EndStep: %v", err)
	}
	if res.Status != cycle.StatusAccepted || len(res.Errors) != 0 {
		t.Fatalf("result: got %+v, want accepted without errors", res)
	}
	if ended := eventsOfType(t, db, store.EventStepEnded); len(ended) != 0 {
		t.Fatalf("EndStep wrote %d step_ended, want 0 (ApplyStep writes it)", len(ended))
	}
	if applied, err := gate.ApplyStep(context.Background(), "p1", "turn-1", output); err != nil || !applied.Applied {
		t.Fatalf("ApplyStep: %+v, %v", applied, err)
	}
	ended := eventsOfType(t, db, store.EventStepEnded)
	if len(ended) != 1 {
		t.Fatalf("step_ended events: got %d, want 1", len(ended))
	}
	if ended[0].TurnID != "turn-1" {
		t.Fatalf("step_ended turn: got %q, want turn-1", ended[0].TurnID)
	}
	data := eventData(t, ended[0])
	if data["triage_line"] == "" || data["closing_risk"] != "none" {
		t.Fatalf("step_ended data: got %v", data)
	}
	note, _ := data["note"].(string)
	if !strings.Contains(note, "Intenté una primera sección mínima") {
		t.Fatalf("step_ended note not kept complete: %q", note)
	}
	deferred, _ := data["deferred"].([]any)
	if len(deferred) != 1 {
		t.Fatalf("step_ended deferred: got %v, want 1 item", data["deferred"])
	}
	if got := eventsOfType(t, db, store.EventOutputRetry); len(got) != 0 {
		t.Fatalf("output_retry events: got %d, want 0", len(got))
	}
}

// The real Meta recording carries nulls for the optional properties
// (transport schema, 0.8); stripping them must make it validate
// against the full schema in Go (B-2).
func TestB2_MicrotestRecordingWithOptionalNullsIsAccepted(t *testing.T) {
	db := openTemp(t)
	gate := cycle.NewGate(db)

	res, err := gate.EndStep(context.Background(), "p1", "turn-1", 1,
		recordedOutput(t, "microtest-valid.json"))
	if err != nil {
		t.Fatalf("EndStep: %v", err)
	}
	if res.Status != cycle.StatusAccepted {
		t.Fatalf("result: got %+v, want accepted", res)
	}
}

func TestB2_InvalidOutputsRetryOnceThenAgentPending(t *testing.T) {
	cases := []struct {
		name   string
		output json.RawMessage
	}{
		{"recorded without note", recordedOutput(t, filepath.Join("app", "02-principal-invalid.json"))},
		{"without operation", json.RawMessage(`{"triage_line":"Build: draft.","closing_risk":"none","note":"Tried.","new_text":[{"section_id":"new-1","base_version":0,"anchor":"","text":"# Scope"}]}`)},
		{"replace with empty anchor", json.RawMessage(`{"triage_line":"Build: draft.","closing_risk":"none","note":"Tried.","new_text":[{"section_id":"sec-1","base_version":1,"anchor":"","operation":"replace","text":"new"}]}`)},
		{"reviewed_unchanged without reason", json.RawMessage(`{"triage_line":"Build: draft.","closing_risk":"none","note":"Tried.","handled_changes":[{"change_id":"chg-1","result":"reviewed_unchanged"}]}`)},
		{"not json at all", json.RawMessage(`I drafted the section and all is well.`)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db := openTemp(t)
			gate := cycle.NewGate(db)

			first, err := gate.EndStep(context.Background(), "p1", "turn-1", 1, tc.output)
			if err != nil {
				t.Fatalf("EndStep attempt 1: %v", err)
			}
			if first.Status != cycle.StatusRetry || len(first.Errors) == 0 {
				t.Fatalf("attempt 1: got %+v, want retry with errors", first)
			}
			for _, e := range first.Errors {
				if strings.Contains(e, "p1") {
					t.Fatalf("error leaks an internal id: %q", e)
				}
			}
			retries := eventsOfType(t, db, store.EventOutputRetry)
			if len(retries) != 1 || eventData(t, retries[0])["attempt"] != float64(1) {
				t.Fatalf("output_retry after attempt 1: got %v", retries)
			}
			if got := eventsOfType(t, db, store.EventStepEnded); len(got) != 0 {
				t.Fatalf("step_ended after attempt 1: got %d, want 0", len(got))
			}

			second, err := gate.EndStep(context.Background(), "p1", "turn-1", 2, tc.output)
			if err != nil {
				t.Fatalf("EndStep attempt 2: %v", err)
			}
			if second.Status != cycle.StatusAgentPending || len(second.Errors) == 0 {
				t.Fatalf("attempt 2: got %+v, want agent_pending with errors", second)
			}
			retries = eventsOfType(t, db, store.EventOutputRetry)
			if len(retries) != 2 || eventData(t, retries[1])["result"] != "agent_pending" {
				t.Fatalf("output_retry after attempt 2: got %v", retries)
			}
			if got := eventsOfType(t, db, store.EventStepEnded); len(got) != 0 {
				t.Fatalf("step_ended after attempt 2: got %d, want 0", len(got))
			}
		})
	}
}

// T-6: the note is never rejected for length and is kept complete.
func TestT6_NoteOf500WordsIsNotRejectedAndIsKeptComplete(t *testing.T) {
	db := openTemp(t)
	gate := cycle.NewGate(db)

	note := strings.TrimSpace(strings.Repeat("palabra ", 500))
	output, err := json.Marshal(map[string]any{
		"triage_line": "Build: draft.", "closing_risk": "none", "note": note,
	})
	if err != nil {
		t.Fatalf("marshal output: %v", err)
	}
	res, err := gate.EndStep(context.Background(), "p1", "turn-1", 1, output)
	if err != nil {
		t.Fatalf("EndStep: %v", err)
	}
	if res.Status != cycle.StatusAccepted {
		t.Fatalf("result: got %+v, want accepted", res)
	}
	if applied, err := gate.ApplyStep(context.Background(), "p1", "turn-1", output); err != nil || !applied.Applied {
		t.Fatalf("ApplyStep: %+v, %v", applied, err)
	}
	ended := eventsOfType(t, db, store.EventStepEnded)
	if len(ended) != 1 {
		t.Fatalf("step_ended events: got %d, want 1", len(ended))
	}
	if got, _ := eventData(t, ended[0])["note"].(string); got != note {
		t.Fatalf("note: got %d words kept, want the complete 500", len(strings.Fields(got)))
	}
}

func TestB2_RetryMessageCarriesTheErrorsVerbatim(t *testing.T) {
	got := cycle.RetryMessage([]string{"/note: missing", "/closing_risk: invalid"})
	want := "Your output did not match the required schema: /note: missing; /closing_risk: invalid. Return the complete output again."
	if got != want {
		t.Fatalf("RetryMessage:\n got %q\nwant %q", got, want)
	}
}
