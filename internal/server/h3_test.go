package server_test

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/LaurenteEber/tupaqnin/internal/server"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// Piece 2.8 (backlog-r2): H-3's actions after the request is sent:
// change the audience or a criterion, the role end to end (K-13), and
// download the document as .md with what is not signed marked (B-47).

// submittedProject is a project whose request was sent: its criterion
// and premise exist, and the request's evidence_required is confirmed.
func submittedProject(t *testing.T, h http.Handler) (string, string) {
	t.Helper()
	projectID := newProject(t, h, map[string]any{"name": "Tesis"})
	base := "/api/projects/" + projectID
	if rec := do(t, h, http.MethodPut, base+"/request", map[string]any{
		"text": "Escribe la síntesis.", "audience": "Mi comité", "criteria": []string{"Solo estudios de campo."},
		"premise": "La embotellada trae más.", "exploratory": false,
	}); rec.Code != http.StatusOK {
		t.Fatalf("put request: %d %s", rec.Code, rec.Body.String())
	}
	if rec := do(t, h, http.MethodPost, base+"/request/submit", nil); rec.Code != http.StatusAccepted {
		t.Fatalf("submit: %d %s", rec.Code, rec.Body.String())
	}
	if rec := do(t, h, http.MethodPut, base+"/request/evidence", map[string]any{"required": true}); rec.Code != http.StatusOK {
		t.Fatalf("evidence: %d %s", rec.Code, rec.Body.String())
	}
	return projectID, base
}

func eventCount(t *testing.T, db *store.DB, projectID string) int {
	t.Helper()
	events, err := db.EventsSince(context.Background(), projectID, 0)
	if err != nil {
		t.Fatalf("EventsSince: %v", err)
	}
	return len(events)
}

func requestBodyOf(t *testing.T, db *store.DB, projectID string) map[string]any {
	t.Helper()
	obj, err := db.GetObject(context.Background(), projectID, "req-1")
	if err != nil {
		t.Fatalf("req-1: %v", err)
	}
	var b map[string]any
	_ = json.Unmarshal(obj.Body, &b)
	return b
}

func setState(t *testing.T, db *store.DB, projectID, label, state string) {
	t.Helper()
	if _, err := db.Writer.ExecContext(context.Background(),
		`UPDATE object SET state = ? WHERE project_id = ? AND label = ?`, state, projectID, label); err != nil {
		t.Fatalf("set %s %s: %v", label, state, err)
	}
}

// After the request is sent, PUT /request answers 409 request_submitted
// and writes nothing: it would rewrite the audience and the criteria
// with no audience_changed and no criterion version, and erase
// evidence_required (2.11).
func TestH3_PutRequestAfterSubmitIs409AndWritesNothing(t *testing.T) {
	h, db, _ := newAPI(t)
	projectID, base := submittedProject(t, h)
	before := eventCount(t, db, projectID)

	rec := do(t, h, http.MethodPut, base+"/request", map[string]any{
		"text": "Otro pedido.", "audience": "Otro", "criteria": []string{"Otro criterio."}, "premise": "Otra.",
	})
	if rec.Code != http.StatusConflict || decode[map[string]string](t, rec)["error"] != "request_submitted" {
		t.Fatalf("put request after submit: %d %s, want 409 request_submitted", rec.Code, rec.Body.String())
	}
	if after := eventCount(t, db, projectID); after != before {
		t.Fatalf("events: %d → %d, want nothing written", before, after)
	}
	got := decode[server.RequestView](t, do(t, h, http.MethodGet, base+"/request", nil))
	if got.Text != "Escribe la síntesis." || got.Audience != "Mi comité" || len(got.Criteria) != 1 || got.Criteria[0] != "Solo estudios de campo." {
		t.Fatalf("request after the refused put: %+v", got)
	}
	if ev, _ := requestBodyOf(t, db, projectID)["evidence_required"].(map[string]any); ev["state"] != "confirmed" {
		t.Fatalf("evidence_required: %v, want kept confirmed", ev)
	}
	// The evidence route (2.11) still works after submit.
	if rec := do(t, h, http.MethodPut, base+"/request/evidence", map[string]any{"required": false}); rec.Code != http.StatusOK {
		t.Fatalf("evidence after submit: %d %s", rec.Code, rec.Body.String())
	}
}

