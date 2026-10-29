package graph

import (
	"context"
	"encoding/json"

	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// Agent pendings are computed, never stored (B-15). Each one comes from
// one function here.

func bodyString(obj store.Object, key string) string {
	var b map[string]any
	_ = json.Unmarshal(obj.Body, &b)
	s, _ := b[key].(string)
	return s
}

func hasCurrentEdge(ctx context.Context, db *store.DB, projectID, fromID string, t store.EdgeType) (bool, error) {
	edges, err := db.EdgesFrom(ctx, projectID, fromID)
	if err != nil {
		return false, err
	}
	for _, e := range edges {
		if e.Type == t {
			return true, nil
		}
	}
	return false, nil
}

// FiguresWithoutDecision returns, in text order, the numbers of a
// section's text that have no decision (fichas §3, «cifra sin
// decisión»; B-15): not inside the anchor of a figure that a current
// covers edge links to a decision, and not a datum linked to its source
// by comes_from (A-3). It closes when the number is linked or leaves the
// text.
func FiguresWithoutDecision(ctx context.Context, db *store.DB, projectID, sectionID string) ([]string, error) {
	section, err := db.GetObject(ctx, projectID, sectionID)
	if err != nil {
		return nil, err
	}
	covered := make(map[string]bool)
	figures, err := db.ListObjects(ctx, projectID, store.ObjectFigure)
	if err != nil {
		return nil, err
	}
	for _, fig := range figures {
		if bodyString(fig, "section_id") != section.ID {
			continue
		}
		ok, err := hasCurrentEdge(ctx, db, projectID, fig.ID, store.EdgeCovers)
		if err != nil {
			return nil, err
		}
		if ok {
			for v := range NumberValues(bodyString(fig, "anchor")) {
				covered[v] = true
			}
		}
	}
	data, err := db.ListObjects(ctx, projectID, store.ObjectDatum)
	if err != nil {
		return nil, err
	}
	for _, d := range data {
		if bodyString(d, "section_id") != section.ID {
			continue
		}
		ok, err := hasCurrentEdge(ctx, db, projectID, d.ID, store.EdgeComesFrom)
		if err != nil {
			return nil, err
		}
		if ok {
			covered[NormalizeNumber(bodyString(d, "value"))] = true
		}
	}
	var out []string
	seen := make(map[string]bool)
	for _, n := range Numbers(bodyString(section, "text")) {
		if covered[n.Value] || seen[n.Value] {
			continue
		}
		seen[n.Value] = true
		out = append(out, n.Raw)
	}
	return out, nil
}

// SourcesNotSupporting returns the labels of a section's citations that
// have no comes_from edge, because their passage is not literal in their
// source (fichas §3, glossary §9 «fuente que no sostiene»; B-15), while
// their claim is still in the text. It closes when the citation is
// corrected or removed.
func SourcesNotSupporting(ctx context.Context, db *store.DB, projectID, sectionID string) ([]string, error) {
	section, err := db.GetObject(ctx, projectID, sectionID)
	if err != nil {
		return nil, err
	}
	citations, err := db.ListObjects(ctx, projectID, store.ObjectCitation)
	if err != nil {
		return nil, err
	}
	text := Compare(bodyString(section, "text"))
	var out []string
	for _, c := range citations {
		if bodyString(c, "section_id") != section.ID {
			continue
		}
		if len(text.Find(bodyString(c, "anchor"))) == 0 {
			continue // the citation left the text
		}
		ok, err := hasCurrentEdge(ctx, db, projectID, c.ID, store.EdgeComesFrom)
		if err != nil {
			return nil, err
		}
		if !ok {
			out = append(out, c.Label)
		}
	}
	return out, nil
}
