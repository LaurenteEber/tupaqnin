package cycle

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

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
	// The digest (backlog-r3 3.4): its status, reduced_doubt, the
	// principal's part, and the app's (B-63): «Descartados (n)», the
	// unverified, «Sin revisar» by reader; «Repetir con otra pregunta»
	// with its cost when the doubt was not reduced, and the scope «por
	// aclarar» you opened, by label.
	DigestStatus string         `json:"digest_status,omitempty"`
	ReducedDoubt string         `json:"reduced_doubt,omitempty"`
	Digest       *DigestView    `json:"digest,omitempty"`
	Discarded    []FindingView  `json:"discarded"`
	Unverified   []FindingView  `json:"unverified"`
	Coverage     []CoverageView `json:"coverage"`
	// Unused are the findings the principal received and its digest
	// does not use (all of them when there is no digest), and
	// ReaderContradictions every reader finding of kind contradiction,
	// whatever the principal returned (flujos §2: never resolved in
	// silence).
	Unused               []FindingView `json:"unused"`
	ReaderContradictions []FindingView `json:"reader_contradictions"`
	Repeat               *RepeatOffer  `json:"repeat,omitempty"`
	ScopeQuestion        string        `json:"scope_question,omitempty"`
	// A judge round (backlog-r3 3.6): the section, why (risk · audit), its
	// risks, the judge's status, the flaws and whether the batch started.
	Section      string     `json:"section,omitempty"`
	SectionTitle string     `json:"section_title,omitempty"`
	Reason       string     `json:"reason,omitempty"`
	Risks        []string   `json:"risks,omitempty"`
	JudgeStatus  string     `json:"judge_status,omitempty"`
	Flaws        []FlawView `json:"flaws,omitempty"`
	LateFlaws    []FlawView `json:"late_flaws,omitempty"`
	BatchStarted bool       `json:"batch_started,omitempty"`
	// Changes and Proposed are what the correction batch did (B-99): each change
	// with its flaw, and how many sections it left as proposals.
	Changes  []graph.JudgeChange `json:"changes,omitempty"`
	Proposed int                 `json:"proposed,omitempty"`
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
	if err := r.digestView(ctx, projectID, body, labeler, &v); err != nil {
		return RoundView{}, err
	}
	if body.Kind == "judge" {
		if err := r.judgeView(ctx, projectID, body, labeler, &v); err != nil {
			return RoundView{}, err
		}
	}
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
	return r.startRoundWith(projectID, nil, func(ctx context.Context) (RoundBody, error) {
		request, err := r.requestText(ctx, projectID)
		if err == nil && request == "" {
			err = ErrNoRequest
		}
		if err != nil {
			return RoundBody{}, err
		}
		return r.yourRoundBody(ctx, projectID, request)
	})
}

// yourRoundBody is the body of a round you start (B-72), on question.
func (r *Runner) yourRoundBody(ctx context.Context, projectID, question string) (RoundBody, error) {
	all, err := r.db.ListObjects(ctx, projectID, store.ObjectSource)
	if err != nil {
		return RoundBody{}, err
	}
	var sources []store.Object
	for _, s := range all {
		// [EL DISEÑO NO LO DICE]: «si el proyecto tiene material»
		// (B-72) counts your material, not the external sources
		// earlier rounds brought in (B-61).
		if bodyStr(bodyOf(s), "kind") != graph.SourceKindWeb {
			sources = append(sources, s)
		}
	}
	parts := []RoundPart{{Question: question, Where: []string{"openalex"},
		Sources: []RoundPartSource{}, Queries: []string{}}}
	if len(sources) > 0 {
		project := RoundPart{Question: question, Where: []string{"project"}, Queries: []string{}}
		for _, s := range sources {
			project.Sources = append(project.Sources, RoundPartSource{SourceID: s.ID})
		}
		parts = append(parts, project)
	}
	reason := "too_much_to_read"
	if len(parts) > 1 {
		reason = "independent_parts"
	}
	return RoundBody{Kind: "research", Question: question, DelegationReason: reason, Parts: parts,
		References: []namedReference{}, EstimatedCostUSD: EstimatedRoundCost(len(parts))}, nil
}

// ErrCandidateNotFound is a number that is no candidate of the project.
var ErrCandidateNotFound = errors.New("cycle: not a candidate of the project")

// StartCandidateRound is «Buscarla» (B-69, B-72): like StartRound, with the
// candidates' text as the question. references are their numbers in the
// project (graph.Candidates). Each one with a title or a DOI goes through
// Lookup without a reader (B-59 d). ErrCandidateNotFound when one is no
// candidate.
// [EL DISEÑO NO LO DICE]: a candidate with neither title nor DOI gives
// Lookup nothing to look for, and Lookup would read «fuente inexistente»
// without looking: it stays out of the references and the reader searches
// its text. With several references, the question joins their texts.
func (r *Runner) StartCandidateRound(projectID string, references []int) (label, turnID string, err error) {
	extra := map[string]any{"via": "candidate", "references": references}
	return r.startRoundWith(projectID, extra, func(ctx context.Context) (RoundBody, error) {
		events, err := r.db.EventsSince(ctx, projectID, 0)
		if err != nil {
			return RoundBody{}, err
		}
		candidates := graph.Candidates(events)
		texts := make([]string, 0, len(references))
		refs := []namedReference{}
		for _, n := range references {
			if n < 1 || n > len(candidates) {
				return RoundBody{}, ErrCandidateNotFound
			}
			c := candidates[n-1]
			texts = append(texts, c.Text)
			if c.Title != "" || c.DOI != "" {
				refs = append(refs, candidateReference(c))
			}
		}
		body, err := r.yourRoundBody(ctx, projectID, strings.Join(texts, " · "))
		if err != nil {
			return RoundBody{}, err
		}
		body.References, body.FromCandidate = refs, true
		return body, nil
	})
}

func candidateReference(c graph.Candidate) namedReference {
	opt := func(s string) *string {
		if s == "" {
			return nil
		}
		return &s
	}
	ref := namedReference{Text: c.Text, Title: opt(c.Title), Authors: opt(c.Authors), DOI: opt(c.DOI)}
	if c.Year != 0 {
		ref.Year = &c.Year
	}
	return ref
}

// startRoundWith creates a round from your act, open, with the body
// build returns once the project's turn slot is taken (P-20:
// ErrTurnRunning while one runs; its expectation and turn are set
// here), and runs it in the background in that slot. extra goes in the
// creation event's data.
func (r *Runner) startRoundWith(projectID string, extra map[string]any, build func(context.Context) (RoundBody, error)) (label, turnID string, err error) {
	if !r.tryAcquireTurn(projectID) {
		return "", "", ErrTurnRunning
	}
	ctx := context.Background()
	body, err := build(ctx)
	if err == nil {
		turnID, err = newTurnID()
	}
	var roundID string
	if err == nil {
		err = r.db.Write(ctx, projectID, func(tx *store.Tx) error {
			expectation, err := premiseText(tx)
			if err != nil {
				return err
			}
			body.Expectation, body.TurnID = expectation, turnID
			data := map[string]any{"to": "open", "kind": "research"}
			for k, v := range extra {
				data[k] = v
			}
			obj, err := tx.CreateObject(store.ObjectRound, "open", body, store.EventInput{
				Type: store.EventStateChanged, Actor: "human", TurnID: turnID, Data: data,
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
