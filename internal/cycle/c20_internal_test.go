package cycle

import (
	"encoding/json"
	"testing"

	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// A-1: «Actualizado» rested on the edits a group's undo took back, all of them.
func TestC20_TheUndoOfAGroupLeavesNoEditStanding(t *testing.T) {
	events := []store.Event{
		{Seq: 1, ObjectID: "s", Type: store.EventEdit, Actor: "human", NewVersion: 2},
		{Seq: 2, ObjectID: "s", Type: store.EventEdit, Actor: "human", NewVersion: 3},
		{Seq: 3, ObjectID: "s", Type: store.EventEditUndone, Actor: "human", NewVersion: 4,
			Data: json.RawMessage(`{"undone_version":3,"undone_versions":[2,3],"restored_version":1}`)},
	}
	if editStandsSince(events, "s", 0) {
		t.Fatal("both saves were undone")
	}
}
