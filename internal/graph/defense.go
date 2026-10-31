package graph

import (
	"context"
	"fmt"
	"strings"

	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// DefenseNeeded reports whether the stop that proposes signing a section
// prepares its defense (backlog-r2 2.6, B-38): someone else than you
// reads it («para quién» has text, R6), or it has role conclusion or
// recommendation (R-8).
func DefenseNeeded(ctx context.Context, q store.Querier, projectID, sectionLabel string) (bool, error) {
	sec, err := q.GetObject(ctx, projectID, sectionLabel)
	if err != nil {
		return false, err
	}
	role, _ := bodyValue(sec, "role").(map[string]any)
	if v, _ := role["value"].(string); v == "conclusion" || v == "recommendation" {
		return true, nil
	}
	reqs, err := q.ListObjects(ctx, projectID, store.ObjectRequest)
	if err != nil || len(reqs) == 0 {
		return false, err
	}
	return strings.TrimSpace(bodyString(reqs[0], "audience")) != "", nil
}

// DefenseBrief is the judge's reduced brief at the stop (backlog-r2 2.6,
// fichas §7): the section's current text, the request with its audience,
// the criteria and the section's critical decisions, in literal. Labels
// only (rule 7).
func DefenseBrief(ctx context.Context, q store.Querier, projectID, sectionLabel string) (string, error) {
	sec, err := q.GetObject(ctx, projectID, sectionLabel)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	line := func(format string, args ...any) { fmt.Fprintf(&b, format+"\n", args...) }
	line("# Defense brief")
	line("")
	reqs, err := q.ListObjects(ctx, projectID, store.ObjectRequest)
	if err != nil {
		return "", err
	}
	if len(reqs) > 0 {
		line("## Request of the person (literal)")
		line("%s", bodyString(reqs[0], "text"))
		if a := strings.TrimSpace(bodyString(reqs[0], "audience")); a != "" {
			line("Audience: %s", a)
		} else {
			line("Audience: not given; the section has conclusions or recommendations.")
		}
		line("")
	}
	crits, err := q.ListObjects(ctx, projectID, store.ObjectCriterion)
	if err != nil {
		return "", err
	}
	if len(crits) > 0 {
		line("## Criteria (literal)")
		for _, c := range crits {
			line("- %s: %s", c.Label, bodyString(c, "text"))
		}
		line("")
	}
	line("## Section %s «%s», current text", sec.Label, bodyString(sec, "title"))
	line("%s", Delimit(DataSection, bodyString(sec, "text")))
	line("")
	critical, err := SectionCriticalDecisions(ctx, q, projectID, sec.ID)
	if err != nil {
		return "", err
	}
	line("## Critical decisions of this section")
	if len(critical) == 0 {
		line("None.")
	}
	for _, dec := range critical {
		line("- %s: %s", dec.Label, bodyString(dec, "statement"))
		if v := bodyString(dec, "instead_of"); v != "" {
			line("  Instead of: %s", v)
		}
	}
	line("")
	line("Every entry names the section %s.", sec.Label)
	return b.String(), nil
}

// AudienceQuestion is one audience question as the stop report shows it
// (P6, estados §9): the judge's literal question, or what the work does
// not cover, its state and the label of the section it is about.
type AudienceQuestion struct {
	Label      string `json:"label"`
	State      string `json:"state"`
	Question   string `json:"question"`
	NotCovered bool   `json:"not_covered"`
	Section    string `json:"section"`
}

// OpenAudienceQuestions are the open audience questions about a section,
// by their about edge, in the order they were born (P6 reads «las
// preguntas del destinatario abiertas de la sección»).
func OpenAudienceQuestions(ctx context.Context, q store.Querier, projectID, sectionLabel string) ([]AudienceQuestion, error) {
	sec, err := q.GetObject(ctx, projectID, sectionLabel)
	if err != nil {
		return nil, err
	}
	edges, err := q.EdgesTo(ctx, projectID, sec.ID)
	if err != nil {
		return nil, err
	}
	var objs []store.Object
	for _, e := range edges {
		if e.Type != store.EdgeAbout {
			continue
		}
		aq, err := q.GetObject(ctx, projectID, e.FromID)
		if err != nil {
			return nil, err
		}
		if aq.Type == store.ObjectAudienceQuestion && aq.State == "open" {
			objs = append(objs, aq)
		}
	}
	sortByLabelNumber(objs)
	out := make([]AudienceQuestion, 0, len(objs))
	for _, aq := range objs {
		nc, _ := bodyValue(aq, "not_covered").(bool)
		out = append(out, AudienceQuestion{Label: aq.Label, State: aq.State,
			Question: bodyString(aq, "question"), NotCovered: nc, Section: sec.Label})
	}
	return out, nil
}

func sortByLabelNumber(objs []store.Object) {
	for i := 1; i < len(objs); i++ {
		for j := i; j > 0 && labelNumber(objs[j].Label) < labelNumber(objs[j-1].Label); j-- {
			objs[j], objs[j-1] = objs[j-1], objs[j]
		}
	}
}
