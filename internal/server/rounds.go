package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/LaurenteEber/tupaqnin/internal/cycle"
)

// RoundRunner reads a round's card and starts a round from your act
// (backlog-r3 3.2, B-72). *cycle.Runner satisfies it.
type RoundRunner interface {
	Round(ctx context.Context, projectID, label string) (cycle.RoundView, error)
	StartRound(projectID string) (label, turnID string, err error)
	// RepeatRound and LeaveRoundOpen are «Repetir con otra pregunta» and
	// «dejarlo por aclarar» (backlog-r3 3.4, flujos §2).
	RepeatRound(projectID, label, question string) (newLabel, turnID string, cost float64, err error)
	LeaveRoundOpen(ctx context.Context, projectID, label string) (string, error)
	// AuditSection and DiscardFlaw are «Auditar sección» and «No aplica»
	// (backlog-r3 3.6, B-67).
	AuditSection(projectID, section string) (label, turnID string, err error)
	DiscardFlaw(ctx context.Context, projectID, label string, n int) error
}

// RepeatRoundRequest is «otra pregunta» for POST /rounds/{label}/repeat.
type RepeatRoundRequest struct {
	Question string `json:"question"`
}

// RepeatRoundResponse is the round your click created, the turn to
// follow it on and its estimated cost.
type RepeatRoundResponse struct {
	Label            string  `json:"label"`
	TurnID           string  `json:"turn_id"`
	EstimatedCostUSD float64 `json:"estimated_cost_usd"`
}

// LeaveRoundOpenResponse is the scope «por aclarar» it opened, by label.
type LeaveRoundOpenResponse struct {
	OpenQuestion string `json:"open_question"`
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
	// The digest (backlog-r3 3.4, B-63), as Go computed it.
	DigestStatus string          `json:"digest_status,omitempty"`
	ReducedDoubt string          `json:"reduced_doubt,omitempty"`
	Digest       *RoundDigest    `json:"digest,omitempty"`
	Discarded    []RoundFinding  `json:"discarded"`
	Unverified   []RoundFinding  `json:"unverified"`
	Coverage     []RoundCoverage `json:"coverage"`
	// Unused are the findings the digest does not use, and
	// ReaderContradictions every reader contradiction, always shown
	// (flujos §2).
	Unused               []RoundFinding `json:"unused"`
	ReaderContradictions []RoundFinding `json:"reader_contradictions"`
	Repeat               *RoundRepeat   `json:"repeat,omitempty"`
	ScopeQuestion        string         `json:"scope_question,omitempty"`
	// A judge round (backlog-r3 3.6, B-67), as Go computed it: the section,
	// why it entered (risk · audit), its risks for the triage line (K-3),
	// the judge's status, the flaws, and whether their batch started.
	Section      *SectionRef `json:"section,omitempty"`
	Reason       string      `json:"reason,omitempty"`
	Risks        []string    `json:"risks,omitempty"`
	JudgeStatus  string      `json:"judge_status,omitempty"`
	Flaws        []RoundFlaw `json:"flaws,omitempty"`
	LateFlaws    []RoundFlaw `json:"late_flaws,omitempty"`
	BatchStarted bool        `json:"batch_started,omitempty"`
}

// RoundFlaw is one of the judge's flaws (cycle.FlawView): its number, the
// passage literal, the risk, the scenario, your «No aplica», the recheck's
// verdict and the «por aclarar» it opened.
type RoundFlaw struct {
	N              int    `json:"n"`
	Passage        string `json:"passage"`
	Risk           string `json:"risk"`
	Scenario       string `json:"scenario"`
	Discarded      bool   `json:"discarded,omitempty"`
	Verdict        string `json:"verdict,omitempty"`
	RecheckPassage string `json:"recheck_passage,omitempty"`
	OpenQuestion   string `json:"open_question,omitempty"`
}

func roundFlaws(list []cycle.FlawView) []RoundFlaw {
	var out []RoundFlaw
	for _, f := range list {
		out = append(out, RoundFlaw{N: f.N, Passage: f.Passage, Risk: f.Risk, Scenario: f.Scenario, Discarded: f.Discarded,
			Verdict: f.Verdict, RecheckPassage: f.RecheckPassage, OpenQuestion: f.OpenQuestion})
	}
	return out
}

// RoundFinding is one finding as the card shows it (cycle.FindingView):
// its number in the digest (0 if none), reader, kind, passage and note
// as written, its source by label with its title and URL, and its
// verification with why.
type RoundFinding struct {
	N            int    `json:"n"`
	Part         int    `json:"part"`
	Finding      int    `json:"finding"`
	Kind         string `json:"kind"`
	Passage      string `json:"passage"`
	Note         string `json:"note,omitempty"`
	Source       string `json:"source,omitempty"`
	SourceTitle  string `json:"source_title,omitempty"`
	SourceURL    string `json:"source_url,omitempty"`
	Hit          string `json:"hit,omitempty"`
	Reference    string `json:"reference,omitempty"`
	Verification string `json:"verification"`
	Reason       string `json:"reason,omitempty"`
	Access       string `json:"access,omitempty"`
	OpenQuestion string `json:"open_question,omitempty"`
}

