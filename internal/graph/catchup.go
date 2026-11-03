package graph

import (
	"context"
	"encoding/json"

	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// B-103 (a): what the stop report adds so you catch up before signing. Both lists
// are read from the history, with no model, and limited to the stretch since the
// previous report. Everything you or the agent wrote stays literal (rule 8).

// ReportDecided is one decision of the stretch: its statement, its «porque» (yours;
// a decision of the agent has none and says what it chose it instead of), the
// sections that use it, by label, and who decided: human or agent.
type ReportDecided struct {
	Statement string   `json:"statement"`
	InsteadOf string   `json:"instead_of,omitempty"`
	Reason    string   `json:"reason,omitempty"`
	Sections  []string `json:"sections"`
	Who       string   `json:"who"`
}

// ReportDecisions lists the stretch's decisions, in the order they happened: those
// born in it and still in force, and those you kept in it with your reason. An
// answer that changes the text with no decision behind it counts too, as yours.
// [EL DISEÑO NO LO DICE]: B-103 says «cada decisión del tramo». A decision replaced
// inside the stretch is not listed (its replacement is), a kept one is, and the
// answer to a «por aclarar» that makes no decision is, by what you chose.
func ReportDecisions(ctx context.Context, q store.Querier, projectID string) ([]ReportDecided, error) {
	events, err := q.EventsSince(ctx, projectID, 0)
	if err != nil {
		return nil, err
	}
	since := LastStopSeq(events)
	created := map[string]bool{}
	out := []ReportDecided{}
	place := map[string]int{} // a decision's place in out
	for _, ev := range events {
		first := ev.ObjectID != "" && !created[ev.ObjectID]
		if ev.ObjectID != "" {
			created[ev.ObjectID] = true
		}
		if ev.Seq <= since {
			continue
		}
		var data struct {
			To       string   `json:"to"`
			Applies  []string `json:"applies_to"`
			Text     string   `json:"applies_text"`
			Reason   string   `json:"applies_reason"`
			Decision string   `json:"applies_decision"`
			Option   *int     `json:"option"`
		}
		_ = json.Unmarshal(ev.Data, &data)
		if len(data.Applies) > 0 && data.Decision == "" {
			labels, err := sectionLabels(ctx, q, projectID, data.Applies)
			if err != nil {
				return nil, err
			}
			out = append(out, ReportDecided{Statement: data.Text, Reason: data.Reason, Sections: labels, Who: "human"})
			continue
		}
		decisionID := ev.ObjectID
		kept := ev.Type == store.EventStateChanged && ev.Actor == "human" && data.To == "confirmed"
		if ev.Type == store.EventStateChanged && ev.Actor == "human" && data.To == "resolved" && data.Option != nil {
			if decisionID, kept, err = keptByQuestion(ctx, q, projectID, ev.ObjectID, *data.Option); err != nil {
				return nil, err
			}
		}
		if decisionID == "" || (!first && !kept) {
			continue
		}
		dec, err := q.GetObject(ctx, projectID, decisionID)
		if err != nil {
			return nil, err
		}
		if dec.Type != store.ObjectDecision {
			continue
		}
		item := ReportDecided{Statement: bodyString(dec, "statement"), InsteadOf: bodyString(dec, "instead_of"),
			Reason: bodyString(dec, "reason"), Who: ev.Actor}
		if kept { // your answer, from its own event, whatever the decision's body says
			item.Reason = ev.Reason
		}
		if item.Sections, err = decisionSections(ctx, q, projectID, dec); err != nil {
			return nil, err
		}
		if i, listed := place[dec.ID]; listed {
			out[i] = item
			continue
		}
		place[dec.ID] = len(out)
		out = append(out, item)
	}
	// A decision replaced or revoked since is out; what you kept stays by its id.
	kept := out[:0]
	gone := map[int]bool{}
	for id, i := range place {
		dec, err := q.GetObject(ctx, projectID, id)
		if err != nil {
			return nil, err
		}
		gone[i] = dec.State == "superseded" || dec.State == "revoked"
	}
	for i, item := range out {
		if !gone[i] {
			kept = append(kept, item)
		}
	}
	return kept, nil
}

// keptByQuestion reads a resolved «por aclarar»: the decision it kept, "" when the
// answer kept none. Its answering event is the only one an ordinary «por aclarar»
// writes: applyKept leaves the kept decision as it is.
func keptByQuestion(ctx context.Context, q store.Querier, projectID, questionID string, option int) (string, bool, error) {
	question, err := q.GetObject(ctx, projectID, questionID)
	if err != nil {
		return "", false, err
	}
	var body struct {
		Options []struct {
			Keep bool `json:"keep"`
		} `json:"options"`
		Keeps struct {
			Kind string `json:"kind"`
			ID   string `json:"id"`
		} `json:"keeps"`
	}
	if question.Type != store.ObjectOpenQuestion || json.Unmarshal(question.Body, &body) != nil ||
		option < 0 || option >= len(body.Options) || !body.Options[option].Keep || body.Keeps.Kind != "decision" {
		return "", false, nil
	}
	return body.Keeps.ID, true, nil
}

// decisionSections are the labels of the sections that use a decision, in
// document order.
func decisionSections(ctx context.Context, q store.Querier, projectID string, dec store.Object) ([]string, error) {
	edges, err := q.EdgesFrom(ctx, projectID, dec.ID)
	if err != nil {
		return nil, err
	}
	used := map[string]bool{}
	for _, e := range edges {
		if e.Type == store.EdgeUsedIn {
			used[e.ToID] = true
		}
	}
	sections, err := OrderedSections(ctx, q, projectID)
	if err != nil {
		return nil, err
	}
	out := []string{}
	for _, sec := range sections {
		if used[sec.ID] {
			out = append(out, sec.Label)
		}
	}
	return out, nil
}

func sectionLabels(ctx context.Context, q store.Querier, projectID string, ids []string) ([]string, error) {
	out := []string{}
	for _, id := range ids {
		sec, err := q.GetObject(ctx, projectID, id)
		if err != nil {
			return nil, err
		}
		out = append(out, sec.Label)
	}
	return out, nil
}

// ReportAgentChange is what one step changed in one section on its own: the
// section's label, the step's place in the stretch (from 1), its one line, and what
// left and entered the text («Sale / Entra», B-99).
type ReportAgentChange struct {
	Section string        `json:"section"`
	Step    int           `json:"step"`
	Summary string        `json:"summary"`
	Changes []JudgeChange `json:"changes"`
}

// ReportAgentChanges lists what the stretch's steps changed with no decision or
// message of yours behind it. A step is not on its own when it read a message of
// yours (answers_you in its step_ended); a section is not when it is the one your
// correction named (answers_section) or the step reached it with an answer of yours
// waiting (appliedLedger, B-100). A judge's
// correction batch is on its own; the text of a proposal you accepted is yours.
// [EL DISEÑO NO LO DICE]: only the step that read your message answers it; the
// chain's next steps, on the app's «write the next section», are on their own. A
// history written before B-103 has no answers_you: its steps read as on their own.
func ReportAgentChanges(ctx context.Context, q store.Querier, projectID string) ([]ReportAgentChange, error) {
	events, err := q.EventsSince(ctx, projectID, 0)
	if err != nil {
		return nil, err
	}
	since := LastStopSeq(events)
	ledger := appliedLedger{ctx: ctx, q: q, projectID: projectID}
	out := []ReportAgentChange{}
	var order []string // sections the running step wrote, in order
	written := map[string][]JudgeChange{}
	applied := map[string]bool{}
	step := 0
	for _, ev := range events {
		for _, section := range ledger.observe(ev) {
			applied[section] = true
		}
		var data struct {
			Anchors    []string `json:"anchors"`
			Texts      []string `json:"texts"`
			AcceptedBy string   `json:"accepted_by"`
			TurnKind   string   `json:"turn_kind"`
			Triage     string   `json:"triage_line"`
			AnswersYou bool     `json:"answers_you"`
			Section    string   `json:"answers_section"`
		}
		_ = json.Unmarshal(ev.Data, &data)
		switch {
		case ev.Seq <= since:
			if ev.Type == store.EventStepEnded {
				order, written, applied = nil, map[string][]JudgeChange{}, map[string]bool{}
			}
		case ev.Type == store.EventStepEnded:
			if data.TurnKind == "step" {
				step++
				if !data.AnswersYou {
					for _, id := range order {
						if applied[id] {
							continue
						}
						sec, err := q.GetObject(ctx, projectID, id)
						if err != nil {
							return nil, err
						}
						if sec.Label == data.Section {
							continue
						}
						out = append(out, ReportAgentChange{Section: sec.Label, Step: step, Summary: data.Triage, Changes: written[id]})
					}
				}
			}
			order, written, applied = nil, map[string][]JudgeChange{}, map[string]bool{}
		case ev.Actor == "agent" && ev.ObjectID != "" && len(data.Texts) > 0 && data.AcceptedBy == "":
			obj, err := q.GetObject(ctx, projectID, ev.ObjectID)
			if err != nil {
				return nil, err
			}
			if obj.Type != store.ObjectSection {
				continue
			}
			if _, seen := written[ev.ObjectID]; !seen {
				order = append(order, ev.ObjectID)
			}
			written[ev.ObjectID] = append(written[ev.ObjectID], textChanges(data.Anchors, data.Texts)...)
		}
	}
	return out, nil
}

// textChanges pairs what left with what entered, as JudgeCorrections reads them. A
// text with no anchor (a new section) has nothing leaving.
func textChanges(anchors, texts []string) []JudgeChange {
	out := make([]JudgeChange, 0, len(texts))
	for i, text := range texts {
		change := JudgeChange{In: text}
		if i < len(anchors) {
			change.Out = anchors[i]
		}
		out = append(out, change)
	}
	return out
}