// K-13: changing the audience records audience_changed with the old and
// new value, keeps the rest of the request, and moves no section: a
// signed one stays signed.
func TestK13_ChangingTheAudienceLeavesASignedSectionSigned(t *testing.T) {
	h, db, _ := newAPI(t)
	projectID, base := submittedProject(t, h)
	seedSignSection(t, db, projectID, plainText, true)
	setState(t, db, projectID, "sec-1", "signed")
	seen, _ := db.EventsSince(context.Background(), projectID, 0)
	last := seen[len(seen)-1].Seq

	rec := do(t, h, http.MethodPut, base+"/request/audience", map[string]any{"audience": "Jurado externo"})
	if rec.Code != http.StatusOK {
		t.Fatalf("put audience: %d %s", rec.Code, rec.Body.String())
	}
	if got := decode[server.RequestView](t, rec); got.Audience != "Jurado externo" || got.Text != "Escribe la síntesis." {
		t.Fatalf("view: %+v", got)
	}
	sec, err := db.GetObject(context.Background(), projectID, "sec-1")
	if err != nil || sec.State != "signed" {
		t.Fatalf("sec-1: %s, %v; want signed", sec.State, err)
	}
	events, _ := db.EventsSince(context.Background(), projectID, last)
	var changed []map[string]any
	for _, ev := range events {
		if ev.Type == store.EventAudienceChanged {
			var d map[string]any
			_ = json.Unmarshal(ev.Data, &d)
			if ev.Actor != "human" {
				t.Fatalf("audience_changed actor: %s", ev.Actor)
			}
			changed = append(changed, d)
		}
		if ev.Type == store.EventStateChanged && strings.Contains(string(ev.Data), "sec-1") {
			t.Fatalf("a state change of sec-1 after the audience changed: %s", ev.Data)
		}
	}
	if len(changed) != 1 || changed[0]["from"] != "Mi comité" || changed[0]["to"] != "Jurado externo" {
		t.Fatalf("audience_changed: %v, want one from «Mi comité» to «Jurado externo»", changed)
	}
	b := requestBodyOf(t, db, projectID)
	if ev, _ := b["evidence_required"].(map[string]any); ev["state"] != "confirmed" || b["text"] != "Escribe la síntesis." {
		t.Fatalf("request body: %v, want the rest kept", b)
	}
	// The same audience writes nothing.
	before := eventCount(t, db, projectID)
	if rec := do(t, h, http.MethodPut, base+"/request/audience", map[string]any{"audience": "Jurado externo"}); rec.Code != http.StatusOK {
		t.Fatalf("same audience: %d", rec.Code)
	}
	if after := eventCount(t, db, projectID); after != before {
		t.Fatalf("same audience wrote %d events", after-before)
	}
}

