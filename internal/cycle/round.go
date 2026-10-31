package cycle

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// A round of readers (backlog-r3 3.2; fichas §2 «Investigar» and §7;
// flujos §2; estados §7): the principal asks for it with research_request
// in a discover step (B-57), the app creates it open and launches it on
// its own (B-64), and up to 3 readers search and read in parallel.

// Round limits (B-65, [inicial]).
const (
	// MaxRoundParts is one reader per part, at most 3 (espec. §3).
	MaxRoundParts = 3
	// MaxReaderToolCalls is the reader's tool cap: every call counts,
	// the one refused by the cap too, and a ninth is never executed
	// (espec. §3, P-97).
	MaxReaderToolCalls = 8
	// RoundCapUSD is what one round may spend, readers and digest
	// (B-65, [inicial]).
	RoundCapUSD = 0.60
	// EstimatedReaderCostUSD is the mean reader cost the evidence
	// micro-test measured (3.0: 0,035 USD with Spark 1.3 low), the base
	// of a round's estimated cost (B-65 [inferencia]).
	EstimatedReaderCostUSD = 0.035
	// DefaultProjectCapUSD is the project cap when the runner is given
	// none (espec. §6, B3-20: 3 USD).
	DefaultProjectCapUSD = 3.0
)

// The delegation reasons a round accepts (fichas §8.3; «riesgo o
// auditoría» is the judge's, 3.6).
var roundReasons = map[string]bool{"too_much_to_read": true, "independent_parts": true}

// The places a reader searches (B-55, P-97: no web).
var roundWhere = map[string]bool{"project": true, "openalex": true, "arxiv": true}

type researchPart struct {
	Question string `json:"question"`
	Sources  []struct {
		SourceID string `json:"source_id"`
		WhatFor  string `json:"what_for"`
	} `json:"sources"`
	Where   []string `json:"where"`
	Queries []string `json:"queries"`
}

type researchRequest struct {
	Question         string         `json:"question"`
	Doubt            string         `json:"doubt"`
	DelegationReason string         `json:"delegation_reason"`
	Scope            string         `json:"scope"`
	Parts            []researchPart `json:"parts"`
	References       []int          `json:"references"`
}

// namedReference is one entry of the principal's named_references
// (B-69): what it names without a citation.
type namedReference struct {
	Text    string  `json:"text"`
	Title   *string `json:"title"`
	Authors *string `json:"authors"`
	Year    *int    `json:"year"`
	DOI     *string `json:"doi"`
}

// RoundPart is one part of a stored round: one reader. Sources hold
// internal ids; they become labels only in what reaches a model or the
// page.
type RoundPart struct {
	Question string            `json:"question"`
	Sources  []RoundPartSource `json:"sources"`
	Where    []string          `json:"where"`
	Queries  []string          `json:"queries"`
}

// RoundPartSource is a project source a part reads, with what for.
type RoundPartSource struct {
	SourceID string `json:"source_id"`
	WhatFor  string `json:"what_for"`
}

// RoundBody is the body of a round object (espec. §1.1 `round`, B-57,
// B-65). Readers and Hits are written once, when the readers end.
type RoundBody struct {
	Kind             string           `json:"kind"`
	Question         string           `json:"question"`
	Doubt            string           `json:"doubt"`
	DelegationReason string           `json:"delegation_reason"`
	Scope            string           `json:"scope"`
	Parts            []RoundPart      `json:"parts"`
	References       []namedReference `json:"references"`
	// Expectation is the premise of the request, literal, or "" for
	// «sin expectativa» (K-5, B-64).
	Expectation      string  `json:"expectation"`
	EstimatedCostUSD float64 `json:"estimated_cost_usd"`
	// TurnID is the round's one turn id: its readers' calls (and 3.4's
	// digest) carry it, and its spend is theirs (B-65).
	TurnID string `json:"turn_id"`
	// RequestedIn is the turn of the step that asked for it.
	RequestedIn string `json:"requested_in"`
	// Readers and Hits are what the readers returned and the hit
	// registry, written in one write when they end (backlog-r3 3.2).
	Readers []ReaderResult `json:"readers,omitempty"`
	Hits    []HitRecord    `json:"hits,omitempty"`
	// Findings is each finding's verification and reason (backlog-r3
	// 3.3, B-59): the app's own (the round's references) when the round
	// starts, the readers' when they end.
	Findings []FindingCheck `json:"findings,omitempty"`
	// Stopped says why the round launched no reader (project_cap).
	Stopped string `json:"stopped,omitempty"`
}

