package graph

import (
	"encoding/json"

	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// PlanLine is one section of the «Entendí»'s plan, by label (R-6,
// backlog-r2 2.12): what it concludes or addresses (focus) and where
// its evidence will come from (evidence_from).
type PlanLine struct {
	Section      string `json:"section"`
	Focus        string `json:"focus"`
	EvidenceFrom string `json:"evidence_from"`
}

// Plan is the plan of the last accepted «Entendí», kept in its
// step_ended, in its order; nil before one. The cycle and the memory
// read it from here (P-70).
func Plan(events []store.Event) []PlanLine {
	var plan []PlanLine
	for _, ev := range events {
		if ev.Type != store.EventStepEnded {
			continue
		}
		var data struct {
			TurnKind string     `json:"turn_kind"`
			Plan     []PlanLine `json:"plan"`
		}
		if json.Unmarshal(ev.Data, &data) != nil || data.TurnKind != "understanding" {
			continue
		}
		plan = data.Plan
	}
	return plan
}

// PlanConfirmed reports whether you answered the plan (B-53, B-54).
func PlanConfirmed(events []store.Event) bool {
	for _, ev := range events {
		if ev.Type == store.EventPlanConfirmed {
			return true
		}
	}
	return false
}
