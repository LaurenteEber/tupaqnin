package cycle

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/LaurenteEber/tupaqnin/internal/checks"
	"github.com/LaurenteEber/tupaqnin/internal/provider"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// spendGate is the one door of the runner's model calls (B-89, espec. §6
// «reserva del máximo antes de cada llamada»): before each call it reserves
// the call's maximum, lets it through only if it fits in the four caps, and
// releases it when the call ends. Reservations in flight live in memory, so
// the calls of different projects of one link cannot overshoot together.
// The cap is looked at before each call, never during one.
type spendGate struct {
	db *store.DB
	// projectCap, dayCap, totalCap and now are read per call: main and the
	// tests set them on the runner after NewRunner.
	projectCap, dayCap, totalCap func() float64
	now                          func() time.Time
	inner                        provider.Provider

	mu      sync.Mutex
	project map[string]float64 // reserved by project
	link    map[string]float64 // reserved by link
	day     float64            // reserved by the links that are not the jury's
	total   float64            // reserved by every link
	// contraster is reserved by the contraster, by project: what is in flight
	// against the project's signoff balance.
	contraster map[string]float64
}

func newSpendGate(r *Runner, db *store.DB, inner provider.Provider) *spendGate {
	return &spendGate{db: db, inner: inner, projectCap: r.projectCap, dayCap: r.dayCap, totalCap: r.totalCap, now: r.now,
		project: map[string]float64{}, link: map[string]float64{}, contraster: map[string]float64{}}
}

func (r *Runner) dayCap() float64 {
	if r.SpendCapDayUSD > 0 {
		return r.SpendCapDayUSD
	}
	return DefaultDayCapUSD
}

func (r *Runner) totalCap() float64 {
	if r.SpendCapTotalUSD > 0 {
		return r.SpendCapTotalUSD
	}
	return DefaultTotalCapUSD
}

type signoffCallKey struct{}

// withSignoffBalance marks the calls of ctx as the signoff's: they may use
// the balance set aside of the project's cap (B3-20).
// [EL DISEÑO NO LO DICE]: B-89 names the contraster. The checker's batch
// that asking to sign sends first (B-36) uses it too: without it, an edit
// at the balance could never be signed, and B3-20 says the cap never keeps
// you from signing what you built.
func withSignoffBalance(ctx context.Context) context.Context {
	return context.WithValue(ctx, signoffCallKey{}, true)
}

func usesBalance(ctx context.Context, role provider.AgentRole) bool {
	marked, _ := ctx.Value(signoffCallKey{}).(bool)
	return marked || role == provider.RoleContraster
}

// over is the cap that spent, what is reserved and amount would pass, in
// B-89's order, or nil; g.mu is held. The contraster's calls may pass the link's
// and the day's caps while they fit in the project's signoff balance, never the
// total's (B-89, Eber, 2-nov). The test is on the role: the checker batch that
// uses the balance of the project's cap stays inside these two.
func (g *spendGate) over(st store.SpendState, projectID string, role provider.AgentRole, amount float64, balance bool) *provider.CapError {
	const eps = 1e-9
	limit := g.projectCap()
	if !balance {
		limit -= provider.SignoffReserveUSD
	}
	if st.ProjectUSD+g.project[projectID]+amount > limit+eps {
		return &provider.CapError{Reason: provider.CapProjectStop}
	}
	if st.InvitationID == "" {
		return nil // an older local database: the project's cap only
	}
	inBalance := role == provider.RoleContraster &&
		st.ContrasterUSD+g.contraster[projectID]+amount <= provider.SignoffReserveUSD+eps
	if !inBalance {
		if st.SpentUSD+g.link[st.InvitationID]+amount > st.BudgetUSD+eps {
			return &provider.CapError{Reason: provider.CapDemoStop, Cap: provider.CapLink}
		}
		if st.Label != store.LabelJury && st.DayUSD+g.day+amount > g.dayCap()+eps {
			return &provider.CapError{Reason: provider.CapDemoStop, Cap: provider.CapDay}
		}
	}
	if st.TotalUSD+g.total+amount > g.totalCap()+eps {
		return &provider.CapError{Reason: provider.CapDemoStop, Cap: provider.CapTotal}
	}
	return nil
}

// Respond reserves, makes the call and releases on every way out. A call
// that does not fit never reaches the provider: it answers *provider.CapError.
func (g *spendGate) Respond(ctx context.Context, req provider.Request, onDelta func(provider.Delta)) (provider.Response, error) {
	release, err := g.reserve(ctx, req)
	if err != nil {
		return provider.Response{}, err
	}
	defer release()
	return g.inner.Respond(ctx, req, onDelta)
}