// RoundDigest is the principal's part: first the answer, the outline
// with its findings, the contrast and the contradictions.
type RoundDigest struct {
	Answer              string               `json:"answer"`
	Outline             []RoundDigestLine    `json:"outline"`
	ExpectationContrast string               `json:"expectation_contrast,omitempty"`
	Contradictions      []RoundContradiction `json:"contradictions"`
}

// RoundDigestLine is one outline line and the findings it uses.
type RoundDigestLine struct {
	Text     string         `json:"text"`
	Findings []RoundFinding `json:"findings"`
}

// RoundContradiction is a contradiction shown, never resolved, and the
// «por aclarar» it opened, if any.
type RoundContradiction struct {
	A            RoundFinding `json:"a"`
	B            RoundFinding `json:"b"`
	About        string       `json:"about"`
	SameSource   bool         `json:"same_source"`
	OpenQuestion string       `json:"open_question,omitempty"`
}

// RoundCoverage is one reader's «Sin revisar» (A-5): how many sources
// and results it was given and which it left unread, and its gaps.
type RoundCoverage struct {
	Part          int      `json:"part"`
	Status        string   `json:"status"`
	Reason        string   `json:"reason,omitempty"`
	Sources       int      `json:"sources"`
	SourcesUnread []string `json:"sources_unread"`
	Hits          int      `json:"hits"`
	HitsUnread    []string `json:"hits_unread"`
	Gaps          []string `json:"gaps"`
}

// RoundRepeat is «Repetir con otra pregunta» with its estimated cost.
type RoundRepeat struct {
	EstimatedCostUSD float64 `json:"estimated_cost_usd"`
}

func roundFinding(f cycle.FindingView) RoundFinding {
	return RoundFinding{N: f.N, Part: f.Part, Finding: f.Finding, Kind: f.Kind, Passage: f.Passage, Note: f.Note,
		Source: f.Source, SourceTitle: f.SourceTitle, SourceURL: f.SourceURL, Hit: f.Hit, Reference: f.Reference,
		Verification: f.Verification, Reason: f.Reason, Access: f.Access, OpenQuestion: f.OpenQuestion}
}

