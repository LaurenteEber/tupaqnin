package store_test

import (
	"context"
	"errors"
	"testing"

	"github.com/LaurenteEber/tupaqnin/internal/store"
)

func seedProject(t *testing.T, db *store.DB, id string) {
	t.Helper()
	if _, err := db.Writer.ExecContext(context.Background(),
		`INSERT INTO project (id, created_at) VALUES (?, '2026-10-28T00:00:00.000Z')`, id); err != nil {
		t.Fatalf("seed project %s: %v", id, err)
	}
}

func humanEvent() store.EventInput {
	return store.EventInput{Type: store.EventStateChanged, Actor: "human"}
}

func createDecision(t *testing.T, db *store.DB, project string) store.Object {
	t.Helper()
	var obj store.Object
	err := db.Write(context.Background(), project, func(tx *store.Tx) error {
		var err error
		obj, err = tx.CreateObject(store.ObjectDecision, "agent",
			map[string]any{"statement": "use X"}, humanEvent())
		return err
	})
	if err != nil {
		t.Fatalf("CreateObject: %v", err)
	}
	return obj
}

func TestT2_LabelsAreSequentialImmutableAndNeverReused(t *testing.T) {
	db := openTemp(t)
	seedProject(t, db, "p1")

	first := createDecision(t, db, "p1")
	second := createDecision(t, db, "p1")
	// A replacement is a new object, so it gets the next label (T-2).
	third := createDecision(t, db, "p1")

	if first.Label != "dec-1" || second.Label != "dec-2" || third.Label != "dec-3" {
		t.Fatalf("labels: got %q, %q, %q, want dec-1, dec-2, dec-3",
			first.Label, second.Label, third.Label)
	}
	if first.ID == second.ID || first.ID == "" {
		t.Fatalf("internal ids must be distinct and non-empty: %q, %q", first.ID, second.ID)
	}
	got, err := db.GetObject(context.Background(), "p1", "dec-1")
	if err != nil {
		t.Fatalf("GetObject by label: %v", err)
	}
	if got.ID != first.ID || got.CurrentVersion != 1 {
		t.Fatalf("GetObject by label: got %+v, want id %q version 1", got, first.ID)
	}
}

func TestS2_OldVersionReturnsConflictAndWritesNothing(t *testing.T) {
	db := openTemp(t)
	ctx := context.Background()
	seedProject(t, db, "p1")
	obj := createDecision(t, db, "p1")

	err := db.Write(ctx, "p1", func(tx *store.Tx) error {
		_, err := tx.UpdateObject(obj.ID, 99, "human",
			map[string]any{"statement": "changed"}, humanEvent())
		return err
	})
	if !errors.Is(err, store.ErrVersionConflict) {
		t.Fatalf("UpdateObject old version: got %v, want ErrVersionConflict", err)
	}
	got, err := db.GetObject(ctx, "p1", obj.ID)
	if err != nil {
		t.Fatalf("GetObject: %v", err)
	}
	if got.CurrentVersion != 1 || got.State != "agent" {
		t.Fatalf("conflict wrote something: got version %d state %q, want 1/agent",
			got.CurrentVersion, got.State)
	}
	events, err := db.EventsSince(ctx, "p1", 0)
	if err != nil {
		t.Fatalf("EventsSince: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("events after conflict: got %d, want 1 (only the creation)", len(events))
	}
}

func TestS2_ObjectAndEventAreInTheSameTransaction(t *testing.T) {
	db := openTemp(t)
	ctx := context.Background()
	seedProject(t, db, "p1")

	err := db.Write(ctx, "p1", func(tx *store.Tx) error {
		_, err := tx.CreateObject(store.ObjectDecision, "agent",
			map[string]any{"statement": "use X"},
			store.EventInput{Type: store.EventType("bogus")})
		return err
	})
	if err == nil {
		t.Fatal("CreateObject with an invalid event: got no error, want failure")
	}
	if _, err := db.GetObject(ctx, "p1", "dec-1"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("failed event left the object behind: GetObject err = %v", err)
	}
}

func TestT11_StateTheTypeDoesNotAdmitIsRejected(t *testing.T) {
	db := openTemp(t)
	seedProject(t, db, "p1")

	cases := []struct {
		name  string
		typ   store.ObjectType
		state string
	}{
		{"section ready is computed, never stored", store.ObjectSection, "ready"},
		{"decision with a section state", store.ObjectDecision, "draft"},
		{"stateless type with a state", store.ObjectRequest, "current"},
		{"unknown type", store.ObjectType("bogus"), ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := db.Write(context.Background(), "p1", func(tx *store.Tx) error {
				_, err := tx.CreateObject(tc.typ, tc.state, map[string]any{}, humanEvent())
				return err
			})
			if err == nil {
				t.Fatalf("CreateObject(%q, %q): got no error, want rejection", tc.typ, tc.state)
			}
		})
	}
}

