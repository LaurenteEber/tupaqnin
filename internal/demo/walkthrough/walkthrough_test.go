package walkthrough

import (
	"context"
	"encoding/json"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/LaurenteEber/tupaqnin/internal/memory"
	"github.com/LaurenteEber/tupaqnin/internal/provider"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

const recordedDir = "testdata/recorded"

func recordedFixtures(t *testing.T) []provider.Fixture {
	t.Helper()
	names, err := filepath.Glob(filepath.Join(recordedDir, "*.json"))
	if err != nil || len(names) == 0 {
		t.Fatalf("recorded answers: %v (%d files)", err, len(names))
	}
	sort.Strings(names)
	out := make([]provider.Fixture, 0, len(names))
	for _, name := range names {
		fx, err := provider.LoadFixture(name)
		if err != nil {
			t.Fatalf("load %s: %v", name, err)
		}
		out = append(out, fx)
	}
	return out
}

// H-1 (backlog-r1 1.8): the case 5 walkthrough passes end to end with
// the fake provider and the recorded answers, and the history holds the
// events of each step in order. Wonder's first round is checked as the
// run with Meta checks it (P-57).
func TestH1_Case5WalkthroughPassesEndToEnd(t *testing.T) {
	w := newWalk(t, func(db *store.DB) (provider.Provider, memory.TokenCounter) {
		fake := provider.NewFake(recordedFixtures(t)...)
		fake.Recorder = db
		return fake, func(ctx context.Context, input []json.RawMessage) (int, error) {
			return fake.CountTokens(ctx, provider.PrincipalModel, input)
		}
	}, nil)
	section, sign, historyDecision := w.toSignoff()
	// The recordings link 1,5 to the rule, so its history shows it.
	if !historyDecision {
		t.Fatalf("the history of the block with %q shows no decision", caseFigure)
	}
	w.wonderFirstRound(sign)
	w.signAfterRounds(section, sign)
	w.orderedEvents(stepEvents(section, historyDecision))
}

// P-114: a walkthrough with the fake leaves nothing behind, passing or failing; only
// a Meta run keeps its database, which goes under its own name, outside the test's.
func TestP114_OnlyAMetaWalkthroughKeepsItsDatabase(t *testing.T) {
	base := filepath.Dir(t.TempDir())
	t.Setenv("TQ_PROVIDER", "")
	if dir := walkDir(t); !strings.HasPrefix(dir, base) {
		t.Fatalf("with the fake, %s is outside the test's temp dir %s", dir, base)
	}
	t.Setenv("TQ_PROVIDER", "meta")
	dir := walkDir(t)
	if strings.HasPrefix(dir, base) || !strings.Contains(filepath.Base(dir), "tupaqnin-walk-") {
		t.Fatalf("with Meta, %s must survive the test's temp dir", dir)
	}
}
