package graph

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// newFragment is the length of a new fragment that puts the person's
// edit in a block's history (B-10 [inicial]).
const newFragment = 20

// DecisionEntry is a decision in a block's history: its contrast, the
// person's «porque» and whether it was discarded (replaced).
type DecisionEntry struct {
	Label         string `json:"label"`
	Statement     string `json:"statement"`
	InsteadOf     string `json:"instead_of"`
	WouldChooseIf string `json:"would_choose_if"`
	Reason        string `json:"reason,omitempty"`
	State         string `json:"state"`
	Origin        string `json:"origin"`
	Discarded     bool   `json:"discarded"`
	Figure        string `json:"figure,omitempty"` // the anchor of the figure it covers in this block
}

// DatumEntry is a datum copied from a source in a block (A-3, P-60):
// its value, the source's title and the source passage that holds it,
// literal (rule 8).
type DatumEntry struct {
	Value   string `json:"value"`
	Source  string `json:"source"`
	Passage string `json:"passage"`
}

// Entry is one entry of a block's history: a text change (a step or an
// edit, with the section version it made), a decision or a copied
// datum.
type Entry struct {
	Kind     string         `json:"kind"` // text_change · decision · datum
	Version  int            `json:"version,omitempty"`
	Actor    string         `json:"actor"`
	At       string         `json:"at"`
	Decision *DecisionEntry `json:"decision,omitempty"`
	Datum    *DatumEntry    `json:"datum,omitempty"`
	seq      int
}

// Entry kinds.
const (
	EntryTextChange = "text_change"
	EntryDecision   = "decision"
	EntryDatum      = "datum"
)

// BlockHistory is how a block of a section's current text came to be
// (D7, fichas §8.1, B-10), from the history and the graph, without a
// model: the steps whose new text or anchor is in the block, the
// person's edits that brought a new fragment of 20 characters or more
// into it, the decisions linked to its figures and, for each step in
// it, the decisions that step declared for the section (P-60), with the
// ones they replaced as discarded; and the data copied from a source
// into it (A-3). Newest first, as P5 shows it.
// [EL DISEÑO NO LO DICE]: the order; newest first, like the P5 sketch.
func BlockHistory(ctx context.Context, db *store.DB, projectID, sectionLabel string, block int) ([]Entry, error) {
	section, err := db.GetObject(ctx, projectID, sectionLabel)
	if err != nil || section.Type != store.ObjectSection {
		return nil, fmt.Errorf("%w: section %s", store.ErrNotFound, sectionLabel)
	}
	text := bodyString(section, "text")
	blocks := Blocks(text)
	if block < 1 || block > len(blocks) {
		return nil, fmt.Errorf("%w: block %d of %s", store.ErrNotFound, block, sectionLabel)
	}
	b := Compare(blocks[block-1])

	events, err := db.EventsSince(ctx, projectID, 0)
	if err != nil {
		return nil, err
	}
	versions, err := db.Versions(ctx, projectID, section.ID)
	if err != nil {
		return nil, err
	}
	textAt := map[int]string{}
	for _, v := range versions {
		var body map[string]any
		_ = json.Unmarshal(v.Body, &body)
		textAt[v.Version], _ = body["text"].(string)
	}

	var out []Entry
	var steps []string
	for _, ev := range events {
		if ev.ObjectID != section.ID || ev.NewVersion == 0 {
			continue
		}
		if ev.Type != store.EventEdit && !(ev.Type == store.EventStateChanged && ev.NewVersion == 1) {
			continue
		}
		var in bool
		if ev.Actor == "agent" {
			in = stepInBlock(eventData(ev), b)
		} else {
			in = editInBlock(b.Text, CompareString(textAt[ev.BaseVersion]), CompareString(textAt[ev.NewVersion]))
		}
		if in {
			out = append(out, Entry{Kind: EntryTextChange, Version: ev.NewVersion, Actor: ev.Actor, At: ev.CreatedAt, seq: ev.Seq})
			if ev.Actor == "agent" && ev.TurnID != "" {
				steps = append(steps, ev.TurnID)
			}
		}
	}

	decisions, err := blockDecisions(ctx, db, projectID, section.ID, b, events, steps)
	if err != nil {
		return nil, err
	}
	out = append(out, decisions...)
	data, err := blockData(ctx, db, projectID, section.ID, b, events)
	if err != nil {
		return nil, err
	}
	out = append(out, data...)
	sort.SliceStable(out, func(i, j int) bool { return out[i].seq > out[j].seq })
	return out, nil
}

// stepInBlock: a step is in the block if one of its new texts or anchors
// appears in it.
// [EL DISEÑO NO LO DICE]: a new text that spans several blocks never fits
// inside one; it counts too when the block appears inside it.
func stepInBlock(data map[string]any, b Compared) bool {
	for _, key := range []string{"texts", "anchors"} {
		items, _ := data[key].([]any)
		for _, item := range items {
			s, _ := item.(string)
			t := CompareString(s)
			if t == "" || b.Text == "" {
				continue
			}
			if strings.Contains(b.Text, t) || (key == "texts" && strings.Contains(t, b.Text)) {
				return true
			}
		}
	}
	return false
}

