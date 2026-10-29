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

// Entry is one entry of a block's history: a text change (a step or an
// edit, with the section version it made) or a decision.
type Entry struct {
	Kind     string         `json:"kind"` // text_change · decision
	Version  int            `json:"version,omitempty"`
	Actor    string         `json:"actor"`
	At       string         `json:"at"`
	Decision *DecisionEntry `json:"decision,omitempty"`
	seq      int
}

// Entry kinds.
const (
	EntryTextChange = "text_change"
	EntryDecision   = "decision"
)

// BlockHistory is how a block of a section's current text came to be
// (D7, fichas §8.1, B-10), from the history and the graph, without a
// model: the steps whose new text or anchor is in the block, the
// person's edits that brought a new fragment of 20 characters or more
// into it, and the decisions linked to its figures, with the ones they
// replaced as discarded. Newest first, as P5 shows it.
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
		}
	}

	decisions, err := blockDecisions(ctx, db, projectID, section.ID, b, events)
	if err != nil {
		return nil, err
	}
	out = append(out, decisions...)
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

// blockDecisions are the decisions covering the block's figures, each
// at its birth, and the decisions they replaced, discarded (fichas §9).
func blockDecisions(ctx context.Context, db *store.DB, projectID, sectionID string, b Compared, events []store.Event) ([]Entry, error) {
	born := map[string]store.Event{}
	for _, ev := range events {
		if _, ok := born[ev.ObjectID]; !ok && ev.ObjectID != "" {
			born[ev.ObjectID] = ev
		}
	}
	reasons, err := cardReasons(ctx, db, projectID)
	if err != nil {
		return nil, err
	}
	figures, err := db.ListObjects(ctx, projectID, store.ObjectFigure)
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	var out []Entry
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
		out = append(out, Entry{Kind: EntryDecision, Actor: ev.Actor, At: ev.CreatedAt, seq: ev.Seq,
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
	return out, nil
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