// K-13 with K-6, end to end through the role route: making «conclusion»
// the role of a signed section with an agent's decision makes that
// decision critical, so the section moves to changed_since_signoff and
// the decision's card opens.
func TestK13_RoleToConclusionOnASignedSectionWithAnAgentDecisionChangesItAndOpensItsCard(t *testing.T) {
	h, db, _ := newAPI(t)
	projectID := newProject(t, h, nil)
	base := "/api/projects/" + projectID
	err := db.Write(context.Background(), projectID, func(tx *store.Tx) error {
		sec, err := tx.CreateObject(store.ObjectSection, "draft", map[string]any{
			"title": "Regla de comparación", "text": plainText,
			"role": map[string]any{"value": "other", "state": "confirmed"}, "open_changes": []any{},
		}, store.EventInput{Type: store.EventStateChanged, Actor: "agent"})
		if err != nil {
			return err
		}
		dec, err := tx.CreateObject(store.ObjectDecision, "agent", map[string]any{
			"statement": "Comparar solo estudios con el mismo tamaño mínimo.", "instead_of": "Comparar tal cual.",
			"would_choose_if": "Todos usaran el mismo filtro.", "assumption": false, "origin": "agent",
		}, store.EventInput{Type: store.EventStateChanged, Actor: "agent"})
		if err != nil {
			return err
		}
		_, err = tx.AddEdge(store.EdgeUsedIn, dec.ID, sec.ID, "", nil, store.EventInput{Type: store.EventEdge, Actor: "agent"})
		return err
	})
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	if cards, _ := db.ListObjects(context.Background(), projectID, store.ObjectCard); len(cards) != 0 {
		t.Fatalf("cards before the role change: %d, want none (dec-1 not critical yet)", len(cards))
	}
	setState(t, db, projectID, "sec-1", "signed")

	if rec := do(t, h, http.MethodPut, base+"/sections/sec-1/role", map[string]any{"role": "conclusion"}); rec.Code != http.StatusOK {
		t.Fatalf("put role: %d %s", rec.Code, rec.Body.String())
	}
	sec, _ := db.GetObject(context.Background(), projectID, "sec-1")
	if sec.State != "changed_since_signoff" {
		t.Fatalf("sec-1: %s, want changed_since_signoff", sec.State)
	}
	cards, err := db.ListObjects(context.Background(), projectID, store.ObjectCard)
	if err != nil || len(cards) != 1 || cards[0].State != "open" {
		t.Fatalf("cards: %+v, %v; want dec-1's card open", cards, err)
	}
	if rec := do(t, h, http.MethodGet, base+"/cards/"+cards[0].Label, nil); rec.Code != http.StatusOK {
		t.Fatalf("get card: %d %s", rec.Code, rec.Body.String())
	}
}

// PUT /criteria/{label}: the new version, yours, through the route (the
// probe itself is tested in internal/cycle); an empty text is 400, a
// label that is not a criterion 404, both writing nothing.
func TestB46_PutCriterionCreatesItsNewVersion(t *testing.T) {
	h, db, _ := newAPI(t)
	projectID, base := submittedProject(t, h)

	rec := do(t, h, http.MethodPut, base+"/criteria/crit-1", map[string]any{"text": "Solo estudios de campo en Europa."})
	if rec.Code != http.StatusOK {
		t.Fatalf("put criterion: %d %s", rec.Code, rec.Body.String())
	}
	if got := decode[server.CriterionView](t, rec); got.Label != "crit-1" || got.Text != "Solo estudios de campo en Europa." || got.Version != 2 {
		t.Fatalf("criterion view: %+v", got)
	}
	view := decode[server.RequestView](t, do(t, h, http.MethodGet, base+"/request", nil))
	if len(view.SubmittedCriteria) != 1 || view.SubmittedCriteria[0].Label != "crit-1" ||
		view.SubmittedCriteria[0].Text != "Solo estudios de campo en Europa." || view.Criteria[0] != "Solo estudios de campo en Europa." {
		t.Fatalf("request view: %+v", view)
	}
	before := eventCount(t, db, projectID)
	if rec := do(t, h, http.MethodPut, base+"/criteria/crit-1", map[string]any{"text": " "}); rec.Code != http.StatusBadRequest ||
		decode[map[string]string](t, rec)["error"] != "criterion_text_required" {
		t.Fatalf("empty criterion: %d %s", rec.Code, rec.Body.String())
	}
	if rec := do(t, h, http.MethodPut, base+"/criteria/prem-1", map[string]any{"text": "Otra."}); rec.Code != http.StatusNotFound ||
		decode[map[string]string](t, rec)["error"] != "criterion_not_found" {
		t.Fatalf("not a criterion: %d %s", rec.Code, rec.Body.String())
	}
	if after := eventCount(t, db, projectID); after != before {
		t.Fatalf("refused changes wrote %d events", after-before)
	}
}

