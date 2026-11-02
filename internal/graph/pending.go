package graph

import (
	"context"
	"encoding/json"
	"regexp"

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

func hasCurrentEdge(ctx context.Context, db store.Querier, projectID, fromID string, t store.EdgeType) (bool, error) {
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

// CoverHolds says a covers edge to target still gives the figure its decision:
// a revoked decision has no successor and rules nothing (B-82). The edge stays
// for the history.
// [EL DISEÑO NO LO DICE]: B-82 and estados §1 do not name the figure of a revoked
// decision; it is read as «cifra sin decisión», so closing the notice does not
// free the section.
func CoverHolds(target store.Object) bool {
	return target.State != "revoked"
}

func figureHasDecision(ctx context.Context, db store.Querier, projectID, figureID string) (bool, error) {
	edges, err := db.EdgesFrom(ctx, projectID, figureID)
	if err != nil {
		return false, err
	}
	for _, e := range edges {
		if e.Type != store.EdgeCovers {
			continue
		}
		target, err := db.GetObject(ctx, projectID, e.ToID)
		if err != nil {
			return false, err
		}
		if CoverHolds(target) {
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
func FiguresWithoutDecision(ctx context.Context, db store.Querier, projectID, sectionID string) ([]string, error) {
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
		ok, err := figureHasDecision(ctx, db, projectID, fig.ID)
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
func SourcesNotSupporting(ctx context.Context, db store.Querier, projectID, sectionID string) ([]string, error) {
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

// projectCodeInText is a token shaped like a short label (glossary §8),
// after NormalizeLabel: letters, a dash and digits, not glued to a
// letter or digit before it. Whether it counts is decided against the
// project's labels, so «Formulario F-40» does not (T-2).
var projectCodeInText = regexp.MustCompile(`(?:^|[^a-z0-9])([a-z]+-[0-9]+)`)

// InternalCodes returns, in text order and once each, the internal codes
// in a section's current text (fichas §3 «códigos internos», rule 5 of
// flujos §1): short labels that exist in this project and any
// provisional id new-n (T-2, T-3), compared after NormalizeLabel as rule
// 4 does. It is computed from the current text, so it closes when the
// code leaves the text (backlog-r2 2.1).
func InternalCodes(ctx context.Context, q store.Querier, projectID, sectionID string) ([]string, error) {
	section, err := q.GetObject(ctx, projectID, sectionID)
	if err != nil {
		return nil, err
	}
	objects, err := q.AllObjects(ctx, projectID)
	if err != nil {
		return nil, err
	}
	labels := make(map[string]bool, len(objects))
	for _, obj := range objects {
		labels[obj.Label] = true
	}
	var out []string
	seen := map[string]bool{}
	for _, m := range projectCodeInText.FindAllStringSubmatch(NormalizeLabel(bodyString(section, "text")), -1) {
		code := m[1]
		if seen[code] || !(labels[code] || provisionalID.MatchString(code)) {
			continue
		}
		seen[code] = true
		out = append(out, code)
	}
	return out, nil
}

var provisionalID = regexp.MustCompile(`^new-[1-9][0-9]*$`)

// TextWithoutDecision reports «escrito sin decisión» found by the app
// (rule 2, B-42) on a section: an agent step left it in its step_ended,
// and no later agent step that wrote the section declared a decision
// used_in it (backlog-r2 2.1, how it closes). The step records both
// lists, text_without_decision and decided_sections, by label.
func TextWithoutDecision(ctx context.Context, q store.Querier, projectID, sectionID string) (bool, error) {
	section, err := q.GetObject(ctx, projectID, sectionID)
	if err != nil {
		return false, err
	}
	events, err := q.EventsSince(ctx, projectID, 0)
	if err != nil {
		return false, err
	}
	return textWithoutDecision(events, section.Label), nil
}

func textWithoutDecision(events []store.Event, sectionLabel string) bool {
	open := false
	for _, ev := range events {
		data := eventData(ev)
		// An accepted proposal's close carries them too (P-114).
		if ev.Type != store.EventStepEnded && data["accepted_by"] != "human" {
			continue
		}
		switch {
		case contains(data["text_without_decision"], sectionLabel):
			open = true
		case contains(data["decided_sections"], sectionLabel):
			open = false
		}
	}
	return open
}
