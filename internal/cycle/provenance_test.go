package cycle_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/LaurenteEber/tupaqnin/internal/cycle"
	"github.com/LaurenteEber/tupaqnin/internal/graph"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// Piece 2.11 (backlog-r2): where each paragraph comes from (R-1, R-2).

const (
	paraVerified = "Los filtros finos retienen más partículas, como mide el estudio."
	paraNoCite   = "El agua embotellada suele traer más partículas."
	paraUnread   = "El agua de grifo trae 3 partículas por litro según otro estudio."
)

// seedProvenance is seedStepProject plus src-2, a source the principal
// never read, and a step that writes three material paragraphs after
// «Cierre.»: one cited from the read src-1, one without a citation and
// one cited from the unread src-2.
func seedProvenance(t *testing.T) *store.DB {
	t.Helper()
	db := seedStepProject(t)
	err := db.Write(context.Background(), "p1", func(tx *store.Tx) error {
		_, err := tx.CreateObject(store.ObjectSource, "", map[string]any{
			"kind": "file", "filename": "otro.md", "text": "El agua de grifo trae 3 partículas por litro.",
		}, store.EventInput{Type: store.EventStateChanged, Actor: "human"})
		return err
	})
	if err != nil {
		t.Fatalf("seed src-2: %v", err)
	}
	step := json.RawMessage(`{"stretch": "build", "triage_line":"Construir.","closing_risk":"none","note":"Tres párrafos.",
		"new_text":[{"section_id":"sec-1","base_version":1,"anchor":"Cierre.","claim_origin":"material","operation":"insert_after",
		"text":"` + paraVerified + `\n\n` + paraNoCite + `\n\n` + paraUnread + `"}],
		"citations":[
		  {"claim_anchor":"` + paraVerified + `","source_id":"src-1","passage":"Los filtros finos retienen más partículas."},
		  {"claim_anchor":"` + paraUnread + `","source_id":"src-2","passage":"El agua de grifo trae 3 partículas por litro."}]}`)
	if applied := applyRecorded(t, db, step); !applied.Applied {
		t.Fatalf("step not applied: %+v", applied)
	}
	return db
}

func setEvidenceRequired(t *testing.T, db *store.DB, value bool) {
	t.Helper()
	err := db.Write(context.Background(), "p1", func(tx *store.Tx) error {
		req, err := tx.GetObject(context.Background(), "p1", "req-1")
		if err != nil {
			return err
		}
		var b map[string]any
		_ = json.Unmarshal(req.Body, &b)
		b["evidence_required"] = map[string]any{"value": value, "why": "Resume estudios.", "state": "proposed"}
		_, err = tx.UpdateObject(req.ID, req.CurrentVersion, req.State, b, store.EventInput{Type: store.EventStateChanged, Actor: "agent"})
		return err
	})
	if err != nil {
		t.Fatalf("set evidence_required: %v", err)
	}
}

func provenances(t *testing.T, db *store.DB) map[string]graph.Provenance {
	t.Helper()
	list, err := graph.Provenances(context.Background(), db, "p1", "sec-1")
	if err != nil {
		t.Fatalf("Provenances: %v", err)
	}
	out := map[string]graph.Provenance{}
	for _, p := range list {
		out[p.Passage] = p
	}
	return out
}

// humanEdit replaces old with new in sec-1's text, as your save does.
func humanEdit(t *testing.T, db *store.DB, old, new string) {
	t.Helper()
	err := db.Write(context.Background(), "p1", func(tx *store.Tx) error {
		sec, err := tx.GetObject(context.Background(), "p1", "sec-1")
		if err != nil {
			return err
		}
		var b map[string]any
		_ = json.Unmarshal(sec.Body, &b)
		text, _ := b["text"].(string)
		if !strings.Contains(text, old) {
			t.Fatalf("text has no %q: %q", old, text)
		}
		b["text"] = strings.Replace(text, old, new, 1)
		_, err = tx.UpdateObject(sec.ID, sec.CurrentVersion, sec.State, b, store.EventInput{Type: store.EventEdit, Actor: "human"})
		return err
	})
	if err != nil {
		t.Fatalf("edit: %v", err)
	}
}

