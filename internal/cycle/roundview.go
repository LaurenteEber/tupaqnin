package cycle

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/LaurenteEber/tupaqnin/internal/graph"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// ErrNotRound is a label that is not a round of the project.
var ErrNotRound = errors.New("cycle: not a round")

// RoundView is the round card (B-64, pantallas «Dónde va cada vista de
// la rebanada 3»): the question, what each reader searches and where,
// the expectation in literal, the estimated cost and what was spent,
// and each reader's progress. Go computes it; the page shows it. Labels
// only, never ids.
type RoundView struct {
	Label            string          `json:"label"`
	State            string          `json:"state"`
	Kind             string          `json:"kind"`
	Question         string          `json:"question"`
	Doubt            string          `json:"doubt"`
	DelegationReason string          `json:"delegation_reason"`
	Scope            string          `json:"scope"`
	Expectation      string          `json:"expectation"`
	Parts            []RoundPartView `json:"parts"`
	EstimatedCostUSD float64         `json:"estimated_cost_usd"`
	SpentUSD         float64         `json:"spent_usd"`
	// Running is true while its readers run.
	Running bool `json:"running"`
	// Stopped says why no reader was launched (project_cap).
	Stopped string      `json:"stopped,omitempty"`
	Hits    []HitRecord `json:"hits"`
}

// RoundPartView is one reader of the card.
type RoundPartView struct {
	Part     int                   `json:"part"`
	Question string                `json:"question"`
	Where    []string              `json:"where"`
	Sources  []RoundPartSourceView `json:"sources"`
	Queries  []string              `json:"queries"`
	// Status is pending · running · returned · empty; Reason says why
	// empty (invalid_output, round_cap, project_cap, provider_error).
	Status    string `json:"status"`
	Reason    string `json:"reason,omitempty"`
	ToolCalls int    `json:"tool_calls"`
	Findings  int    `json:"findings"`
}

// RoundPartSourceView is a project source of a part, by label.
type RoundPartSourceView struct {
	Label   string `json:"label"`
	WhatFor string `json:"what_for"`
}

// Round reads a round's card: its stored proposal and readers' ends,
// the live progress of readers running now, and what its turn spent
// (B-65: the sum of model_call.cost of its turn_id).
func (r *Runner) Round(ctx context.Context, projectID, label string) (RoundView, error) {
	obj, err := r.db.GetObject(ctx, projectID, label)
	if errors.Is(err, store.ErrNotFound) || (err == nil && obj.Type != store.ObjectRound) {
		return RoundView{}, ErrNotRound
	}
	if err != nil {
		return RoundView{}, err
	}
	var body RoundBody
	if err := json.Unmarshal(obj.Body, &body); err != nil {
		return RoundView{}, fmt.Errorf("cycle: round body: %w", err)
	}
	labeler, err := graph.NewLabeler(ctx, r.db, projectID)
	if err != nil {
		return RoundView{}, err
	}
	spent, err := r.db.TurnSpend(ctx, projectID, body.TurnID)
	if err != nil {
		return RoundView{}, err
	}
	v := RoundView{Label: obj.Label, State: obj.State, Kind: body.Kind, Question: body.Question,
		Doubt: body.Doubt, DelegationReason: body.DelegationReason, Scope: body.Scope,
		Expectation: body.Expectation, EstimatedCostUSD: body.EstimatedCostUSD, SpentUSD: spent,
		Stopped: body.Stopped, Hits: body.Hits, Parts: []RoundPartView{}}
	if v.Hits == nil {
		v.Hits = []HitRecord{}
	}
	r.mu.Lock()
	live := r.rounds[obj.ID]
	r.mu.Unlock()
	for i, p := range body.Parts {
		pv := RoundPartView{Part: i + 1, Question: p.Question, Where: p.Where, Queries: p.Queries,
			Sources: []RoundPartSourceView{}, Status: ReaderPending}
		for _, s := range p.Sources {
			l, err := labeler.ToLabel(s.SourceID)
			if err != nil {
				continue
			}
			pv.Sources = append(pv.Sources, RoundPartSourceView{Label: l, WhatFor: s.WhatFor})
		}
		if i < len(body.Readers) {
			rd := body.Readers[i]
			pv.Status, pv.Reason, pv.ToolCalls = rd.Status, rd.Reason, rd.ToolCalls
			var out struct {
				Findings []json.RawMessage `json:"findings"`
			}
			_ = json.Unmarshal(rd.Output, &out)
			pv.Findings = len(out.Findings)
		} else if live != nil {
			live.mu.Lock()
			pv.Status, pv.Reason, pv.ToolCalls = live.status[i], live.reasons[i], live.toolCalls[i]
			live.mu.Unlock()
		}
		v.Parts = append(v.Parts, pv)
	}
	v.Running = live != nil
	return v, nil
}

