package provider_test

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
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
	if len(app) != 7 || app[0].Name != "01-principal-valid" || app[1].Name != "01b-principal-critical-decision" ||
		app[2].Name != "02-principal-invalid" || app[3].Name != "03-understanding-valid" || app[4].Name != "04-principal-no-work" ||
		app[5].Name != "05-signoff-first-round" || app[6].Name != "06-signoff-second-round" {
		t.Fatalf("AppFixtures: got %d fixtures, want valid, critical decision, invalid, understanding, no-work and two signoff rounds", len(app))
	}
	// The demo's signoff (backlog-r1 1.7): Wonder outputs, first with
	// the origin question unanswered, then judging the answer.
	if text := providerResponseText(t, app[5]); !strings.Contains(text, `"origin_question"`) || !strings.Contains(text, `"answer": null`) {
		t.Fatalf("first signoff round is not a Wonder output awaiting the origin answer: %s", text)
	}
	if text := providerResponseText(t, app[6]); !strings.Contains(text, `"answer": "answers"`) {
		t.Fatalf("second signoff round must judge the origin answer: %s", text)
	}
	if text := providerResponseText(t, app[3]); !strings.Contains(text, `"understood"`) {
		t.Fatalf("understanding fixture is not an understanding output: %s", text)
	}
	valid := providerResponseText(t, app[0])
	if !strings.Contains(valid, `"note"`) || !strings.Contains(valid, `"triage_line"`) {
		t.Fatalf("valid fixture is not a principal output with note and triage: %s", valid)
	}
	invalid := providerResponseText(t, app[2])
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
		"01b-principal-critical-decision": provider.TurnStep, "05-signoff-first-round": provider.TurnSignoff,
		"06-signoff-second-round": provider.TurnSignoff,
	}
	for _, fx := range app {
		if fx.TurnKind != want[fx.Name] {
			t.Fatalf("%s: turn_kind %q, want %q", fx.Name, fx.TurnKind, want[fx.Name])
		}
	}
}

// P-29, backlog-r2 2.3: a checker call gets only checker recordings,
// never a recording without a kind (those are the principal's); without
// one, the fake judges every probe «no» with no candidates.
func TestP29_CheckerCallGetsOnlyCheckerRecordings(t *testing.T) {
	kindless := messageFixture("principal", "p", 0)
	checker := messageFixture("checker", "c", 0)
	checker.TurnKind = provider.TurnChecker
	respond := func(f *provider.Fake, kind provider.TurnKind) string {
		t.Helper()
		resp, err := f.Respond(context.Background(), provider.Request{Model: provider.CheckerModel,
			TurnKind: kind, ProbeLabels: []string{"req-1", "prem-1"}}, nil)
		if err != nil {
			t.Fatalf("Respond(%s): %v", kind, err)
		}
		return resp.Text
	}
	f := provider.NewFake(kindless, checker)
	if got := respond(f, provider.TurnChecker); got != "c" {
		t.Fatalf("checker call: %q, want the checker recording", got)
	}
	if got := respond(f, provider.TurnStep); got != "p" {
		t.Fatalf("step call: %q, want the kindless recording, as before", got)
	}
	got := respond(provider.NewFake(kindless), provider.TurnChecker)
	want := `{"candidates":[],"probes":[{"passage":null,"protected":"req-1","verdict":"no"},{"passage":null,"protected":"prem-1","verdict":"no"}]}`
	if got != want {
		t.Fatalf("checker call without a checker recording: %s, want %s", got, want)
	}
}