func claimAcceptedEvents(t *testing.T, db *store.DB) []store.Event {
	t.Helper()
	events, err := db.EventsSince(context.Background(), "p1", 0)
	if err != nil {
		t.Fatalf("events: %v", err)
	}
	var out []store.Event
	for _, ev := range events {
		if ev.Type == store.EventClaimAccepted {
			out = append(out, ev)
		}
	}
	return out
}

// R-1: a material paragraph with a citation whose passage is literal in
// a read source is verified, with that source and passage; without a
// citation, or with the source unread, it shows «sin verificar», and the
// paragraph you wrote has no declared origin.
func TestR1_MaterialParagraphIsVerifiedOnlyWithACitedPassageInAReadSource(t *testing.T) {
	db := seedProvenance(t)
	got := provenances(t, db)
	if len(got) != 4 {
		t.Fatalf("provenances: %d paragraphs, want 4: %+v", len(got), got)
	}
	v := got[paraVerified]
	if !v.Verified || v.Declared != graph.OriginMaterial || v.Block != 2 || len(v.Sources) != 1 ||
		v.Sources[0].Source != "estudio.md" || v.Sources[0].Passage != "Los filtros finos retienen más partículas." {
		t.Fatalf("cited from a read source: %+v, want verified from estudio.md with its passage", v)
	}
	for _, para := range []string{paraNoCite, paraUnread} {
		if p := got[para]; p.Verified || p.Declared != graph.OriginMaterial || len(p.Sources) != 0 {
			t.Fatalf("%q: %+v, want declared material, not verified, no source", para, p)
		}
	}
	if p := got["La síntesis compara estudios. Los estudios usan filtros. Cierre."]; p.Verified || p.Declared != "" || p.Block != 1 {
		t.Fatalf("your paragraph: %+v, want block 1 unverified with no declared origin", p)
	}
	// Without evidence_required, the mark shows and blocks nothing.
	if _, pendings := status(t, db, "sec-1"); hasPending(pendings, graph.PendingUnsourcedClaim, "") {
		t.Fatalf("pendings: %v, want no unsourced_claim without evidence_required", pendings)
	}
}

// R-1: model_knowledge and external without a citation are never verified,
// even when the text is literal in a read source.
func TestR1_ModelKnowledgeAndExternalParagraphsAreUnverified(t *testing.T) {
	db := seedStepProject(t)
	step := json.RawMessage(`{"stretch": "build", "triage_line":"Construir.","closing_risk":"none","note":"Dos párrafos.",
		"new_text":[{"section_id":"sec-1","base_version":1,"anchor":"Cierre.","claim_origin":"model_knowledge","operation":"insert_after",
		"text":"Se sabe que el plástico se degrada lento."},
		{"section_id":"sec-1","base_version":1,"anchor":"La síntesis compara estudios.","claim_origin":"external","operation":"replace",
		"text":"Un estudio externo compara conteos."}]}`)
	if applied := applyRecorded(t, db, step); !applied.Applied {
		t.Fatalf("step not applied: %+v", applied)
	}
	got := provenances(t, db)
	if p := got["Se sabe que el plástico se degrada lento."]; p.Verified || p.Declared != graph.OriginModelKnowledge {
		t.Fatalf("model_knowledge paragraph: %+v", p)
	}
	if p := got["Un estudio externo compara conteos. Los estudios usan filtros. Cierre."]; p.Verified || p.Declared != graph.OriginExternal {
		t.Fatalf("external paragraph: %+v (all: %+v)", p, got)
	}
}

