package server_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/LaurenteEber/tupaqnin/internal/cycle"
	"github.com/LaurenteEber/tupaqnin/internal/server"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

type stubStarter struct {
	projectID, text string
	err             error
}

func (s *stubStarter) StartTurn(projectID, text string) (string, error) {
	s.projectID, s.text = projectID, text
	if s.err != nil {
		return "", s.err
	}
	return "turn-1", nil
}

func (s *stubStarter) StartUnderstandingTurn(projectID string) (string, error) {
	s.projectID, s.text = projectID, "understanding"
	if s.err != nil {
		return "", s.err
	}
	return "turn-1", nil
}

func newAPI(t *testing.T) (http.Handler, *store.DB, *stubStarter) {
	t.Helper()
	db, err := store.Open(context.Background(), t.TempDir())
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	starter := &stubStarter{}
	return server.NewWithAPI(nil, server.NewHub(), db, starter), db, starter
}

func do(t *testing.T, h http.Handler, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var raw bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&raw).Encode(body); err != nil {
			t.Fatalf("encode body: %v", err)
		}
	}
	req := httptest.NewRequest(method, path, &raw)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func decode[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var out T
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode response %q: %v", rec.Body.String(), err)
	}
	return out
}

func TestAPI_ProjectSectionSaveAndMessageFlow(t *testing.T) {
	h, _, starter := newAPI(t)

	rec := do(t, h, http.MethodPost, "/api/projects", nil)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create project: got %d (%s)", rec.Code, rec.Body.String())
	}
	project := decode[struct {
		ProjectID string `json:"project_id"`
	}](t, rec)
	if project.ProjectID == "" {
		t.Fatal("create project: empty project_id")
	}
	base := "/api/projects/" + project.ProjectID

	rec = do(t, h, http.MethodPost, base+"/sections", map[string]any{"title": "Alcance"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create section: got %d (%s)", rec.Code, rec.Body.String())
	}
	section := decode[struct {
		Label   string `json:"label"`
		Version int    `json:"version"`
	}](t, rec)
	if section.Label != "sec-1" || section.Version != 1 {
		t.Fatalf("create section: got %+v", section)
	}

	// Open and save without editing: no new version (B-3).
	rec = do(t, h, http.MethodPut, base+"/sections/sec-1",
		map[string]any{"base_version": 1, "text": ""})
	if rec.Code != http.StatusNoContent {
		t.Fatalf("save without change: got %d, want 204", rec.Code)
	}

	rec = do(t, h, http.MethodPut, base+"/sections/sec-1",
		map[string]any{"base_version": 1, "text": "# Alcance\n\nPrimera versión."})
	if rec.Code != http.StatusOK {
		t.Fatalf("save: got %d (%s)", rec.Code, rec.Body.String())
	}
	saved := decode[struct {
		Label   string `json:"label"`
		Version int    `json:"version"`
	}](t, rec)
	if saved.Version != 2 {
		t.Fatalf("save: got version %d, want 2", saved.Version)
	}

	// A stale base version conflicts (specification §2).
	rec = do(t, h, http.MethodPut, base+"/sections/sec-1",
		map[string]any{"base_version": 1, "text": "otro texto"})
	if rec.Code != http.StatusConflict {
		t.Fatalf("save with old version: got %d, want 409", rec.Code)
	}

	rec = do(t, h, http.MethodGet, base+"/sections", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("list sections: got %d", rec.Code)
	}
	list := decode[[]struct {
		Label   string `json:"label"`
		Version int    `json:"version"`
		Text    string `json:"text"`
	}](t, rec)
	if len(list) != 1 || list[0].Text != "# Alcance\n\nPrimera versión." || list[0].Version != 2 {
		t.Fatalf("list sections: got %+v", list)
	}

	rec = do(t, h, http.MethodPost, base+"/messages", map[string]any{"text": "Escribe el alcance."})
	if rec.Code != http.StatusAccepted {
		t.Fatalf("post message: got %d (%s)", rec.Code, rec.Body.String())
	}
	msg := decode[struct {
		TurnID string `json:"turn_id"`
	}](t, rec)
	if msg.TurnID != "turn-1" || starter.projectID != project.ProjectID || starter.text != "Escribe el alcance." {
		t.Fatalf("post message: got %+v, starter %+v", msg, starter)
	}

	rec = do(t, h, http.MethodPost, base+"/messages", map[string]any{"text": "  "})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("post empty message: got %d, want 400", rec.Code)
	}
	rec = do(t, h, http.MethodGet, "/api/projects/nope/sections", nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("unknown project: got %d, want 404", rec.Code)
	}
}

// [EL DISEÑO NO LO DICE] A second message while a turn runs in the
// same project is refused with 409 turn_running, not queued.
func TestAPI_SecondMessageWhileTurnRunningIs409(t *testing.T) {
	h, _, starter := newAPI(t)
	rec := do(t, h, http.MethodPost, "/api/projects", nil)
	project := decode[struct {
		ProjectID string `json:"project_id"`
	}](t, rec)
	starter.err = cycle.ErrTurnRunning

	rec = do(t, h, http.MethodPost, "/api/projects/"+project.ProjectID+"/messages",
		map[string]any{"text": "another"})
	if rec.Code != http.StatusConflict {
		t.Fatalf("message while turn running: got %d (%s), want 409", rec.Code, rec.Body.String())
	}
	body := decode[struct {
		Error string `json:"error"`
	}](t, rec)
	if body.Error != "turn_running" {
		t.Fatalf("message while turn running: got error %q, want turn_running", body.Error)
	}
}
