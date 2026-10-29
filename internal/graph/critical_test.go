package graph_test

import (
	"context"
	"testing"

	"github.com/LaurenteEber/tupaqnin/internal/graph"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

func openGraphDB(t *testing.T) *store.DB {
	t.Helper()
	db, err := store.Open(context.Background(), t.TempDir())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := db.Writer.ExecContext(context.Background(),
		`INSERT INTO project (id, created_at) VALUES ('p1', '2026-10-29T00:00:00.000Z')`); err != nil {
		t.Fatalf("seed project: %v", err)
	}
	return db
}

// criticalFixture writes, in one transaction: sec-1 (role other),
// sec-2 (role role2), dec-1 and the edges the case asks for.
func criticalFixture(t *testing.T, role2 string, build func(tx *store.Tx, sec1, sec2, dec store.Object) error) *store.DB {
	t.Helper()
	db := openGraphDB(t)
	ev := store.EventInput{Type: store.EventStateChanged, Actor: "agent"}
	err := db.Write(context.Background(), "p1", func(tx *store.Tx) error {
		sec1, err := tx.CreateObject(store.ObjectSection, "draft", map[string]any{
			"text": "La comparación da 1,5 veces.", "role": map[string]any{"value": "other", "state": "proposed"}}, ev)
		if err != nil {
			return err
		}
		sec2, err := tx.CreateObject(store.ObjectSection, "draft", map[string]any{
			"text": "Otro texto.", "role": map[string]any{"value": role2, "state": "proposed"}}, ev)
		if err != nil {
			return err
		}
		dec, err := tx.CreateObject(store.ObjectDecision, "agent", map[string]any{"statement": "Comparar por veces.", "origin": "agent"}, ev)
		if err != nil {
			return err
		}
		return build(tx, sec1, sec2, dec)
	})
	if err != nil {
		t.Fatalf("fixture: %v", err)
	}
	return db
}

func requireCritical(t *testing.T, db *store.DB, want bool, reason graph.Reason) {
	t.Helper()
	got, reasons, err := graph.Critical(context.Background(), db, "p1", "dec-1")
	if err != nil {
		t.Fatalf("Critical: %v", err)
	}
	has := false
	for _, r := range reasons {
		has = has || r == reason
	}
	if got != want || (want && !has) {
		t.Fatalf("Critical: %v %v, want %v with %s", got, reasons, want, reason)
	}
}

func TestT5_DecisionCoveringAFigureIsCritical(t *testing.T) {
	db := criticalFixture(t, "other", func(tx *store.Tx, sec1, _, dec store.Object) error {
		fig, err := tx.CreateObject(store.ObjectFigure, "", map[string]any{"section_id": sec1.ID, "anchor": "1,5 veces", "value": "1,5"},
			store.EventInput{Type: store.EventStateChanged, Actor: "agent"})
		if err != nil {
			return err
		}
		_, err = tx.AddEdge(store.EdgeCovers, fig.ID, dec.ID, "1,5 veces", store.EventInput{Type: store.EventEdge, Actor: "agent"})
		return err
	})
	requireCritical(t, db, true, graph.ReasonCoversFigure)
}

func TestT5_DecisionInAConclusionSectionIsCritical(t *testing.T) {
	db := criticalFixture(t, "conclusion", func(tx *store.Tx, _, sec2, dec store.Object) error {
		_, err := tx.AddEdge(store.EdgeUsedIn, dec.ID, sec2.ID, "", store.EventInput{Type: store.EventEdge, Actor: "agent"})
		return err
	})
	requireCritical(t, db, true, graph.ReasonConclusionSection)
}

func TestT5_DecisionInTwoSectionsIsCritical(t *testing.T) {
	db := criticalFixture(t, "other", func(tx *store.Tx, sec1, sec2, dec store.Object) error {
		for _, sec := range []store.Object{sec1, sec2} {
			if _, err := tx.AddEdge(store.EdgeUsedIn, dec.ID, sec.ID, "", store.EventInput{Type: store.EventEdge, Actor: "agent"}); err != nil {
				return err
			}
		}
		return nil
	})
	requireCritical(t, db, true, graph.ReasonTwoOrMoreSections)
}

// The second section counts by text search: the figure linked to the
// decision appears in a section the decision does not declare.
func TestT5_SectionWhereALinkedFigureAppearsCountsTowardTwoSections(t *testing.T) {
	db := criticalFixture(t, "other", func(tx *store.Tx, sec1, sec2, dec store.Object) error {
		if _, err := tx.AddEdge(store.EdgeUsedIn, dec.ID, sec2.ID, "", store.EventInput{Type: store.EventEdge, Actor: "agent"}); err != nil {
			return err
		}
		fig, err := tx.CreateObject(store.ObjectFigure, "", map[string]any{"section_id": sec1.ID, "anchor": "1,5 veces", "value": "1,5"},
			store.EventInput{Type: store.EventStateChanged, Actor: "agent"})
		if err != nil {
			return err
		}
		_, err = tx.AddEdge(store.EdgeCovers, fig.ID, dec.ID, "1,5 veces", store.EventInput{Type: store.EventEdge, Actor: "agent"})
		return err
	})
	requireCritical(t, db, true, graph.ReasonTwoOrMoreSections)
}

func TestT5_DecisionInOneOtherSectionIsNotCritical(t *testing.T) {
	db := criticalFixture(t, "other", func(tx *store.Tx, sec1, _, dec store.Object) error {
		_, err := tx.AddEdge(store.EdgeUsedIn, dec.ID, sec1.ID, "", store.EventInput{Type: store.EventEdge, Actor: "agent"})
		return err
	})
	requireCritical(t, db, false, "")
}

// Inside a transaction, Critical sees that transaction's own writes.
func TestT5_CriticalInsideATransactionSeesItsOwnWrites(t *testing.T) {
	db := criticalFixture(t, "conclusion", func(*store.Tx, store.Object, store.Object, store.Object) error { return nil })
	err := db.Write(context.Background(), "p1", func(tx *store.Tx) error {
		dec, _ := tx.GetObject(context.Background(), "p1", "dec-1")
		sec2, _ := tx.GetObject(context.Background(), "p1", "sec-2")
		if _, err := tx.AddEdge(store.EdgeUsedIn, dec.ID, sec2.ID, "", store.EventInput{Type: store.EventEdge, Actor: "agent"}); err != nil {
			return err
		}
		got, _, err := graph.Critical(context.Background(), tx, "p1", "dec-1")
		if err == nil && !got {
			t.Error("Critical inside the transaction did not see its new edge")
		}
		return err
	})
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
}
