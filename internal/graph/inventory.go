package graph

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// RoundInventory is the coverage inventory of one round (A-5, flujos §2),
// as the app stored it when the round closed: labels only, never ids.
type RoundInventory struct {
	Round    string
	Question string
	Parts    []InventoryPart
}

// InventoryPart is what one reader was assigned and left unread, with its
// gaps.
type InventoryPart struct {
	Part          int
	Status        string
	Reason        string
	Sources       int
	SourcesUnread []string
	Hits          int
	HitsUnread    []InventoryHit
	Gaps          []string
}

// InventoryHit is a search result a reader left unread.
type InventoryHit struct {
	Label string
	Title string
}

type inventoryBody struct {
	Kind     string `json:"kind"`
	Question string `json:"question"`
	Parts    []struct {
		Sources []sourceRef `json:"sources"`
	} `json:"parts"`
	Findings       []sourceRef `json:"findings"`
	DigestFindings []sourceRef `json:"digest_findings"`
	Hits           []struct {
		Label string `json:"label"`
		Part  int    `json:"part"`
		Title string `json:"title"`
	} `json:"hits"`
	CoverageInventory []struct {
		Part        int      `json:"part"`
		Status      string   `json:"status"`
		Reason      string   `json:"reason"`
		Sources     []string `json:"sources"`
		SourcesRead []string `json:"sources_read"`
		Hits        []string `json:"hits"`
		HitsRead    []string `json:"hits_read"`
		Gaps        []string `json:"gaps"`
	} `json:"coverage_inventory"`
}

type sourceRef struct {
	SourceID string `json:"source_id"`
}

// SectionRoundInventories returns the inventories of the rounds whose
// sources the section uses (B-68, fichas §5), oldest first; none for a
// section that uses no round's source.
// [EL DISEÑO NO LO DICE]: what «usa» counts and what «fuentes de la ronda»
// count. A section uses a source with a uses edge or a citation of it with
// a comes_from edge; a round's sources are those it assigned to its readers
// and those its findings verified.
func SectionRoundInventories(ctx context.Context, q store.Querier, projectID, sectionID string) ([]RoundInventory, error) {
	used, err := sourcesOfSection(ctx, q, projectID, sectionID)
	if err != nil || len(used) == 0 {
		return nil, err
	}
	rounds, err := q.ListObjects(ctx, projectID, store.ObjectRound)
	if err != nil {
		return nil, err
	}
	var out []RoundInventory
	for _, round := range rounds {
		var b inventoryBody
		if err := json.Unmarshal(round.Body, &b); err != nil {
			return nil, fmt.Errorf("graph: round body: %w", err)
		}
		if b.Kind != "research" || len(b.CoverageInventory) == 0 || !b.usesAny(used) {
			continue
		}
		inv, err := b.inventory(ctx, q, projectID, round.Label)
		if err != nil {
			return nil, err
		}
		out = append(out, inv)
	}
	return out, nil
}

func (b inventoryBody) usesAny(used map[string]bool) bool {
	refs := append(append([]sourceRef{}, b.Findings...), b.DigestFindings...)
	for _, p := range b.Parts {
		refs = append(refs, p.Sources...)
	}
	for _, r := range refs {
		if r.SourceID != "" && used[r.SourceID] {
			return true
		}
	}
	return false
}

func (b inventoryBody) inventory(ctx context.Context, q store.Querier, projectID, label string) (RoundInventory, error) {
	titles := map[string]string{}
	for _, h := range b.Hits {
		titles[h.Label] = h.Title
	}
	inv := RoundInventory{Round: label, Question: b.Question}
	for _, e := range b.CoverageInventory {
		part := InventoryPart{Part: e.Part, Status: e.Status, Reason: e.Reason, Sources: len(e.Sources), Hits: len(e.Hits), Gaps: e.Gaps}
		read := map[string]bool{}
		for _, id := range append(append([]string{}, e.SourcesRead...), e.HitsRead...) {
			read[id] = true
		}
		for _, id := range e.Sources {
			if read[id] {
				continue
			}
			// Rule 7: the inventory keeps ids; a model sees labels.
			src, err := q.GetObject(ctx, projectID, id)
			if err != nil {
				return RoundInventory{}, err
			}
			part.SourcesUnread = append(part.SourcesUnread, src.Label)
		}
		for _, h := range e.Hits {
			if !read[h] {
				part.HitsUnread = append(part.HitsUnread, InventoryHit{Label: h, Title: titles[h]})
			}
		}
		inv.Parts = append(inv.Parts, part)
	}
	return inv, nil
}

func sourcesOfSection(ctx context.Context, q store.Querier, projectID, sectionID string) (map[string]bool, error) {
	used := map[string]bool{}
	edges, err := q.EdgesFrom(ctx, projectID, sectionID)
	if err != nil {
		return nil, err
	}
	for _, e := range edges {
		if e.Type == store.EdgeUses {
			used[e.ToID] = true
		}
	}
	citations, err := q.ListObjects(ctx, projectID, store.ObjectCitation)
	if err != nil {
		return nil, err
	}
	for _, c := range citations {
		if bodyString(c, "section_id") != sectionID {
			continue
		}
		edges, err := q.EdgesFrom(ctx, projectID, c.ID)
		if err != nil {
			return nil, err
		}
		for _, e := range edges {
			if e.Type == store.EdgeComesFrom {
				used[e.ToID] = true
			}
		}
	}
	return used, nil
}

// RenderInventories writes inventories for a model (the checker's brief,
// the principal's startup): the counts the app computed, then what comes
// from a person, an API or a reader between the markers (P-83: a question
// can derive from the document, a title comes from an API).
// [EL DISEÑO NO LO DICE]: the shape of these lines.
func RenderInventories(list []RoundInventory) string {
	var b strings.Builder
	for _, inv := range list {
		fmt.Fprintf(&b, "Round %s:\n", inv.Round)
		data := []string{inv.Question}
		for _, p := range inv.Parts {
			state := p.Status
			if p.Reason != "" {
				state += ": " + p.Reason
			}
			fmt.Fprintf(&b, "- Reader %d (%s): %d of %d results unread; %d of %d sources unread; %d gaps.\n",
				p.Part, state, len(p.HitsUnread), p.Hits, len(p.SourcesUnread), p.Sources, len(p.Gaps))
			for _, h := range p.HitsUnread {
				data = append(data, fmt.Sprintf("Reader %d left unread: %s %s", p.Part, h.Label, strings.Join(strings.Fields(h.Title), " ")))
			}
			for _, s := range p.SourcesUnread {
				data = append(data, fmt.Sprintf("Reader %d left unread: %s", p.Part, s))
			}
			for _, g := range p.Gaps {
				data = append(data, fmt.Sprintf("Reader %d gap: %s", p.Part, g))
			}
		}
		fmt.Fprintf(&b, "%s\n", Delimit(DataPassage, strings.Join(data, "\n")))
	}
	return b.String()
}
