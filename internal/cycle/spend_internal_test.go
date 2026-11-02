package cycle

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/LaurenteEber/tupaqnin/internal/provider"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

func spendRunner(t *testing.T, inner provider.Provider) (*store.DB, *Runner) {
	t.Helper()
	db, err := store.Open(context.Background(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	r := NewRunner(db, inner, func(context.Context, []json.RawMessage) (int, error) { return 0, nil }, nil)
	t.Cleanup(r.Close)
	return db, r
}

func linkedProjects(t *testing.T, db *store.DB, budget float64, ids ...string) {
	t.Helper()
	ctx := context.Background()
	_, hash, err := store.NewSecret()
	if err != nil {
		t.Fatal(err)
	}
	link, err := db.CreateInvitation(ctx, store.InvitationInput{Label: "guests", BudgetUSD: budget, MaxAccounts: 3, MaxAnonProjects: 5}, hash, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range ids {
		if _, err := db.Writer.ExecContext(ctx, `INSERT INTO project (id, invitation_id, created_at) VALUES (?, ?, '2026-10-28T00:00:00.000Z')`, id, link); err != nil {
			t.Fatal(err)
		}
	}
}

// B-89, espec. §6: reservations live in memory, so two projects of one
// link calling at once cannot overshoot the link together: the second
// call does not fit while the first holds its reserve.
func TestB3_22_2_ConcurrentProjectsOfALinkDoNotOvershoot(t *testing.T) {
	inner := &blockingProvider{release: make(chan struct{})}
	db, r := spendRunner(t, inner)
	linkedProjects(t, db, 0.20, "p1", "p2")
	first := make(chan error, 1)
	go func() {
		_, err := r.prov.Respond(context.Background(), provider.Request{ProjectID: "p1", Role: provider.RolePrincipal}, nil)
		first <- err
	}()
	deadline := time.Now().Add(2 * time.Second)
	for inner.count() < 1 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	_, err := r.prov.Respond(context.Background(), provider.Request{ProjectID: "p2", Role: provider.RolePrincipal}, nil)
	var capErr *provider.CapError
	if !errors.As(err, &capErr) || capErr.Reason != provider.CapDemoStop || capErr.Cap != provider.CapLink {
		t.Fatalf("the second call: %v, want demo_cap link", err)
	}
	if inner.count() != 1 {
		t.Fatalf("calls that reached the provider: %d, want 1", inner.count())
	}
	close(inner.release)
	if err := <-first; err != nil {
		t.Fatalf("the first call: %v", err)
	}
	// Released: the same call fits now.
	if _, err := r.prov.Respond(context.Background(), provider.Request{ProjectID: "p2", Role: provider.RolePrincipal}, nil); err != nil {
		t.Fatalf("after the first ended: %v", err)
	}
}

type failingProvider struct {
	err   error
	panic bool
}

func (p failingProvider) Respond(context.Context, provider.Request, func(provider.Delta)) (provider.Response, error) {
	if p.panic {
		panic("boom")
	}
	return provider.Response{}, p.err
}

// The reservation is released on every exit: a failed call and a panic
// leave nothing held.
func TestB89_TheReservationIsReleasedOnEveryExit(t *testing.T) {
	for name, inner := range map[string]failingProvider{
		"error": {err: errors.New("provider down")},
		"panic": {panic: true},
	} {
		t.Run(name, func(t *testing.T) {
			db, r := spendRunner(t, inner)
			linkedProjects(t, db, 0.20, "p1")
			func() {
				defer func() { _ = recover() }()
				_, _ = r.prov.Respond(context.Background(), provider.Request{ProjectID: "p1", Role: provider.RolePrincipal}, nil)
			}()
			g := r.spend
			g.mu.Lock()
			defer g.mu.Unlock()
			if g.total != 0 || g.day != 0 || g.project["p1"] != 0 || len(g.project) != 0 || len(g.link) != 0 {
				t.Fatalf("held after the call: total %v, day %v, project %v, link %v", g.total, g.day, g.project, g.link)
			}
		})
	}
}

// Every call goes through the gate: the checker's provider is the gate
// too, not the provider underneath.
func TestB89_EveryCallOfTheRunnerGoesThroughTheGate(t *testing.T) {
	var mu sync.Mutex
	calls := 0
	inner := providerFunc(func(context.Context, provider.Request, func(provider.Delta)) (provider.Response, error) {
		mu.Lock()
		defer mu.Unlock()
		calls++
		return provider.Response{}, nil
	})
	db, r := spendRunner(t, inner)
	linkedProjects(t, db, 0.01, "p1")
	r.SpendCapTotalUSD = 0.01 // the signoff passes the link's cap, never the total's
	if r.checker.Provider != provider.Provider(r.spend) || r.prov != provider.Provider(r.spend) {
		t.Fatal("the runner's providers are not the gate")
	}
	roles := []provider.Request{
		{Role: provider.RolePrincipal, TurnKind: provider.TurnStep},
		{Role: provider.RolePrincipal, TurnKind: provider.TurnStopReport},
		{Role: provider.RolePrincipal, TurnKind: provider.TurnUnderstanding},
		{Role: provider.RolePrincipal, TurnKind: provider.TurnDigest},
		{Role: provider.RoleReader, TurnKind: provider.TurnReader},
		{Role: provider.RoleChecker, TurnKind: provider.TurnChecker},
		{Role: provider.RoleJudge, TurnKind: provider.TurnJudgeVerify},
		{Role: provider.RoleJudge, TurnKind: provider.TurnJudgeDefense},
		{Role: provider.RoleContraster, TurnKind: provider.TurnSignoff},
	}
	for _, req := range roles {
		req.ProjectID = "p1"
		_, err := r.prov.Respond(context.Background(), req, nil)
		var capErr *provider.CapError
		if !errors.As(err, &capErr) {
			t.Fatalf("%s %s: %v, want refused", req.Role, req.TurnKind, err)
		}
	}
	if calls != 0 {
		t.Fatalf("calls that reached the provider: %d", calls)
	}
}

var contrast = provider.Request{Role: provider.RoleContraster, TurnKind: provider.TurnSignoff}

func contrastOf(projectID string) provider.Request {
	req := contrast
	req.ProjectID = projectID
	return req
}

func okProvider() provider.Provider {
	return providerFunc(func(context.Context, provider.Request, func(provider.Delta)) (provider.Response, error) {
		return provider.Response{}, nil
	})
}

func recordCost(t *testing.T, db *store.DB, projectID string, role provider.AgentRole, usd float64) {
	t.Helper()
	if err := db.RecordModelCall(context.Background(), store.ModelCall{ProjectID: projectID, Role: string(role), CostUSD: usd}); err != nil {
		t.Fatal(err)
	}
}

func wantCap(t *testing.T, what string, err error, reason, capName string) {
	t.Helper()
	var capErr *provider.CapError
	if !errors.As(err, &capErr) || capErr.Reason != reason || capErr.Cap != capName {
		t.Fatalf("%s: %v, want %s %s", what, err, reason, capName)
	}
}

// B-89 (Eber, 2-nov): a signoff call may pass the link's cap up to the
// project's signoff balance. The balance is each project's: one that spent it
// stops, and another of the same link still signs. Only the contraster's cost
// counts against it.
func TestB89_AContrasterPassesTheLinkCapUpToTheProjectBalance(t *testing.T) {
	db, r := spendRunner(t, okProvider())
	linkedProjects(t, db, 0.01, "p1", "p2")
	ctx := context.Background()
	recordCost(t, db, "p1", provider.RolePrincipal, 1) // past the link: the principal's cost is not the balance

	for i := range 2 {
		if _, err := r.prov.Respond(ctx, contrastOf("p1"), nil); err != nil {
			t.Fatalf("contrast %d: %v", i+1, err)
		}
		recordCost(t, db, "p1", provider.RoleContraster, 0.20)
	}
	_, err := r.prov.Respond(ctx, contrastOf("p1"), nil)
	wantCap(t, "the balance spent", err, provider.CapDemoStop, provider.CapLink)

	if _, err := r.prov.Respond(ctx, contrastOf("p2"), nil); err != nil {
		t.Fatalf("another project of the link: %v", err)
	}
}

// B-89: the same for the day's cap.
func TestB89_AContrasterPassesTheDayCapUpToTheProjectBalance(t *testing.T) {
	db, r := spendRunner(t, okProvider())
	linkedProjects(t, db, 10, "p1")
	r.SpendCapDayUSD = 0.01
	ctx := context.Background()
	if _, err := r.prov.Respond(ctx, contrastOf("p1"), nil); err != nil {
		t.Fatalf("the contrast: %v", err)
	}
	recordCost(t, db, "p1", provider.RoleContraster, 0.40)
	_, err := r.prov.Respond(ctx, contrastOf("p1"), nil)
	wantCap(t, "the balance spent", err, provider.CapDemoStop, provider.CapDay)
}

// B-89: what is in flight counts against the balance: three contrasts at once
// cannot pass it together.
func TestB89_ContrastsInFlightCountAgainstTheBalance(t *testing.T) {
	inner := &blockingProvider{release: make(chan struct{})}
	db, r := spendRunner(t, inner)
	linkedProjects(t, db, 0.01, "p1")
	ctx := context.Background()
	done := make(chan error, 2)
	for range 2 {
		go func() {
			_, err := r.prov.Respond(ctx, contrastOf("p1"), nil)
			done <- err
		}()
	}
	deadline := time.Now().Add(2 * time.Second)
	for inner.count() < 2 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	_, err := r.prov.Respond(ctx, contrastOf("p1"), nil)
	wantCap(t, "the third at once", err, provider.CapDemoStop, provider.CapLink)
	close(inner.release)
	for range 2 {
		if err := <-done; err != nil {
			t.Fatalf("in flight: %v", err)
		}
	}
	r.spend.mu.Lock()
	defer r.spend.mu.Unlock()
	if len(r.spend.contraster) != 0 {
		t.Fatalf("held after the calls: %v", r.spend.contraster)
	}
}

// B-89: «nunca el total», and the project's cap still reads first.
func TestB89_AContrasterNeverPassesTheTotalNorTheProjectCap(t *testing.T) {
	db, r := spendRunner(t, okProvider())
	linkedProjects(t, db, 0.01, "p1", "p2")
	ctx := context.Background()
	r.SpendCapTotalUSD = 0.01
	_, err := r.prov.Respond(ctx, contrastOf("p1"), nil)
	wantCap(t, "the total", err, provider.CapDemoStop, provider.CapTotal)

	r.SpendCapTotalUSD = 0
	recordCost(t, db, "p2", provider.RolePrincipal, 2.99)
	_, err = r.prov.Respond(ctx, contrastOf("p2"), nil)
	wantCap(t, "the project", err, provider.CapProjectStop, "")
}

// B-89: only the contraster passes. The checker that asking to sign sends, with
// the project's balance, stays inside the link's cap, and so does any other
// call. Below the caps nothing changes.
func TestB89_OtherCallsStayInsideTheLinkCap(t *testing.T) {
	db, r := spendRunner(t, okProvider())
	linkedProjects(t, db, 0.01, "p1")
	ctx := context.Background()
	marked := withSignoffBalance(ctx)
	_, err := r.prov.Respond(marked, provider.Request{ProjectID: "p1", Role: provider.RoleChecker, TurnKind: provider.TurnChecker}, nil)
	wantCap(t, "the signoff's checker", err, provider.CapDemoStop, provider.CapLink)
	_, err = r.prov.Respond(ctx, provider.Request{ProjectID: "p1", Role: provider.RolePrincipal, TurnKind: provider.TurnStep}, nil)
	wantCap(t, "the principal", err, provider.CapDemoStop, provider.CapLink)

	linkedProjects(t, db, 10, "p2")
	recordCost(t, db, "p2", provider.RoleContraster, 0.40)
	if _, err := r.prov.Respond(ctx, contrastOf("p2"), nil); err != nil {
		t.Fatalf("a contrast inside the caps needs no balance: %v", err)
	}
}

type providerFunc func(context.Context, provider.Request, func(provider.Delta)) (provider.Response, error)

func (f providerFunc) Respond(ctx context.Context, req provider.Request, d func(provider.Delta)) (provider.Response, error) {
	return f(ctx, req, d)
}
