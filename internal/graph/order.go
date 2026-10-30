package graph

import (
	"context"
	"encoding/json"
	"sort"

	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// SectionPosition is the order of a section in the document (K-17).
// [EL DISEÑO NO LO DICE]: sections born before `position` existed (and
// those of the understanding) carry none; theirs is the number of their
// label, which follows their creation.
func SectionPosition(obj store.Object) int {
	var b map[string]any
	_ = json.Unmarshal(obj.Body, &b)
	if p, ok := b["position"].(float64); ok {
		return int(p)
	}
	return labelNumber(obj.Label)
}

// SortSections orders sections by position, then by label number. The
// frame, the right column, the startup and /sections all use it (K-17).
func SortSections(sections []store.Object) {
	sort.SliceStable(sections, func(i, j int) bool {
		pi, pj := SectionPosition(sections[i]), SectionPosition(sections[j])
		if pi != pj {
			return pi < pj
		}
		return labelNumber(sections[i].Label) < labelNumber(sections[j].Label)
	})
}

// OrderedSections lists the project's sections in document order.
func OrderedSections(ctx context.Context, q store.Querier, projectID string) ([]store.Object, error) {
	sections, err := q.ListObjects(ctx, projectID, store.ObjectSection)
	if err != nil {
		return nil, err
	}
	SortSections(sections)
	return sections, nil
}

// NextSectionPosition is the position of a new section: the end
// (K-17, also for a principal's new-n).
func NextSectionPosition(ctx context.Context, q store.Querier, projectID string) (int, error) {
	sections, err := q.ListObjects(ctx, projectID, store.ObjectSection)
	if err != nil {
		return 0, err
	}
	last := 0
	for _, s := range sections {
		if p := SectionPosition(s); p > last {
			last = p
		}
	}
	return last + 1, nil
}
