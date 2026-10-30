package cycle

import (
	"context"
	"errors"

	"github.com/LaurenteEber/tupaqnin/internal/checks"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// ErrDecisionNotFound: the label names no decision of the project.
var ErrDecisionNotFound = errors.New("cycle: decision not found")

// ErrDecisionNotCurrent: a superseded or revoked decision is not marked.
var ErrDecisionNotCurrent = errors.New("cycle: decision is not current")

// CheckYourEdit runs the app's rules 1 to 5 on your save, inside its
// transaction tx (B-36: on each save that changes the fingerprint; the
// caller only calls it then). baseText is the section's text before the
// save. Rule 3 opens the card of your critical edit (R4, B-49) for each
// critical decision whose figure anchor your edit removed or changed,
// unless that decision already has one open. Nothing here stops you.
func CheckYourEdit(ctx context.Context, tx *store.Tx, sectionID, baseText string) ([]checks.Finding, error) {
	projectID := tx.ProjectID()
	findings, err := checks.RunRules(ctx, tx, projectID,
		checks.Change{Kind: checks.YourEdit, Sections: []string{sectionID}, BaseText: baseText})
	if err != nil {
		return nil, err
	}
	cards, err := tx.ListObjects(ctx, projectID, store.ObjectCard)
	if err != nil {
		return nil, err
	}
	open := map[string]bool{} // decision id → an own-edit card is open
	for _, c := range cards {
		if c.State == "open" && bodyStr(bodyOf(c), "kind") == CardOwnEdit {
			open[bodyStr(bodyOf(c), "decision_id")] = true
		}
	}
	for _, f := range findings {
		if f.Kind != checks.FindingCriticalEdit {
			continue
		}
		dec, err := tx.GetObject(ctx, projectID, f.Decision)
		if err != nil {
			return nil, err
		}
		if open[dec.ID] {
			continue
		}
		open[dec.ID] = true
		if _, err := tx.CreateObject(store.ObjectCard, "open", map[string]any{
			"decision_id": dec.ID, "chosen_option": "", "reason": "",
			"kind": CardOwnEdit, "section_id": sectionID, "anchor": f.About,
		}, appEvent("", map[string]any{"decision": dec.Label, "to": "open", "reasons": f.Reasons,
			"why": "your_critical_edit"})); err != nil {
			return nil, err
		}
	}
	return findings, nil
}

// MarkCritical marks or unmarks a decision as critical (fichas §2, Tu
// turno; the fourth reason of T-5): human_marked_critical on the
// decision and marked_critical in the history, in a transaction that
// compares criticality before and after (specification §1.5), so an
// agent's decision that becomes critical opens its card. Marking what
// is already marked writes nothing.
func MarkCritical(ctx context.Context, db *store.DB, projectID, label string, marked bool) error {
	err := db.Write(ctx, projectID, func(tx *store.Tx) error {
		return TrackCriticality(ctx, tx, "", func() error {
			dec, err := tx.GetObject(ctx, projectID, label)
			if err != nil || dec.Type != store.ObjectDecision {
				return ErrDecisionNotFound
			}
			if dec.State == "superseded" || dec.State == "revoked" {
				return ErrDecisionNotCurrent
			}
			b := bodyOf(dec)
			b["human_marked_critical"] = marked
			_, err = tx.UpdateObject(dec.ID, dec.CurrentVersion, dec.State, b,
				store.EventInput{Type: store.EventMarkedCritical, Actor: "human",
					Data: map[string]any{"label": dec.Label, "marked": marked}})
			return err
		})
	})
	if errors.Is(err, store.ErrNoChange) {
		return nil
	}
	return err
}
