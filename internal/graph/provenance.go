package graph

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// Claim origins (glossary §11, R-1): what the principal declares for
// each new text. The app never takes the declaration for the origin: it
// computes Provenance.Origin from the graph (backlog-r3 3.5).
const (
	OriginMaterial       = "material"
	OriginModelKnowledge = "model_knowledge"
	OriginExternal       = "external"
	// OriginUnverified is what the page marks «sin verificar» when the
	// paragraph has no verified origin and did not declare model_knowledge.
	OriginUnverified = "unverified"
)

// PendingUnsourcedClaim is «afirmación sin fuente» (estados §1, R-1):
// only in a document that asks for evidence (B-51).
const PendingUnsourcedClaim = "unsourced_claim"

// CitedPassage is a source and the literal passage of it that backs a
// paragraph (R-2).
type CitedPassage struct {
	Source  string `json:"source"`
	Passage string `json:"passage"`
	// Searched is a source a round found (kind web, R-1), with its year
	// and its link; the title is Source.
	Searched *SearchedSource `json:"searched,omitempty"`
}

// SearchedSource is what the page shows of a source a round found: title,
// year and link (backlog-r3 3.5). Year is 0 when the API gave none. The
// URL comes from the API, never from a model (B-58).
type SearchedSource struct {
	Label string `json:"label"`
	Title string `json:"title"`
	Year  int    `json:"year,omitempty"`
	URL   string `json:"url,omitempty"`
}

// Provenance is where one paragraph of a section's current text comes
// from (R-1, R-2), computed from the graph, never by a model.
type Provenance struct {
	// Block is the paragraph's place in the section, from 1, as the
	// history numbers it (B-10).
	Block int `json:"block"`
	// Passage is the paragraph, literal.
	Passage string `json:"passage"`
	// Declared is the claim_origin the last agent step that wrote the
	// paragraph declared; empty when no agent step wrote it.
	Declared string `json:"declared"`
	// Verified: the paragraph holds the claim of a citation whose
	// passage is literal in a source the principal read (a comes_from
	// edge, R-3). Origin says from what.
	Verified bool           `json:"verified"`
	Sources  []CitedPassage `json:"sources"`
	// Origin is what the page says beside the paragraph, computed here and
	// never taken from a model (backlog-r3 3.5): material · external ·
	// model_knowledge · unverified.
	Origin string `json:"origin"`
	// Candidates are what the agent named in this paragraph without a
	// citation (R-2, B-69): shown «candidata, sin verificar», never sources.
	Candidates []Candidate `json:"candidates"`
	// Accepted is your «porque» when you accepted the paragraph as
	// general knowledge (claim_accepted) and its passage is still
	// literal in the section.
	Accepted string `json:"accepted,omitempty"`
}

// Provenances returns the provenance of each paragraph of a section's
// current text, in text order (R-1, R-2). A paragraph is a block of the
// history (Blocks, B-10).
// [EL DISEÑO NO LO DICE]: whether a citation verifies a paragraph the
// agent declared model_knowledge; it does: the app's check, not the
// declaration, decides (R-2, «la procedencia sale del grafo»).
// [EL DISEÑO NO LO DICE]: a paragraph you wrote has no declared origin
// and counts like any other: verified only by a citation.
// [EL DISEÑO NO LO DICE]: a paragraph declared external backed only by your
// material; it stays «sin verificar» (backlog-r3 3.5: «se verifica si su
// cita está literal en una fuente web leída»): the declaration can lower
// the verification, never raise it or name the origin.
func Provenances(ctx context.Context, q store.Querier, projectID, sectionID string) ([]Provenance, error) {
	section, err := q.GetObject(ctx, projectID, sectionID)
	if err != nil {
		return nil, err
	}
	text := bodyString(section, "text")
	cited, err := supportedCitations(ctx, q, projectID, section.ID)
	if err != nil {
		return nil, err
	}
	events, err := q.EventsSince(ctx, projectID, 0)
	if err != nil {
		return nil, err
	}
	accepted := acceptedPassages(events, section.ID, Compare(text))
	candidates := Candidates(events)

	out := []Provenance{}
	for i, block := range Blocks(text) {
		b := Compare(block)
		if strings.TrimSpace(b.Text) == "" {
			continue
		}
		p := Provenance{Block: i + 1, Passage: block, Declared: declaredOrigin(events, section.ID, b),
			Sources: []CitedPassage{}, Candidates: paragraphCandidates(candidates, b)}
		for _, c := range cited {
			if p.Declared == OriginExternal && c.Searched == nil {
				continue
			}
			if len(b.Find(c.anchor)) > 0 {
				p.Verified = true
				p.Sources = append(p.Sources, c.CitedPassage)
			}
		}
		p.Origin = computedOrigin(p)
		for _, a := range accepted {
			if strings.Contains(b.Text, a.passage) {
				p.Accepted = a.reason
			}
		}
		out = append(out, p)
	}
	return out, nil
}

