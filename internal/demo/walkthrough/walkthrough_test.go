package walkthrough

import (
	"context"
	"encoding/json"
	"path/filepath"
	"sort"
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
// events of each step in order.
func TestH1_Case5WalkthroughPassesEndToEnd(t *testing.T) {
	w := newWalk(t, func(db *store.DB) (provider.Provider, memory.TokenCounter) {
		fake := provider.NewFake(recordedFixtures(t)...)
		fake.Recorder = db
		return fake, func(ctx context.Context, input []json.RawMessage) (int, error) {
			return fake.CountTokens(ctx, provider.PrincipalModel, input)
		}
	}, nil)
	signed := w.run()
	w.orderedEvents(stepEvents(signed))
}