func (g *spendGate) reserve(ctx context.Context, req provider.Request) (func(), error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	st, err := g.db.ProjectSpendState(ctx, req.ProjectID, string(req.Role), g.now())
	if err != nil {
		return nil, err // no figures, no call
	}
	amount := max(provider.ReserveUSD(req.Role, req.TurnKind), st.DearestUSD)
	if capErr := g.over(st, req.ProjectID, req.Role, amount, usesBalance(ctx, req.Role)); capErr != nil {
		return nil, capErr
	}
	g.project[req.ProjectID] += amount
	if req.Role == provider.RoleContraster {
		g.contraster[req.ProjectID] += amount
	}
	if st.InvitationID != "" {
		g.link[st.InvitationID] += amount
		g.total += amount
		if st.Label != store.LabelJury {
			g.day += amount
		}
	}
	return func() {
		g.mu.Lock()
		defer g.mu.Unlock()
		drop(g.project, req.ProjectID, amount)
		if req.Role == provider.RoleContraster {
			drop(g.contraster, req.ProjectID, amount)
		}
		if st.InvitationID != "" {
			drop(g.link, st.InvitationID, amount)
			g.total -= amount
			if st.Label != store.LabelJury {
				g.day -= amount
			}
			if len(g.link) == 0 { // nothing in flight: no rounding left over
				g.total, g.day = 0, 0
			}
		}
	}, nil
}

func drop(held map[string]float64, key string, amount float64) {
	if held[key] -= amount; held[key] < 1e-9 {
		delete(held, key)
	}
}

// fits is the same check for what is not one call yet, with no reservation
// (a round before it launches, a batch before it goes out).
func (g *spendGate) fits(ctx context.Context, projectID string, role provider.AgentRole, amount float64) (*provider.CapError, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	st, err := g.db.ProjectSpendState(ctx, projectID, string(role), g.now())
	if err != nil {
		return nil, err
	}
	return g.over(st, projectID, role, amount, usesBalance(ctx, role)), nil
}

// overCap is the cap one more call of this role and turn kind would pass,
// or nil, without reserving: the callers that decide before building a call.
func (r *Runner) overCap(ctx context.Context, projectID string, role provider.AgentRole, kind provider.TurnKind) (*provider.CapError, error) {
	st, err := r.db.ProjectSpendState(ctx, projectID, string(role), r.now())
	if err != nil {
		return nil, err
	}
	return r.spend.fits(ctx, projectID, role, max(provider.ReserveUSD(role, kind), st.DearestUSD))
}

// checkAgentStep is the checker of an applied step (backlog-r2 2.3). When it
// does not fit, it does not go out and the sections stay «sin comprobar» with
// the cap on record, no failed attempts: asking to sign sends them with the
// balance (B3-20, checkRefusedSection).
// [EL DISEÑO NO LO DICE]: the record is an output_retry of the checker with
// result unchecked, which is what «sin comprobar» reads, marked refused.
func (r *Runner) checkAgentStep(ctx context.Context, projectID, turnID string, change checks.Change) error {
	capErr, err := r.overCap(ctx, projectID, provider.RoleChecker, provider.TurnChecker)
	if err != nil {
		return err
	}
	if capErr == nil {
		return r.checker.Check(ctx, r.db, projectID, turnID, change)
	}
	sections := make([]string, 0, len(change.Sections))
	for _, id := range change.Sections {
		sec, err := r.db.GetObject(ctx, projectID, id)
		if err != nil {
			return err
		}
		sections = append(sections, sec.Label)
	}
	return r.recordRefusedCheck(ctx, projectID, turnID, sections, capErr)
}

// recordRefusedCheck leaves the sections «sin comprobar» because a cap refused
// their checker, with no failed attempts.
func (r *Runner) recordRefusedCheck(ctx context.Context, projectID, turnID string, sections []string, capErr *provider.CapError) error {
	data := map[string]any{"turn_kind": "checker", "result": "unchecked", "refused": capErr.Reason,
		"errors": []string{}, "sections": sections}
	if capErr.Cap != "" {
		data["cap"] = capErr.Cap
	}
	return r.gate.append(ctx, projectID, store.EventInput{Type: store.EventOutputRetry, Actor: "agent",
		AgentRole: string(provider.RoleChecker), ModelAndEffort: provider.CheckerModel + " " + provider.CheckerEffort,
		TurnID: turnID, Data: data})
}