// ReaderResult is one reader's end: its output (salida-lector, as the
// model wrote it, with labels) or «vacío» with why.
type ReaderResult struct {
	Part      int             `json:"part"`
	Status    string          `json:"status"` // returned · empty
	Reason    string          `json:"reason,omitempty"`
	ToolCalls int             `json:"tool_calls"`
	Output    json.RawMessage `json:"output,omitempty"`
}

// Reader statuses and the reasons a reader is empty (flujos §2, B-65).
const (
	ReaderPending  = "pending"
	ReaderRunning  = "running"
	ReaderReturned = "returned"
	ReaderEmpty    = "empty"

	EmptyInvalidOutput = "invalid_output" // the fixed shape failed twice
	EmptyRoundCap      = "round_cap"      // the round reached RoundCapUSD
	EmptyProjectCap    = "project_cap"    // the project reached its cap
	EmptyProviderError = "provider_error" // the model call failed
)

// HitRecord is one search result the app numbered in the round (B-58):
// URL and DOI come from the API, never from a model.
type HitRecord struct {
	Label    string   `json:"label"`
	Part     int      `json:"part"`
	Service  string   `json:"service"`
	ID       string   `json:"id"`
	Title    string   `json:"title"`
	Authors  []string `json:"authors"`
	Year     int      `json:"year"`
	DOI      string   `json:"doi"`
	URL      string   `json:"url"`
	OpenText bool     `json:"open_text"`
	// Abstract is the API's abstract, kept for the verification of
	// findings (B-59 b); the card does not show it.
	Abstract string `json:"abstract,omitempty"`
}

// rejectResearch leaves why a research request created no round in
// step_ended (B-57) and, as a notice, for the next step (B-14).
func rejectResearch(data map[string]any, message string) {
	data["research_request_rejected"] = message
	if na, ok := data["not_applied"].(map[string]any); ok {
		na["research_request"] = map[string]any{"message": message}
	}
}

const researchNotCreated = " The round was not created; the rest of your step was applied, and you work alone."

