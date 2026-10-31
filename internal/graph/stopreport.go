package graph

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// LastStopSeq is the seq of the project's last stop_report, 0 when
// there is none: «since the last stop» starts after it (fichas §2).
func LastStopSeq(events []store.Event) int {
	last := 0
	for _, ev := range events {
		if ev.Type == store.EventStopReport {
			last = ev.Seq
		}
	}
	return last
}

// UnseenDecision is one decision of the stop's list (B-39): its label,
// its statement and «instead of», and the labels of the sections that
// use it, in document order.
type UnseenDecision struct {
	Label     string   `json:"label"`
	Statement string   `json:"statement"`
	InsteadOf string   `json:"instead_of"`
	Sections  []string `json:"sections"`
}

// UnseenDecisions is B-39's list, the one the stop counts and the stop
// report sketches: the current decisions the agent created since the
// last stop report that have no decision_seen by any way, critical or
// not, in the order they were created. Your decisions do not count: you
// saw them when you took them.
// [EL DISEÑO NO LO DICE]: «vigente» is any state but superseded and
// revoked, as in the frame.
func UnseenDecisions(ctx context.Context, q store.Querier, projectID string) ([]UnseenDecision, error) {
	events, err := q.EventsSince(ctx, projectID, 0)
	if err != nil {
		return nil, err
	}
	since := LastStopSeq(events)
	seen := map[string]bool{}
	created := map[string]bool{}
	var byAgent []string // decision ids, in creation order
	for _, ev := range events {
		if ev.ObjectID == "" {
			continue
		}
		if ev.Type == store.EventDecisionSeen {
			seen[ev.ObjectID] = true
		}
		if created[ev.ObjectID] {
			continue
		}
		created[ev.ObjectID] = true // an object's first event creates it
		if ev.Seq > since && ev.Actor == "agent" {
			byAgent = append(byAgent, ev.ObjectID)
		}
	}
	sections, err := OrderedSections(ctx, q, projectID)
	if err != nil {
		return nil, err
	}
	out := []UnseenDecision{}
	for _, id := range byAgent {
		if seen[id] {
			continue
		}
		dec, err := q.GetObject(ctx, projectID, id)
		if err != nil {
			return nil, err
		}
		if dec.Type != store.ObjectDecision || dec.State == "superseded" || dec.State == "revoked" {
			continue
		}
		edges, err := q.EdgesFrom(ctx, projectID, dec.ID)
		if err != nil {
			return nil, err
		}
		usedIn := map[string]bool{}
		for _, e := range edges {
			if e.Type == store.EdgeUsedIn {
				usedIn[e.ToID] = true
			}
		}
		u := UnseenDecision{Label: dec.Label, Statement: bodyString(dec, "statement"),
			InsteadOf: bodyString(dec, "instead_of"), Sections: []string{}}
		for _, sec := range sections {
			if usedIn[sec.ID] {
				u.Sections = append(u.Sections, sec.Label)
			}
		}
		out = append(out, u)
	}
	return out, nil
}

// TouchedSinceStop are the labels of the sections the agent's steps
// wrote since the last stop report, each once, the last touched last
// (backlog-r2 2.5: «sección sin pendientes» is evaluated only on them).
func TouchedSinceStop(ctx context.Context, q store.Querier, projectID string) ([]string, error) {
	events, err := q.EventsSince(ctx, projectID, 0)
	if err != nil {
		return nil, err
	}
	since := LastStopSeq(events)
	var out []string
	for _, ev := range events {
		if ev.Seq <= since || ev.Type != store.EventStepEnded {
			continue
		}
		var data struct {
			Written []string `json:"written_sections"`
		}
		_ = json.Unmarshal(ev.Data, &data)
		for _, label := range data.Written {
			for i, l := range out {
				if l == label {
					out = append(out[:i], out[i+1:]...)
					break
				}
			}
			out = append(out, label)
		}
	}
	return out, nil
}