func roundFindings(list []cycle.FindingView) []RoundFinding {
	out := []RoundFinding{}
	for _, f := range list {
		out = append(out, roundFinding(f))
	}
	return out
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
	c.DigestStatus, c.ReducedDoubt, c.ScopeQuestion = v.DigestStatus, v.ReducedDoubt, v.ScopeQuestion
	c.Discarded, c.Unverified, c.Coverage = roundFindings(v.Discarded), roundFindings(v.Unverified), []RoundCoverage{}
	c.Unused, c.ReaderContradictions = roundFindings(v.Unused), roundFindings(v.ReaderContradictions)
	for _, cv := range v.Coverage {
		c.Coverage = append(c.Coverage, RoundCoverage{Part: cv.Part, Status: cv.Status, Reason: cv.Reason,
			Sources: cv.Sources, SourcesUnread: cv.SourcesUnread, Hits: cv.Hits, HitsUnread: cv.HitsUnread, Gaps: cv.Gaps})
	}
	if v.Section != "" {
		c.Section = &SectionRef{Label: v.Section, Title: v.SectionTitle}
	}
	c.Reason, c.Risks, c.JudgeStatus, c.BatchStarted = v.Reason, v.Risks, v.JudgeStatus, v.BatchStarted
	c.Flaws, c.LateFlaws = roundFlaws(v.Flaws), roundFlaws(v.LateFlaws)
	if v.Repeat != nil {
		c.Repeat = &RoundRepeat{EstimatedCostUSD: v.Repeat.EstimatedCostUSD}
	}
	if d := v.Digest; d != nil {
		rd := &RoundDigest{Answer: d.Answer, ExpectationContrast: d.ExpectationContrast,
			Outline: []RoundDigestLine{}, Contradictions: []RoundContradiction{}}
		for _, l := range d.Outline {
			rd.Outline = append(rd.Outline, RoundDigestLine{Text: l.Text, Findings: roundFindings(l.Findings)})
		}
		for _, x := range d.Contradictions {
			rd.Contradictions = append(rd.Contradictions, RoundContradiction{A: roundFinding(x.A), B: roundFinding(x.B),
				About: x.About, SameSource: x.SameSource, OpenQuestion: x.OpenQuestion})
		}
		c.Digest = rd
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

// POST /api/projects/{projectID}/rounds/{label}/repeat with {question}:
// «Repetir con otra pregunta» (backlog-r3 3.4, flujos §2): your click
// creates the new round with your question and runs it → 202 {label,
// turn_id, estimated_cost_usd}. 422 question_required (empty or the
// same question), 409 round_not_repeatable (its doubt was reduced, or
// it is not closed), 409 turn_running.
// [EL DISEÑO NO LO DICE]: the body and the status codes.
func (s *Server) repeatRound(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectID")
	if !s.requireProject(w, r, projectID) {
		return
	}
	rr, ok := s.starter.(RoundRunner)
	if !ok {
		writeError(w, http.StatusNotFound, "not_found")
		return
	}
	var req RepeatRoundRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body")
		return
	}
	label, turnID, cost, err := rr.RepeatRound(projectID, r.PathValue("label"), req.Question)
	switch {
	case errors.Is(err, cycle.ErrNotRound):
		writeError(w, http.StatusNotFound, "not_found")
	case errors.Is(err, cycle.ErrQuestionRequired):
		writeError(w, http.StatusUnprocessableEntity, "question_required")
	case errors.Is(err, cycle.ErrRoundNotRepeatable):
		writeError(w, http.StatusConflict, "round_not_repeatable")
	case errors.Is(err, cycle.ErrTurnRunning):
		writeError(w, http.StatusConflict, "turn_running")
	case err != nil:
		writeError(w, http.StatusInternalServerError, "internal")
	default:
		writeJSON(w, http.StatusAccepted, RepeatRoundResponse{Label: label, TurnID: turnID, EstimatedCostUSD: cost})
	}
}

// POST /api/projects/{projectID}/rounds/{label}/leave-open: «dejarlo por
// aclarar» (flujos §2) opens a scope «por aclarar» → 201 {open_question}.
// 409 round_not_repeatable (its doubt was reduced, or it is not
// closed), 409 already_open (done before).
// [EL DISEÑO NO LO DICE]: the route and the status codes.
func (s *Server) leaveRoundOpen(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectID")
	if !s.requireProject(w, r, projectID) {
		return
	}
	rr, ok := s.starter.(RoundRunner)
	if !ok {
		writeError(w, http.StatusNotFound, "not_found")
		return
	}
	label, err := rr.LeaveRoundOpen(r.Context(), projectID, r.PathValue("label"))
	switch {
	case errors.Is(err, cycle.ErrNotRound):
		writeError(w, http.StatusNotFound, "not_found")
	case errors.Is(err, cycle.ErrRoundNotRepeatable):
		writeError(w, http.StatusConflict, "round_not_repeatable")
	case errors.Is(err, cycle.ErrScopeAlreadyOpen):
		writeError(w, http.StatusConflict, "already_open")
	case err != nil:
		writeError(w, http.StatusInternalServerError, "internal")
	default:
		writeJSON(w, http.StatusCreated, LeaveRoundOpenResponse{OpenQuestion: label})
	}
}

// POST /api/projects/{projectID}/sections/{label}/audit: «Auditar sección»
// (B-67 b): your act calls the judge always, and it runs in the background
// in the turn's slot → 202 {label, turn_id}. 404 not_found (not a section),
// 422 section_empty, 409 turn_running, 409 round_open (its judge round has
// not ended).
// [EL DISEÑO NO LO DICE]: the status codes.
func (s *Server) auditSection(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectID")
	if !s.requireProject(w, r, projectID) {
		return
	}
	rr, ok := s.starter.(RoundRunner)
	if !ok {
		writeError(w, http.StatusNotFound, "not_found")
		return
	}
	label, turnID, err := rr.AuditSection(projectID, r.PathValue("label"))
	switch {
	case errors.Is(err, cycle.ErrNotSection):
		writeError(w, http.StatusNotFound, "not_found")
	case errors.Is(err, cycle.ErrSectionEmpty):
		writeError(w, http.StatusUnprocessableEntity, "section_empty")
	case errors.Is(err, cycle.ErrTurnRunning):
		writeError(w, http.StatusConflict, "turn_running")
	case errors.Is(err, cycle.ErrJudgeRoundOpen):
		writeError(w, http.StatusConflict, "round_open")
	case err != nil:
		writeError(w, http.StatusInternalServerError, "internal")
	default:
		writeJSON(w, http.StatusAccepted, StartRoundResponse{Label: label, TurnID: turnID})
	}
}

// POST /api/projects/{projectID}/rounds/{label}/flaws/{n}/discard: «No
// aplica» (B-67, K-4) writes flaw_discarded → 204. 404 not_found (not a
// judge round, or no such flaw), 409 batch_started (the batch started or
// the round closed), 409 already_discarded.
// [EL DISEÑO NO LO DICE]: the status codes.
func (s *Server) discardFlaw(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectID")
	if !s.requireProject(w, r, projectID) {
		return
	}
	rr, ok := s.starter.(RoundRunner)
	if !ok {
		writeError(w, http.StatusNotFound, "not_found")
		return
	}
	n, err := strconv.Atoi(r.PathValue("n"))
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found")
		return
	}
	err = rr.DiscardFlaw(r.Context(), projectID, r.PathValue("label"), n)
	switch {
	case errors.Is(err, cycle.ErrNotRound), errors.Is(err, cycle.ErrFlawNotFound):
		writeError(w, http.StatusNotFound, "not_found")
	case errors.Is(err, cycle.ErrFlawTooLate):
		writeError(w, http.StatusConflict, "batch_started")
	case errors.Is(err, cycle.ErrFlawDiscardedTwice):
		writeError(w, http.StatusConflict, "already_discarded")
	case err != nil:
		writeError(w, http.StatusInternalServerError, "internal")
	default:
		w.WriteHeader(http.StatusNoContent)
	}
}
