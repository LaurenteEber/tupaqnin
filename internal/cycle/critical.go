package cycle

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/LaurenteEber/tupaqnin/internal/graph"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// TrackCriticality runs fn inside tx, a transaction that changes edges or
// a section's role, and then compares criticality before and after
// (T-5): an agent's decision that becomes critical gets its open card
// (estados §2); a decision of a signed section that becomes critical
// moves that section to changed_since_signoff, in every case (K-6,
// K-13); an open card whose decision changed becomes obsolete (estados
// §3). All of it in the same transaction.
func TrackCriticality(ctx context.Context, tx *store.Tx, turnID string, fn func() error) error {
	before, err := graph.CriticalAll(ctx, tx, tx.ProjectID())
	if err != nil {
		return err
	}
	if err := fn(); err != nil {
		return err
	}
	return reconcileCritical(ctx, tx, turnID, before)
}

func appEvent(turnID string, data map[string]any) store.EventInput {
	return store.EventInput{Type: store.EventStateChanged, Actor: "app", TurnID: turnID, Data: data}
}

func reconcileCritical(ctx context.Context, tx *store.Tx, turnID string, before map[string][]graph.Reason) error {
	projectID := tx.ProjectID()
	after, err := graph.CriticalAll(ctx, tx, projectID)
	if err != nil {
		return err
	}
	cards, err := tx.ListObjects(ctx, projectID, store.ObjectCard)
	if err != nil {
		return err
	}
	openCard := map[string]bool{}
	for _, card := range cards {
		if card.State != "open" {
			continue
		}
		decID := bodyStr(bodyOf(card), "decision_id")
		dec, err := tx.GetObject(ctx, projectID, decID)
		if err != nil {
			return err
		}
		// [EL DISEÑO NO LO DICE]: «la decisión cambió» is read as no
		// longer the agent's current one; in slice 1 only superseding
		// changes a decision, and the card stores no decision version.
		if dec.State == "agent" {
			openCard[decID] = true
			continue
		}
		// The decision changed before the answer (estados §3).
		if _, err := tx.UpdateObject(card.ID, card.CurrentVersion, "obsolete", bodyOf(card),
			appEvent(turnID, map[string]any{"label": card.Label, "from": "open", "to": "obsolete",
				"why": "decision_changed"})); err != nil {
			return err
		}
	}
	decisions, err := tx.ListObjects(ctx, projectID, store.ObjectDecision)
	if err != nil {
		return err
	}
	for _, dec := range decisions {
		reasons := after[dec.ID]
		if len(reasons) == 0 || len(before[dec.ID]) > 0 {
			continue // not critical, or already was
		}
		if dec.State == "agent" && !openCard[dec.ID] {
			if _, err := tx.CreateObject(store.ObjectCard, "open",
				map[string]any{"decision_id": dec.ID, "chosen_option": "", "reason": ""},
				appEvent(turnID, map[string]any{"decision": dec.Label, "to": "open", "reasons": reasons})); err != nil {
				return err
			}
		}
		if err := markSignedSectionsChanged(ctx, tx, turnID, dec); err != nil {
			return err
		}
	}
	return nil
}

// markSignedSectionsChanged moves each signed section that uses the
// decision to changed_since_signoff (K-6).
// [EL DISEÑO NO LO DICE]: «una sección firmada» of the decision is read
// as the sections it is used_in, not those found by text search.
func markSignedSectionsChanged(ctx context.Context, tx *store.Tx, turnID string, dec store.Object) error {
	edges, err := tx.EdgesFrom(ctx, tx.ProjectID(), dec.ID)
	if err != nil {
		return err
	}
	for _, e := range edges {
		if e.Type != store.EdgeUsedIn {
			continue
		}
		sec, err := tx.GetObject(ctx, tx.ProjectID(), e.ToID)
		if err != nil {
			return err
		}
		if sec.State != "signed" {
			continue
		}
		var body map[string]any
		_ = json.Unmarshal(sec.Body, &body)
		_, err = tx.UpdateObject(sec.ID, sec.CurrentVersion, "changed_since_signoff", body,
			appEvent(turnID, map[string]any{"label": sec.Label, "from": "signed", "to": "changed_since_signoff",
				"why": "decision_became_critical", "decision": dec.Label}))
		if err != nil && !errors.Is(err, store.ErrNoChange) {
			return err
		}
	}
	return nil
}