// StartRound creates a round from your act (B-72, «Buscar evidencia» on
// the understanding card) and runs it in the background, returning its
// label and turn id at once. The app sets the question (your request),
// the parts (openalex and, if the project has material, project; never
// arxiv: the app does not judge the topic) and the reason
// (independent_parts with more than one part, too_much_to_read with
// one). It does not go through the veto of B-57. It takes the project's
// turn slot like a turn (P-20): ErrTurnRunning while one runs.
// [EL DISEÑO NO LO DICE]: the doubt and the scope of such a round; both
// empty, the request carries the question.
func (r *Runner) StartRound(projectID string) (label, turnID string, err error) {
	if !r.tryAcquireTurn(projectID) {
		return "", "", ErrTurnRunning
	}
	ctx := context.Background()
	request, err := r.requestText(ctx, projectID)
	if err == nil && request == "" {
		err = errors.New("cycle: the project has no request")
	}
	var sources []store.Object
	if err == nil {
		sources, err = r.db.ListObjects(ctx, projectID, store.ObjectSource)
	}
	if err == nil {
		turnID, err = newTurnID()
	}
	var roundID string
	if err == nil {
		parts := []RoundPart{{Question: request, Where: []string{"openalex"},
			Sources: []RoundPartSource{}, Queries: []string{}}}
		if len(sources) > 0 {
			project := RoundPart{Question: request, Where: []string{"project"}, Queries: []string{}}
			for _, s := range sources {
				project.Sources = append(project.Sources, RoundPartSource{SourceID: s.ID})
			}
			parts = append(parts, project)
		}
		reason := "too_much_to_read"
		if len(parts) > 1 {
			reason = "independent_parts"
		}
		err = r.db.Write(ctx, projectID, func(tx *store.Tx) error {
			expectation, err := premiseText(tx)
			if err != nil {
				return err
			}
			body := RoundBody{Kind: "research", Question: request, DelegationReason: reason, Parts: parts,
				References: []namedReference{}, Expectation: expectation,
				EstimatedCostUSD: EstimatedRoundCost(len(parts)), TurnID: turnID}
			obj, err := tx.CreateObject(store.ObjectRound, "open", body, store.EventInput{
				Type: store.EventStateChanged, Actor: "human", TurnID: turnID,
				Data: map[string]any{"to": "open", "kind": "research"},
			})
			roundID, label = obj.ID, obj.Label
			return err
		})
	}
	if err != nil {
		r.releaseTurn(projectID)
		return "", "", err
	}
	if r.pub != nil {
		r.pub.BeginTurn(projectID, turnID)
	}
	r.roundTurns.Add(1)
	go func() {
		defer r.roundTurns.Done()
		defer r.endTurn(context.Background(), projectID, turnID)
		if err := r.runRound(r.ctx, projectID, roundID, turnID); err != nil {
			if r.Logger != nil {
				r.Logger.Error("round failed", "error", err.Error())
			}
			r.fail(projectID, turnID, "turn_failed")
			return
		}
		if r.pub != nil {
			if raw, err := json.Marshal(map[string]any{"status": StatusAccepted, "round": label}); err == nil {
				r.pub.PublishStep(projectID, turnID, raw)
			}
		}
	}()
	return label, turnID, nil
}
