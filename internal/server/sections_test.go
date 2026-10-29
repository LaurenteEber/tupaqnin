package server_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/LaurenteEber/tupaqnin/internal/server"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

func sectionVersion(t *testing.T, h http.Handler, base string) server.SectionDetail {
	t.Helper()
	return decode[server.SectionDetail](t, do(t, h, http.MethodGet, base+"/sections/sec-1", nil))
}

// S7 (1.5): opening and saving without editing makes no version; the
// fingerprint decides, so a change of formatting only is no change.
func TestS4_SavingWithoutEditingMakesNoVersion(t *testing.T) {
	h, db, _ := newAPI(t)
	projectID := newProject(t, h, nil)
	seedUnderstanding(t, db, projectID)
	base := "/api/projects/" + projectID
	if rec := do(t, h, http.MethodPut, base+"/sections/sec-1", map[string]any{"base_version": 1, "text": "Un *énfasis* aquí."}); rec.Code != http.StatusOK {
		t.Fatalf("first save: %d", rec.Code)
	}
	for _, text := range []string{"Un *énfasis* aquí.", "Un _énfasis_   aquí."} {
		if rec := do(t, h, http.MethodPut, base+"/sections/sec-1", map[string]any{"base_version": 2, "text": text}); rec.Code != http.StatusNoContent {
			t.Fatalf("save %q: %d, want 204", text, rec.Code)
		}
	}
	if got := sectionVersion(t, h, base); got.Version != 2 || got.Text != "Un *énfasis* aquí." {
		t.Fatalf("section: version %d text %q, want version 2 unchanged", got.Version, got.Text)
	}
}

// Estados §1: your edit on a signed section moves it to
// changed_since_signoff.
func TestE1_YourEditOnASignedSectionChangesIt(t *testing.T) {
	h, db, _ := newAPI(t)
	projectID := newProject(t, h, nil)
	seedUnderstanding(t, db, projectID)
	if _, err := db.Writer.ExecContext(context.Background(),
		`UPDATE object SET state = 'signed' WHERE project_id = ? AND label = 'sec-1'`, projectID); err != nil {
		t.Fatalf("sign: %v", err)
	}
	base := "/api/projects/" + projectID
	if rec := do(t, h, http.MethodPut, base+"/sections/sec-1", map[string]any{"base_version": 1, "text": "Edito lo firmado."}); rec.Code != http.StatusOK {
		t.Fatalf("edit: %d %s", rec.Code, rec.Body.String())
	}
	if got := sectionVersion(t, h, base); got.Status != "changed_since_signoff" {
		t.Fatalf("status: %s, want changed_since_signoff", got.Status)
	}
}

// Specification §2: only one tab edits; a second tab opens read-only and
// cannot save.
func TestS2_ASecondTabCannotSave(t *testing.T) {
	h, db, _ := newAPI(t)
	projectID := newProject(t, h, nil)
	seedUnderstanding(t, db, projectID)
	base := "/api/projects/" + projectID
	if got := decode[server.EditorResponse](t, do(t, h, http.MethodPost, base+"/editor", map[string]any{"tab_id": "tab-a"})); !got.Editor {
		t.Fatal("first tab is not the editor")
	}
	if got := decode[server.EditorResponse](t, do(t, h, http.MethodPost, base+"/editor", map[string]any{"tab_id": "tab-b"})); got.Editor {
		t.Fatal("second tab became the editor")
	}
	if rec := do(t, h, http.MethodPut, base+"/sections/sec-1", map[string]any{"tab_id": "tab-b", "base_version": 1, "text": "Desde la segunda."}); rec.Code != http.StatusConflict {
		t.Fatalf("second tab save: %d, want 409", rec.Code)
	}
	if got := sectionVersion(t, h, base); got.Version != 1 {
		t.Fatalf("version after the refused save: %d, want 1", got.Version)
	}
	if rec := do(t, h, http.MethodPut, base+"/sections/sec-1", map[string]any{"tab_id": "tab-a", "base_version": 1, "text": "Desde la primera."}); rec.Code != http.StatusOK {
		t.Fatalf("editor tab save: %d, want 200", rec.Code)
	}
}

// P3: the section detail carries its status, pendings and linked
// figures; the summary carries what was spent against the cap.
func TestP3_SectionDetailAndSummary(t *testing.T) {
	h, db, _ := newAPI(t)
	projectID := newProject(t, h, nil)
	seedUnderstanding(t, db, projectID)
	seedCriticalDecision(t, db, projectID)
	base := "/api/projects/" + projectID
	got := sectionVersion(t, h, base)
	if got.Status != "draft" || len(got.Pendings) != 1 || got.Pendings[0].Kind != "open_card" || got.Title != "Síntesis" {
		t.Fatalf("detail: %+v", got)
	}
	list := decode[[]server.SectionView](t, do(t, h, http.MethodGet, base+"/sections", nil))
	if list[0].State != "draft" || list[0].Pendings != 1 {
		t.Fatalf("list: %+v, want the same status and pending count", list)
	}
	if err := db.RecordModelCall(context.Background(), store.ModelCall{ProjectID: projectID, Role: "principal", CostUSD: 0.25}); err != nil {
		t.Fatalf("record call: %v", err)
	}
	sum := decode[server.SummaryView](t, do(t, h, http.MethodGet, base+"/summary", nil))
	if sum.SpentUSD != 0.25 || sum.CapUSD != 3 {
		t.Fatalf("summary: %+v", sum)
	}
}
