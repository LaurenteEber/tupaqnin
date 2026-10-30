package cycle_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/LaurenteEber/tupaqnin/internal/cycle"
	"github.com/LaurenteEber/tupaqnin/internal/graph"
	"github.com/LaurenteEber/tupaqnin/internal/provider"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

func ptr(s string) *string { return &s }

// K-17: the title is required.
func TestK17_SectionWithoutTitleIsRefused(t *testing.T) {
	db := openTemp(t)
	if _, err := cycle.CreateSection(context.Background(), db, "p1", "   ", "", ""); !errors.Is(err, cycle.ErrTitleRequired) {
		t.Fatalf("CreateSection without title: got %v, want ErrTitleRequired", err)
	}
}

// K-17: «qué debe abordar» up to 60 words [inicial].
func TestK17_FocusLongerThan60WordsIsRefused(t *testing.T) {
	db := openTemp(t)
	long := strings.Repeat("palabra ", 61)
	if _, err := cycle.CreateSection(context.Background(), db, "p1", "Recomendaciones", "", long); !errors.Is(err, cycle.ErrFocusTooLong) {
		t.Fatalf("CreateSection with 61 words: got %v, want ErrFocusTooLong", err)
	}
	if _, err := cycle.CreateSection(context.Background(), db, "p1", "Recomendaciones", "", strings.Repeat("palabra ", 60)); err != nil {
		t.Fatalf("CreateSection with 60 words: %v", err)
	}
}

// K-17: with the «aquí se concluye» mark the role is born conclusion,
// confirmed; without it no role is set.
func TestK17_ConclusionMarkConfirmsTheRole(t *testing.T) {
	db := openTemp(t)
	ctx := context.Background()
	marked, err := cycle.CreateSection(ctx, db, "p1", "Conclusión", "conclusion", "")
	if err != nil {
		t.Fatalf("CreateSection marked: %v", err)
	}
	unmarked, err := cycle.CreateSection(ctx, db, "p1", "Recomendaciones", "", "")
	if err != nil {
		t.Fatalf("CreateSection unmarked: %v", err)
	}
	for _, c := range []struct {
		obj         store.Object
		value, stat string
	}{{marked, "conclusion", "confirmed"}, {unmarked, "", ""}} {
		obj, _ := db.GetObject(ctx, "p1", c.obj.Label)
		b := body(t, obj)
		role, _ := b["role"].(map[string]any)
		if v, _ := role["value"].(string); v != c.value {
			t.Fatalf("%s role: got %q, want %q", obj.Label, v, c.value)
		}
		if s, _ := role["state"].(string); s != c.stat {
			t.Fatalf("%s role state: got %q, want %q", obj.Label, s, c.stat)
		}
		if obj.State != "draft" || b["text"] != "" {
			t.Fatalf("%s: state %q text %q, want an empty draft", obj.Label, obj.State, b["text"])
		}
	}
	if n := len(eventsOfType(t, db, store.EventRoleChanged)); n != 1 {
		t.Fatalf("role_changed events: got %d, want 1 (only the marked one)", n)
	}
	if _, err := cycle.CreateSection(ctx, db, "p1", "Otra", "recommendation", ""); !errors.Is(err, cycle.ErrInvalidRole) {
		t.Fatalf("role other than conclusion: got %v, want ErrInvalidRole", err)
	}
}

// B-21: «qué debe abordar» is not text; a section with focus and no
// text stays draft.
func TestB21_SectionWithFocusAndNoTextStaysDraft(t *testing.T) {
	db := openTemp(t)
	ctx := context.Background()
	sec, err := cycle.CreateSection(ctx, db, "p1", "Recomendaciones", "", "Qué hacer con el aire de las escuelas.")
	if err != nil {
		t.Fatalf("CreateSection: %v", err)
	}
	status, _, err := graph.SectionStatus(ctx, db, "p1", sec.Label)
	if err != nil {
		t.Fatalf("SectionStatus: %v", err)
	}
	if status != graph.StatusDraft {
		t.Fatalf("status: got %q, want draft", status)
	}
}

