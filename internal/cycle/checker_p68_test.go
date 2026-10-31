package cycle_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/LaurenteEber/tupaqnin/internal/cycle"
	"github.com/LaurenteEber/tupaqnin/internal/graph"
	"github.com/LaurenteEber/tupaqnin/internal/provider"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// failingChecker answers every checker call with a provider error.
type failingChecker struct{ inner provider.Provider }

func (f *failingChecker) Respond(ctx context.Context, req provider.Request, onDelta func(provider.Delta)) (provider.Response, error) {
	if req.TurnKind == provider.TurnChecker {
		return provider.Response{}, errors.New("provider: 503 upstream body that must never be kept")
	}
	return f.inner.Respond(ctx, req, onDelta)
}

// B-50 (P-68.1): a provider error counts as a failed attempt, like a
// shape failure: two of them leave «sin comprobar», which blocks the
// signoff; the provider's error body is never kept; the next valid
// judgment clears it.
func TestB50_ProviderErrorCountsTowardUnchecked(t *testing.T) {
	db := seedCheckerProject(t)
	wrap := func(p provider.Provider) provider.Provider { return &failingChecker{inner: p} }
	cap := runCheckedTurn(t, db, wrap, stepWriting(t, "Fin del texto.", ""))
	if len(cap.checker) != 2 {
		t.Fatalf("checker calls: %d, want the call and its retry", len(cap.checker))
	}
	var results []string
	for _, ev := range eventsOfType(t, db, store.EventOutputRetry) {
		data := eventData(t, ev)
		if data["turn_kind"] != "checker" {
			continue
		}
		if strings.Contains(string(ev.Data), "503") || strings.Join(anyStrings(data["errors"]), ",") != "provider_error" {
			t.Fatalf("output_retry data: %s, want only provider_error", ev.Data)
		}
		results = append(results, data["result"].(string))
	}
	if strings.Join(results, " ") != "retry unchecked" {
		t.Fatalf("checker output_retry: %v", results)
	}
	if _, pendings := status(t, db, "sec-1"); !hasPending(pendings, graph.PendingUnchecked, "") {
		t.Fatalf("sec-1 pendings: %v, want unchecked", pendings)
	}
	open, err := cycle.NewGate(db).RequestSignoff(context.Background(), "p1", "sec-1")
	if err != nil || open.Opened {
		t.Fatalf("RequestSignoff: %+v, %v; want blocked by unchecked", open, err)
	}
}

// P-68.2, flujos §1: when the same step proposed an open question on the
// protected decision, the checker does not open another; without that
// proposal, the same judgment opens one marked «no lo declaró el
// agente».
func TestP68_ProposedOpenQuestionOnTheProtectedDecisionOpensNoSecond(t *testing.T) {
	passage := "Usamos solo datos de tesis, no el informe del comité."
	proposed := `"proposed_open_questions":[{"id":"new-1","question":"¿Informe o tesis?","options":[` +
		`{"option":"Informe","contrast":"c","consequence":"q","risk":"r"},{"option":"Tesis","contrast":"c","consequence":"q","risk":"r"}],` +
		`"depends_on":["dec-1"]}],`

	db := seedCheckerProject(t)
	runCheckedTurn(t, db, nil, stepWriting(t, passage, proposed), checkerFixture(t, "contradicts-decision"))
	if objs, err := db.ListObjects(context.Background(), "p1", store.ObjectOpenQuestion); err != nil || len(objs) != 0 {
		t.Fatalf("open questions: %d, %v; want none from the checker", len(objs), err)
	}
	cands := probeEvents(t, db, graph.ProbeKindCandidate)
	if len(cands) != 1 || cands[0]["declared"] != true || cands[0]["open_question"] != nil {
		t.Fatalf("candidate events: %v, want declared and nothing opened", cands)
	}

	undeclared := seedCheckerProject(t)
	runCheckedTurn(t, undeclared, nil, stepWriting(t, passage, ""), checkerFixture(t, "contradicts-decision"))
	oq, body := object(t, undeclared, "oq-1")
	dec, _ := object(t, undeclared, "dec-1")
	if oq.State != "open" || body["undeclared_by_agent"] != true || body["protected_id"] != dec.ID {
		t.Fatalf("oq-1: %s %v, want open, undeclared by the agent, on dec-1", oq.State, body)
	}
}