// R-1, B-51: with evidence_required, each paragraph without a verified
// origin leaves unsourced_claim with its passage, and the section cannot
// be signed; the verified paragraph leaves none.
func TestR1_WithEvidenceRequiredAnUncitedParagraphBlocksSignoff(t *testing.T) {
	db := seedProvenance(t)
	setEvidenceRequired(t, db, true)
	st, pendings := status(t, db, "sec-1")
	if st != graph.StatusDraft {
		t.Fatalf("status %s, want draft", st)
	}
	for _, para := range []string{paraNoCite, paraUnread, "La síntesis compara estudios. Los estudios usan filtros. Cierre."} {
		if !hasPending(pendings, graph.PendingUnsourcedClaim, para) {
			t.Fatalf("pendings: %v, want unsourced_claim for %q", pendings, para)
		}
	}
	if hasPending(pendings, graph.PendingUnsourcedClaim, paraVerified) {
		t.Fatalf("pendings: %v, the verified paragraph must not be pending", pendings)
	}
	open, err := cycle.NewGate(db).RequestSignoff(context.Background(), "p1", "sec-1")
	if err != nil {
		t.Fatalf("RequestSignoff: %v", err)
	}
	if open.Opened || !hasPending(open.Pendings, graph.PendingUnsourcedClaim, paraNoCite) {
		t.Fatalf("signoff: %+v, want not opened with the unsourced claim", open)
	}
	// evidence_required false: only the mark.
	setEvidenceRequired(t, db, false)
	if _, pendings := status(t, db, "sec-1"); hasPending(pendings, graph.PendingUnsourcedClaim, "") {
		t.Fatalf("pendings: %v, want none with evidence_required false", pendings)
	}
}

// R-1: a later step that brings the citation backing the paragraph
// closes its unsourced_claim and verifies it.
func TestR1_ALaterCitationClosesTheUnsourcedClaim(t *testing.T) {
	db := seedProvenance(t)
	setEvidenceRequired(t, db, true)
	step := json.RawMessage(`{"stretch": "build", "triage_line":"Construir.","closing_risk":"none","note":"Cito.",
		"citations":[{"claim_anchor":"` + paraNoCite + `","source_id":"src-1","passage":"Los filtros finos retienen más partículas."}]}`)
	if applied := applyRecorded(t, db, step); !applied.Applied {
		t.Fatalf("step not applied: %+v", applied)
	}
	if _, pendings := status(t, db, "sec-1"); hasPending(pendings, graph.PendingUnsourcedClaim, paraNoCite) {
		t.Fatalf("pendings: %v, want the cited paragraph closed", pendings)
	}
	if p := provenances(t, db)[paraNoCite]; !p.Verified {
		t.Fatalf("provenance: %+v, want verified", p)
	}
}

// R-1, T-1: accepting with «ok» does not close; with a valid «porque»
// it closes, leaves claim_accepted with the literal passage and your
// «porque», and the pending does not come back while the passage stays
// the same; changing the paragraph brings it back.
func TestR1_AcceptingNeedsAValidReasonAndHoldsWhileThePassageStays(t *testing.T) {
	db := seedProvenance(t)
	setEvidenceRequired(t, db, true)
	g := cycle.NewGate(db)
	ctx := context.Background()

	if err := g.AcceptClaim(ctx, "p1", "sec-1", paraNoCite, "ok"); !errors.Is(err, cycle.ErrInvalidReason) {
		t.Fatalf("accept with ok: %v, want ErrInvalidReason", err)
	}
	if evs := claimAcceptedEvents(t, db); len(evs) != 0 {
		t.Fatalf("claim_accepted after ok: %d", len(evs))
	}
	if _, pendings := status(t, db, "sec-1"); !hasPending(pendings, graph.PendingUnsourcedClaim, paraNoCite) {
		t.Fatalf("pendings: %v, want it still open after ok", pendings)
	}
	if err := g.AcceptClaim(ctx, "p1", "sec-1", "No está en la sección.", "Es conocimiento general."); !errors.Is(err, cycle.ErrPassageNotInSection) {
		t.Fatalf("accept a passage not in the section: %v, want ErrPassageNotInSection", err)
	}

	const reason = "Es conocimiento general del área."
	if err := g.AcceptClaim(ctx, "p1", "sec-1", paraNoCite, reason); err != nil {
		t.Fatalf("accept: %v", err)
	}
	evs := claimAcceptedEvents(t, db)
	if len(evs) != 1 || evs[0].Actor != "human" || evs[0].Reason != reason {
		t.Fatalf("claim_accepted: %+v, want one by human with the reason", evs)
	}
	var data map[string]any
	_ = json.Unmarshal(evs[0].Data, &data)
	sec, _ := object(t, db, "sec-1")
	if data["passage"] != paraNoCite || data["label"] != "sec-1" || evs[0].ObjectID != sec.ID {
		t.Fatalf("claim_accepted data: %v on %s, want the literal passage on sec-1", data, evs[0].ObjectID)
	}
	if _, pendings := status(t, db, "sec-1"); hasPending(pendings, graph.PendingUnsourcedClaim, paraNoCite) {
		t.Fatalf("pendings: %v, want it closed by the acceptance", pendings)
	}
	if p := provenances(t, db)[paraNoCite]; p.Verified || p.Accepted != reason {
		t.Fatalf("provenance: %+v, want unverified and accepted with the reason", p)
	}

	// Another paragraph changes: the acceptance holds.
	humanEdit(t, db, "Cierre.", "Final.")
	if _, pendings := status(t, db, "sec-1"); hasPending(pendings, graph.PendingUnsourcedClaim, paraNoCite) {
		t.Fatalf("pendings after another edit: %v, want it still closed", pendings)
	}
	// The accepted passage changes: the pending comes back.
	changed := strings.Replace(paraNoCite, "suele traer", "trae", 1)
	humanEdit(t, db, paraNoCite, changed)
	if _, pendings := status(t, db, "sec-1"); !hasPending(pendings, graph.PendingUnsourcedClaim, changed) {
		t.Fatalf("pendings after changing the paragraph: %v, want it back", pendings)
	}
}

