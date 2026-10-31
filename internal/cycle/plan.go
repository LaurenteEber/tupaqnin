package cycle

import (
	"context"
	"errors"
	"fmt"

	"github.com/LaurenteEber/tupaqnin/internal/graph"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// The plan agreed before writing (R-6, backlog-r2 2.12). The
// «Entendí» proposes, per section, what it concludes or addresses
// (focus) and where its evidence will come from (evidence_from); they
// are kept in its step_ended until you answer. Confirming or correcting
// answers the plan: plan_confirmed is recorded once, and each focus
// passes to its section (B-53). Nothing waits for the answer: without
// it, the sections have no focus and new sections are created as
// before (B-54).

// PlanLine is one section of the «Entendí»'s plan (graph.PlanLine).
type PlanLine = graph.PlanLine

// ProposedPlan is the plan of the last accepted «Entendí», in its
// order; nil before one.
func ProposedPlan(ctx context.Context, db *store.DB, projectID string) ([]PlanLine, error) {
	events, err := db.EventsSince(ctx, projectID, 0)
	if err != nil {
		return nil, err
	}
	return planOf(events), nil
}

func planOf(events []store.Event) []PlanLine { return graph.Plan(events) }

func planConfirmedIn(events []store.Event) bool { return graph.PlanConfirmed(events) }

// PlanConfirmed reports whether you answered the plan (B-54 reads it).
func PlanConfirmed(ctx context.Context, db *store.DB, projectID string) (bool, error) {
	events, err := db.EventsSince(ctx, projectID, 0)
	if err != nil {
		return false, err
	}
	return planConfirmedIn(events), nil
}

// ConfirmPlan is «Confirmar» on the plan (R-6): it records
// plan_confirmed and copies each focus to its section (B-53). It
// reports whether this call confirmed it; a plan already confirmed is
// left as it is.
func ConfirmPlan(ctx context.Context, db *store.DB, projectID string) (bool, error) {
	var confirmed bool
	err := db.Write(ctx, projectID, func(tx *store.Tx) error {
		var err error
		confirmed, err = ConfirmPlanTx(ctx, tx, projectID)
		return err
	})
	return confirmed, err
}

// ConfirmPlanTx is ConfirmPlan inside a write, for a correction from
// the card: correcting also answers the plan, so the first correction
// records plan_confirmed (R-6). Each focus passes to its section with
// section_details_changed by you, recorded as proposed by the agent
// (B-53: «propuesto por el agente, confirmado por ti»). A section that
// already has a focus keeps it: it is yours (K-17), or the correction
// that came with this call.
// [EL DISEÑO NO LO DICE]: how the record says «proposed by the agent»;
// proposed_by: agent in the event's data. A section of the plan that no
// longer exists, or an empty focus, is skipped.
func ConfirmPlanTx(ctx context.Context, tx *store.Tx, projectID string) (bool, error) {
	events, err := tx.EventsSince(ctx, projectID, 0)
	if err != nil {
		return false, err
	}
	if planConfirmedIn(events) {
		return false, nil
	}
	plan := planOf(events)
	labels := []string{}
	for _, line := range plan {
		labels = append(labels, line.Section)
	}
	if _, err := tx.AppendEvent(store.EventInput{Type: store.EventPlanConfirmed, Actor: "human",
		Data: map[string]any{"sections": labels}}); err != nil {
		return false, err
	}
	for _, line := range plan {
		if line.Focus == "" {
			continue
		}
		obj, err := tx.GetObject(ctx, projectID, line.Section)
		if err != nil || obj.Type != store.ObjectSection {
			continue
		}
		b := bodyOf(obj)
		if bodyStr(b, "focus") != "" {
			continue
		}
		b["focus"] = line.Focus
		_, err = tx.UpdateObject(obj.ID, obj.CurrentVersion, obj.State, b, store.EventInput{
			Type: store.EventSectionDetailsChanged, Actor: "human",
			Data: map[string]any{"label": obj.Label, "proposed_by": "agent",
				"changes": []any{map[string]any{"field": "focus", "from": "", "to": line.Focus}}},
		})
		if err != nil && !errors.Is(err, store.ErrNoChange) {
			return false, err
		}
	}
	return true, nil
}

// planMessage tells the agent why its new section was not created
// (B-54): with the plan confirmed, it proposes the section in reply and
// the person creates it with «+ Section» (K-17).
func planMessage(id, title string) string {
	return fmt.Sprintf("the plan is confirmed, so the app does not create new sections: %s («%s») was not created and "+
		"its new_text was not applied; the rest of your step was. Propose the section in reply; "+
		"the person creates it with «+ Section» if they agree", id, title)
}
