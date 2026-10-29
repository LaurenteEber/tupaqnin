package provider_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/LaurenteEber/tupaqnin/internal/provider"
)

func messageFixture(name, text string, delay time.Duration) provider.Fixture {
	return provider.Fixture{
		Name:   name,
		Deltas: []string{text},
		Items: []json.RawMessage{json.RawMessage(
			`{"type":"message","role":"assistant","content":[{"type":"output_text","text":"` + text + `"}]}`)},
		Usage:      provider.Usage{InputTokens: 8, OutputTokens: 2},
		DeltaDelay: delay,
	}
}

func TestT11_FakeEmitsDeltasAtItsDeltaDelayRhythm(t *testing.T) {
	fx := messageFixture("slow", "abc", 25*time.Millisecond)
	fx.Deltas = []string{"a", "b", "c"}
	f := provider.NewFake(fx)

	var got []string
	start := time.Now()
	resp, err := f.Respond(context.Background(), provider.Request{Model: provider.PrincipalModel},
		func(d provider.Delta) { got = append(got, d.Text) })
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("Respond: %v", err)
	}
	if strings.Join(got, "") != "abc" || resp.Text != "abc" {
		t.Fatalf("deltas/text: got %v / %q, want abc", got, resp.Text)
	}
	if elapsed < 50*time.Millisecond {
		t.Fatalf("deltas did not wait DeltaDelay: elapsed %v for 3 deltas at 25ms", elapsed)
	}
}

func TestT11_FakeReplaysFixturesInOrderAndCycles(t *testing.T) {
	f := provider.NewFake(messageFixture("one", "first", 0), messageFixture("two", "second", 0))
	ctx := context.Background()

	for i, want := range []string{"first", "second", "first"} {
		resp, err := f.Respond(ctx, provider.Request{Model: provider.PrincipalModel}, nil)
		if err != nil {
			t.Fatalf("Respond %d: %v", i, err)
		}
		if resp.Text != want {
			t.Fatalf("Respond %d: got %q, want %q", i, resp.Text, want)
		}
	}
}

func TestB5_FakeCountTokensIsOnePerFourCharacters(t *testing.T) {
	f := provider.NewFake()
	n, err := f.CountTokens(context.Background(), provider.PrincipalModel,
		[]json.RawMessage{json.RawMessage("12345678"), json.RawMessage("12345678")})
	if err != nil {
		t.Fatalf("CountTokens: %v", err)
	}
	if n != 4 {
		t.Fatalf("CountTokens: got %d, want 4 (16 chars / 4)", n)
	}
}