// refusedCheckOpen reports a section whose «sin comprobar» comes from a
// checker the gate refused: its last unchecked is the refused one, and no
// judgment came after it. One that really failed twice stays as B-50 says.
func (r *Runner) refusedCheckOpen(ctx context.Context, projectID, label string) (bool, error) {
	events, err := r.db.EventsSince(ctx, projectID, 0)
	if err != nil {
		return false, err
	}
	open := false
	for _, ev := range events {
		if ev.Type != store.EventOutputRetry && ev.Type != store.EventProbe {
			continue
		}
		var data struct {
			TurnKind string   `json:"turn_kind"`
			Result   string   `json:"result"`
			Refused  string   `json:"refused"`
			Sections []string `json:"sections"`
		}
		if json.Unmarshal(ev.Data, &data) != nil || !slices.Contains(data.Sections, label) {
			continue
		}
		switch {
		case ev.Type == store.EventProbe:
			open = false
		case data.TurnKind == "checker" && data.Result == "unchecked":
			open = data.Refused != ""
		}
	}
	return open, nil
}

// checkRefusedSection sends the section a refused checker left, whole, over
// its current text, as the agent's step it was. The caller holds the turn
// slot and asks with the balance. If it does not fit, the answer is the cap.
// [EL DISEÑO NO LO DICE]: only the section you ask to sign, so the balance
// goes to what you sign.
func (r *Runner) checkRefusedSection(ctx context.Context, projectID, turnID, label string) error {
	if err := r.checkerFits(ctx, projectID); err != nil {
		return err
	}
	sec, err := r.db.GetObject(ctx, projectID, label)
	if err != nil {
		return err
	}
	var body struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(sec.Body, &body); err != nil {
		return fmt.Errorf("cycle: section body: %w", err)
	}
	return r.checker.Check(ctx, r.db, projectID, turnID, checks.Change{Kind: checks.AgentStep, Actor: "agent",
		Sections: []string{sec.ID}, Passages: []checks.Passage{{Section: sec.ID, Text: body.Text}}})
}

// asCap reports a call the gate refused.
func asCap(err error) (*provider.CapError, bool) {
	var capErr *provider.CapError
	ok := errors.As(err, &capErr)
	return capErr, ok
}

// StoppedBy is the cap that keeps the agent from its next call, for the
// bar (B-89): project_cap, link, day or total; "" when the principal's
// reservation fits. It reads what was spent, not the calls in flight, so a
// turn running does not make it flicker.
func (r *Runner) StoppedBy(ctx context.Context, projectID string) (string, error) {
	st, err := r.db.ProjectSpendState(ctx, projectID, string(provider.RolePrincipal), r.now())
	if err != nil {
		return "", err
	}
	idle := &spendGate{projectCap: r.projectCap, dayCap: r.dayCap, totalCap: r.totalCap}
	amount := max(provider.ReserveUSD(provider.RolePrincipal, provider.TurnStep), st.DearestUSD)
	capErr := idle.over(st, projectID, provider.RolePrincipal, amount, false)
	switch {
	case capErr == nil:
		return "", nil
	case capErr.Cap == "":
		return capErr.Reason, nil
	}
	return capErr.Cap, nil
}

// SignoffBalanceSpent reports that one more contrast would not fit, whatever cap
// refuses it (B3-20, P-128). Like StoppedBy, it reads what was spent, not the
// calls in flight, so a round running does not make it flicker.
func (r *Runner) SignoffBalanceSpent(ctx context.Context, projectID string) (bool, error) {
	st, err := r.db.ProjectSpendState(ctx, projectID, string(provider.RoleContraster), r.now())
	if err != nil {
		return false, err
	}
	idle := &spendGate{projectCap: r.projectCap, dayCap: r.dayCap, totalCap: r.totalCap}
	amount := max(provider.ReserveUSD(provider.RoleContraster, provider.TurnSignoff), st.DearestUSD)
	return idle.over(st, projectID, provider.RoleContraster, amount, true) != nil, nil
}

// CreateBlocked is why a link opens no new project (B-89, B3-20): the code
// POST /api/projects answers, or "". A link «no alcanza» when the first
// call of the principal would not fit in it, so a new project is not one
// that stops at its first turn.
// [EL DISEÑO NO LO DICE]: the measure of «no alcanza» at creation: the
// principal's reservation, on what was spent, without the calls in flight.
// The total's cap has no code in the row (E.3): it stops the calls only.
func (r *Runner) CreateBlocked(ctx context.Context, invitationID string) (string, error) {
	st, err := r.db.LinkSpendState(ctx, invitationID, r.now())
	if err != nil {
		return "", err
	}
	const eps = 1e-9
	reserve := provider.ReserveUSD(provider.RolePrincipal, provider.TurnStep)
	if st.SpentUSD+reserve > st.BudgetUSD+eps {
		return "link_budget", nil
	}
	if st.Label != store.LabelJury && st.DayUSD+reserve > r.dayCap()+eps {
		return "day_cap", nil
	}
	return "", nil
}