// K-17, P-49: changing the title, the focus or the order of a signed
// section leaves it signed and records section_details_changed, one
// per section that changes.
func TestK17_DetailsAndOrderLeaveASignedSectionSigned(t *testing.T) {
	db := openTemp(t)
	ctx := context.Background()
	seedSignable(t, db, false)
	other, err := cycle.CreateSection(ctx, db, "p1", "Recomendaciones", "", "")
	if err != nil {
		t.Fatalf("CreateSection: %v", err)
	}
	if _, err := db.Writer.ExecContext(ctx, `UPDATE object SET state = 'signed' WHERE project_id = 'p1' AND label = 'sec-1'`); err != nil {
		t.Fatalf("sign: %v", err)
	}
	if err := cycle.ChangeSectionDetails(ctx, db, "p1", "sec-1", ptr("Regla nueva"), ptr("Explicar la comparación.")); err != nil {
		t.Fatalf("ChangeSectionDetails: %v", err)
	}
	if err := cycle.ChangeSectionDetails(ctx, db, "p1", "sec-1", ptr("Regla nueva"), nil); err != nil {
		t.Fatalf("ChangeSectionDetails without change: %v", err)
	}
	if err := cycle.ReorderSections(ctx, db, "p1", []string{other.Label, "sec-1"}); err != nil {
		t.Fatalf("ReorderSections: %v", err)
	}
	if got := sectionState(t, db, "sec-1"); got != "signed" {
		t.Fatalf("state: got %q, want signed", got)
	}
	events := eventsOfType(t, db, store.EventSectionDetailsChanged)
	// One for the title and focus of sec-1, then one per section that
	// changed position (both).
	if len(events) != 3 {
		t.Fatalf("section_details_changed: got %d, want 3", len(events))
	}
	if !strings.Contains(string(events[0].Data), `"field":"title"`) || !strings.Contains(string(events[0].Data), `"field":"focus"`) {
		t.Fatalf("details event data: %s", events[0].Data)
	}
	ordered, err := graph.OrderedSections(ctx, db, "p1")
	if err != nil {
		t.Fatalf("OrderedSections: %v", err)
	}
	if len(ordered) != 2 || ordered[0].Label != other.Label || ordered[1].Label != "sec-1" {
		t.Fatalf("order: got %v", labelsOf(ordered))
	}
	if err := cycle.ReorderSections(ctx, db, "p1", []string{"sec-1"}); !errors.Is(err, cycle.ErrInvalidOrder) {
		t.Fatalf("partial order: got %v, want ErrInvalidOrder", err)
	}
}

// K-17: the title stays out of what is signed; renaming the section
// during its signoff does not void it.
func TestK17_RenamingDuringTheSignoffStillSigns(t *testing.T) {
	db := openTemp(t)
	seedSignable(t, db, false)
	g := cycle.NewGate(db)
	ctx := context.Background()
	open, err := g.RequestSignoff(ctx, "p1", "sec-1")
	if err != nil || !open.Opened {
		t.Fatalf("RequestSignoff: %+v, %v", open, err)
	}
	r := cycle.NewRunner(db, provider.NewFake(), nil, nil)
	if _, err := r.SignoffExplanation("p1", open.Label, "Cuenta cómo se compararon los estudios."); err != nil {
		t.Fatalf("explanation: %v", err)
	}
	if err := cycle.ChangeSectionDetails(ctx, db, "p1", "sec-1", ptr("Otro título"), nil); err != nil {
		t.Fatalf("rename: %v", err)
	}
	if err := g.Sign(ctx, "p1", open.Label); err != nil {
		t.Fatalf("Sign after a rename: %v", err)
	}
	if got := sectionState(t, db, "sec-1"); got != "signed" {
		t.Fatalf("section: got %q, want signed", got)
	}
}

// K-17: a new section, yours or the principal's, goes last.
func TestK17_NewSectionGoesLast(t *testing.T) {
	db := openTemp(t)
	ctx := context.Background()
	for _, title := range []string{"Uno", "Dos", "Tres"} {
		if _, err := cycle.CreateSection(ctx, db, "p1", title, "", ""); err != nil {
			t.Fatalf("CreateSection %s: %v", title, err)
		}
	}
	if err := cycle.ReorderSections(ctx, db, "p1", []string{"sec-3", "sec-1", "sec-2"}); err != nil {
		t.Fatalf("ReorderSections: %v", err)
	}
	if _, err := cycle.CreateSection(ctx, db, "p1", "Cuatro", "", ""); err != nil {
		t.Fatalf("CreateSection: %v", err)
	}
	ordered, _ := graph.OrderedSections(ctx, db, "p1")
	if got := strings.Join(labelsOf(ordered), ","); got != "sec-3,sec-1,sec-2,sec-4" {
		t.Fatalf("order: got %s", got)
	}
}

func labelsOf(objs []store.Object) []string {
	out := make([]string, len(objs))
	for i, o := range objs {
		out[i] = o.Label
	}
	return out
}

func body(t *testing.T, obj store.Object) map[string]any {
	t.Helper()
	var b map[string]any
	if err := json.Unmarshal(obj.Body, &b); err != nil {
		t.Fatalf("decode %s: %v", obj.Label, err)
	}
	return b
}
