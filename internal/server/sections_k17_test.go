package server_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/LaurenteEber/tupaqnin/internal/graph"
	"github.com/LaurenteEber/tupaqnin/internal/server"
)

// K-17: «+ Sección» without a title answers 400 title_required; a
// focus over 60 words, 400 focus_too_long.
func TestK17_CreateSectionWithoutTitleAnswers400(t *testing.T) {
	h, _, _ := newAPI(t)
	base := "/api/projects/" + newProject(t, h, nil)
	for _, c := range []struct {
		body map[string]any
		code string
	}{
		{map[string]any{"title": "  "}, "title_required"},
		{map[string]any{"title": "Recomendaciones", "focus": strings.Repeat("palabra ", 61)}, "focus_too_long"},
		{map[string]any{"title": "Recomendaciones", "role": "other"}, "invalid_role"},
	} {
		rec := do(t, h, http.MethodPost, base+"/sections", c.body)
		if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), c.code) {
			t.Fatalf("%v: got %d %s, want 400 %s", c.body, rec.Code, rec.Body.String(), c.code)
		}
	}
	rec := do(t, h, http.MethodPost, base+"/sections", map[string]any{"title": "Conclusión", "role": "conclusion", "focus": "La regla."})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
	}
	got := decode[server.SectionDetail](t, do(t, h, http.MethodGet, base+"/sections/sec-1", nil))
	if got.Title != "Conclusión" || got.Focus != "La regla." || got.Role != "conclusion" || got.RoleState != "confirmed" || got.Status != "draft" {
		t.Fatalf("section: %+v", got)
	}
}

// K-17, P-49: reordering changes the order of /sections and of the
// frame; the details route changes title and focus.
func TestK17_ReorderChangesTheOrderOfSectionsAndTheFrame(t *testing.T) {
	h, _, _ := newAPI(t)
	base := "/api/projects/" + newProject(t, h, nil)
	for _, title := range []string{"Uno", "Dos", "Tres"} {
		if rec := do(t, h, http.MethodPost, base+"/sections", map[string]any{"title": title}); rec.Code != http.StatusCreated {
			t.Fatalf("create %s: %d", title, rec.Code)
		}
	}
	if rec := do(t, h, http.MethodPut, base+"/sections/order", map[string]any{"labels": []string{"sec-3", "sec-1", "sec-2"}}); rec.Code != http.StatusNoContent {
		t.Fatalf("order: %d %s", rec.Code, rec.Body.String())
	}
	if rec := do(t, h, http.MethodPut, base+"/sections/order", map[string]any{"labels": []string{"sec-3", "sec-1"}}); rec.Code != http.StatusBadRequest {
		t.Fatalf("partial order: %d, want 400", rec.Code)
	}
	if rec := do(t, h, http.MethodPut, base+"/sections/sec-2/details", map[string]any{"title": "Dos bis", "focus": "Lo segundo."}); rec.Code != http.StatusNoContent {
		t.Fatalf("details: %d %s", rec.Code, rec.Body.String())
	}
	var list []string
	for _, s := range decode[[]server.SectionView](t, do(t, h, http.MethodGet, base+"/sections", nil)) {
		list = append(list, s.Label+":"+s.Title)
	}
	if got := strings.Join(list, ","); got != "sec-3:Tres,sec-1:Uno,sec-2:Dos bis" {
		t.Fatalf("/sections order: %s", got)
	}
	var frame []string
	for _, s := range decode[graph.Frame](t, do(t, h, http.MethodGet, base+"/frame", nil)).Sections {
		frame = append(frame, s.Label)
	}
	if got := strings.Join(frame, ","); got != "sec-3,sec-1,sec-2" {
		t.Fatalf("frame order: %s", got)
	}
}
