package server_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/LaurenteEber/tupaqnin/internal/graph"
	"github.com/LaurenteEber/tupaqnin/internal/server"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// Piece 2.11 (backlog-r2) through the routes.

const (
	claimOne = "La embotellada trae más partículas."
	claimTwo = "Los filtros finos retienen más."
)

// seedClaims sends a request whose evidence_required the «Entendí»
// proposed as true (B-51) and creates sec-1 with two paragraphs you
// wrote, neither cited.
func seedClaims(t *testing.T, h http.Handler, db *store.DB) string {
	t.Helper()
	projectID := newProject(t, h, nil)
	if rec := do(t, h, http.MethodPut, "/api/projects/"+projectID+"/request", map[string]any{"text": "Estado del arte."}); rec.Code != http.StatusOK {
		t.Fatalf("put request: %d %s", rec.Code, rec.Body.String())
	}
	ctx := context.Background()
	err := db.Write(ctx, projectID, func(tx *store.Tx) error {
		req, err := tx.GetObject(ctx, projectID, "req-1")
		if err != nil {
			return err
		}
		var b map[string]any
		_ = json.Unmarshal(req.Body, &b)
		b["evidence_required"] = map[string]any{"value": true, "why": "Compara estudios.", "state": "proposed"}
		if _, err := tx.UpdateObject(req.ID, req.CurrentVersion, req.State, b,
			store.EventInput{Type: store.EventStateChanged, Actor: "agent"}); err != nil {
			return err
		}
		_, err = tx.CreateObject(store.ObjectSection, "draft", map[string]any{
			"title": "Síntesis", "text": claimOne + "\n\n" + claimTwo,
			"role": map[string]any{"value": "other", "state": "confirmed"}, "open_changes": []any{},
		}, store.EventInput{Type: store.EventStateChanged, Actor: "human"})
		return err
	})
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	return projectID
}

func unsourced(pendings []graph.Pending) []string {
	var out []string
	for _, p := range pendings {
		if p.Kind == graph.PendingUnsourcedClaim {
			out = append(out, p.About)
		}
	}
	return out
}

// R-1, R-2, T-1 through POST /sections/{label}/claims/accept: GET
// /sections/{label} brings each paragraph's provenance and its
// unsourced_claim; «ok» is refused (422), a passage that is not a
// paragraph of the section too (409); a valid «porque» closes it (204),
// leaves claim_accepted, and the paragraph's history says so.
func TestR1_AcceptRouteRefusesOkAndClosesWithAReason(t *testing.T) {
	h, db, _ := newAPI(t)
	projectID := seedClaims(t, h, db)
	base := "/api/projects/" + projectID

	detail := decode[server.SectionDetail](t, do(t, h, http.MethodGet, base+"/sections/sec-1", nil))
	if len(detail.Provenance) != 2 || detail.Provenance[0].Verified || detail.Provenance[0].Passage != claimOne ||
		detail.Provenance[1].Block != 2 {
		t.Fatalf("provenance: %+v, want two unverified paragraphs", detail.Provenance)
	}
	if got := unsourced(detail.Pendings); len(got) != 2 || got[0] != claimOne || got[1] != claimTwo {
		t.Fatalf("unsourced claims: %v, want both paragraphs", got)
	}
	// The list the page draws the editor from marks both paragraphs.
	list := decode[[]server.SectionView](t, do(t, h, http.MethodGet, base+"/sections", nil))
	if len(list) != 1 || len(list[0].Unverified) != 2 || list[0].Unverified[0] != 1 || list[0].Unverified[1] != 2 {
		t.Fatalf("sections list: %+v, want sec-1 with paragraphs 1 and 2 unverified", list)
	}

	if rec := do(t, h, http.MethodPost, base+"/sections/sec-1/claims/accept",
		map[string]any{"passage": claimOne, "reason": "ok"}); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("accept with ok: %d %s, want 422", rec.Code, rec.Body.String())
	}
	if rec := do(t, h, http.MethodPost, base+"/sections/sec-1/claims/accept",
		map[string]any{"passage": "La embotellada", "reason": "Es sabido."}); rec.Code != http.StatusConflict {
		t.Fatalf("accept a fragment: %d %s, want 409", rec.Code, rec.Body.String())
	}
	if rec := do(t, h, http.MethodPost, base+"/sections/sec-9/claims/accept",
		map[string]any{"passage": claimOne, "reason": "Es sabido."}); rec.Code != http.StatusNotFound {
		t.Fatalf("accept on sec-9: %d, want 404", rec.Code)
	}
	if n := countEvents(t, db, projectID, store.EventClaimAccepted); n != 0 {
		t.Fatalf("claim_accepted after refusals: %d", n)
	}

	const reason = "Lo sabe cualquiera del área."
	if rec := do(t, h, http.MethodPost, base+"/sections/sec-1/claims/accept",
		map[string]any{"passage": claimOne, "reason": reason}); rec.Code != http.StatusNoContent {
		t.Fatalf("accept: %d %s, want 204", rec.Code, rec.Body.String())
	}
	if n := countEvents(t, db, projectID, store.EventClaimAccepted); n != 1 {
		t.Fatalf("claim_accepted: %d, want 1", n)
	}
	detail = decode[server.SectionDetail](t, do(t, h, http.MethodGet, base+"/sections/sec-1", nil))
	if got := unsourced(detail.Pendings); len(got) != 1 || got[0] != claimTwo {
		t.Fatalf("unsourced claims after accepting: %v, want only the second", got)
	}
	if detail.Provenance[0].Accepted != reason {
		t.Fatalf("provenance: %+v, want the first accepted with your reason", detail.Provenance[0])
	}

	rec := do(t, h, http.MethodGet, base+"/sections/sec-1/history?block=1", nil)
	view := decode[server.BlockHistoryView](t, rec)
	if rec.Code != http.StatusOK || view.Provenance.Passage != claimOne || view.Provenance.Accepted != reason ||
		view.Provenance.Verified {
		t.Fatalf("history provenance: %d %+v", rec.Code, view.Provenance)
	}
}