// applyResearchRequest validates the step's research_request (B-57:
// discover stretch, a reader's reason, 1 to 3 parts, sources that exist)
// and, if valid, creates the round open with its proposal and estimated
// cost, in the step's transaction. If not valid, the step applies
// without it and the reason stays in step_ended. It returns the new
// round's id, or "".
func (p *project) applyResearchRequest(tx *store.Tx, turnID string, out stepOutput, data map[string]any) (string, error) {
	raw := strings.TrimSpace(string(out.ResearchRequest))
	if raw == "" || raw == "null" {
		return "", nil
	}
	if stretch, _ := data["stretch"].(string); stretch != "discover" {
		rejectResearch(data, "research_request goes only in a step whose stretch is discover."+researchNotCreated)
		return "", nil
	}
	var req researchRequest
	if err := json.Unmarshal(out.ResearchRequest, &req); err != nil {
		rejectResearch(data, "research_request does not have its fixed shape."+researchNotCreated)
		return "", nil
	}
	if !roundReasons[req.DelegationReason] {
		rejectResearch(data, fmt.Sprintf("research_request: %q is not a reason to delegate to readers; "+
			"it is too_much_to_read or independent_parts.", req.DelegationReason)+researchNotCreated)
		return "", nil
	}
	if strings.TrimSpace(req.Question) == "" {
		rejectResearch(data, "research_request has no question."+researchNotCreated)
		return "", nil
	}
	if len(req.Parts) < 1 || len(req.Parts) > MaxRoundParts {
		rejectResearch(data, fmt.Sprintf("research_request has %d parts; a round has 1 to %d, one per reader.",
			len(req.Parts), MaxRoundParts)+researchNotCreated)
		return "", nil
	}
	var named []namedReference
	_ = json.Unmarshal(out.NamedReferences, &named)
	parts := make([]RoundPart, 0, len(req.Parts))
	for i, part := range req.Parts {
		if len(part.Where) == 0 {
			rejectResearch(data, fmt.Sprintf("research_request: part %d says nowhere to search.", i+1)+researchNotCreated)
			return "", nil
		}
		for _, w := range part.Where {
			if !roundWhere[w] {
				rejectResearch(data, fmt.Sprintf("research_request: part %d searches %q; where is project, openalex or arxiv.",
					i+1, w)+researchNotCreated)
				return "", nil
			}
		}
		rp := RoundPart{Question: part.Question, Where: part.Where, Queries: part.Queries,
			Sources: []RoundPartSource{}}
		if rp.Queries == nil {
			rp.Queries = []string{}
		}
		for _, s := range part.Sources {
			obj, ok := p.object(s.SourceID)
			if !ok || obj.Type != store.ObjectSource {
				rejectResearch(data, fmt.Sprintf("research_request: part %d names %s, which is not a source of the project.",
					i+1, s.SourceID)+researchNotCreated)
				return "", nil
			}
			rp.Sources = append(rp.Sources, RoundPartSource{SourceID: obj.ID, WhatFor: s.WhatFor})
		}
		parts = append(parts, rp)
	}
	refs := []namedReference{}
	for _, n := range req.References {
		if n < 1 || n > len(named) {
			rejectResearch(data, fmt.Sprintf("research_request: reference %d is not in named_references (1 to %d).",
				n, len(named))+researchNotCreated)
			return "", nil
		}
		refs = append(refs, named[n-1])
	}
	expectation, err := premiseText(tx)
	if err != nil {
		return "", err
	}
	roundTurn, err := newTurnID()
	if err != nil {
		return "", err
	}
	body := RoundBody{Kind: "research", Question: req.Question, Doubt: req.Doubt,
		DelegationReason: req.DelegationReason, Scope: req.Scope, Parts: parts, References: refs,
		Expectation: expectation, EstimatedCostUSD: EstimatedRoundCost(len(parts)),
		TurnID: roundTurn, RequestedIn: turnID}
	obj, err := tx.CreateObject(store.ObjectRound, "open", body, store.EventInput{
		Type: store.EventStateChanged, Actor: "agent", AgentRole: "principal", TurnID: turnID,
		Data: map[string]any{"to": "open", "kind": "research"},
	})
	if err != nil {
		return "", err
	}
	data["round"] = obj.Label
	return obj.ID, nil
}

// EstimatedRoundCost is a round's estimated cost before it runs (B-65
// [inferencia]): the mean reader cost 3.0 measured, per reader.
// [EL DISEÑO NO LO DICE]: the digest is not in the estimate until 3.4
// builds it.
func EstimatedRoundCost(parts int) float64 {
	return float64(parts) * EstimatedReaderCostUSD
}

// premiseText is the request's premise, literal: the round's expectation
// (K-5; B-64: «tu idea o la premisa del pedido»). "" when there is none.
func premiseText(tx *store.Tx) (string, error) {
	premises, err := tx.ListObjects(context.Background(), tx.ProjectID(), store.ObjectPremise)
	if err != nil {
		return "", err
	}
	for i := len(premises) - 1; i >= 0; i-- {
		if text := bodyStr(bodyOf(premises[i]), "text"); text != "" {
			return text, nil
		}
	}
	return "", nil
}
