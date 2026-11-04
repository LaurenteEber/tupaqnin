package server_test

import (
	"net/http"
	"testing"

	"github.com/LaurenteEber/tupaqnin/internal/server"
)

func claimEditor(t *testing.T, h http.Handler, base, tab string) bool {
	t.Helper()
	return decode[server.EditorResponse](t, do(t, h, http.MethodPost, base+"/editor", map[string]any{"tab_id": tab})).Editor
}

// C.24: closing a tab fires its release and its last save. The save still saves,
// but it leaves the edit free, so the next tab opens with no read-only notice.
func TestC24_TheSaveOfATabThatLeavesSavesAndDoesNotClaimTheEdit(t *testing.T) {
	h, base := sectionAPI(t)
	if !claimEditor(t, h, base, "tab-a") {
		t.Fatal("the first tab is not the editor")
	}
	if rec := do(t, h, http.MethodDelete, base+"/editor", map[string]any{"tab_id": "tab-a"}); rec.Code != http.StatusNoContent {
		t.Fatalf("release: %d", rec.Code)
	}
	rec := do(t, h, http.MethodPut, base+"/sections/sec-1", map[string]any{"tab_id": "tab-a", "base_version": 1, "text": "Lo último que escribí.", "leaving": true})
	if rec.Code != http.StatusOK {
		t.Fatalf("the save at leaving: %d %s", rec.Code, rec.Body.String())
	}
	if got := decode[map[string]any](t, do(t, h, http.MethodGet, base+"/sections/sec-1", nil))["text"]; got != "Lo último que escribí." {
		t.Fatalf("text: %v", got)
	}
	if !claimEditor(t, h, base, "tab-b") {
		t.Fatal("the next tab is read-only: the save claimed the edit for a tab that left")
	}
}

// C.24: leaving does not skip the read-only check: another tab's lease refuses it,
// and nothing is saved.
func TestC24_TheSaveOfATabThatLeavesIsRefusedWhenAnotherTabEdits(t *testing.T) {
	h, base := sectionAPI(t)
	if !claimEditor(t, h, base, "tab-b") {
		t.Fatal("the first tab is not the editor")
	}
	rec := do(t, h, http.MethodPut, base+"/sections/sec-1", map[string]any{"tab_id": "tab-a", "base_version": 1, "text": "Desde la que se va.", "leaving": true})
	if rec.Code != http.StatusConflict || decode[map[string]any](t, rec)["error"] != "read_only_tab" {
		t.Fatalf("got %d %s", rec.Code, rec.Body.String())
	}
	if got := decode[map[string]any](t, do(t, h, http.MethodGet, base+"/sections/sec-1", nil))["text"]; got == "Desde la que se va." {
		t.Fatal("a refused save changed the text")
	}
}

// C.24, previous behavior: an ordinary save still claims a free edit.
func TestC24_AnOrdinarySaveStillClaimsTheEdit(t *testing.T) {
	h, base := sectionAPI(t)
	rec := do(t, h, http.MethodPut, base+"/sections/sec-1", map[string]any{"tab_id": "tab-a", "base_version": 1, "text": "Una edición normal."})
	if rec.Code != http.StatusOK {
		t.Fatalf("save: %d %s", rec.Code, rec.Body.String())
	}
	if claimEditor(t, h, base, "tab-b") {
		t.Fatal("the save did not hold the edit")
	}
}

func sectionAPI(t *testing.T) (http.Handler, string) {
	t.Helper()
	h, db, _ := newAPI(t)
	projectID := newProject(t, h, nil)
	seedUnderstanding(t, db, projectID)
	return h, "/api/projects/" + projectID
}
