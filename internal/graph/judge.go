package graph

import (
	"context"
	"encoding/json"

	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// OpenJudgeRound is a judge round that has flaws and has not ended
// (backlog-r3 3.6): its label, section, whether its correction batch
// started, and the flaws you did not discard.
type OpenJudgeRound struct {
	Label        string
	SectionID    string
	BatchStarted bool
	Flaws        []OpenFlaw
}

// OpenFlaw is one flaw of an open judge round.
type OpenFlaw struct {
	N        int
	Passage  string
	Scenario string
}

// OpenJudgeRounds are the project's open judge rounds with flaws, for «Lo
// que te toca» and the underlines (pantallas, «Juez (3.6)»).
func OpenJudgeRounds(ctx context.Context, q store.Querier, projectID string) ([]OpenJudgeRound, error) {
	rounds, err := q.ListObjects(ctx, projectID, store.ObjectRound)
	if err != nil {
		return nil, err
	}
	var out []OpenJudgeRound
	for _, round := range rounds {
		if round.State != "open" {
			continue
		}
		var b struct {
			Kind         string `json:"kind"`
			SectionID    string `json:"section_id"`
			BatchStarted bool   `json:"batch_started"`
			Flaws        []struct {
				N         int    `json:"n"`
				Passage   string `json:"passage"`
				Scenario  string `json:"scenario"`
				Discarded bool   `json:"discarded"`
			} `json:"flaws"`
		}
		if err := json.Unmarshal(round.Body, &b); err != nil || b.Kind != "judge" {
			continue
		}
		jr := OpenJudgeRound{Label: round.Label, SectionID: b.SectionID, BatchStarted: b.BatchStarted}
		for _, f := range b.Flaws {
			if !f.Discarded {
				jr.Flaws = append(jr.Flaws, OpenFlaw{N: f.N, Passage: f.Passage, Scenario: f.Scenario})
			}
		}
		if len(jr.Flaws) > 0 {
			out = append(out, jr)
		}
	}
	return out, nil
}