// P-29, backlog-r2 2.6: a judge's defense call gets only judge
// recordings, never a principal's recording without a kind; without one,
// the fake asks three plain questions on the section it prepares, in
// the judge's fixed shape.
func TestP29_JudgeDefenseCallGetsOnlyJudgeRecordings(t *testing.T) {
	kindless := messageFixture("principal", "p", 0)
	judge := messageFixture("judge", "j", 0)
	judge.TurnKind = provider.TurnJudgeDefense
	respond := func(f *provider.Fake, kind provider.TurnKind) string {
		t.Helper()
		resp, err := f.Respond(context.Background(), provider.Request{Model: provider.JudgeDefenseModel,
			TurnKind: kind, DefenseSection: "sec-2"}, nil)
		if err != nil {
			t.Fatalf("Respond(%s): %v", kind, err)
		}
		return resp.Text
	}
	if got := respond(provider.NewFake(kindless, judge), provider.TurnJudgeDefense); got != "j" {
		t.Fatalf("judge call: %q, want the judge recording", got)
	}
	if got := respond(provider.NewFake(kindless, judge), provider.TurnStep); got != "p" {
		t.Fatalf("step call: %q, want the kindless recording, as before", got)
	}
	var out struct {
		AudienceQuestions []struct{ Question, Section string } `json:"audience_questions"`
		NotCovered        []any                                `json:"not_covered"`
	}
	if err := json.Unmarshal([]byte(respond(provider.NewFake(kindless), provider.TurnJudgeDefense)), &out); err != nil {
		t.Fatalf("judge call without a recording: %v", err)
	}
	if len(out.AudienceQuestions) != 3 || out.NotCovered == nil || len(out.NotCovered) != 0 {
		t.Fatalf("judge call without a recording: %+v", out)
	}
	for _, q := range out.AudienceQuestions {
		if q.Section != "sec-2" || q.Question == "" {
			t.Fatalf("question %+v, want one on sec-2", q)
		}
	}
}

// Backlog-r3 3.2: the fake picks a reader call's recording by turn kind
// and by part, each part in its own order, and is safe for the readers
// calling it in parallel.
func TestB65_FakePicksReaderRecordingsByPartInParallel(t *testing.T) {
	msg := func(text string) json.RawMessage {
		return json.RawMessage(`{"type":"message","role":"assistant","content":[{"type":"output_text","text":"` + text + `"}]}`)
	}
	fake := provider.NewFake(
		provider.Fixture{TurnKind: provider.TurnStep, Items: []json.RawMessage{msg("step")}},
		provider.Fixture{TurnKind: provider.TurnReader, Part: 1, Items: []json.RawMessage{msg("p1-a")}},
		provider.Fixture{TurnKind: provider.TurnReader, Part: 1, Items: []json.RawMessage{msg("p1-b")}},
		provider.Fixture{TurnKind: provider.TurnReader, Part: 2, Items: []json.RawMessage{msg("p2")}},
	)
	var wg sync.WaitGroup
	got := make([][]string, 3)
	for part := 1; part <= 2; part++ {
		wg.Add(1)
		go func(part int) {
			defer wg.Done()
			for i := 0; i < 2; i++ {
				resp, err := fake.Respond(context.Background(), provider.Request{TurnKind: provider.TurnReader, Part: part}, nil)
				if err != nil {
					t.Errorf("part %d: %v", part, err)
					return
				}
				got[part] = append(got[part], resp.Text)
			}
		}(part)
	}
	wg.Wait()
	if strings.Join(got[1], ",") != "p1-a,p1-b" || strings.Join(got[2], ",") != "p2,p2" {
		t.Fatalf("by part: %v", got)
	}
	if resp, _ := fake.Respond(context.Background(), provider.Request{TurnKind: provider.TurnStep}, nil); resp.Text != "step" {
		t.Fatalf("a step got %q", resp.Text)
	}
	readers, err := provider.ReaderFixtures()
	if err != nil || len(readers) != 5 || readers[0].Name != "reader-call-1" || readers[0].TurnKind != provider.TurnReader {
		t.Fatalf("ReaderFixtures: %d %v", len(readers), err)
	}
}

// Backlog-r3 3.6: a judge's verification only gets recordings of its own
// kind; without one the fake answers no flaws, or every flaw resolved.
func TestB67_FakeAnswersTheJudgeOnlyWithItsOwnRecordings(t *testing.T) {
	kindless := provider.Fixture{Items: []json.RawMessage{json.RawMessage(`{"type":"message","role":"assistant","content":[{"type":"output_text","text":"step"}]}`)}}
	fake := provider.NewFake(kindless)
	resp, err := fake.Respond(context.Background(), provider.Request{TurnKind: provider.TurnJudgeVerify}, nil)
	if err != nil || resp.Text != `{"flaws":[],"recheck":[]}` {
		t.Fatalf("first call: %q, %v", resp.Text, err)
	}
	resp, err = fake.Respond(context.Background(), provider.Request{TurnKind: provider.TurnJudgeVerify, JudgeRecheck: []int{2}}, nil)
	if err != nil || resp.Text != `{"flaws":[],"recheck":[{"flaw":2,"passage":null,"verdict":"resolved"}]}` {
		t.Fatalf("recheck: %q, %v", resp.Text, err)
	}
}