// computedOrigin is what the page says beside a paragraph: from the
// sources that verify it, never from a model. Your material comes first
// when both back it (the citation of a searched source still shows).
func computedOrigin(p Provenance) string {
	if p.Verified {
		for _, s := range p.Sources {
			if s.Searched == nil {
				return OriginMaterial
			}
		}
		return OriginExternal
	}
	if p.Declared == OriginModelKnowledge {
		return OriginModelKnowledge
	}
	return OriginUnverified
}

// SearchedSources are the searched sources behind a section's verified
// paragraphs, each once, in text order.
func SearchedSources(provenances []Provenance) []SearchedSource {
	out := []SearchedSource{}
	seen := map[string]bool{}
	for _, p := range provenances {
		for _, s := range p.Sources {
			if s.Searched != nil && !seen[s.Searched.Label] {
				seen[s.Searched.Label] = true
				out = append(out, *s.Searched)
			}
		}
	}
	return out
}

// BlockProvenance is the provenance of one block of a section, for its
// history (R-2, P5).
func BlockProvenance(ctx context.Context, q store.Querier, projectID, sectionLabel string, block int) (Provenance, error) {
	section, err := q.GetObject(ctx, projectID, sectionLabel)
	if err != nil || section.Type != store.ObjectSection {
		return Provenance{}, fmt.Errorf("%w: section %s", store.ErrNotFound, sectionLabel)
	}
	all, err := Provenances(ctx, q, projectID, section.ID)
	if err != nil {
		return Provenance{}, err
	}
	for _, p := range all {
		if p.Block == block {
			return p, nil
		}
	}
	return Provenance{}, fmt.Errorf("%w: block %d of %s", store.ErrNotFound, block, sectionLabel)
}

// EvidenceRequired reports whether the document asks for evidence: the
// value in the request, proposed by the «Entendí» or confirmed by you
// (B-51). Without one, it does not.
func EvidenceRequired(ctx context.Context, q store.Querier, projectID string) (bool, error) {
	requests, err := q.ListObjects(ctx, projectID, store.ObjectRequest)
	if err != nil || len(requests) == 0 {
		return false, err
	}
	var b struct {
		EvidenceRequired struct {
			Value bool `json:"value"`
		} `json:"evidence_required"`
	}
	if err := json.Unmarshal(requests[0].Body, &b); err != nil {
		return false, nil
	}
	return b.EvidenceRequired.Value, nil
}

// UnsourcedClaims returns, in text order, the paragraphs of a section
// that leave «afirmación sin fuente» (R-1): only when the document asks
// for evidence, each paragraph without a verified origin and without an
// acceptance of yours. It is computed from the current text, never
// stored, like «códigos internos»: it closes when a step brings the
// citation, when the paragraph leaves the text or when you accept it.
func UnsourcedClaims(ctx context.Context, q store.Querier, projectID, sectionID string) ([]string, error) {
	required, err := EvidenceRequired(ctx, q, projectID)
	if err != nil || !required {
		return nil, err
	}
	all, err := Provenances(ctx, q, projectID, sectionID)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, p := range all {
		if !p.Verified && p.Accepted == "" {
			out = append(out, p.Passage)
		}
	}
	return out, nil
}

