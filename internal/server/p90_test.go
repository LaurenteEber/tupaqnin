package server_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/LaurenteEber/tupaqnin/internal/cycle"
	"github.com/LaurenteEber/tupaqnin/internal/graph"
	"github.com/LaurenteEber/tupaqnin/internal/server"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// P-90 (as P-89, B-14, K-16): a signoff that starts between your edit's
// read and its write is cancelled by your edit, in the write's
// transaction; it never stays in progress over a text it did not see.
func TestP90_ASignoffStartedBetweenYourEditsReadAndWriteIsCancelled(t *testing.T) {
	h, db, projectID := newSignoffAPI(t)
	seedSignSection(t, db, projectID, plainText, true)
	base := "/api/projects/" + projectID

	var opened cycle.SignoffOpen
	restore := server.SetBeforeSectionWriteForTest(func() {
		var err error
		if opened, err = cycle.NewGate(db).RequestSignoff(context.Background(), projectID, "sec-1"); err != nil || !opened.Opened {
			t.Errorf("signoff between the read and the write: %+v %v", opened, err)
		}
	})
	defer restore()
	if rec := do(t, h, http.MethodPut, base+"/sections/sec-1", map[string]any{"base_version": 1, "text": plainText + " Edito."}); rec.Code != http.StatusOK {
		t.Fatalf("edit: %d %s", rec.Code, rec.Body.String())
	}
	restore()
	signs, err := db.ListObjects(context.Background(), projectID, store.ObjectSignoff)
	if err != nil || len(signs) != 1 {
		t.Fatalf("signoffs: %d %v, want the one started in between", len(signs), err)
	}
	if v := getSignoff(t, h, base, signs[0].Label); v.Phase != graph.SignoffCancelled || v.Cancelled != graph.CancelEdited {
		t.Fatalf("signoff after your edit: %+v, want cancelled edited", v)
	}
	if got := sectionVersion(t, h, base); got.Version != 2 {
		t.Fatalf("the edit applies: %+v", got)
	}
}