var anyLabel = regexp.MustCompile(`\b(req|crit|case|prem|sec|dec|fig|dat|cit|src|oq|card|prop|sig|aq|round|chg)-\d+\b`)

// B-47, K-10 (b): the .md has the sections in position order with their
// title and current text, without «qué debe abordar», and the «Not
// signed» quote line under the title of each section that is not
// signed; no label and no history; the file name comes from
// project.name.
func TestB47_ExportMarksOnlyTheUnsignedSectionAndCarriesNoLabel(t *testing.T) {
	h, db, _ := newAPI(t)
	projectID := newProject(t, h, map[string]any{"name": "Tesis: agua/grifo"})
	base := "/api/projects/" + projectID
	err := db.Write(context.Background(), projectID, func(tx *store.Tx) error {
		for _, s := range []struct {
			title, focus, text string
			position           int
		}{
			{"Regla de comparación", "Qué regla usar.", "Comparamos solo estudios de campo.", 2},
			{"Método", "Cómo se cuenta.", "Contamos partículas por litro.", 1},
		} {
			if _, err := tx.CreateObject(store.ObjectSection, "draft", map[string]any{
				"title": s.title, "focus": s.focus, "position": s.position, "text": s.text,
				"role": map[string]any{"value": "other", "state": "confirmed"}, "open_changes": []any{},
			}, store.EventInput{Type: store.EventStateChanged, Actor: "human"}); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	setState(t, db, projectID, "sec-1", "signed")

	rec := do(t, h, http.MethodGet, base+"/export.md", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("export: %d %s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/markdown") {
		t.Fatalf("content type: %q", ct)
	}
	cd := rec.Header().Get("Content-Disposition")
	if !strings.HasPrefix(cd, "attachment;") || !strings.Contains(cd, "Tesis") || strings.Contains(cd, "/") || !strings.Contains(cd, ".md") {
		t.Fatalf("content disposition: %q, want an attachment named after the project, without a path", cd)
	}
	want := "## Método\n\n> Not signed\n\nContamos partículas por litro.\n\n## Regla de comparación\n\nComparamos solo estudios de campo.\n"
	if got := rec.Body.String(); got != want {
		t.Fatalf("export:\n%q\nwant:\n%q", got, want)
	}
	if anyLabel.MatchString(rec.Body.String()) || strings.Contains(rec.Body.String(), "Qué regla usar.") {
		t.Fatalf("export carries a label or «qué debe abordar»:\n%s", rec.Body.String())
	}
	es := do(t, h, http.MethodGet, base+"/export.md?lang=es", nil).Body.String()
	if strings.Count(es, "> Sin firmar") != 1 || strings.Contains(es, "Not signed") {
		t.Fatalf("export in Spanish:\n%s", es)
	}
	if rec := do(t, h, http.MethodGet, "/api/projects/nope/export.md", nil); rec.Code != http.StatusNotFound {
		t.Fatalf("unknown project: %d", rec.Code)
	}
}

// B-47's mark is the on-screen catalog's (T-1): Go's copy matches
// web/src/i18n's export.not_signed in both languages.
func TestB47_ExportMarkMatchesTheCatalog(t *testing.T) {
	for lang, want := range server.ExportNotSigned {
		raw, err := os.ReadFile(filepath.Join("..", "..", "web", "src", "i18n", lang+".json"))
		if err != nil {
			t.Fatalf("catalog %s: %v", lang, err)
		}
		var catalog map[string]string
		if err := json.Unmarshal(raw, &catalog); err != nil {
			t.Fatalf("catalog %s: %v", lang, err)
		}
		if catalog["export.not_signed"] != want {
			t.Fatalf("%s: catalog %q, Go %q", lang, catalog["export.not_signed"], want)
		}
	}
	if len(server.ExportNotSigned) != 2 {
		t.Fatalf("languages: %v, want en and es", server.ExportNotSigned)
	}
}
