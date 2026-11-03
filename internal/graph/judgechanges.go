package graph

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// JudgeChange is one change of a correction batch (B-99): what left the text, what
// entered, and the flaw it answers in the judge's words. Both texts are literal
// (rule 8). Round, Seq and Section are for the app; the page never shows them.
type JudgeChange struct {
	Round    string `json:"-"`
	Seq      int    `json:"-"`
	Section  string `json:"-"`
	N        int    `json:"n,omitempty"`
	Scenario string `json:"scenario,omitempty"`
	Out      string `json:"out"`
	In       string `json:"in"`
}

// JudgeCorrections reads, from the history, what each correction batch did: the
// step right after a batch started. Proposed counts, by round, the sections whose
// changes went to a proposal (B-73, B-99) instead of the text.
func JudgeCorrections(ctx context.Context, q store.Querier, projectID string) (changes []JudgeChange, proposed map[string]int, err error) {
	events, err := q.EventsSince(ctx, projectID, 0)
	if err != nil {
		return nil, nil, err
	}
	proposed = map[string]int{}
	var batch []string
	for _, ev := range events {
		var data struct {
			BatchStarted bool     `json:"batch_started"`
			Anchors      []string `json:"anchors"`
			Texts        []string `json:"texts"`
			Section      string   `json:"section"`
			To           string   `json:"to"`
		}
		_ = json.Unmarshal(ev.Data, &data)
		switch {
		case data.BatchStarted:
			batch = append(batch, ev.ObjectID)
		case len(batch) == 0:
		case ev.Type == store.EventStepEnded:
			batch = nil
		case ev.Actor == "agent" && ev.Type == store.EventEdit:
			for i, anchor := range data.Anchors {
				change := JudgeChange{Round: batch[0], Seq: ev.Seq, Section: ev.ObjectID, Out: anchor}
				if i < len(data.Texts) {
					change.In = data.Texts[i]
				}
				for _, round := range batch {
					if flawOf(ctx, q, projectID, round, anchor, &change) {
						break
					}
				}
				changes = append(changes, change)
			}
		case ev.Actor == "agent" && data.To == "open" && data.Section != "":
			proposed[batch[0]]++
		}
	}
	return changes, proposed, nil
}

// flawOf fills the change with the round's flaw whose passage it touches.
func flawOf(ctx context.Context, q store.Querier, projectID, round, anchor string, change *JudgeChange) bool {
	obj, err := q.GetObject(ctx, projectID, round)
	if err != nil {
		return false
	}
	var body struct {
		Flaws []struct {
			N        int    `json:"n"`
			Passage  string `json:"passage"`
			Scenario string `json:"scenario"`
		} `json:"flaws"`
	}
	if json.Unmarshal(obj.Body, &body) != nil {
		return false
	}
	for _, f := range body.Flaws {
		if f.Passage != "" && (strings.Contains(f.Passage, anchor) || strings.Contains(anchor, f.Passage)) {
			change.Round, change.N, change.Scenario = round, f.N, f.Scenario
			return true
		}
	}
	return false
}