// B-51 through PUT /request/evidence: «Corregir» leaves your value
// confirmed, keeps the «Entendí»'s why, and with false the paragraphs
// keep their mark but leave no unsourced_claim.
func TestB51_CorrectingEvidenceRequiredConfirmsYourValue(t *testing.T) {
	h, db, _ := newAPI(t)
	projectID := seedClaims(t, h, db)
	base := "/api/projects/" + projectID

	und := decode[server.UnderstandingView](t, do(t, h, http.MethodGet, base+"/understanding", nil))
	if und.EvidenceRequired == nil || !und.EvidenceRequired.Value || und.EvidenceRequired.State != "proposed" {
		t.Fatalf("understanding evidence_required: %+v, want proposed true", und.EvidenceRequired)
	}
	if rec := do(t, h, http.MethodPut, base+"/request/evidence", map[string]any{}); rec.Code != http.StatusBadRequest {
		t.Fatalf("put without required: %d, want 400", rec.Code)
	}
	before := countEvents(t, db, projectID, store.EventStateChanged)
	rec := do(t, h, http.MethodPut, base+"/request/evidence", map[string]any{"required": false})
	got := decode[server.EvidenceRequiredView](t, rec)
	if rec.Code != http.StatusOK || got.Value || got.State != "confirmed" || got.Why != "Compara estudios." {
		t.Fatalf("put evidence: %d %+v, want confirmed false with the why", rec.Code, got)
	}
	if n := countEvents(t, db, projectID, store.EventStateChanged); n != before+1 {
		t.Fatalf("state_changed: %d, want one more than %d", n, before)
	}
	und = decode[server.UnderstandingView](t, do(t, h, http.MethodGet, base+"/understanding", nil))
	if und.EvidenceRequired == nil || und.EvidenceRequired.Value || und.EvidenceRequired.State != "confirmed" {
		t.Fatalf("understanding after correcting: %+v", und.EvidenceRequired)
	}
	detail := decode[server.SectionDetail](t, do(t, h, http.MethodGet, base+"/sections/sec-1", nil))
	if got := unsourced(detail.Pendings); len(got) != 0 || len(detail.Provenance) != 2 || detail.Provenance[0].Verified {
		t.Fatalf("after correcting to false: pendings %v, provenance %+v", got, detail.Provenance)
	}

	// Without an account, a second project is a second visitor's (B3-18).
	h2 := anotherVisitor(t, db, h)
	other := newProject(t, h2, nil)
	if rec := do(t, h2, http.MethodPut, "/api/projects/"+other+"/request/evidence", map[string]any{"required": true}); rec.Code != http.StatusNotFound {
		t.Fatalf("put evidence without a request: %d, want 404", rec.Code)
	}
}
