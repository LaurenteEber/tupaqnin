package server_test

import (
	"context"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"github.com/LaurenteEber/tupaqnin/internal/cycle"
	"github.com/LaurenteEber/tupaqnin/internal/graph"
	"github.com/LaurenteEber/tupaqnin/internal/provider"
	"github.com/LaurenteEber/tupaqnin/internal/server"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// newSignoffAPI is the API over a real runner whose fake replays the
// app's signoff recordings (the demo's Wonder rounds).
func newSignoffAPI(t *testing.T) (http.Handler, *store.DB, string) {
	t.Helper()
	db, err := store.Open(context.Background(), t.TempDir())
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	var fixtures []provider.Fixture
	for _, name := range []string{"05-signoff-first-round.json", "06-signoff-second-round.json"} {
		fx, err := provider.LoadFixture(filepath.Join("..", "provider", "testdata", "app", name))
		if err != nil {
			t.Fatalf("LoadFixture: %v", err)
		}
		fixtures = append(fixtures, fx)
	}
	hub := server.NewHub()
	runner := cycle.NewRunner(db, provider.NewFake(fixtures...), nil, hub)
	h := server.NewWithAPI(nil, hub, db, runner)
	return h, db, newProject(t, h, nil)
}

// seedSignSection creates sec-1 «Regla de comparación» (a conclusion)
// and, when critical, dec-1 used in it, critical by the role (T-5).
func seedSignSection(t *testing.T, db *store.DB, projectID, text string, critical bool) {
	t.Helper()
	err := db.Write(context.Background(), projectID, func(tx *store.Tx) error {
		sec, err := tx.CreateObject(store.ObjectSection, "draft", map[string]any{
			"title": "Regla de comparación", "text": text,
			"role":         map[string]any{"value": "conclusion", "state": "confirmed"},
			"open_changes": []any{},
		}, store.EventInput{Type: store.EventStateChanged, Actor: "agent"})
		if err != nil || !critical {
			return err
		}
		dec, err := tx.CreateObject(store.ObjectDecision, "agent", map[string]any{
			"statement": "Comparar solo estudios con el mismo tamaño mínimo.", "instead_of": "Comparar tal cual.",
			"would_choose_if": "Todos usaran el mismo filtro.", "assumption": false, "origin": "agent",
		}, store.EventInput{Type: store.EventStateChanged, Actor: "agent"})
		if err != nil {
			return err
		}
		_, err = tx.AddEdge(store.EdgeUsedIn, dec.ID, sec.ID, "", store.EventInput{Type: store.EventEdge, Actor: "agent"})
		return err
	})
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
}

func getSignoff(t *testing.T, h http.Handler, base, label string) graph.SignoffView {
	t.Helper()
	rec := do(t, h, http.MethodGet, base+"/signoffs/"+label, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET signoff: %d %s", rec.Code, rec.Body.String())
	}
	return decode[graph.SignoffView](t, rec)
}

// waitRounds waits for Wonder's background turn to record its round.
func waitRounds(t *testing.T, h http.Handler, base, label string, n int) graph.SignoffView {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		v := getSignoff(t, h, base, label)
		if v.Rounds >= n {
			return v
		}
		if time.Now().After(deadline) {
			t.Fatalf("round %d never arrived: %+v", n, v)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func openSignoffRoute(t *testing.T, h http.Handler, base string) graph.SignoffView {
	t.Helper()
	rec := do(t, h, http.MethodPost, base+"/sections/sec-1/signoff", nil)
	if rec.Code != http.StatusCreated {
		t.Fatalf("ask to sign: %d %s", rec.Code, rec.Body.String())
	}
	return decode[graph.SignoffView](t, rec)
}

const plainText = "Comparamos solo estudios con el mismo tamaño mínimo de partícula."

// P7 routes end to end with the demo's recordings: explain, origin
// question, answer, everything coincides, sign (S5 «todo coincide y se
// firma»).
func TestP7_SignoffRoutesSignWhenEverythingCoincides(t *testing.T) {
	h, db, projectID := newSignoffAPI(t)
	seedSignSection(t, db, projectID, plainText, true)
	base := "/api/projects/" + projectID

	v := openSignoffRoute(t, h, base)
	if v.Phase != graph.SignoffInProgress || v.Brief || len(v.CriticalDecisions) != 1 || v.CriticalDecisions[0] != "dec-1" {
		t.Fatalf("opened: %+v", v)
	}
	rec := do(t, h, http.MethodPost, base+"/signoffs/"+v.Label+"/explanation", map[string]any{"text": "Solo comparo estudios con el mismo tamaño, porque si no la diferencia sale del filtro."})
	if rec.Code != http.StatusAccepted || decode[server.PostMessageResponse](t, rec).TurnID == "" {
		t.Fatalf("explanation: %d %s", rec.Code, rec.Body.String())
	}
	v = waitRounds(t, h, base, v.Label, 1)
	if v.OriginQuestion == "" || v.CanSign || v.ContrastedBy != "muse-wonder-1.0 high" {
		t.Fatalf("after the first round: %+v", v)
	}
	if rec := do(t, h, http.MethodPost, base+"/signoffs/"+v.Label+"/sign", nil); rec.Code != http.StatusConflict {
		t.Fatalf("sign before coinciding: %d, want 409", rec.Code)
	}
	rec = do(t, h, http.MethodPost, base+"/signoffs/"+v.Label+"/answers", map[string]any{"origin_answer": "Del supuesto de que todos cuentan desde el mismo tamaño.", "case_answer": ""})
	if rec.Code != http.StatusAccepted {
		t.Fatalf("answers: %d %s", rec.Code, rec.Body.String())
	}
	if v = waitRounds(t, h, base, v.Label, 2); !v.CanSign {
		t.Fatalf("after the second round: %+v, want can sign", v)
	}
	rec = do(t, h, http.MethodPost, base+"/signoffs/"+v.Label+"/sign", nil)
	if rec.Code != http.StatusOK || decode[graph.SignoffView](t, rec).Phase != graph.SignoffSigned {
		t.Fatalf("sign: %d %s", rec.Code, rec.Body.String())
	}
	if got := sectionVersion(t, h, base); got.Status != "signed" {
		t.Fatalf("section: %s, want signed", got.Status)
	}
}

// S4: asking to sign with pendings lists them and opens nothing.
func TestB14_AskingToSignWithPendingsIs409WithTheList(t *testing.T) {
	h, db, projectID := newSignoffAPI(t)
	seedSignSection(t, db, projectID, plainText+" La diferencia es de 60 veces.", true)
	base := "/api/projects/" + projectID
	rec := do(t, h, http.MethodPost, base+"/sections/sec-1/signoff", nil)
	if rec.Code != http.StatusConflict {
		t.Fatalf("got %d %s, want 409", rec.Code, rec.Body.String())
	}
	body := decode[server.SignoffBlocked](t, rec)
	if body.Error != "pendings" || len(body.Pendings) != 1 || body.Pendings[0].Kind != graph.PendingFigureWithoutDecision {
		t.Fatalf("body: %+v", body)
	}
	if signs, _ := db.ListObjects(context.Background(), projectID, store.ObjectSignoff); len(signs) != 0 {
		t.Fatalf("sign-n created: %d", len(signs))
	}
}

// S5 «tu edición durante la firma la cancela» (K-16, B-14, 7h).
func TestK16_YourEditDuringTheSignoffCancelsIt(t *testing.T) {
	h, db, projectID := newSignoffAPI(t)
	seedSignSection(t, db, projectID, plainText, true)
	base := "/api/projects/" + projectID
	v := openSignoffRoute(t, h, base)
	if rec := do(t, h, http.MethodPut, base+"/sections/sec-1", map[string]any{"base_version": 1, "text": plainText + " Edito."}); rec.Code != http.StatusOK {
		t.Fatalf("edit: %d %s", rec.Code, rec.Body.String())
	}
	if v = getSignoff(t, h, base, v.Label); v.Phase != graph.SignoffCancelled || v.Cancelled != graph.CancelEdited {
		t.Fatalf("after your edit: %+v, want cancelled edited", v)
	}
	if got := sectionVersion(t, h, base); got.Version != 2 || got.Status != "ready" {
		t.Fatalf("the edit applies: %+v", got)
	}
	if rec := do(t, h, http.MethodPost, base+"/signoffs/"+v.Label+"/explanation", map[string]any{"text": "x y z"}); rec.Code != http.StatusConflict {
		t.Fatalf("explaining a cancelled signoff: %d, want 409", rec.Code)
	}
}

// 7g: leaving it for later cancels it (B-14, left).
func TestB14_LeavingTheSignoffCancelsIt(t *testing.T) {
	h, db, projectID := newSignoffAPI(t)
	seedSignSection(t, db, projectID, plainText, true)
	base := "/api/projects/" + projectID
	v := openSignoffRoute(t, h, base)
	rec := do(t, h, http.MethodPost, base+"/signoffs/"+v.Label+"/leave", nil)
	if rec.Code != http.StatusOK || decode[graph.SignoffView](t, rec).Cancelled != graph.CancelLeft {
		t.Fatalf("leave: %d %s", rec.Code, rec.Body.String())
	}
}

// 7f: a section without critical decisions signs brief, with no turn.
func TestA4_BriefSignoffThroughTheRoutes(t *testing.T) {
	h, db, projectID := newSignoffAPI(t)
	seedSignSection(t, db, projectID, plainText, false)
	base := "/api/projects/" + projectID
	v := openSignoffRoute(t, h, base)
	if !v.Brief {
		t.Fatalf("want a brief signoff: %+v", v)
	}
	rec := do(t, h, http.MethodPost, base+"/signoffs/"+v.Label+"/explanation", map[string]any{"text": "Dice qué estudios se comparan."})
	if rec.Code != http.StatusOK || decode[server.PostMessageResponse](t, rec).TurnID != "" {
		t.Fatalf("brief explanation: %d %s, want 200 without a turn", rec.Code, rec.Body.String())
	}
	if rec := do(t, h, http.MethodPost, base+"/signoffs/"+v.Label+"/sign", nil); rec.Code != http.StatusOK {
		t.Fatalf("sign: %d %s", rec.Code, rec.Body.String())
	}
}

func TestP7_UnknownSignoffIs404(t *testing.T) {
	h, _, projectID := newSignoffAPI(t)
	if rec := do(t, h, http.MethodGet, "/api/projects/"+projectID+"/signoffs/sign-9", nil); rec.Code != http.StatusNotFound {
		t.Fatalf("got %d, want 404", rec.Code)
	}
}
