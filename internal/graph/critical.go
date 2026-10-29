package graph

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// Reason is why a decision is critical (fichas §3, «Criticidad»).
type Reason string

const (
	ReasonCoversFigure      Reason = "covers_figure"
	ReasonConclusionSection Reason = "conclusion_or_recommendation_section"
	ReasonTwoOrMoreSections Reason = "two_or_more_sections"
	// ReasonHumanMarked is the fourth reason; the mark itself arrives in
	// slice 2. The flag is read because the memory already read it.
	// [EL DISEÑO NO LO DICE]: whether slice 1 reads a mark it cannot set;
	// it does, so moving the memory's check here changes nothing.
	ReasonHumanMarked Reason = "human_marked"
)

// [EL DISEÑO NO LO DICE]: the backlog's signature has no reader; q is
// added so the same function reads the pool or the open transaction.
//
// Critical is the one function that says whether a decision is
// critical, and why (T-5, specification §1.5). It is never stored: it
// is called when reading and inside each transaction that changes
// edges or a section's role, with that transaction as q, so it sees
// the transaction's own writes.
func Critical(ctx context.Context, q store.Querier, projectID, decisionLabel string) (bool, []Reason, error) {
	dec, err := q.GetObject(ctx, projectID, decisionLabel)
	if err != nil {
		return false, nil, err
	}
	if dec.Type != store.ObjectDecision {
		return false, nil, fmt.Errorf("%w: %s is not a decision", store.ErrNotFound, decisionLabel)
	}
	g, err := loadCriticality(ctx, q, projectID)
	if err != nil {
		return false, nil, err
	}
	reasons, err := g.reasons(ctx, dec)
	return len(reasons) > 0, reasons, err
}

// CriticalAll applies Critical to every decision of the project, with
// the project read once. It maps each critical decision's id to its
// reasons.
func CriticalAll(ctx context.Context, q store.Querier, projectID string) (map[string][]Reason, error) {
	g, err := loadCriticality(ctx, q, projectID)
	if err != nil {
		return nil, err
	}
	out := make(map[string][]Reason)
	for _, dec := range g.decisions {
		reasons, err := g.reasons(ctx, dec)
		if err != nil {
			return nil, err
		}
		if len(reasons) > 0 {
			out[dec.ID] = reasons
		}
	}
	return out, nil
}

type criticality struct {
	q         store.Querier
	projectID string
	decisions []store.Object
	sections  map[string]store.Object
	texts     map[string]Compared
	byID      map[string]store.Object
}

func loadCriticality(ctx context.Context, q store.Querier, projectID string) (*criticality, error) {
	objects, err := q.AllObjects(ctx, projectID)
	if err != nil {
		return nil, err
	}
	g := &criticality{q: q, projectID: projectID, sections: map[string]store.Object{},
		texts: map[string]Compared{}, byID: map[string]store.Object{}}
	for _, obj := range objects {
		g.byID[obj.ID] = obj
		switch obj.Type {
		case store.ObjectDecision:
			g.decisions = append(g.decisions, obj)
		case store.ObjectSection:
			g.sections[obj.ID] = obj
			g.texts[obj.ID] = Compare(bodyString(obj, "text"))
		}
	}
	return g, nil
}

// reasons: it covers a figure (a datum is never covered: it comes from
// its source, A-3); it is in a section with role conclusion or
// recommendation (K-2, proposed or confirmed); it affects 2 or more
// sections: those the principal declared (used_in) plus those where a
// figure linked to it appears, by text search (fichas §3).
func (g *criticality) reasons(ctx context.Context, dec store.Object) ([]Reason, error) {
	var out []Reason
	if marked, _ := bodyValue(dec, "human_marked_critical").(bool); marked {
		out = append(out, ReasonHumanMarked)
	}
	in, err := g.q.EdgesTo(ctx, g.projectID, dec.ID)
	if err != nil {
		return nil, err
	}
	sections := map[string]bool{}
	covers := false
	for _, e := range in {
		if e.Type != store.EdgeCovers {
			continue
		}
		covers = true
		anchor := bodyString(g.byID[e.FromID], "anchor")
		for id, text := range g.texts {
			if len(text.Find(anchor)) > 0 {
				sections[id] = true
			}
		}
	}
	if covers {
		out = append(out, ReasonCoversFigure)
	}
	outEdges, err := g.q.EdgesFrom(ctx, g.projectID, dec.ID)
	if err != nil {
		return nil, err
	}
	for _, e := range outEdges {
		if e.Type == store.EdgeUsedIn {
			if _, ok := g.sections[e.ToID]; ok {
				sections[e.ToID] = true
			}
		}
	}
	for id := range sections {
		role, _ := bodyValue(g.sections[id], "role").(map[string]any)
		if v, _ := role["value"].(string); v == "conclusion" || v == "recommendation" {
			out = append(out, ReasonConclusionSection)
			break
		}
	}
	if len(sections) >= 2 {
		out = append(out, ReasonTwoOrMoreSections)
	}
	return out, nil
}

func bodyValue(obj store.Object, key string) any {
	var b map[string]any
	_ = json.Unmarshal(obj.Body, &b)
	return b[key]
}