// editInBlock: the person's edit is in the block if the block holds a
// fragment of 20 characters or more that the edit brought: present in
// the new version, absent from the previous one (B-10).
func editInBlock(block, prev, next string) bool {
	if utf8.RuneCountInString(block) < newFragment {
		return false
	}
	runes := []rune(block)
	for i := 0; i+newFragment <= len(runes); i++ {
		window := string(runes[i : i+newFragment])
		if strings.Contains(next, window) && !strings.Contains(prev, window) {
			return true
		}
	}
	return false
}

// blockDecisions are the decisions covering the block's figures and
// the decisions the block's steps declared for the section (P-60), each
// at its birth, and the decisions they replaced, discarded (fichas §9).
func blockDecisions(ctx context.Context, db *store.DB, projectID, sectionID string, b Compared, events []store.Event,
	steps []string) ([]Entry, error) {
	add, out, err := decisionAdder(ctx, db, projectID, events)
	if err != nil {
		return nil, err
	}
	figures, err := db.ListObjects(ctx, projectID, store.ObjectFigure)
	if err != nil {
		return nil, err
	}
	for _, fig := range figures {
		anchor := bodyString(fig, "anchor")
		if bodyString(fig, "section_id") != sectionID || len(b.Find(anchor)) == 0 {
			continue
		}
		covers, err := db.EdgesFrom(ctx, projectID, fig.ID)
		if err != nil {
			return nil, err
		}
		for _, e := range covers {
			if e.Type == store.EdgeCovers {
				if err := add(e.ToID, anchor, false); err != nil {
					return nil, err
				}
			}
		}
	}
	// After the figures, so a decision that covers one keeps its anchor.
	declared, err := stepDecisions(ctx, db, projectID, sectionID, events, steps)
	if err != nil {
		return nil, err
	}
	for _, id := range declared {
		if err := add(id, "", false); err != nil {
			return nil, err
		}
	}
	return *out, nil
}

// stepDecisions are the decisions the given steps declared for the
// section (P-60, pantallas P5): new in the step or in its
// used_decisions, and with a used_in edge to the section (the app adds
// it for used_in_sections, for each used decision and section written,
// and for a figure link).
// [EL DISEÑO NO LO DICE]: «declared for the section» is read as
// declared by the step and used in the section; in the order the
// steps declared them.
func stepDecisions(ctx context.Context, db *store.DB, projectID, sectionID string, events []store.Event,
	steps []string) ([]string, error) {
	inBlock := map[string]bool{}
	for _, t := range steps {
		inBlock[t] = true
	}
	var candidates []string
	seen := map[string]bool{}
	note := func(idOrLabel string) {
		dec, err := db.GetObject(ctx, projectID, idOrLabel)
		if err != nil || dec.Type != store.ObjectDecision || seen[dec.ID] {
			return
		}
		seen[dec.ID] = true
		candidates = append(candidates, dec.ID)
	}
	born := map[string]bool{}
	for _, ev := range events {
		first := ev.ObjectID != "" && !born[ev.ObjectID]
		if ev.ObjectID != "" {
			born[ev.ObjectID] = true
		}
		if !inBlock[ev.TurnID] {
			continue
		}
		if first {
			note(ev.ObjectID)
		}
		if ev.Type == store.EventStepEnded {
			used, _ := eventData(ev)["used_decisions"].([]any)
			for _, u := range used {
				if l, ok := u.(string); ok {
					note(l)
				}
			}
		}
	}
	var out []string
	for _, id := range candidates {
		edges, err := db.EdgesFrom(ctx, projectID, id)
		if err != nil {
			return nil, err
		}
		for _, e := range edges {
			if e.Type == store.EdgeUsedIn && e.ToID == sectionID {
				out = append(out, id)
				break
			}
		}
	}
	return out, nil
}

// blockData are the data copied from a source into the block (A-3,
// P-60), each at its birth, with the source's title and the passage of
// the source that holds the value.
func blockData(ctx context.Context, db *store.DB, projectID, sectionID string, b Compared, events []store.Event) ([]Entry, error) {
	born := map[string]store.Event{}
	for _, ev := range events {
		if _, ok := born[ev.ObjectID]; !ok && ev.ObjectID != "" {
			born[ev.ObjectID] = ev
		}
	}
	data, err := db.ListObjects(ctx, projectID, store.ObjectDatum)
	if err != nil {
		return nil, err
	}
	var out []Entry
	for _, d := range data {
		value := bodyString(d, "value")
		if bodyString(d, "section_id") != sectionID || len(b.Find(bodyString(d, "anchor"))) == 0 {
			continue
		}
		edges, err := db.EdgesFrom(ctx, projectID, d.ID)
		if err != nil {
			return nil, err
		}
		for _, e := range edges {
			if e.Type != store.EdgeComesFrom {
				continue
			}
			src, err := db.GetObject(ctx, projectID, e.ToID)
			if err != nil {
				return nil, err
			}
			text := bodyString(src, "text")
			ev := born[d.ID]
			out = append(out, Entry{Kind: EntryDatum, Actor: ev.Actor, At: ev.CreatedAt, seq: ev.Seq,
				Datum: &DatumEntry{Value: value, Source: SourceTitle(src), Passage: SourcePassage(text, value)}})
			break
		}
	}
	return out, nil
}