func TestT11_FakeCallWritesAModelCallRow(t *testing.T) {
	db := openTempDB(t)
	f := provider.NewFake(messageFixture("one", "hello", 0))
	f.Recorder = db

	if _, err := f.Respond(context.Background(), provider.Request{
		ProjectID: "p1", TurnID: "turn-1", InstanceID: "inst-1",
		Role: provider.RolePrincipal, Model: provider.PrincipalModel, Effort: provider.PrincipalEffort,
		Input: []json.RawMessage{json.RawMessage(`{"role":"user","content":"hi"}`)},
	}, nil); err != nil {
		t.Fatalf("Respond: %v", err)
	}
	var count int
	if err := db.Reader.QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM model_call WHERE project_id = 'p1'`).Scan(&count); err != nil {
		t.Fatalf("count model_call: %v", err)
	}
	if count != 1 {
		t.Fatalf("model_call rows: got %d, want 1", count)
	}
}

func TestT11_AppFixturesLoadInOrderAndStream90sHas90Deltas(t *testing.T) {
	app, err := provider.AppFixtures()
	if err != nil {
		t.Fatalf("AppFixtures: %v", err)
	}
	if len(app) != 4 || app[0].Name != "01-principal-valid" || app[1].Name != "02-principal-invalid" ||
		app[2].Name != "03-understanding-valid" || app[3].Name != "04-principal-no-work" {
		t.Fatalf("AppFixtures: got %d fixtures, want the valid, invalid, understanding and no-work ones", len(app))
	}
	if text := providerResponseText(t, app[2]); !strings.Contains(text, `"understood"`) {
		t.Fatalf("understanding fixture is not an understanding output: %s", text)
	}
	valid := providerResponseText(t, app[0])
	if !strings.Contains(valid, `"note"`) || !strings.Contains(valid, `"triage_line"`) {
		t.Fatalf("valid fixture is not a principal output with note and triage: %s", valid)
	}
	invalid := providerResponseText(t, app[1])
	if strings.Contains(invalid, `"note"`) {
		t.Fatalf("invalid fixture must miss the required note: %s", invalid)
	}

	stream, err := provider.LoadFixture("testdata/stream-90s.json")
	if err != nil {
		t.Fatalf("LoadFixture stream-90s: %v", err)
	}
	if len(stream.Deltas) != 90 || stream.DeltaDelay != time.Second {
		t.Fatalf("stream-90s: got %d deltas at %v, want 90 at 1s",
			len(stream.Deltas), stream.DeltaDelay)
	}
}

func TestT11_MicrotestFixturesReplayWithTheFake(t *testing.T) {
	// Real recordings from micro-test 0.8 (run by Eber with the key).
	valid, err := provider.LoadFixture("testdata/microtest-valid.json")
	if err != nil {
		t.Fatalf("LoadFixture microtest-valid: %v", err)
	}
	resp, err := provider.NewFake(valid).Respond(context.Background(),
		provider.Request{Model: provider.PrincipalModel}, nil)
	if err != nil {
		t.Fatalf("Respond microtest-valid: %v", err)
	}
	if !strings.Contains(resp.Text, `"triage_line"`) || !strings.Contains(resp.Text, `"note"`) {
		t.Fatalf("valid recording does not replay as a principal output: %.120s", resp.Text)
	}

	invalid, err := provider.LoadFixture("testdata/microtest-invalid.json")
	if err != nil {
		t.Fatalf("LoadFixture microtest-invalid: %v", err)
	}
	resp, err = provider.NewFake(invalid).Respond(context.Background(),
		provider.Request{Model: provider.PrincipalModel}, nil)
	if err != nil {
		t.Fatalf("Respond microtest-invalid: %v", err)
	}
	if strings.Contains(resp.Text, `"triage_line"`) {
		t.Fatalf("invalid recording must not look like a principal output: %.120s", resp.Text)
	}
}

func providerResponseText(t *testing.T, fx provider.Fixture) string {
	t.Helper()
	f := provider.NewFake(fx)
	resp, err := f.Respond(context.Background(), provider.Request{Model: provider.PrincipalModel}, nil)
	if err != nil {
		t.Fatalf("Respond fixture %s: %v", fx.Name, err)
	}
	return resp.Text
}

func TestP29_FakePicksTheRecordingByTurnKind(t *testing.T) {
	step1 := messageFixture("step-1", "s1", 0)
	step1.TurnKind = provider.TurnStep
	understanding := messageFixture("understanding", "u", 0)
	understanding.TurnKind = provider.TurnUnderstanding
	step2 := messageFixture("step-2", "s2", 0)
	step2.TurnKind = provider.TurnStep
	f := provider.NewFake(step1, understanding, step2)

	respond := func(kind provider.TurnKind) string {
		t.Helper()
		resp, err := f.Respond(context.Background(), provider.Request{Model: provider.PrincipalModel, TurnKind: kind}, nil)
		if err != nil {
			t.Fatalf("Respond(%s): %v", kind, err)
		}
		return resp.Text
	}
	// The understanding comes first even though a step recording is
	// first in order; then a message turn gets a step recording, never
	// the understanding one; each kind cycles over its own recordings.
	for i, c := range []struct {
		kind provider.TurnKind
		want string
	}{
		{provider.TurnUnderstanding, "u"},
		{provider.TurnStep, "s1"},
		{provider.TurnStep, "s2"},
		{provider.TurnStep, "s1"},
		{provider.TurnUnderstanding, "u"},
	} {
		if got := respond(c.kind); got != c.want {
			t.Fatalf("call %d (%s): got %q, want %q", i, c.kind, got, c.want)
		}
	}
	if _, err := f.Respond(context.Background(), provider.Request{Model: provider.PrincipalModel, TurnKind: provider.TurnSignoff}, nil); err == nil {
		t.Fatal("signoff turn without a signoff recording: want an error, got a recording of another kind")
	}
}

func TestP29_AppFixturesDeclareTheirTurnKind(t *testing.T) {
	app, err := provider.AppFixtures()
	if err != nil {
		t.Fatalf("AppFixtures: %v", err)
	}
	want := map[string]provider.TurnKind{
		"01-principal-valid": provider.TurnStep, "02-principal-invalid": provider.TurnStep,
		"03-understanding-valid": provider.TurnUnderstanding, "04-principal-no-work": provider.TurnStep,
	}
	for _, fx := range app {
		if fx.TurnKind != want[fx.Name] {
			t.Fatalf("%s: turn_kind %q, want %q", fx.Name, fx.TurnKind, want[fx.Name])
		}
	}
}