type citedAnchor struct {
	CitedPassage
	anchor string
}

// supportedCitations are a section's citations with a comes_from edge:
// their passage is literal in a source the principal read (the gate
// draws it only then, R-3).
func supportedCitations(ctx context.Context, q store.Querier, projectID, sectionID string) ([]citedAnchor, error) {
	citations, err := q.ListObjects(ctx, projectID, store.ObjectCitation)
	if err != nil {
		return nil, err
	}
	var out []citedAnchor
	for _, c := range citations {
		if bodyString(c, "section_id") != sectionID {
			continue
		}
		edges, err := q.EdgesFrom(ctx, projectID, c.ID)
		if err != nil {
			return nil, err
		}
		for _, e := range edges {
			if e.Type != store.EdgeComesFrom {
				continue
			}
			src, err := q.GetObject(ctx, projectID, e.ToID)
			if err != nil {
				return nil, err
			}
			cited := CitedPassage{Source: SourceTitle(src), Passage: bodyString(c, "passage")}
			if cited.Searched = searchedSource(src); cited.Searched != nil && cited.Searched.Title != "" {
				cited.Source = cited.Searched.Title
			}
			out = append(out, citedAnchor{anchor: bodyString(c, "anchor"), CitedPassage: cited})
			break
		}
	}
	return out, nil
}

// searchedSource is what the page shows of a source a round found (kind
// web, B-61), nil for your material.
func searchedSource(src store.Object) *SearchedSource {
	var b struct {
		Kind  string `json:"kind"`
		Title string `json:"title"`
		Year  int    `json:"year"`
		URL   string `json:"url"`
	}
	if err := json.Unmarshal(src.Body, &b); err != nil || b.Kind != SourceKindWeb {
		return nil
	}
	return &SearchedSource{Label: src.Label, Title: b.Title, Year: b.Year, URL: b.URL}
}

// declaredOrigin is the claim_origin of the last agent step whose new
// text is in the block, or holds it (as stepInBlock). The step keeps
// claim_origins by text, in the order of texts (piece 2.1).
func declaredOrigin(events []store.Event, sectionID string, b Compared) string {
	declared := ""
	for _, ev := range events {
		if ev.ObjectID != sectionID || ev.Actor != "agent" || ev.NewVersion == 0 ||
			(ev.Type != store.EventEdit && ev.Type != store.EventStateChanged) {
			continue
		}
		data := eventData(ev)
		texts, _ := data["texts"].([]any)
		origins, _ := data["claim_origins"].([]any)
		for i, item := range texts {
			s, _ := item.(string)
			t := CompareString(s)
			if t == "" || i >= len(origins) {
				continue
			}
			if strings.Contains(b.Text, t) || strings.Contains(t, b.Text) {
				declared, _ = origins[i].(string)
				break
			}
		}
	}
	return declared
}

type acceptance struct {
	passage string // compared form
	reason  string
}

// acceptedPassages are your claim_accepted on the section whose passage
// is still literal in its current text (R-1: it holds while the passage
// stays the same), oldest first.
func acceptedPassages(events []store.Event, sectionID string, text Compared) []acceptance {
	var out []acceptance
	for _, ev := range events {
		if ev.Type != store.EventClaimAccepted || ev.ObjectID != sectionID {
			continue
		}
		passage, _ := eventData(ev)["passage"].(string)
		if len(text.Find(passage)) == 0 {
			continue
		}
		out = append(out, acceptance{passage: CompareString(passage), reason: ev.Reason})
	}
	return out
}
