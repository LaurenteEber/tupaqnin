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
