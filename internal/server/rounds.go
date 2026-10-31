package server

import (
	"context"
	"errors"
	"net/http"

	"github.com/LaurenteEber/tupaqnin/internal/cycle"
)

// RoundRunner reads a round's card and starts a round from your act
// (backlog-r3 3.2, B-72). *cycle.Runner satisfies it.
type RoundRunner interface {
	Round(ctx context.Context, projectID, label string) (cycle.RoundView, error)
	StartRound(projectID string) (label, turnID string, err error)
}

// StartRoundResponse is the round POST /rounds created and the turn to
// follow it on (its stream).
type StartRoundResponse struct {
	Label  string `json:"label"`
	TurnID string `json:"turn_id"`
}

// RoundCard is the round card as the page reads it (B-64), copied from
// what Go computed (cycle.RoundView); labels only.
type RoundCard struct {
	Label            string          `json:"label"`
	State            string          `json:"state"`
	Kind             string          `json:"kind"`
	Question         string          `json:"question"`
	Doubt            string          `json:"doubt"`
	DelegationReason string          `json:"delegation_reason"`
	Scope            string          `json:"scope"`
	Expectation      string          `json:"expectation"`
	Parts            []RoundCardPart `json:"parts"`
	EstimatedCostUSD float64         `json:"estimated_cost_usd"`
	SpentUSD         float64         `json:"spent_usd"`
	Running          bool            `json:"running"`
	Stopped          string          `json:"stopped,omitempty"`
	Hits             []RoundCardHit  `json:"hits"`
}

// RoundCardPart is one reader: what it searches, where, and how it goes
// (pending · running · returned · empty, with why).
type RoundCardPart struct {
	Part      int               `json:"part"`
	Question  string            `json:"question"`
	Where     []string          `json:"where"`
	Sources   []RoundCardSource `json:"sources"`
	Queries   []string          `json:"queries"`
	Status    string            `json:"status"`
	Reason    string            `json:"reason,omitempty"`
	ToolCalls int               `json:"tool_calls"`
	Findings  int               `json:"findings"`
}

// RoundCardSource is a project source a reader reads, with what for.
type RoundCardSource struct {
	Label   string `json:"label"`
	WhatFor string `json:"what_for"`
}

// RoundCardHit is a search result numbered in the round, with the URL
// and DOI the API gave (B-58).
type RoundCardHit struct {
	Label    string   `json:"label"`
	Service  string   `json:"service"`
	Title    string   `json:"title"`
	Authors  []string `json:"authors"`
	Year     int      `json:"year"`
	DOI      string   `json:"doi"`
	URL      string   `json:"url"`
	OpenText bool     `json:"open_text"`
}

func roundCard(v cycle.RoundView) RoundCard {
	c := RoundCard{Label: v.Label, State: v.State, Kind: v.Kind, Question: v.Question, Doubt: v.Doubt,
		DelegationReason: v.DelegationReason, Scope: v.Scope, Expectation: v.Expectation,
		EstimatedCostUSD: v.EstimatedCostUSD, SpentUSD: v.SpentUSD, Running: v.Running, Stopped: v.Stopped,
		Parts: []RoundCardPart{}, Hits: []RoundCardHit{}}
	for _, p := range v.Parts {
		cp := RoundCardPart{Part: p.Part, Question: p.Question, Where: p.Where, Queries: p.Queries,
			Status: p.Status, Reason: p.Reason, ToolCalls: p.ToolCalls, Findings: p.Findings, Sources: []RoundCardSource{}}
		for _, s := range p.Sources {
			cp.Sources = append(cp.Sources, RoundCardSource{Label: s.Label, WhatFor: s.WhatFor})
		}
		c.Parts = append(c.Parts, cp)
	}
	for _, h := range v.Hits {
		c.Hits = append(c.Hits, RoundCardHit{Label: h.Label, Service: h.Service, Title: h.Title, Authors: h.Authors,
			Year: h.Year, DOI: h.DOI, URL: h.URL, OpenText: h.OpenText})
	}
	return c
}

// GET /api/projects/{projectID}/rounds/{label}: the round card (B-64):
// question, what each reader searches and where, the expectation, the
// estimated cost and what was spent, each reader's progress.
func (s *Server) getRound(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectID")
	if !s.requireProject(w, r, projectID) {
		return
	}
	rr, ok := s.starter.(RoundRunner)
	if !ok {
		writeError(w, http.StatusNotFound, "not_found")
		return
	}
	view, err := rr.Round(r.Context(), projectID, r.PathValue("label"))
	if errors.Is(err, cycle.ErrNotRound) {
		writeError(w, http.StatusNotFound, "not_found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}
	writeJSON(w, http.StatusOK, roundCard(view))
}

// POST /api/projects/{projectID}/rounds: «Buscar evidencia» (B-72)
// creates a round from your act and runs it in the background → 202
// {label, turn_id}. The app sets the question, the parts and the reason.
// It takes the project's turn slot like a turn: 409 turn_running while
// one runs.
// [EL DISEÑO NO LO DICE]: the body; {references} from a candidate (B-69)
// is 3.5's, so the body is not read here.
func (s *Server) postRound(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectID")
	if !s.requireProject(w, r, projectID) {
		return
	}
	rr, ok := s.starter.(RoundRunner)
	if !ok {
		writeError(w, http.StatusNotFound, "not_found")
		return
	}
	label, turnID, err := rr.StartRound(projectID)
	if errors.Is(err, cycle.ErrTurnRunning) {
		writeError(w, http.StatusConflict, "turn_running")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}
	writeJSON(w, http.StatusAccepted, StartRoundResponse{Label: label, TurnID: turnID})
}