// SourceTitle is how a source is named: its file name, or its first
// line (the index card's title, R-3).
func SourceTitle(src store.Object) string {
	if title := bodyString(src, "filename"); title != "" {
		return title
	}
	for _, line := range strings.Split(bodyString(src, "text"), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			return line
		}
	}
	return ""
}

// SourcePassage is the sentence of a source's text that holds a number
// with the given value, literal.
// [EL DISEÑO NO LO DICE]: the passage is the sentence (up to «. » or a
// line break) of the first occurrence of the value, without a list or
// heading marker.
func SourcePassage(text, value string) string {
	want := NormalizeNumber(value)
	for _, line := range strings.Split(text, "\n") {
		for _, loc := range numberPattern.FindAllStringIndex(line, -1) {
			if NormalizeNumber(line[loc[0]:loc[1]]) != want {
				continue
			}
			start := strings.LastIndex(line[:loc[0]], ". ")
			if start < 0 {
				start = 0
			} else {
				start += 2
			}
			end := strings.Index(line[loc[1]:], ". ")
			if end < 0 {
				end = len(line)
			} else {
				end += loc[1] + 1
			}
			// A list or heading marker is not part of the passage.
			return strings.TrimSpace(strings.TrimLeft(line[start:end], " \t-*+#>"))
		}
	}
	return ""
}

// DecisionHistory is the history of one decision with no point in the
// text (P-41, Eber 30-oct): its proposal, its contrast, the person's
// «porque», and the decisions it replaced, discarded. P7 opens it from
// «Ver la historia de ese punto» when the decision has no block.
func DecisionHistory(ctx context.Context, db *store.DB, projectID, decisionLabel string) ([]Entry, error) {
	dec, err := db.GetObject(ctx, projectID, decisionLabel)
	if err != nil || dec.Type != store.ObjectDecision {
		return nil, fmt.Errorf("%w: decision %s", store.ErrNotFound, decisionLabel)
	}
	events, err := db.EventsSince(ctx, projectID, 0)
	if err != nil {
		return nil, err
	}
	add, out, err := decisionAdder(ctx, db, projectID, events)
	if err != nil {
		return nil, err
	}
	if err := add(dec.ID, "", false); err != nil {
		return nil, err
	}
	return *out, nil
}

// decisionAdder returns add, which appends a decision entry (with the
// ones it replaced, discarded) to *out once per decision.
func decisionAdder(ctx context.Context, db *store.DB, projectID string, events []store.Event) (
	func(decID, figure string, discarded bool) error, *[]Entry, error) {
	born := map[string]store.Event{}
	for _, ev := range events {
		if _, ok := born[ev.ObjectID]; !ok && ev.ObjectID != "" {
			born[ev.ObjectID] = ev
		}
	}
	reasons, err := cardReasons(ctx, db, projectID)
	if err != nil {
		return nil, nil, err
	}
	seen := map[string]bool{}
	out := &[]Entry{}
	var add func(decID, figure string, discarded bool) error
	add = func(decID, figure string, discarded bool) error {
		if seen[decID] {
			return nil
		}
		seen[decID] = true
		dec, err := db.GetObject(ctx, projectID, decID)
		if err != nil {
			return err
		}
		reason := bodyString(dec, "reason")
		if reason == "" {
			reason = reasons[decID]
		}
		ev := born[decID]
		*out = append(*out, Entry{Kind: EntryDecision, Actor: ev.Actor, At: ev.CreatedAt, seq: ev.Seq,
			Decision: &DecisionEntry{Label: dec.Label, Statement: bodyString(dec, "statement"),
				InsteadOf: bodyString(dec, "instead_of"), WouldChooseIf: bodyString(dec, "would_choose_if"),
				Reason: reason, State: dec.State, Origin: bodyString(dec, "origin"),
				Discarded: discarded || dec.State == "superseded" || dec.State == "revoked", Figure: figure}})
		replaced, err := db.EdgesFrom(ctx, projectID, decID)
		if err != nil {
			return err
		}
		for _, e := range replaced {
			if e.Type == store.EdgeSupersedes {
				if err := add(e.ToID, figure, true); err != nil {
					return err
				}
			}
		}
		return nil
	}
	return add, out, nil
}

// cardReasons maps each decision to the «porque» of its answered card.
func cardReasons(ctx context.Context, db *store.DB, projectID string) (map[string]string, error) {
	cards, err := db.ListObjects(ctx, projectID, store.ObjectCard)
	if err != nil {
		return nil, err
	}
	out := map[string]string{}
	for _, c := range cards {
		if c.State == "answered" {
			out[bodyString(c, "decision_id")] = bodyString(c, "reason")
		}
	}
	return out, nil
}
