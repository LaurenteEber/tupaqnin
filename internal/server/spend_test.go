package server_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/LaurenteEber/tupaqnin/internal/cycle"
	"github.com/LaurenteEber/tupaqnin/internal/graph"
	"github.com/LaurenteEber/tupaqnin/internal/provider"
	"github.com/LaurenteEber/tupaqnin/internal/server"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// spendAPI is the API over a runner with the fake, whose calls count.
type spendAPI struct {
	*access
	runner *cycle.Runner
	calls  *countingProvider
}

type countingProvider struct {
	inner provider.Provider
	n     int
}

func (c *countingProvider) Respond(ctx context.Context, req provider.Request, d func(provider.Delta)) (provider.Response, error) {
	c.n++
	return c.inner.Respond(ctx, req, d)
}

func newSpendAPI(t *testing.T, disk func() (bool, error)) *spendAPI {
	t.Helper()
	a := newAccess(t)
	fake := provider.NewFake()
	fake.Recorder = a.db
	calls := &countingProvider{inner: fake}
	count := func(ctx context.Context, input []json.RawMessage) (int, error) {
		return fake.CountTokens(ctx, provider.PrincipalModel, input)
	}
	runner := cycle.NewRunner(a.db, calls, count, a.hub)
	t.Cleanup(runner.Close)
	opts := []server.Option{server.WithProjectCap(3)}
	if disk != nil {
		opts = append(opts, server.WithDiskFull(disk))
	}
	a.h = server.NewWithAPI(nil, a.hub, a.db, runner, opts...)
	return &spendAPI{access: a, runner: runner, calls: calls}
}

