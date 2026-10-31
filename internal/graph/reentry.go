package graph

import (
	"context"
	"encoding/json"
	"time"

	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// returnAfterHours is M-2's threshold for the return line: 3 days
// [inicial, parámetro] since your last event (B-48 a).
const returnAfterHours = 72

// ReturnAfter is that threshold as a duration; the server can be given
// another one (server.WithReturnAfter).
func ReturnAfter() time.Duration { return returnAfterHours * time.Hour }

// ReturnLine is the return line (M-2, spec §9): where you left off (the
// last section and your last act), what changed (state changes since
// your last event, with a link to their history) and what is yours to
// do (the lines of «Lo que te toca»). It never blocks.
type ReturnLine struct {
	// Days since your last event, whole days.
	Days int `json:"days"`
	// Section is the label of the last section your acts touched, "" if
	// none did.
	Section string `json:"section"`
	// Act is the kind of your last act; ActAbout its literal object (a
	// decision's statement, a question), when it has one.
	Act      string `json:"act"`
	ActAbout string `json:"act_about,omitempty"`
	// Changes counts the state changes since your last event;
	// ChangedSections are the sections among them, in document order.
	Changes         int      `json:"changes"`
	ChangedSections []string `json:"changed_sections"`
	// Todo is the number of your pendings: FrameTodo.Count.
	Todo int `json:"todo"`
}

// Detected is one thing the app detected since the last stop report,
// your edits included (B-48 b): the short return shows them under the
// unseen report. Never an internal id: sections by label, the rest by
// their literal text.
type Detected struct {
	// Kind is edit · section_state · decision_state · usage_change ·
	// source_new_version · open_question · card.
	Kind string `json:"kind"`
	// Actor is human · agent · app.
	Actor   string `json:"actor"`
	Section string `json:"section,omitempty"`
	Text    string `json:"text,omitempty"`
	// To is the new state of a section_state or decision_state.
	To string `json:"to,omitempty"`
	// Count is how many edits by the same actor on the same section it
	// groups; 1 for the rest.
	Count int `json:"count"`
}

// lastHumanEvent is your last event, anything done by you (B-48 a);
// false when you did nothing yet.
func lastHumanEvent(events []store.Event) (store.Event, bool) {
	for i := len(events) - 1; i >= 0; i-- {
		if events[i].Actor == "human" {
			return events[i], true
		}
	}
	return store.Event{}, false
}

// eventTime reads an event's created_at.
func eventTime(ev store.Event) (time.Time, bool) {
	t, err := time.Parse(time.RFC3339Nano, ev.CreatedAt)
	return t, err == nil
}

// ReturnDue reports whether the return line is due at now: after or
// more passed since your last event (B-48 a, spec §9). The frame shows
// the line and the stop report route uses it to tell the short return.
func ReturnDue(events []store.Event, now time.Time, after time.Duration) bool {
	last, ok := lastHumanEvent(events)
	if !ok {
		return false
	}
	at, ok := eventTime(last)
	return ok && now.Sub(at) >= after
}

// actOf names your act in one of your events, "" for an event that only
// accompanies another act (its timing, its edges, the warnings and
// state changes it caused, the decisions a card or a report marks seen).
// [EL DISEÑO NO LO DICE]: which of your events are acts; these are the
// ones the history shows as something you did.
func actOf(ev store.Event, obj store.Object) string {
	var data map[string]any
	_ = json.Unmarshal(ev.Data, &data)
	str := func(k string) string { s, _ := data[k].(string); return s }
	switch ev.Type {
	case store.EventActTimed, store.EventEdge, store.EventUsageChange:
		return ""
	case store.EventDecisionSeen:
		if v := str("seen_via"); v == "card" || v == "report" {
			return ""
		}
		return "decision_seen"
	case store.EventEdit:
		return "edit"
	case store.EventEditUndone:
		return "edit_undone"
	case store.EventHumanMessage:
		return "message"
	case store.EventStopReportSeen:
		return "stop_report_seen"
	case store.EventClaimAccepted:
		return "claim_accepted"
	case store.EventPlanConfirmed:
		return "plan_confirmed"
	case store.EventMarkedCritical:
		return "marked_critical"
	case store.EventRoleChanged:
		return "role_changed"
	case store.EventSectionDetailsChanged:
		return "section_details"
	case store.EventSourceNewVersion:
		return "source"
	case store.EventAudienceChanged:
		return "audience"
	case store.EventAnswerWithoutReason:
		return "answer_without_reason"
	case store.EventSignoffCase, store.EventSignoffPaused, store.EventSignoffCancelled, store.EventBriefSignoff:
		return "signoff"
	case store.EventStateChanged:
		if str("why") != "" {
			return ""
		}
		switch obj.Type {
		case store.ObjectCard:
			return "card_answered"
		case store.ObjectOpenQuestion:
			return "open_question_answered"
		case store.ObjectAudienceQuestion:
			return "audience_question_answered"
		case store.ObjectUnderstandingCase:
			return "case_answered"
		case store.ObjectSection:
			if obj.State == string(StatusSigned) && str("to") == string(StatusSigned) {
				return "signed"
			}
		}
		return "changed"
	}
	return "other"
}

// actAbout is the literal object of an act: a decision's statement (a
// card's, too) or a question.
func actAbout(obj store.Object, byID map[string]store.Object) string {
	switch obj.Type {
	case store.ObjectDecision:
		return bodyString(obj, "statement")
	case store.ObjectCard:
		return bodyString(byID[bodyString(obj, "decision_id")], "statement")
	case store.ObjectOpenQuestion, store.ObjectAudienceQuestion:
		return bodyString(obj, "question")
	}
	return ""
}

// sectionOf is the section an event of yours names: its object, the
// section in its data, or the first section (document order) a card's,
// a decision's or a «por aclarar»'s decision is used in.
func sectionOf(ctx context.Context, q store.Querier, projectID string, ev store.Event,
	byID map[string]store.Object, order map[string]int) (string, error) {
	obj := byID[ev.ObjectID]
	if obj.Type == store.ObjectSection {
		return obj.Label, nil
	}
	var data map[string]any
	_ = json.Unmarshal(ev.Data, &data)
	if label, _ := data["section"].(string); label != "" {
		for _, o := range byID {
			if o.Label == label && o.Type == store.ObjectSection {
				return label, nil
			}
		}
	}
	if id := bodyString(obj, "section_id"); byID[id].Type == store.ObjectSection {
		return byID[id].Label, nil
	}
	var decisions []string
	switch obj.Type {
	case store.ObjectCard:
		decisions = []string{bodyString(obj, "decision_id")}
	case store.ObjectDecision:
		decisions = []string{obj.ID}
	case store.ObjectOpenQuestion, store.ObjectAudienceQuestion:
		var b struct {
			DependsOn []string `json:"depends_on"`
			Section   string   `json:"section"`
		}
		_ = json.Unmarshal(obj.Body, &b)
		for _, id := range b.DependsOn {
			if byID[id].Type == store.ObjectSection {
				return byID[id].Label, nil
			}
		}
		decisions = b.DependsOn
	}
	best, bestAt := "", -1
	for _, id := range decisions {
		edges, err := q.EdgesFrom(ctx, projectID, id)
		if err != nil {
			return "", err
		}
		for _, e := range edges {
			sec := byID[e.ToID]
			if e.Type != store.EdgeUsedIn || sec.Type != store.ObjectSection {
				continue
			}
			if at := order[sec.ID]; bestAt < 0 || at < bestAt {
				best, bestAt = sec.Label, at
			}
		}
	}
	return best, nil
}

// returnLine is M-2's line, nil when less than after passed since your
// last event (B-48 a). «What changed» counts the state_changed events
// after your last event that change an existing object (an object's
// first event creates it, as in UnseenDecisions).
// [EL DISEÑO NO LO DICE]: the creation of an object is not a change of
// state; the link to the history leads to the sections that changed.
func returnLine(ctx context.Context, q store.Querier, projectID string, events []store.Event,
	byID map[string]store.Object, sections []store.Object, now time.Time, after time.Duration) (*ReturnLine, error) {
	if !ReturnDue(events, now, after) {
		return nil, nil
	}
	last, _ := lastHumanEvent(events)
	at, _ := eventTime(last)
	order := map[string]int{}
	for i, s := range sections {
		order[s.ID] = i
	}
	line := &ReturnLine{Days: int(now.Sub(at) / (24 * time.Hour)), ChangedSections: []string{}}

	// Where you left off: your last act and the last section your acts
	// named, walking back from your last event.
	for i := len(events) - 1; i >= 0 && (line.Act == "" || line.Section == ""); i-- {
		ev := events[i]
		if ev.Actor != "human" {
			continue
		}
		obj := byID[ev.ObjectID]
		act := actOf(ev, obj)
		if act == "" {
			continue
		}
		if line.Act == "" {
			line.Act, line.ActAbout = act, actAbout(obj, byID)
		}
		if line.Section == "" {
			sec, err := sectionOf(ctx, q, projectID, ev, byID, order)
			if err != nil {
				return nil, err
			}
			line.Section = sec
		}
	}

	// What changed since your last event.
	created := map[string]bool{}
	changed := map[string]bool{}
	for _, ev := range events {
		first := ev.ObjectID != "" && !created[ev.ObjectID]
		if ev.ObjectID != "" {
			created[ev.ObjectID] = true
		}
		if ev.Seq <= last.Seq || ev.Type != store.EventStateChanged || first {
			continue
		}
		line.Changes++
		if byID[ev.ObjectID].Type == store.ObjectSection {
			changed[ev.ObjectID] = true
		}
	}
	for _, s := range sections {
		if changed[s.ID] {
			line.ChangedSections = append(line.ChangedSections, s.Label)
		}
	}
	return line, nil
}

// DetectedSinceStop is what the app detected since the last stop
// report, your edits included (B-48 b): edits (grouped by section and
// actor), state changes of sections and decisions, warnings
// (usage_change), new source versions, and the new cards and «por
// aclarar». In the order they happened.
// [EL DISEÑO NO LO DICE]: «los pendientes nuevos» are the new cards
// (yours); the agent's pendings are computed, not events, and the
// section's state already says them.
func DetectedSinceStop(ctx context.Context, q store.Querier, projectID string) ([]Detected, error) {
	events, err := q.EventsSince(ctx, projectID, 0)
	if err != nil {
		return nil, err
	}
	objects, err := q.AllObjects(ctx, projectID)
	if err != nil {
		return nil, err
	}
	byID := make(map[string]store.Object, len(objects))
	byLabel := make(map[string]store.Object, len(objects))
	for _, o := range objects {
		byID[o.ID], byLabel[o.Label] = o, o
	}
	since := LastStopSeq(events)
	out := []Detected{}
	edits := map[string]int{} // section id + actor → index in out
	created := map[string]bool{}
	for _, ev := range events {
		first := ev.ObjectID != "" && !created[ev.ObjectID]
		if ev.ObjectID != "" {
			created[ev.ObjectID] = true
		}
		if ev.Seq <= since {
			continue
		}
		obj := byID[ev.ObjectID]
		var data map[string]any
		_ = json.Unmarshal(ev.Data, &data)
		str := func(k string) string { s, _ := data[k].(string); return s }
		switch {
		case ev.Type == store.EventEdit && obj.Type == store.ObjectSection:
			key := obj.ID + " " + ev.Actor
			if i, ok := edits[key]; ok {
				out[i].Count++
				continue
			}
			edits[key] = len(out)
			out = append(out, Detected{Kind: "edit", Actor: ev.Actor, Section: obj.Label, Count: 1})
		case ev.Type == store.EventUsageChange && str("kind") == "entered":
			about := byLabel[str("about")]
			out = append(out, Detected{Kind: "usage_change", Actor: ev.Actor, Section: str("section"),
				Text: readable(about, byID), Count: 1})
		case ev.Type == store.EventSourceNewVersion:
			out = append(out, Detected{Kind: "source_new_version", Actor: ev.Actor, Text: str("filename"), Count: 1})
		case ev.Type == store.EventStateChanged && first && obj.Type == store.ObjectOpenQuestion:
			sec, err := sectionOf(ctx, q, projectID, ev, byID, nil)
			if err != nil {
				return nil, err
			}
			out = append(out, Detected{Kind: "open_question", Actor: ev.Actor, Section: sec,
				Text: bodyString(obj, "question"), Count: 1})
		case ev.Type == store.EventStateChanged && first && obj.Type == store.ObjectCard:
			out = append(out, Detected{Kind: "card", Actor: ev.Actor, Text: actAbout(obj, byID), Count: 1})
		case ev.Type == store.EventStateChanged && !first && obj.Type == store.ObjectSection && str("to") != "":
			out = append(out, Detected{Kind: "section_state", Actor: ev.Actor, Section: obj.Label, To: str("to"), Count: 1})
		case ev.Type == store.EventStateChanged && !first && obj.Type == store.ObjectDecision && str("to") != "":
			out = append(out, Detected{Kind: "decision_state", Actor: ev.Actor, Text: bodyString(obj, "statement"),
				To: str("to"), Count: 1})
		}
	}
	return out, nil
}

// readable is an object's literal text for the person: a decision's
// statement, a source's file name, a figure's anchor, a criterion's or
// premise's text.
func readable(obj store.Object, byID map[string]store.Object) string {
	switch obj.Type {
	case store.ObjectSource:
		return bodyString(obj, "filename")
	case store.ObjectFigure:
		return bodyString(obj, "anchor")
	case store.ObjectCriterion, store.ObjectPremise:
		return bodyString(obj, "text")
	}
	return actAbout(obj, byID)
}