func TestB3_NoChangeReturnsErrNoChangeAndWritesNothing(t *testing.T) {
	db := openTemp(t)
	ctx := context.Background()
	seedProject(t, db, "p1")

	var obj store.Object
	err := db.Write(ctx, "p1", func(tx *store.Tx) error {
		var err error
		obj, err = tx.CreateObject(store.ObjectSection, "draft",
			map[string]any{"text": "caf\u00e9\r\nsecond"}, humanEvent()) // precomposed é
		return err
	})
	if err != nil {
		t.Fatalf("CreateObject: %v", err)
	}

	// Same text, NFC-normalized and with LF endings: no change (B-3).
	err = db.Write(ctx, "p1", func(tx *store.Tx) error {
		_, err := tx.UpdateObject(obj.ID, 1, "draft",
			map[string]any{"text": "café\nsecond"}, humanEvent())
		return err
	})
	if !errors.Is(err, store.ErrNoChange) {
		t.Fatalf("UpdateObject unchanged: got %v, want ErrNoChange", err)
	}
	got, err := db.GetObject(ctx, "p1", obj.ID)
	if err != nil {
		t.Fatalf("GetObject: %v", err)
	}
	if got.CurrentVersion != 1 {
		t.Fatalf("no-change wrote a version: got %d, want 1", got.CurrentVersion)
	}
}

func TestT7_ReadingWithAnotherProjectFindsNothing(t *testing.T) {
	db := openTemp(t)
	ctx := context.Background()
	seedProject(t, db, "p1")
	seedProject(t, db, "p2")
	obj := createDecision(t, db, "p1")

	if _, err := db.GetObject(ctx, "p2", obj.ID); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("GetObject other project by id: got %v, want ErrNotFound", err)
	}
	if _, err := db.GetObject(ctx, "p2", obj.Label); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("GetObject other project by label: got %v, want ErrNotFound", err)
	}
	list, err := db.ListObjects(ctx, "p2", store.ObjectDecision)
	if err != nil {
		t.Fatalf("ListObjects other project: %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("ListObjects other project: got %d, want 0", len(list))
	}
}

func TestT7_SeqGrowsWithoutGapsPerProject(t *testing.T) {
	db := openTemp(t)
	ctx := context.Background()
	seedProject(t, db, "p1")
	seedProject(t, db, "p2")

	createDecision(t, db, "p1")
	createDecision(t, db, "p2")
	createDecision(t, db, "p1")

	for project, want := range map[string]int{"p1": 2, "p2": 1} {
		events, err := db.EventsSince(ctx, project, 0)
		if err != nil {
			t.Fatalf("EventsSince %s: %v", project, err)
		}
		if len(events) != want {
			t.Fatalf("%s: got %d events, want %d", project, len(events), want)
		}
		for i, e := range events {
			if e.Seq != i+1 {
				t.Fatalf("%s: seq at %d is %d, want %d (no gaps)", project, i, e.Seq, i+1)
			}
		}
	}
	since, err := db.EventsSince(ctx, "p1", 1)
	if err != nil {
		t.Fatalf("EventsSince after 1: %v", err)
	}
	if len(since) != 1 || since[0].Seq != 2 {
		t.Fatalf("EventsSince after 1: got %+v, want only seq 2", since)
	}
}

func TestT2_EdgesAreCreatedAndSupersededWithTheirEvents(t *testing.T) {
	db := openTemp(t)
	ctx := context.Background()
	seedProject(t, db, "p1")
	dec := createDecision(t, db, "p1")
	var sec store.Object
	if err := db.Write(ctx, "p1", func(tx *store.Tx) error {
		var err error
		sec, err = tx.CreateObject(store.ObjectSection, "draft",
			map[string]any{"text": "body"}, humanEvent())
		return err
	}); err != nil {
		t.Fatalf("CreateObject section: %v", err)
	}

	var edge store.Edge
	if err := db.Write(ctx, "p1", func(tx *store.Tx) error {
		var err error
		edge, err = tx.AddEdge(store.EdgeUsedIn, dec.ID, sec.ID, "",
			store.EventInput{Type: store.EventEdge, Actor: "agent", Data: map[string]any{"kind": "created"}})
		return err
	}); err != nil {
		t.Fatalf("AddEdge: %v", err)
	}
	from, err := db.EdgesFrom(ctx, "p1", dec.ID)
	if err != nil || len(from) != 1 || from[0].ID != edge.ID {
		t.Fatalf("EdgesFrom: got %+v, %v; want the new edge", from, err)
	}

	var next store.Edge
	if err := db.Write(ctx, "p1", func(tx *store.Tx) error {
		var err error
		next, err = tx.SupersedeEdge(edge.ID,
			store.EventInput{Type: store.EventEdge, Actor: "agent", Data: map[string]any{"kind": "superseded"}})
		return err
	}); err != nil {
		t.Fatalf("SupersedeEdge: %v", err)
	}
	if next.Version != 2 || !next.Current || next.ID == edge.ID {
		t.Fatalf("SupersedeEdge: got %+v, want a new current edge at version 2", next)
	}
	to, err := db.EdgesTo(ctx, "p1", sec.ID)
	if err != nil || len(to) != 1 || to[0].ID != next.ID {
		t.Fatalf("EdgesTo after supersede: got %+v, %v; want only the new version", to, err)
	}
	edges, err := db.EdgesFrom(ctx, "p2", dec.ID)
	if err != nil {
		t.Fatalf("EdgesFrom other project: %v", err)
	}
	if len(edges) != 0 {
		t.Fatalf("EdgesFrom other project: got %d, want 0", len(edges))
	}
}
