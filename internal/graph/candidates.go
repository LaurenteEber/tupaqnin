package graph

import (
	"encoding/json"
	"strings"

	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// Candidate is one reference the agent named without a citation (R-2,
// B-69): what it knows of it, kept in the step_ended of its step. It never
// counts as a source. N is its number in the project, what «Buscarla»
// sends in POST /rounds {references: [n]}.
// [EL DISEÑO NO LO DICE]: what n counts (the route's table says only
// {references: [n]}); the project's candidates in the order they were
// named, each once by its text as the app compares. The history only
// grows, so a number never changes.
type Candidate struct {
	N       int    `json:"n"`
	Text    string `json:"text"`
	Title   string `json:"title,omitempty"`
	Authors string `json:"authors,omitempty"`
	Year    int    `json:"year,omitempty"`
	DOI     string `json:"doi,omitempty"`
}

// Candidates returns the project's candidates from the named_references of
// every step_ended.
func Candidates(events []store.Event) []Candidate {
	out := []Candidate{}
	seen := map[string]bool{}
	for _, ev := range events {
		if ev.Type != store.EventStepEnded {
			continue
		}
		var data struct {
			NamedReferences []Candidate `json:"named_references"`
		}
		if err := json.Unmarshal(ev.Data, &data); err != nil {
			continue
		}
		for _, c := range data.NamedReferences {
			key := CompareString(c.Text)
			if strings.TrimSpace(key) == "" || seen[key] {
				continue
			}
			seen[key] = true
			c.N = len(out) + 1
			out = append(out, c)
		}
	}
	return out
}

// paragraphCandidates are the candidates whose text is literal in the
// paragraph.
// [EL DISEÑO NO LO DICE]: which paragraph a candidate belongs to; the one
// that names it, since its text is «cómo la nombra el texto» (B-69). One
// the text does not name literally shows only in the frame.
func paragraphCandidates(all []Candidate, b Compared) []Candidate {
	out := []Candidate{}
	for _, c := range all {
		if len(b.Find(c.Text)) > 0 {
			out = append(out, c)
		}
	}
	return out
}
