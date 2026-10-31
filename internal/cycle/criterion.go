package cycle

import (
	"context"
	"errors"
	"strings"

	"github.com/LaurenteEber/tupaqnin/internal/checks"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

var (
	ErrCriterionNotFound     = errors.New("cycle: criterion not found")
	ErrCriterionTextRequired = errors.New("cycle: the criterion has no text")
)

// CriterionChange is a criterion after your change: its label, its
// current version and its literal text.
type CriterionChange struct {
	Label   string `json:"label"`
	Version int    `json:"version"`
	Text    string `json:"text"`
	// Changed: this change wrote a new version (the same text does not).
	Changed bool `json:"changed"`
}

// ChangeCriterion writes the new version of a criterion, yours only
// (estados §6, backlog-r2 2.8), with your literal text, and queues B-46's
// probe of it against the current text of the affected sections in the
// same transaction (P-72, checks.QueueProbe). The «Entendí»'s cases of
// the criterion are kept (B-46). The same text writes nothing.
// [EL DISEÑO NO LO DICE]: the event of a criterion's new version: a
// state_changed (current → current) with field text, as other changes of
// a field of an object.
func (g *Gate) ChangeCriterion(ctx context.Context, projectID, label, text string) (CriterionChange, error) {
	if strings.TrimSpace(text) == "" {
		return CriterionChange{}, ErrCriterionTextRequired
	}
	var out CriterionChange
	err := g.db.Write(ctx, projectID, func(tx *store.Tx) error {
		crit, err := tx.GetObject(ctx, projectID, label)
		if err != nil || crit.Type != store.ObjectCriterion {
			return ErrCriterionNotFound
		}
		b := bodyOf(crit)
		out = CriterionChange{Label: crit.Label, Version: crit.CurrentVersion, Text: bodyStr(b, "text")}
		if out.Text == text {
			return nil
		}
		b["text"] = text
		updated, err := tx.UpdateObject(crit.ID, crit.CurrentVersion, "current", b,
			store.EventInput{Type: store.EventStateChanged, Actor: "human", ObjectID: crit.ID,
				Data: map[string]any{"label": crit.Label, "from": crit.State, "to": "current", "field": "text"}})
		if err != nil {
			return err
		}
		out = CriterionChange{Label: updated.Label, Version: updated.CurrentVersion, Text: text, Changed: true}
		change, err := checks.CriterionProbe(ctx, tx, projectID, crit.ID)
		if err != nil {
			return err
		}
		return checks.QueueProbe(ctx, tx, crit.ID, change)
	})
	return out, err
}

// ChangeCriterion is the gate's change plus its probe (B-46), run as the
// new premise's (AnswerOpenQuestion, P-72): with the project's turn slot
// free it runs now; with a turn running it stays queued, its sections
// «sin comprobar», and runs when that turn ends or at the next B-36
// trigger. The new version stands either way.
func (r *Runner) ChangeCriterion(ctx context.Context, projectID, label, text string) (CriterionChange, error) {
	changed, err := r.gate.ChangeCriterion(ctx, projectID, label, text)
	if err != nil || !changed.Changed {
		return changed, err
	}
	if !r.tryAcquireTurn(projectID) {
		return changed, nil
	}
	defer r.releaseTurn(projectID)
	turnID, err := newTurnID()
	if err != nil {
		return changed, err
	}
	return changed, r.runQueuedProbes(ctx, projectID, turnID)
}
