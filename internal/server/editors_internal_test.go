package server

import (
	"testing"
	"time"
)

func editorsAt(start time.Time) (*editors, func(time.Duration)) {
	e := newEditors()
	now := start
	e.now = func() time.Time { return now }
	return e, func(d time.Duration) { now = now.Add(d) }
}

// B-102: a tab that leaves frees the lease at once; before, the other tab
// waited out the whole 45 s.
func TestB102_ReleaseLetsAnotherTabClaim(t *testing.T) {
	e, _ := editorsAt(time.Unix(1000, 0))
	if !e.claim("p", "a") {
		t.Fatal("first tab is not the editor")
	}
	if e.claim("p", "b") {
		t.Fatal("second tab took a live lease")
	}
	e.release("p", "a")
	if !e.claim("p", "b") {
		t.Fatal("second tab cannot claim after the release")
	}
}

// B-102: releasing is only for the holder; a stale tab's goodbye does not free
// the lease of the tab that took over.
func TestB102_ReleaseByAnotherTabKeepsTheLease(t *testing.T) {
	e, _ := editorsAt(time.Unix(1000, 0))
	e.claim("p", "a")
	e.release("p", "b")
	if e.claim("p", "b") {
		t.Fatal("a tab that does not hold the lease released it")
	}
	if !e.claim("p", "a") {
		t.Fatal("the holder lost its lease")
	}
}

// B-102: «Editar aquí» takes the edit from a tab that did not renew in 15 s,
// and not from one that did.
func TestB102_TakeOverOnlyFromAStaleTab(t *testing.T) {
	e, advance := editorsAt(time.Unix(1000, 0))
	e.claim("p", "a")
	advance(5 * time.Second)
	if e.takeOver("p", "b", editorStaleAfter) {
		t.Fatal("took the edit from a tab renewed 5 s ago")
	}
	if !e.claim("p", "a") {
		t.Fatal("the refused take-over cost the holder its lease")
	}
	advance(20 * time.Second)
	if !e.takeOver("p", "b", editorStaleAfter) {
		t.Fatal("did not take the edit from a tab renewed 20 s ago")
	}
	if e.claim("p", "a") {
		t.Fatal("the old tab still edits after the take-over")
	}
}

// B-102: taking over with nobody holding, or from yourself, just gives the lease.
func TestB102_TakeOverWithNoHolderOrFromYourself(t *testing.T) {
	e, _ := editorsAt(time.Unix(1000, 0))
	if !e.takeOver("p", "a", editorStaleAfter) {
		t.Fatal("no holder: take-over refused")
	}
	if !e.takeOver("p", "a", editorStaleAfter) {
		t.Fatal("from yourself: take-over refused")
	}
}

// C.24: mayEdit answers what claim would, in every state of the lease, and changes
// nothing.
func TestC24_MayEditAnswersAsClaimWithoutTakingTheLease(t *testing.T) {
	for _, tc := range []struct {
		name string
		held bool
		age  time.Duration
		tab  string
		want bool
	}{
		{name: "no holder", tab: "b", want: true},
		{name: "its own", held: true, tab: "a", want: true},
		{name: "its own, lapsed", held: true, age: editorLease, tab: "a", want: true},
		{name: "another's, live", held: true, age: editorLease - time.Nanosecond, tab: "b", want: false},
		{name: "another's, lapsed", held: true, age: editorLease, tab: "b", want: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e, advance := editorsAt(time.Unix(1000, 0))
			if tc.held {
				e.claim("p", "a")
			}
			advance(tc.age)
			before, ok := e.holders["p"]
			if got := e.mayEdit("p", tc.tab); got != tc.want {
				t.Fatalf("mayEdit: %v, want %v", got, tc.want)
			}
			if after, okAfter := e.holders["p"]; after != before || okAfter != ok {
				t.Fatalf("the lease changed: %+v → %+v", before, after)
			}
			if got := e.claim("p", tc.tab); got != tc.want {
				t.Fatalf("claim: %v, want %v", got, tc.want)
			}
		})
	}
}
