package cycle_test

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/LaurenteEber/tupaqnin/internal/cycle"
	"github.com/LaurenteEber/tupaqnin/internal/graph"
	"github.com/LaurenteEber/tupaqnin/internal/provider"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// newLinkFor creates a link and puts the project on it.
func newLinkFor(t *testing.T, db *store.DB, projectID, label string, budget float64) string {
	t.Helper()
	ctx := context.Background()
	_, hash, err := store.NewSecret()
	if err != nil {
		t.Fatal(err)
	}
	id, err := db.CreateInvitation(ctx, store.InvitationInput{Label: label, BudgetUSD: budget, MaxAccounts: 3, MaxAnonProjects: 5}, hash, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Writer.ExecContext(ctx, `UPDATE project SET invitation_id = ? WHERE id = ?`, id, projectID); err != nil {
		t.Fatal(err)
	}
	return id
}

// addProject adds another project, on a link of its own.
func addProject(t *testing.T, db *store.DB, projectID, label string, budget float64) string {
	t.Helper()
	if _, err := db.Writer.ExecContext(context.Background(),
		`INSERT INTO project (id, created_at) VALUES (?, '2026-10-28T00:00:00.000Z')`, projectID); err != nil {
		t.Fatal(err)
	}
	return newLinkFor(t, db, projectID, label, budget)
}

// spend records what was spent, with a role of no call of the test, so
// it does not raise any role's dearest call.
func spend(t *testing.T, db *store.DB, projectID string, usd float64) {
	t.Helper()
	if err := db.RecordModelCall(context.Background(), store.ModelCall{ProjectID: projectID, Role: "seed", CostUSD: usd}); err != nil {
		t.Fatal(err)
	}
}

func oneReport(t *testing.T, db *store.DB) cycle.StopReport {
	t.Helper()
	reports := stopReports(t, db)
	if len(reports) != 1 {
		t.Fatalf("stop reports: %d, want 1", len(reports))
	}
	return reports[0]
}

// wantCapStop asserts a turn refused by the gate: no step call, no error
// on the stream, the step's stop_reason and data.cap, and the report with
// the app's part and no call to ask for it.
func wantCapStop(t *testing.T, db *store.DB, c *captureProvider, pub *fakePublisher, reason, capName string) cycle.StopReport {
	t.Helper()
	if len(c.sent) != 0 || len(c.reports) != 0 {
		t.Fatalf("calls: %d steps, %d reports; want none", len(c.sent), len(c.reports))
	}
	if len(pub.errs) != 0 {
		t.Fatalf("stream errors: %v, want none", pub.errs)
	}
	step := pub.step(t)
	if step["stop_reason"] != reason {
		t.Fatalf("step stop_reason: %v, want %s", step["stop_reason"], reason)
	}
	if got, _ := step["cap"].(string); got != capName {
		t.Fatalf("step cap: %q, want %q", got, capName)
	}
	report := oneReport(t, db)
	if string(report.StopReason) != reason || report.Cap != capName || !report.PrincipalFailed {
		t.Fatalf("report: %+v", report)
	}
	return report
}

// B3-20, B-89: with 2.50 USD spent, the principal's reserve does not fit
// in the cap less the signoff's balance: the turn ends with project_cap,
// no call goes out, and the report keeps the app's part.
func TestB3_20_PrincipalStopsAtProjectCap(t *testing.T) {
	db, runner, c, pub := newCaptureRunner(t)
	if err := db.Write(context.Background(), "p1", func(tx *store.Tx) error {
		_, err := tx.CreateObject(store.ObjectOpenQuestion, "open", map[string]any{"question": "¿Qué cuenca?"},
			store.EventInput{Type: store.EventStateChanged, Actor: "human"})
		return err
	}); err != nil {
		t.Fatal(err)
	}
	spend(t, db, "p1", 2.50)
	if err := runner.RunTurn(context.Background(), "p1", "turn-1", "Sigue."); err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	report := wantCapStop(t, db, c, pub, "project_cap", "")
	if len(report.Open) != 1 || report.Open[0].Label != "oq-1" {
		t.Fatalf("the app's part of the report: %+v", report.Open)
	}
	// What you wrote stays: the message is in the history.
	if evs := eventsOfType(t, db, store.EventHumanMessage); len(evs) != 1 {
		t.Fatalf("human messages: %d", len(evs))
	}
	spent, _ := db.ProjectSpend(context.Background(), "p1")
	if spent != 2.50 {
		t.Fatalf("spent %.2f: the stop spends nothing", spent)
	}
}

// Previous behavior: just under the limit, the principal's reserve fits
// and the turn runs as ever.
func TestB3_20_PrincipalRunsWhenItsReserveFits(t *testing.T) {
	db, runner, c, pub := newCaptureRunner(t, loadAppFixture(t, "04-principal-no-work.json"))
	spend(t, db, "p1", 3-provider.SignoffReserveUSD-provider.PrincipalReserveUSD)
	if err := runner.RunTurn(context.Background(), "p1", "turn-1", "Sigue."); err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	if len(c.sent) != 1 {
		t.Fatalf("step calls: %d, want 1", len(c.sent))
	}
	if step := pub.step(t); step["stop_reason"] != "no_work" {
		t.Fatalf("step: %v", step)
	}
}

// B3-20: with the project at the cap less the balance, the principal does
// not start and the signoff still calls Wonder.
func TestB3_20_SignoffReserve(t *testing.T) {
	db := openTemp(t)
	seedSignable(t, db, true)
	fake := provider.NewFake(signoffFixture(t, "round1-all-match"))
	fake.Recorder = db
	c := &captureProvider{inner: fake}
	pub := &fakePublisher{}
	r := cycle.NewRunner(db, c, runnerCount, pub)
	t.Cleanup(r.Close)
	g := cycle.NewGate(db)
	sign := openSignoff(t, g)
	if err := g.RecordExplanation(context.Background(), "p1", sign, "Comparo solo estudios con el mismo tamaño."); err != nil {
		t.Fatal(err)
	}
	spend(t, db, "p1", 3-provider.SignoffReserveUSD)
	if err := r.RunTurn(context.Background(), "p1", "turn-1", "Sigue."); err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	if len(c.sent) != 0 || pub.step(t)["stop_reason"] != "project_cap" {
		t.Fatalf("principal calls %d, step %v", len(c.sent), pub.step(t))
	}
	if err := r.RunSignoffRound(context.Background(), "p1", "turn-w", sign); err != nil {
		t.Fatalf("RunSignoffRound: %v", err)
	}
	if v := view(t, db, sign); v.Rounds != 1 {
		t.Fatalf("Wonder's rounds: %d, want 1", v.Rounds)
	}
}

// B-89 (Eber, 2-nov), B3-20: the link's cap does not keep a signoff from
// finishing: Wonder's round runs over an exhausted link.
func TestB89_TheSignoffFinishesAtTheLinkCap(t *testing.T) {
	db := openTemp(t)
	seedSignable(t, db, true)
	fake := provider.NewFake(signoffFixture(t, "round1-all-match"))
	fake.Recorder = db
	r := cycle.NewRunner(db, &captureProvider{inner: fake}, runnerCount, &fakePublisher{})
	t.Cleanup(r.Close)
	g := cycle.NewGate(db)
	sign := openSignoff(t, g)
	if err := g.RecordExplanation(context.Background(), "p1", sign, "Comparo solo estudios con el mismo tamaño."); err != nil {
		t.Fatal(err)
	}
	newLinkFor(t, db, "p1", "guests", 0.01)
	if err := r.RunSignoffRound(context.Background(), "p1", "turn-w", sign); err != nil {
		t.Fatalf("RunSignoffRound: %v", err)
	}
	if v := view(t, db, sign); v.Rounds != 1 {
		t.Fatalf("Wonder's rounds: %d, want 1", v.Rounds)
	}
}

// B3-20: past the whole cap the signoff waits too: no call, no provider
// error, the stream says project_cap.
func TestB3_20_ContrasterStopsAtTheWholeCap(t *testing.T) {
	db := openTemp(t)
	seedSignable(t, db, true)
	c := &captureProvider{inner: provider.NewFake()}
	pub := &fakePublisher{}
	r := cycle.NewRunner(db, c, nil, pub)
	t.Cleanup(r.Close)
	g := cycle.NewGate(db)
	sign := openSignoff(t, g)
	if err := g.RecordExplanation(context.Background(), "p1", sign, "Comparo solo estudios con el mismo tamaño."); err != nil {
		t.Fatal(err)
	}
	spend(t, db, "p1", 2.99)
	if err := r.RunSignoffRound(context.Background(), "p1", "turn-w", sign); err != nil {
		t.Fatalf("RunSignoffRound: %v", err)
	}
	if len(pub.errs) != 0 || pub.step(t)["stop_reason"] != "project_cap" {
		t.Fatalf("errors %v, step %v", pub.errs, pub.step(t))
	}
	if v := view(t, db, sign); v.Rounds != 0 {
		t.Fatalf("Wonder's rounds: %d, want 0", v.Rounds)
	}
}

// B-89 (3): what the links that are not the jury's spent today reaches
// the day's cap and stops them; the jury's link goes on.
func TestB89_DayCapStopsAndTheJuryGoesOn(t *testing.T) {
	db, runner, c, pub := newCaptureRunner(t)
	newLinkFor(t, db, "p1", "guests", 10)
	addProject(t, db, "p2", "guests", 10)
	addProject(t, db, "p3", store.LabelJury, 30)
	runner.SpendCapDayUSD = 1
	spend(t, db, "p2", 1)
	if err := runner.RunTurn(context.Background(), "p1", "turn-1", "Sigue."); err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	wantCapStop(t, db, c, pub, "demo_cap", "day")

	fake := provider.NewFake(loadAppFixture(t, "04-principal-no-work.json"))
	fake.Recorder = db
	jury := &captureProvider{inner: fake}
	juryRunner := cycle.NewRunner(db, jury, runnerCount, &fakePublisher{})
	t.Cleanup(juryRunner.Close)
	juryRunner.SpendCapDayUSD = 1
	spend(t, db, "p3", 2) // over the day's cap: the jury's spend is not the day's, nor does the cap stop it
	if err := juryRunner.RunTurn(context.Background(), "p3", "turn-2", "Sigue."); err != nil {
		t.Fatalf("RunTurn jury: %v", err)
	}
	if len(jury.sent) != 1 {
		t.Fatalf("the jury's calls: %d, want 1", len(jury.sent))
	}
}

// B-89: «today» is the UTC day by created_at: yesterday's spend does not
// count against today's cap.
func TestB89_DayCapCountsTheUTCDay(t *testing.T) {
	db, runner, c, _ := newCaptureRunner(t, loadAppFixture(t, "04-principal-no-work.json"))
	newLinkFor(t, db, "p1", "guests", 10)
	addProject(t, db, "p2", "guests", 10)
	runner.SpendCapDayUSD = 1
	spend(t, db, "p2", 1)
	runner.Now = func() time.Time { return time.Now().UTC().Add(24 * time.Hour) }
	if err := runner.RunTurn(context.Background(), "p1", "turn-1", "Sigue."); err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	if len(c.sent) != 1 {
		t.Fatalf("calls: %d, want 1 (yesterday's spend does not count)", len(c.sent))
	}
}

// B-89 (4): the sum of every link's spend stops every link, the jury's too.
func TestB89_TotalCapStops(t *testing.T) {
	db, runner, c, pub := newCaptureRunner(t)
	newLinkFor(t, db, "p1", store.LabelJury, 30)
	addProject(t, db, "p2", "guests", 10)
	runner.SpendCapTotalUSD = 1
	spend(t, db, "p2", 1)
	if err := runner.RunTurn(context.Background(), "p1", "turn-1", "Sigue."); err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	wantCapStop(t, db, c, pub, "demo_cap", "total")
}

// B-89 (2): the link's balance stops the project's call, whatever the
// project itself has left.
func TestB3_22_2_LinkBudgetStopsTheCall(t *testing.T) {
	db, runner, c, pub := newCaptureRunner(t)
	newLinkFor(t, db, "p1", "guests", 0.10)
	if err := runner.RunTurn(context.Background(), "p1", "turn-1", "Sigue."); err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	wantCapStop(t, db, c, pub, "demo_cap", "link")
}

// A project without a link (an older local database) has the project's
// cap only, and does not fail: not even with the day and total exhausted.
func TestB89_ProjectWithoutLinkHasOnlyTheProjectCap(t *testing.T) {
	db, runner, c, _ := newCaptureRunner(t, loadAppFixture(t, "04-principal-no-work.json"))
	addProject(t, db, "p2", "guests", 10)
	spend(t, db, "p2", 2)
	runner.SpendCapDayUSD = 1
	runner.SpendCapTotalUSD = 1
	if err := runner.RunTurn(context.Background(), "p1", "turn-1", "Sigue."); err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	if len(c.sent) != 1 {
		t.Fatalf("calls: %d, want 1", len(c.sent))
	}
}

// B-89: a role whose dearest call in the project cost more than its
// constant reserves that instead (as B-65).
func TestB89_TheReserveIsTheRolesDearestCall(t *testing.T) {
	db, runner, c, pub := newCaptureRunner(t)
	if err := db.RecordModelCall(context.Background(), store.ModelCall{ProjectID: "p1", Role: "principal", CostUSD: 1.40}); err != nil {
		t.Fatal(err)
	}
	if err := runner.RunTurn(context.Background(), "p1", "turn-1", "Sigue."); err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	wantCapStop(t, db, c, pub, "project_cap", "")
}

// B-89: the stop is a stop of the understanding too: no call, no provider
// error.
func TestB89_UnderstandingStopsAtTheCap(t *testing.T) {
	db, runner, c, pub := newCaptureRunner(t)
	if err := db.Write(context.Background(), "p1", func(tx *store.Tx) error {
		_, err := tx.CreateObject(store.ObjectRequest, "", map[string]any{"text": "Escribe la tesis."},
			store.EventInput{Type: store.EventStateChanged, Actor: "human"})
		return err
	}); err != nil {
		t.Fatal(err)
	}
	newLinkFor(t, db, "p1", "guests", 0.01)
	if err := runner.RunUnderstandingTurn(context.Background(), "p1", "turn-1"); err != nil {
		t.Fatalf("RunUnderstandingTurn: %v", err)
	}
	wantCapStop(t, db, c, pub, "demo_cap", "link")
}

// B-89: a stop report whose own call does not fit keeps the app's part,
// with no retry events, and says the cap.
func TestB89_StopReportWithoutRoomKeepsTheAppsPart(t *testing.T) {
	step := textFixture(t, pendingFigureStep)
	step.Usage.CostUSD = 0.30
	db, runner, c, pub := newCaptureRunner(t, step, loadAppFixture(t, "04-principal-no-work.json"))
	newLinkFor(t, db, "p1", "guests", 0.46)
	if err := runner.RunTurn(context.Background(), "p1", "turn-1", "Sigue."); err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	if len(c.sent) != 1 || len(c.reports) != 0 {
		t.Fatalf("calls: %d steps, %d reports; want 1 and 0", len(c.sent), len(c.reports))
	}
	report := oneReport(t, db)
	if report.StopReason != "demo_cap" || report.Cap != "link" || !report.PrincipalFailed {
		t.Fatalf("report: %+v", report)
	}
	for _, ev := range eventsOfType(t, db, store.EventOutputRetry) {
		var data map[string]any
		_ = json.Unmarshal(ev.Data, &data)
		if data["turn_kind"] == "stop_report" {
			t.Fatalf("a refused report is no failed output: %v", data)
		}
	}
	if len(pub.errs) != 0 {
		t.Fatalf("stream errors: %v", pub.errs)
	}
}

// B3-20 with B-36: at the balance the 60 s batch does not go out and your
// edit stays to check; asking to sign may use the balance (the cap never
// keeps you from signing), so the checker runs then.
func TestB3_20_YourEditWaitsAndTheSignoffChecksItWithTheBalance(t *testing.T) {
	db := seedCheckerProject(t)
	yourEdit(t, db, "sec-1", contradictingEdit)
	runner, c := checkerRunner(t, db, checkerFixture(t, "contradicts-premise"))
	t.Cleanup(runner.Close)
	spend(t, db, "p1", 2.60)

	done := make(chan struct{})
	runner.AfterFunc = func(_ time.Duration, f func()) func() bool {
		go func() { f(); close(done) }()
		return func() bool { return true }
	}
	runner.EditSaved("p1")
	<-done
	if len(c.checker) != 0 {
		t.Fatalf("checker calls in the batch: %d, want 0", len(c.checker))
	}
	for _, ev := range eventsOfType(t, db, store.EventOutputRetry) {
		t.Fatalf("no unchecked may be left by a refused batch: %s", ev.Data)
	}

	open, err := runner.RequestSignoff(context.Background(), "p1", "sec-1")
	if err != nil {
		t.Fatalf("RequestSignoff: %v", err)
	}
	if len(c.checker) != 1 {
		t.Fatalf("checker calls asking to sign: %d, want 1", len(c.checker))
	}
	if open.Opened || !hasPending(open.Pendings, graph.PendingOpenQuestion, "oq-1") {
		t.Fatalf("signoff: %+v, want what the checker found", open)
	}
}

// B3-20 (2-nov) with B-89: with the link out of balance, asking to sign with
// edits unchecked opens the signoff: no call, and the section stays «sin
// comprobar» on record once, however many times you ask.
func TestB89_AskingToSignWithEditsAtTheLinkCapOpensUnchecked(t *testing.T) {
	db := seedCheckerProject(t)
	yourEdit(t, db, "sec-1", contradictingEdit)
	runner, c := checkerRunner(t, db)
	t.Cleanup(runner.Close)
	newLinkFor(t, db, "p1", "guests", 0.01)
	ctx := context.Background()
	open, err := runner.RequestSignoff(ctx, "p1", "sec-1")
	if err != nil || !open.Opened || len(open.Pendings) != 0 {
		t.Fatalf("RequestSignoff: %+v, %v", open, err)
	}
	if len(c.checker) != 0 {
		t.Fatalf("checker calls: %d", len(c.checker))
	}
	if _, pendings := status(t, db, "sec-1"); !hasPending(pendings, graph.PendingUnchecked, "") {
		t.Fatalf("pendings: %v, want unchecked", pendings)
	}
	before := len(eventsOfType(t, db, store.EventOutputRetry))
	if _, err := runner.RequestSignoff(ctx, "p1", "sec-1"); err != nil {
		t.Fatalf("again: %v", err)
	}
	if after := len(eventsOfType(t, db, store.EventOutputRetry)); after != before {
		t.Fatalf("output_retry: %d → %d, want one record", before, after)
	}
}

// B-89: the judge cuts with demo_cap when the link does not reach: no call,
// the round closes, and the section shows as not verified.
func TestB3_22_2_JudgeCutsWithDemoCap(t *testing.T) {
	db, runner, c, _ := newCaptureRunner(t)
	seedJudged(t, db)
	newLinkFor(t, db, "p1", "guests", 0.01)
	label, err := runner.RunJudgeVerify(context.Background(), "p1", "sec-1", cycle.JudgeReasonRisk, nil)
	if err != nil {
		t.Fatalf("RunJudgeVerify: %v", err)
	}
	if len(c.verifies) != 0 {
		t.Fatalf("judge calls: %d", len(c.verifies))
	}
	round, _ := db.GetObject(context.Background(), "p1", label)
	if b := judgeBody(t, round); round.State != "closed" || b.JudgeStatus != cycle.JudgeDemoCap {
		t.Fatalf("round %s: %+v", round.State, b)
	}
	notVerified, err := cycle.JudgeNotVerified(context.Background(), db, "p1")
	if err != nil || len(notVerified) != 1 || notVerified[0] != "sec-1" {
		t.Fatalf("not verified: %v, %v", notVerified, err)
	}
}

// B-89: a round cuts with demo_cap in the same field, before it launches.
func TestB3_22_2_RoundCutsWithDemoCap(t *testing.T) {
	step := discoverStep("too_much_to_read", "["+openAlexPart+"]")
	first := textFixture(t, step)
	first.Usage.CostUSD = 0.12
	db, runner, g := newRoundRunner(t, false, first)
	newLinkFor(t, db, "p1", "guests", 0.18)
	runDiscover(t, runner, step)
	round, body := onlyRound(t, db)
	if round.State != "closed" || body.Stopped != cycle.EmptyDemoCap || len(g.readerCalls()) != 0 ||
		body.Readers[0].Reason != cycle.EmptyDemoCap {
		t.Fatalf("round %s %+v, reader calls %d", round.State, body, len(g.readerCalls()))
	}
}

// B-89: a digest that does not fit in the link cuts with demo_cap.
func TestB3_22_2_DigestCutsWithDemoCap(t *testing.T) {
	reader := readerOutput(t, projectFinding("fact", passageOne, "src-1"))
	step := discoverStep("too_much_to_read", "["+projectPart("src-1")+"]")
	fixtures := append(principalSteps(t, step), readerText(0, reader, 0.27))
	db, runner, g := newRoundRunner(t, false, fixtures...)
	runner.Evidence = &handEvidence{}
	newLinkFor(t, db, "p1", "guests", 0.30)
	runDiscover(t, runner, step)
	round, body := onlyRound(t, db)
	if n := len(g.callsOf(provider.TurnDigest)); n != 0 || body.DigestStatus != cycle.DigestDemoCap || round.State != "closed" {
		t.Fatalf("digest calls %d, status %q, %s", n, body.DigestStatus, round.State)
	}
}

func errorsAs(err error, target **provider.CapError) bool { return errors.As(err, target) }

// refusedCheck runs a step whose checker does not fit in the balance: 2.45
// USD seeded and a step costing 0.12 leave 2.57 spent.
func refusedCheck(t *testing.T) (*store.DB, *cycle.Runner, *captureProvider) {
	t.Helper()
	db := seedCheckerProject(t)
	step := stepWriting(t, "La síntesis compara dos estudios.", "")
	step.Usage.CostUSD = 0.12
	runner, c := checkerRunner(t, db, step, checkerFixture(t, "no-findings"))
	t.Cleanup(runner.Close)
	spend(t, db, "p1", 3-provider.SignoffReserveUSD-provider.PrincipalReserveUSD)
	if err := runner.RunTurn(context.Background(), "p1", "turn-1", "Escribe."); err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	if len(c.checker) != 0 {
		t.Fatalf("checker calls: %d, want 0", len(c.checker))
	}
	return db, runner, c
}

// B3-20: a checker the gate refused did not fail: its record says the cap
// and no failed attempts, and the section is «sin comprobar» meanwhile.
func TestB3_20_ARefusedCheckerIsNoFailedChecker(t *testing.T) {
	db, _, _ := refusedCheck(t)
	retries := eventsOfType(t, db, store.EventOutputRetry)
	if len(retries) != 1 {
		t.Fatalf("output_retry: %d, want 1", len(retries))
	}
	var data map[string]any
	if err := json.Unmarshal(retries[0].Data, &data); err != nil {
		t.Fatal(err)
	}
	if data["refused"] != "project_cap" || data["result"] != "unchecked" || data["turn_kind"] != "checker" {
		t.Fatalf("record: %v", data)
	}
	if errs, _ := data["errors"].([]any); len(errs) != 0 {
		t.Fatalf("errors: %v, want none", errs)
	}
	if _, pendings := status(t, db, "sec-1"); !hasPending(pendings, graph.PendingUnchecked, "") {
		t.Fatalf("pendings: %v, want unchecked", pendings)
	}
}

// B3-20: asking to sign sends the section a refused checker left unchecked,
// with the balance, once: the cap does not keep you from signing it.
func TestB3_20_AskingToSignChecksWhatARefusedCheckerLeft(t *testing.T) {
	db, runner, c := refusedCheck(t)
	open, err := runner.RequestSignoff(context.Background(), "p1", "sec-1")
	if err != nil {
		t.Fatalf("RequestSignoff: %v", err)
	}
	if len(c.checker) != 1 || hasPending(open.Pendings, graph.PendingUnchecked, "") {
		t.Fatalf("checker calls %d, signoff %+v; want checked, no unchecked", len(c.checker), open)
	}
	if !open.Opened {
		t.Fatalf("signoff: %+v, want opened", open)
	}
	if _, pendings := status(t, db, "sec-1"); hasPending(pendings, graph.PendingUnchecked, "") {
		t.Fatalf("pendings: %v", pendings)
	}
	if _, err := runner.RequestSignoff(context.Background(), "p1", "sec-1"); err != nil {
		t.Fatalf("RequestSignoff again: %v", err)
	}
	if len(c.checker) != 1 {
		t.Fatalf("checker calls: %d, want 1 (nothing left to check)", len(c.checker))
	}
}

// B3-20 (2-nov): when that batch does not fit even with the balance, the
// signoff opens anyway: no call, no failure of the checker on record, and
// the section stays «sin comprobar», listed apart.
func TestB3_20_AskingToSignPastTheWholeCapOpensWithoutTheChecker(t *testing.T) {
	db, runner, c := refusedCheck(t)
	spend(t, db, "p1", provider.SignoffReserveUSD+0.02) // 2.99
	before := len(eventsOfType(t, db, store.EventOutputRetry))
	open, err := runner.RequestSignoff(context.Background(), "p1", "sec-1")
	if err != nil || !open.Opened || len(open.Pendings) != 0 {
		t.Fatalf("RequestSignoff: %+v, %v", open, err)
	}
	if after := len(eventsOfType(t, db, store.EventOutputRetry)); len(c.checker) != 0 || after != before {
		t.Fatalf("checker calls %d, output_retry %d → %d", len(c.checker), before, after)
	}
	if _, pendings := status(t, db, "sec-1"); !hasPending(pendings, graph.PendingUnchecked, "") {
		t.Fatalf("pendings: %v, want unchecked still open", pendings)
	}
}

// B3-20 (2-nov): a request the gate rejects writes nothing, and a section
// with no edit of its own gets no «sin comprobar»: the record is written only
// for a signoff that opens on the section whose edits were not checked.
func TestB3_20_ARejectedRequestAtTheCapWritesNothing(t *testing.T) {
	db := seedCheckerProject(t)
	yourEdit(t, db, "sec-1", contradictingEdit)
	seedSignable(t, db, false) // sec-2, with no edit of yours
	runner, c := checkerRunner(t, db)
	t.Cleanup(runner.Close)
	newLinkFor(t, db, "p1", "guests", 0.01)
	ctx := context.Background()
	before := len(eventsOfType(t, db, store.EventOutputRetry))
	if _, err := runner.RequestSignoff(ctx, "p1", "sec-99"); !errors.Is(err, cycle.ErrSectionNotFound) {
		t.Fatalf("unknown section: %v", err)
	}
	if after := len(eventsOfType(t, db, store.EventOutputRetry)); after != before {
		t.Fatalf("a rejected request wrote: output_retry %d → %d", before, after)
	}
	open, err := runner.RequestSignoff(ctx, "p1", "sec-2")
	if err != nil || !open.Opened {
		t.Fatalf("sec-2: %+v, %v", open, err)
	}
	if _, pendings := status(t, db, "sec-2"); hasPending(pendings, graph.PendingUnchecked, "") {
		t.Fatalf("sec-2 pendings: %v, it has no edit of its own", pendings)
	}
	if len(c.checker) != 0 {
		t.Fatalf("checker calls: %d", len(c.checker))
	}
}

// atTheCap is a gate over a project the given cap stopped.
func atTheCap(db *store.DB, stoppedBy string) *cycle.Gate {
	g := cycle.NewGate(db)
	g.StoppedBy = func(context.Context, string) (string, error) { return stoppedBy, nil }
	return g
}

// withFigurePending leaves sec-1 with an agent's pending: a figure without
// its decision (it is the same one TestB14 uses).
func withFigurePending(t *testing.T, db *store.DB, sectionID string) {
	t.Helper()
	err := db.Write(context.Background(), "p1", func(tx *store.Tx) error {
		sec, err := tx.Object(sectionID)
		if err != nil {
			return err
		}
		var b map[string]any
		_ = json.Unmarshal(sec.Body, &b)
		b["text"] = signText + " La diferencia es de 60 veces."
		_, err = tx.UpdateObject(sectionID, sec.CurrentVersion, sec.State, b, store.EventInput{Type: store.EventEdit, Actor: "human"})
		return err
	})
	if err != nil {
		t.Fatalf("seed the pending: %v", err)
	}
}

// B3-20 (2-nov): at the cap the agent's pendings do not block the signoff:
// P7 lists them apart, you can sign, and the history records that you signed
// with them open because of the cap.
func TestB3_20_AgentPendingsDoNotBlockTheSignoffAtTheCap(t *testing.T) {
	db := openTemp(t)
	sectionID, _ := seedSignable(t, db, false)
	withFigurePending(t, db, sectionID)
	g := atTheCap(db, "project_cap")
	ctx := context.Background()
	open, err := g.RequestSignoff(ctx, "p1", "sec-1")
	if err != nil || !open.Opened || len(open.Pendings) != 0 {
		t.Fatalf("RequestSignoff: %+v, %v", open, err)
	}
	if err := g.RecordExplanation(ctx, "p1", open.Label, "Cuenta cómo se compararon."); err != nil {
		t.Fatalf("RecordExplanation: %v", err)
	}
	v, err := graph.SignoffDetailAtCap(ctx, db, "p1", open.Label, "project_cap")
	if err != nil {
		t.Fatal(err)
	}
	if !v.CanSign || v.StoppedBy != "project_cap" || len(v.Pendings) != 0 || len(v.PendingsAtCap) != 1 ||
		v.PendingsAtCap[0].Kind != graph.PendingFigureWithoutDecision {
		t.Fatalf("view: %+v", v)
	}
	if err := g.Sign(ctx, "p1", open.Label); err != nil {
		t.Fatalf("Sign: %v", err)
	}
	if got := sectionState(t, db, "sec-1"); got != "signed" {
		t.Fatalf("section: %q", got)
	}
	briefs := eventsOfType(t, db, store.EventBriefSignoff)
	if len(briefs) != 1 {
		t.Fatalf("brief_signoff: %d", len(briefs))
	}
	for _, part := range []string{`"open_at_cap"`, `"kind":"figure_without_decision"`, `"stopped_by":"project_cap"`} {
		if !strings.Contains(string(briefs[0].Data), part) {
			t.Fatalf("event %s lacks %s", briefs[0].Data, part)
		}
	}
}

// B-76 with B3-20: an open proposal at the cap is discarded in the signing's
// transaction, and the event keeps what you signed with open and which cap.
func TestB76_DiscardingAtSigningLeavesOpenAtCapAndStoppedByIntact(t *testing.T) {
	db := openTemp(t)
	sectionID, _ := seedSignable(t, db, false)
	withFigurePending(t, db, sectionID)
	sec, _ := db.GetObject(context.Background(), "p1", "sec-1")
	g := atTheCap(db, "project_cap")
	ctx := context.Background()
	open, err := g.RequestSignoff(ctx, "p1", "sec-1")
	if err != nil || !open.Opened {
		t.Fatalf("RequestSignoff: %+v, %v", open, err)
	}
	step := `{"stretch": "build", "triage_line": "T.", "closing_risk": "none", "note": "N.",
	  "new_text": [{"section_id": "sec-1", "base_version": ` + strconv.Itoa(sec.CurrentVersion) + `, "anchor": "", "claim_origin": "material", "operation": "insert_after", "text": "Antes."}]}`
	applied := applyRecorded(t, db, json.RawMessage(step))
	if len(applied.Proposals) != 1 {
		t.Fatalf("applied: %+v", applied)
	}
	if err := g.RecordExplanation(ctx, "p1", open.Label, "Cuenta cómo se compararon."); err != nil {
		t.Fatalf("RecordExplanation: %v", err)
	}
	if err := g.Sign(ctx, "p1", open.Label); err != nil {
		t.Fatalf("Sign: %v", err)
	}
	briefs := eventsOfType(t, db, store.EventBriefSignoff)
	if len(briefs) != 1 {
		t.Fatalf("brief_signoff: %d", len(briefs))
	}
	for _, part := range []string{`"open_at_cap"`, `"stopped_by":"project_cap"`} {
		if !strings.Contains(string(briefs[0].Data), part) {
			t.Fatalf("event %s lacks %s", briefs[0].Data, part)
		}
	}
	prop, _ := db.GetObject(ctx, "p1", applied.Proposals[0])
	if prop.State != "discarded" {
		t.Fatalf("proposal: %+v", prop)
	}
}

// B3-20 (2-nov): with the agent still able to work, its pendings block as
// ever, and the signing records nothing about a cap.
func TestB3_20_AgentPendingsBlockBelowTheCap(t *testing.T) {
	db := openTemp(t)
	sectionID, _ := seedSignable(t, db, false)
	withFigurePending(t, db, sectionID)
	open, err := atTheCap(db, "").RequestSignoff(context.Background(), "p1", "sec-1")
	if err != nil || open.Opened || !hasPending(open.Pendings, graph.PendingFigureWithoutDecision, "") {
		t.Fatalf("RequestSignoff: %+v, %v", open, err)
	}
}

// B3-20 (2-nov): your own pendings still block at the cap: an open card keeps
// the signoff from opening and from signing, and it is listed with what is
// yours, not with the agent's.
func TestB3_20_YourPendingsStillBlockAtTheCap(t *testing.T) {
	db := openTemp(t)
	sectionID, _ := seedSignable(t, db, false)
	withFigurePending(t, db, sectionID)
	g := atTheCap(db, "project_cap")
	ctx := context.Background()
	open, err := g.RequestSignoff(ctx, "p1", "sec-1")
	if err != nil || !open.Opened {
		t.Fatalf("RequestSignoff: %+v, %v", open, err)
	}
	if err := g.RecordExplanation(ctx, "p1", open.Label, "Cuenta cómo se compararon."); err != nil {
		t.Fatalf("RecordExplanation: %v", err)
	}
	err = db.Write(ctx, "p1", func(tx *store.Tx) error {
		_, err := tx.CreateObject(store.ObjectCard, "open", map[string]any{"section_id": sectionID},
			store.EventInput{Type: store.EventStateChanged, Actor: "app"})
		return err
	})
	if err != nil {
		t.Fatalf("seed card: %v", err)
	}
	v, err := graph.SignoffDetailAtCap(ctx, db, "p1", open.Label, "project_cap")
	if err != nil {
		t.Fatal(err)
	}
	if v.CanSign || !hasPending(v.Pendings, graph.PendingOpenCard, "") || hasPending(v.Pendings, graph.PendingFigureWithoutDecision, "") ||
		!hasPending(v.PendingsAtCap, graph.PendingFigureWithoutDecision, "") {
		t.Fatalf("view: %+v", v)
	}
	var pend *cycle.PendingsError
	if err := g.Sign(ctx, "p1", open.Label); !errors.As(err, &pend) || len(pend.Pendings) != 1 || pend.Pendings[0].Kind != graph.PendingOpenCard {
		t.Fatalf("Sign: %v", err)
	}
	if err := g.LeaveSignoff(ctx, "p1", open.Label); err != nil {
		t.Fatalf("leave: %v", err)
	}
	again, err := g.RequestSignoff(ctx, "p1", "sec-1")
	if err != nil || again.Opened || len(again.Pendings) != 1 || again.Pendings[0].Kind != graph.PendingOpenCard {
		t.Fatalf("asking again: %+v, %v", again, err)
	}
}

// Previous behavior: «sin comprobar» after the checker really failed twice
// is not sent again by asking to sign (B-50).
func TestB3_20_AFailedCheckerStaysUncheckedWhenAskingToSign(t *testing.T) {
	db := seedCheckerProject(t)
	yourEdit(t, db, "sec-1", contradictingEdit)
	runner, c := checkerRunner(t, db, checkerFixture(t, "malformed"), checkerFixture(t, "malformed"))
	t.Cleanup(runner.Close)
	for range 2 {
		open, err := runner.RequestSignoff(context.Background(), "p1", "sec-1")
		if err != nil || open.Opened || !hasPending(open.Pendings, graph.PendingUnchecked, "") {
			t.Fatalf("RequestSignoff: %+v, %v", open, err)
		}
	}
	if len(c.checker) != 2 {
		t.Fatalf("checker calls: %d, want 2 (its one retry)", len(c.checker))
	}
}

// B3-20: a failed step says nothing about the built text: when the principal
// no longer fits, the project-cap stop closes «retorno sin la forma fija».
func TestB3_20_AFailedStepAtTheBalanceDoesNotBlockSigning(t *testing.T) {
	db := seedCheckerProject(t)
	bad := textFixture(t, `{"stretch":"build"}`)
	bad.Usage.CostUSD = 0.10
	runner, _ := checkerRunner(t, db, bad, bad)
	t.Cleanup(runner.Close)
	spend(t, db, "p1", 2.30)
	if err := runner.RunTurn(context.Background(), "p1", "turn-1", "Sigue."); err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	if r := oneReport(t, db); r.StopReason != "project_cap" {
		t.Fatalf("report: %+v", r)
	}
	open, err := runner.RequestSignoff(context.Background(), "p1", "sec-1")
	if err != nil || !open.Opened {
		t.Fatalf("RequestSignoff: %+v, %v", open, err)
	}
}
