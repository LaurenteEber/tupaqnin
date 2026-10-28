package graph

import (
	"context"
	"fmt"

	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// Labeler is the single place that translates between internal ids
// and short labels (specification §1, T-2). The model only ever sees
// labels; nothing outside this package may build that translation.
type Labeler struct {
	byID    map[string]string
	byLabel map[string]string
}

// NewLabeler loads the label translation of one project. Objects of
// another project are never in the map (project isolation, §1).
func NewLabeler(ctx context.Context, db *store.DB, projectID string) (*Labeler, error) {
	objects, err := db.AllObjects(ctx, projectID)
	if err != nil {
		return nil, err
	}
	l := &Labeler{
		byID:    make(map[string]string, len(objects)),
		byLabel: make(map[string]string, len(objects)),
	}
	for _, obj := range objects {
		l.byID[obj.ID] = obj.Label
		l.byLabel[obj.Label] = obj.ID
	}
	return l, nil
}

// ToLabel returns the short label of an internal id.
func (l *Labeler) ToLabel(id string) (string, error) {
	label, ok := l.byID[id]
	if !ok {
		// The internal id itself is never put in an error (T-2).
		return "", fmt.Errorf("%w: unknown id", store.ErrNotFound)
	}
	return label, nil
}

// ToID returns the internal id of a short label.
func (l *Labeler) ToID(label string) (string, error) {
	id, ok := l.byLabel[label]
	if !ok {
		return "", fmt.Errorf("%w: label %q", store.ErrNotFound, label)
	}
	return id, nil
}

// HasLabel reports whether the label exists in the project.
func (l *Labeler) HasLabel(label string) bool {
	_, ok := l.byLabel[label]
	return ok
}