// SectionClear reports whether a section could be signed now: it has
// text, is not signed, and has no pendings and no open changes (fichas
// §2, Parada: «si no queda ningún pendiente ni cambio sin cerrar»).
// Computed by the app, never by a model (backlog-r2 2.5).
func SectionClear(ctx context.Context, q store.Querier, projectID, label string) (bool, error) {
	section, err := q.GetObject(ctx, projectID, label)
	if err != nil || section.Type != store.ObjectSection {
		return false, err
	}
	if strings.TrimSpace(bodyString(section, "text")) == "" || section.State == string(StatusSigned) ||
		len(OpenChanges(section)) > 0 {
		return false, nil
	}
	_, pendings, err := SectionStatus(ctx, q, projectID, section.Label)
	if err != nil {
		return false, err
	}
	return len(pendings) == 0, nil
}

// ReportCitation is one citation of the stop report, by its section and
// the literal claim it backs: verified when its passage is literal in
// its source (comes_from), not verified when not (B-41).
type ReportCitation struct {
	Section string `json:"section"`
	Claim   string `json:"claim"`
}

// ReportCitations splits the citations still in the text of the given
// sections into verified and not verified (B-41: the app fills both
// from the graph), in the order of the sections given.
// [EL DISEÑO NO LO DICE]: the report counts the citations of the
// sections touched since the last stop, not of the whole project.
func ReportCitations(ctx context.Context, q store.Querier, projectID string, sectionLabels []string) (verified, notVerified []ReportCitation, err error) {
	verified, notVerified = []ReportCitation{}, []ReportCitation{}
	citations, err := q.ListObjects(ctx, projectID, store.ObjectCitation)
	if err != nil {
		return nil, nil, err
	}
	for _, label := range sectionLabels {
		section, err := q.GetObject(ctx, projectID, label)
		if err != nil {
			return nil, nil, err
		}
		text := Compare(bodyString(section, "text"))
		for _, c := range citations {
			anchor := bodyString(c, "anchor")
			if bodyString(c, "section_id") != section.ID || len(text.Find(anchor)) == 0 {
				continue
			}
			ok, err := hasCurrentEdge(ctx, q, projectID, c.ID, store.EdgeComesFrom)
			if err != nil {
				return nil, nil, err
			}
			rc := ReportCitation{Section: section.Label, Claim: anchor}
			if ok {
				verified = append(verified, rc)
			} else {
				notVerified = append(notVerified, rc)
			}
		}
	}
	return verified, notVerified, nil
}

// ReportOpenItem is one open card or «por aclarar» of the stop report
// (B-41): its kind (card · open_question), label and text (the
// decision's statement or the question).
type ReportOpenItem struct {
	Kind  string `json:"kind"`
	Label string `json:"label"`
	Text  string `json:"text"`
}

// ReportOpenItems are the project's open cards and «por aclarar», the
// report's «por aclarar y tarjetas» (B-41), cards first.
func ReportOpenItems(ctx context.Context, q store.Querier, projectID string) ([]ReportOpenItem, error) {
	out := []ReportOpenItem{}
	cards, err := q.ListObjects(ctx, projectID, store.ObjectCard)
	if err != nil {
		return nil, err
	}
	for _, c := range cards {
		if c.State != "open" {
			continue
		}
		statement := ""
		if dec, err := q.GetObject(ctx, projectID, bodyString(c, "decision_id")); err == nil {
			statement = bodyString(dec, "statement")
		}
		out = append(out, ReportOpenItem{Kind: "card", Label: c.Label, Text: statement})
	}
	questions, err := q.ListObjects(ctx, projectID, store.ObjectOpenQuestion)
	if err != nil {
		return nil, err
	}
	for _, oq := range questions {
		if oq.State == "open" {
			out = append(out, ReportOpenItem{Kind: "open_question", Label: oq.Label, Text: bodyString(oq, "question")})
		}
	}
	return out, nil
}
