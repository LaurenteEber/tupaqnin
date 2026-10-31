package provider

import (
	"embed"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
)

func loadFixtureFile(path string) (Fixture, error) {
	b, err := os.ReadFile(path) //nolint:gosec // path comes from the caller, never from a model
	if err != nil {
		return Fixture{}, fmt.Errorf("load fixture: %w", err)
	}
	return parseFixture(b)
}

func parseFixture(b []byte) (Fixture, error) {
	var fx Fixture
	if err := json.Unmarshal(b, &fx); err != nil {
		return Fixture{}, fmt.Errorf("load fixture: %w", err)
	}
	return fx, nil
}

// App fixtures the app replays with TQ_PROVIDER=fake, in filename
// order (backlog 0.5). They are embedded so the single binary carries
// them, and the fake starts again at the first when they run out.
//
//go:embed testdata/app
var appFixturesFS embed.FS

//go:embed testdata/stream-90s.json
var stream90Fixture []byte

// Stream90Fixture returns the hand-made 90-second streaming fixture
// (about 90 deltas, one per second), used by the streaming test and
// the development stream route (backlog 0.5 and 0.6).
func Stream90Fixture() (Fixture, error) {
	return parseFixture(stream90Fixture)
}

// AppFixtures returns the embedded app fixtures in filename order.
func AppFixtures() ([]Fixture, error) {
	entries, err := appFixturesFS.ReadDir("testdata/app")
	if err != nil {
		return nil, fmt.Errorf("app fixtures: %w", err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	out := make([]Fixture, 0, len(names))
	for _, name := range names {
		b, err := appFixturesFS.ReadFile("testdata/app/" + name)
		if err != nil {
			return nil, fmt.Errorf("app fixture %s: %w", name, err)
		}
		fx, err := parseFixture(b)
		if err != nil {
			return nil, fmt.Errorf("app fixture %s: %w", name, err)
		}
		if fx.Name == "" {
			fx.Name = strings.TrimSuffix(name, ".json")
		}
		out = append(out, fx)
	}
	return out, nil
}

// The reader's model calls of the first valid run of the evidence
// micro-test (3.0, part 2), recorded without key or contact: what the
// fake reader replays (backlog-r3 3.2), call by call.
//
//go:embed testdata/reader
var readerFixturesFS embed.FS

// ReaderFixtures returns the recorded reader calls in filename order
// (reader-call-1 … reader-call-N), each of turn kind reader and for any
// part, so every reader of a fake round replays the same run.
func ReaderFixtures() ([]Fixture, error) {
	entries, err := readerFixturesFS.ReadDir("testdata/reader")
	if err != nil {
		return nil, fmt.Errorf("reader fixtures: %w", err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") {
			names = append(names, e.Name())
		}
	}
	// reader-call-10 sorts after reader-call-9.
	sort.Slice(names, func(i, j int) bool {
		if len(names[i]) != len(names[j]) {
			return len(names[i]) < len(names[j])
		}
		return names[i] < names[j]
	})
	out := make([]Fixture, 0, len(names))
	for _, name := range names {
		b, err := readerFixturesFS.ReadFile("testdata/reader/" + name)
		if err != nil {
			return nil, fmt.Errorf("reader fixture %s: %w", name, err)
		}
		fx, err := parseFixture(b)
		if err != nil {
			return nil, fmt.Errorf("reader fixture %s: %w", name, err)
		}
		fx.TurnKind, fx.Part = TurnReader, 0
		out = append(out, fx)
	}
	return out, nil
}