// R-1: removing the paragraph closes its pending.
func TestR1_RemovingTheParagraphClosesThePending(t *testing.T) {
	db := seedProvenance(t)
	setEvidenceRequired(t, db, true)
	humanEdit(t, db, "\n\n"+paraUnread, "")
	_, pendings := status(t, db, "sec-1")
	if hasPending(pendings, graph.PendingUnsourcedClaim, paraUnread) {
		t.Fatalf("pendings: %v, want the removed paragraph closed", pendings)
	}
	if !hasPending(pendings, graph.PendingUnsourcedClaim, paraNoCite) {
		t.Fatalf("pendings: %v, want the other paragraph still pending", pendings)
	}
}

// R-2: the paragraph's history brings its source and passage, computed
// from the graph; a paragraph without one says it is unverified.
func TestR2_ParagraphHistoryBringsItsSourceAndPassage(t *testing.T) {
	db := seedProvenance(t)
	p, err := graph.BlockProvenance(context.Background(), db, "p1", "sec-1", 2)
	if err != nil {
		t.Fatalf("BlockProvenance: %v", err)
	}
	if !p.Verified || len(p.Sources) != 1 || p.Sources[0].Source != "estudio.md" ||
		p.Sources[0].Passage != "Los filtros finos retienen más partículas." {
		t.Fatalf("block 2: %+v, want estudio.md and its passage", p)
	}
	p, err = graph.BlockProvenance(context.Background(), db, "p1", "sec-1", 3)
	if err != nil || p.Verified || p.Declared != graph.OriginMaterial || p.Passage != paraNoCite {
		t.Fatalf("block 3: %+v, %v, want unverified material", p, err)
	}
	if _, err := graph.BlockProvenance(context.Background(), db, "p1", "sec-1", 9); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("block 9: %v, want ErrNotFound", err)
	}
}

// R-2: the frame shows, per section, how many paragraphs are unverified.
func TestR2_FrameCountsTheUnverifiedParagraphs(t *testing.T) {
	db := seedProvenance(t)
	f, err := graph.ComputeFrame(context.Background(), db, "p1", graph.FrameLayers{})
	if err != nil {
		t.Fatalf("ComputeFrame: %v", err)
	}
	if len(f.Sections) != 1 || f.Sections[0].Unverified != 3 {
		t.Fatalf("frame sections: %+v, want sec-1 with 3 unverified paragraphs", f.Sections)
	}
}