// budgetLink is a link with the given balance; its visitor's cookies come
// from newSessionCookie.
func budgetLink(t *testing.T, db *store.DB, label string, budget float64) string {
	t.Helper()
	_, hash, err := store.NewSecret()
	if err != nil {
		t.Fatal(err)
	}
	id, err := db.CreateInvitation(context.Background(), store.InvitationInput{
		Label: label, BudgetUSD: budget, MaxAccounts: 3, MaxAnonProjects: 50}, hash, nowForLinks())
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func wantSpendRefused(t *testing.T, s *spendAPI, cookie *http.Cookie, code string) {
	t.Helper()
	rec := s.do(http.MethodPost, "/api/projects", cookie, map[string]any{"name": "Otro"})
	if rec.Code != http.StatusForbidden {
		t.Fatalf("POST /api/projects: %d (%s), want 403", rec.Code, rec.Body.String())
	}
	var body struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || body.Error != code {
		t.Fatalf("POST /api/projects: %s, want %s", rec.Body.String(), code)
	}
}

func spendOn(t *testing.T, db *store.DB, projectID string, usd float64) {
	t.Helper()
	if err := db.RecordModelCall(context.Background(), store.ModelCall{ProjectID: projectID, Role: "seed", CostUSD: usd}); err != nil {
		t.Fatal(err)
	}
}

// B3-22 row 2, B-89: two projects of one link. When the link does not
// reach, the next call does not go out (demo_cap, link), a new project
// answers link_budget, and the one that exists is read, edited and
// downloaded.
func TestB3_22_2_LinkBudget(t *testing.T) {
	s := newSpendAPI(t, nil)
	link := budgetLink(t, s.db, "guests", 1)
	one, two := newSessionCookie(t, s.db, link), newSessionCookie(t, s.db, link)
	first := s.project(t, one)
	second := s.project(t, two)
	spendOn(t, s.db, first, 0.60)
	spendOn(t, s.db, second, 0.30)

	if err := s.runner.RunTurn(context.Background(), second, "turn-1", "Sigue."); err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	if s.calls.n != 0 {
		t.Fatalf("provider calls: %d, want 0", s.calls.n)
	}
	report := decodeBody[cycle.StopReport](t, s.do(http.MethodGet, "/api/projects/"+second+"/stop-report", two, nil).Body.Bytes())
	if report.StopReason != "demo_cap" || report.Cap != "link" {
		t.Fatalf("report: %+v", report)
	}

	wantSpendRefused(t, s, newSessionCookie(t, s.db, link), "link_budget")

	// The existing one: read, edit, download.
	base := "/api/projects/" + second
	rec := s.do(http.MethodPost, base+"/sections", two, map[string]any{"title": "Una"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create section: %d %s", rec.Code, rec.Body.String())
	}
	var sec struct {
		Label          string `json:"label"`
		CurrentVersion int    `json:"version"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &sec)
	if rec := s.do(http.MethodPut, base+"/sections/"+sec.Label, two, map[string]any{"text": "Mi texto.", "base_version": sec.CurrentVersion}); rec.Code != http.StatusOK {
		t.Fatalf("edit: %d %s", rec.Code, rec.Body.String())
	}
	if rec := s.do(http.MethodGet, base+"/sections/"+sec.Label, two, nil); rec.Code != http.StatusOK {
		t.Fatalf("read: %d", rec.Code)
	}
	if rec := s.do(http.MethodGet, base+"/export.md", two, nil); rec.Code != http.StatusOK {
		t.Fatalf("download: %d", rec.Code)
	}
}

// Previous behavior: with the link's balance intact a project is created
// as ever, and the other codes of E.1 stay.
func TestB3_22_2_CreateStillWorksWithBalance(t *testing.T) {
	s := newSpendAPI(t, nil)
	link := budgetLink(t, s.db, "guests", 1)
	cookie := newSessionCookie(t, s.db, link)
	s.project(t, cookie)
	wantSpendRefused(t, s, cookie, "account_required")
}

// B-89 (3), B3-20: at the day's cap a new project answers day_cap; the
// jury's link still creates.
func TestB89_CreateAtTheDayCap(t *testing.T) {
	s := newSpendAPI(t, nil)
	s.runner.SpendCapDayUSD = 1
	guests := budgetLink(t, s.db, "guests", 10)
	p := s.project(t, newSessionCookie(t, s.db, guests))
	spendOn(t, s.db, p, 1)
	wantSpendRefused(t, s, newSessionCookie(t, s.db, budgetLink(t, s.db, "guests", 10)), "day_cap")
	jury := budgetLink(t, s.db, store.LabelJury, 30)
	s.project(t, newSessionCookie(t, s.db, jury))
}

// Espec. §6: with the disk simulated at 80 %, a new project answers
// disk_full, except on the jury's link.
func TestB89_CreateWithTheDiskFull(t *testing.T) {
	s := newSpendAPI(t, func() (bool, error) { return true, nil })
	wantSpendRefused(t, s, newSessionCookie(t, s.db, budgetLink(t, s.db, "guests", 10)), "disk_full")
	s.project(t, newSessionCookie(t, s.db, budgetLink(t, s.db, store.LabelJury, 30)))
}

// A disk that cannot be read does not close the door: it is a soft guard.
func TestB89_CreateWhenTheDiskCannotBeRead(t *testing.T) {
	s := newSpendAPI(t, func() (bool, error) { return false, errors.New("statfs") })
	s.project(t, newSessionCookie(t, s.db, budgetLink(t, s.db, "guests", 10)))
}

// The real check reads the disk of a directory that exists, and fails on
// one that does not.
func TestB89_DiskFullReadsTheDirectory(t *testing.T) {
	if _, err := server.DiskFull(t.TempDir())(); err != nil {
		t.Fatalf("DiskFull: %v", err)
	}
	if _, err := server.DiskFull("/no/such/dir")(); err == nil {
		t.Fatal("DiskFull of no directory: want an error")
	}
}

// B3-20: warn at 80 % of the project's cap; stopped_by says which cap
// stops the next call. Computed in Go.
func TestB3_20_SummaryWarnAndStoppedBy(t *testing.T) {
	s := newSpendAPI(t, nil)
	link := budgetLink(t, s.db, "guests", 3)
	cookie := newSessionCookie(t, s.db, link)
	p := s.project(t, cookie)
	summary := func() server.SummaryView {
		t.Helper()
		return decodeBody[server.SummaryView](t, s.do(http.MethodGet, "/api/projects/"+p+"/summary", cookie, nil).Body.Bytes())
	}
	if v := summary(); v.Warn || v.StoppedBy != "" {
		t.Fatalf("fresh: %+v", v)
	}
	spendOn(t, s.db, p, 2)
	if v := summary(); v.Warn || v.StoppedBy != "" {
		t.Fatalf("under the cap less the balance: %+v", v)
	}
	// P-128: with a balance of 0.80 USD the agent stops before the 80 % of 3 USD.
	spendOn(t, s.db, p, 0.39)
	if v := summary(); v.Warn || v.StoppedBy != "project_cap" {
		t.Fatalf("at the cap less the balance: %+v", v)
	}
	spendOn(t, s.db, p, 0.01)
	if v := summary(); !v.Warn || v.StoppedBy != "project_cap" {
		t.Fatalf("at 80 %%: %+v", v)
	}
	spendOn(t, s.db, p, 0.20)
	s.runner.SpendCapDayUSD = 2
	if v := summary(); v.StoppedBy != "project_cap" {
		t.Fatalf("the project's cap comes first: %+v", v)
	}
	s.runner.ProjectCapUSD = 100
	if v := summary(); v.StoppedBy != "day" {
		t.Fatalf("the day: %+v", v)
	}
	s.runner.SpendCapDayUSD = 100
	s.runner.SpendCapTotalUSD = 2
	if v := summary(); v.StoppedBy != "total" {
		t.Fatalf("the total: %+v", v)
	}
	s.runner.SpendCapTotalUSD = 100
	if _, err := s.db.Writer.ExecContext(context.Background(), `UPDATE invitation SET budget_usd = 2.65 WHERE id = ?`, link); err != nil {
		t.Fatal(err)
	}
	if v := summary(); v.StoppedBy != "link" {
		t.Fatalf("the link: %+v", v)
	}
}

// B3-20 (2-nov) with B-89: asking to sign with edits unchecked and a link
// without balance opens the signoff: the checker cannot run, so «sin
// comprobar» is listed apart, with the cap, and signing records it. No call
// goes out.
func TestB3_20_AskingToSignAtTheLinkCapSignsWithTheAgentPendingsListed(t *testing.T) {
	s := newSpendAPI(t, nil)
	link := budgetLink(t, s.db, "guests", 1)
	cookie := newSessionCookie(t, s.db, link)
	p := s.project(t, cookie)
	base := "/api/projects/" + p
	rec := s.do(http.MethodPost, base+"/sections", cookie, map[string]any{"title": "Una"})
	var sec struct {
		Label          string `json:"label"`
		CurrentVersion int    `json:"version"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &sec)
	if rec := s.do(http.MethodPut, base+"/sections/"+sec.Label, cookie, map[string]any{"text": "Mi texto.", "base_version": sec.CurrentVersion}); rec.Code != http.StatusOK {
		t.Fatalf("edit: %d %s", rec.Code, rec.Body.String())
	}
	spendOn(t, s.db, p, 1)
	rec = s.do(http.MethodPost, base+"/sections/"+sec.Label+"/signoff", cookie, nil)
	if rec.Code != http.StatusCreated {
		t.Fatalf("signoff: %d %s", rec.Code, rec.Body.String())
	}
	var view graph.SignoffView
	if err := json.Unmarshal(rec.Body.Bytes(), &view); err != nil {
		t.Fatal(err)
	}
	if view.StoppedBy != "link" || len(view.Pendings) != 0 || len(view.PendingsAtCap) != 1 || view.PendingsAtCap[0].Kind != graph.PendingUnchecked {
		t.Fatalf("view: %+v", view)
	}
	if view.CanSign {
		t.Fatalf("can sign before your explanation: %+v", view)
	}
	signoff := base + "/signoffs/" + view.Label
	if rec := s.do(http.MethodPost, signoff+"/explanation", cookie, map[string]any{"text": "Mi texto dice esto."}); rec.Code >= 300 {
		t.Fatalf("explanation: %d %s", rec.Code, rec.Body.String())
	}
	rec = s.do(http.MethodPost, signoff+"/sign", cookie, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("sign: %d %s", rec.Code, rec.Body.String())
	}
	var data string
	if err := s.db.Reader.QueryRowContext(context.Background(),
		`SELECT data FROM event WHERE project_id = ? AND type = 'brief_signoff'`, p).Scan(&data); err != nil {
		t.Fatalf("the signing's event: %v", err)
	}
	for _, part := range []string{`"open_at_cap"`, `"kind":"unchecked"`, `"stopped_by":"link"`} {
		if !strings.Contains(data, part) {
			t.Fatalf("event data %s lacks %s", data, part)
		}
	}
	if s.calls.n != 0 {
		t.Fatalf("provider calls: %d", s.calls.n)
	}
}

func containsCode(body []byte, code string) bool {
	var out struct {
		Error string `json:"error"`
	}
	return json.Unmarshal(body, &out) == nil && out.Error == code
}

func decodeBody[T any](t *testing.T, raw []byte) T {
	t.Helper()
	var out T
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("decode %s: %v", raw, err)
	}
	return out
}

func nowForLinks() time.Time { return time.Now() }

// B3-20, P-128: with critical decisions and no room for another contrast, the
// signoff's view says the balance is spent, so the screen can say why signing
// stays impossible. Before that, and on a section that needs no contrast, it
// does not.
func TestB3_20_TheSignoffViewSaysWhenTheBalanceIsSpent(t *testing.T) {
	s := newSpendAPI(t, nil)
	cookie := newSessionCookie(t, s.db, budgetLink(t, s.db, "guests", 100))
	p := s.project(t, cookie)
	seedSignSection(t, s.db, p, plainText, true)
	rec := s.do(http.MethodPost, "/api/projects/"+p+"/sections/sec-1/signoff", cookie, nil)
	if rec.Code != http.StatusCreated {
		t.Fatalf("ask to sign: %d %s", rec.Code, rec.Body.String())
	}
	label := decodeBody[graph.SignoffView](t, rec.Body.Bytes()).Label
	view := func() graph.SignoffView {
		t.Helper()
		return decodeBody[graph.SignoffView](t, s.do(http.MethodGet, "/api/projects/"+p+"/signoffs/"+label, cookie, nil).Body.Bytes())
	}
	if v := view(); v.BalanceSpent {
		t.Fatalf("fresh: %+v", v)
	}
	// The agent's stop is not the balance's: the reserve still pays a contrast.
	spendOn(t, s.db, p, 3-provider.SignoffReserveUSD)
	if v := view(); v.BalanceSpent || v.StoppedBy != "project_cap" {
		t.Fatalf("at the cap less the balance: spent %v, stopped by %q", v.BalanceSpent, v.StoppedBy)
	}
	spendOn(t, s.db, p, provider.SignoffReserveUSD-provider.ContrasterReserveUSD+0.01)
	v := view()
	if !v.BalanceSpent || v.CanSign {
		t.Fatalf("no room for a contrast: spent %v, can sign %v", v.BalanceSpent, v.CanSign)
	}
}

// B3-20, P-128: the notice promises that a section with no critical decisions
// can still be signed with the balance spent: it needs no contrast, and its view
// says nothing is spent.
func TestB3_20_ASectionWithNoCriticalDecisionsSignsWithTheBalanceSpent(t *testing.T) {
	s := newSpendAPI(t, nil)
	cookie := newSessionCookie(t, s.db, budgetLink(t, s.db, "guests", 100))
	p := s.project(t, cookie)
	seedSignSection(t, s.db, p, plainText, false)
	spendOn(t, s.db, p, 3)
	rec := s.do(http.MethodPost, "/api/projects/"+p+"/sections/sec-1/signoff", cookie, nil)
	if rec.Code != http.StatusCreated {
		t.Fatalf("ask to sign: %d %s", rec.Code, rec.Body.String())
	}
	v := decodeBody[graph.SignoffView](t, rec.Body.Bytes())
	if !v.Brief || v.BalanceSpent {
		t.Fatalf("brief %v, balance spent %v", v.Brief, v.BalanceSpent)
	}
	if rec := s.do(http.MethodPost, "/api/projects/"+p+"/signoffs/"+v.Label+"/explanation", cookie, map[string]string{"text": "Comparo solo estudios iguales."}); rec.Code >= 300 {
		t.Fatalf("explain: %d %s", rec.Code, rec.Body.String())
	}
	before := s.calls.n
	rec = s.do(http.MethodPost, "/api/projects/"+p+"/signoffs/"+v.Label+"/sign", cookie, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("sign: %d %s", rec.Code, rec.Body.String())
	}
	if s.calls.n != before {
		t.Fatalf("calls %d, want %d", s.calls.n, before)
	}
}
