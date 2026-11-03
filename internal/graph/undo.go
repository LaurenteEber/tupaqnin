package graph

import (
	"context"
	"encoding/json"
	"slices"

	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// Undo (backlog-r4 4.2, B-79, G-1): what «Deshacer» would revert on a
// section. The route obeys this one answer and the page shows it. It is read
// from the history, by the events that write a section's text: a save that
// changed the fingerprint is one edit, an agent's step or a proposal you
// accepted ends your run, and an undo takes the last edition off it. Saves of a
// section in a row and close in time are one edition (C.20): each stays its
// own event, for rule 6 and for the history, and an undo takes them together.

// Why «Deshacer» is off (B-79).
const (
	UndoReasonNoEdit           = "no_edit"
	UndoReasonAgentWrote       = "agent_wrote"
	UndoReasonProposalAccepted = "proposal_accepted"
	// UndoReasonLaterEdit: the edit named (a card's, an alert's) is not your
	// last one; only the last one is reverted (B-79).
	UndoReasonLaterEdit = "later_edit"
)

// EditReadVersion is the key, in the data of your edit that displaced the
// agent's text (B-73 d), of the version your tab read: the one undoing it
// brings back.
const EditReadVersion = "read_version"

// EditSameEdition is the key, in the data of your edit, of a save that came right
// after your previous save of that section: the route decides by its clock and
// the history keeps the answer (C.20). A write of text in between still ends the
// edition: SectionUndo reads that from the events.
const EditSameEdition = "same_edition"

// UndoState is that answer: on, with the edit it reverts and the version whose
// text comes back; or off, with its reason.
type UndoState struct {
	Available bool
	Reason    string
	// Version is the version your last edit made; Restores the one it reverts
	// to, under the first save of the edition; Versions every save of it.
	Version  int
	Restores int
	Versions []int
}

// Covers says the version is a save of the edition «Deshacer» reverts now.
func (u UndoState) Covers(version int) bool {
	return u.Available && slices.Contains(u.Versions, version)
}

type undoEntry struct {
	versions []int
	restores int
}

// UndoneVersions are the saves an edit_undone took back: every save of the
// edition, or undone_version alone in an undo from before C.20.
func UndoneVersions(ev store.Event) []int {
	var data struct {
		One int   `json:"undone_version"`
		All []int `json:"undone_versions"`
	}
	_ = json.Unmarshal(ev.Data, &data)
	if len(data.All) > 0 {
		return data.All
	}
	return []int{data.One}
}

// SectionUndo says what «Deshacer» does on a section now (B-79).
func SectionUndo(ctx context.Context, q store.Querier, projectID, sectionID string) (UndoState, error) {
	events, err := q.EventsSince(ctx, projectID, 0)
	if err != nil {
		return UndoState{}, err
	}
	var run []undoEntry
	reason := UndoReasonNoEdit
	// joinable: the next save can still join the last edition; any other write of
	// text ends it, on this section or another (C.20).
	joinable := false
	for _, ev := range events {
		if ev.ObjectID != sectionID {
			if ev.Type == store.EventEdit || ev.Type == store.EventEditUndone {
				joinable = false
			}
			continue
		}
		switch ev.Type {
		case store.EventEdit:
			data := eventData(ev)
			if ev.Actor != "human" {
				// It takes away an edit of yours only if there is one under it (P-115).
				if len(run) > 0 || reason != UndoReasonNoEdit {
					reason = UndoReasonAgentWrote
					if data["accepted_by"] == "human" {
						reason = UndoReasonProposalAccepted
					}
				}
				run = nil
				joinable = false
				continue
			}
			if joinable && data[EditSameEdition] == true {
				top := &run[len(run)-1]
				top.versions = append(top.versions, ev.NewVersion)
				continue
			}
			entry := undoEntry{versions: []int{ev.NewVersion}, restores: ev.BaseVersion}
			if read, ok := data[EditReadVersion].(float64); ok {
				entry.restores = int(read)
			}
			run = append(run, entry)
			joinable = true
		case store.EventEditUndone:
			if len(run) > 0 {
				run = run[:len(run)-1]
			}
			joinable = false
		}
	}
	if len(run) == 0 {
		return UndoState{Reason: reason}, nil
	}
	top := run[len(run)-1]
	return UndoState{Available: true, Version: top.versions[len(top.versions)-1], Restores: top.restores, Versions: top.versions}, nil
}

// LastWriteUndid says whether the last text written on a section is the undo
// of the edit that made version: nothing moved since (B-79), so the proposal
// that edit displaced can still be accepted as it is.
func LastWriteUndid(ctx context.Context, q store.Querier, projectID, sectionID string, version int) (bool, error) {
	events, err := q.EventsSince(ctx, projectID, 0)
	if err != nil {
		return false, err
	}
	undid := false
	for _, ev := range events {
		if ev.ObjectID != sectionID {
			continue
		}
		switch ev.Type {
		case store.EventEdit:
			undid = false
		case store.EventEditUndone:
			undid = slices.Contains(UndoneVersions(ev), version)
		}
	}
	return undid, nil
}

// EditThatOpened is the version of your edit that opened a card, or 0: the
// card is born in its edit's transaction, right after it. It tells which R4
// card an undo leaves obsolete (B-79).
func EditThatOpened(events []store.Event, sectionID, cardID string) int {
	last := 0
	for _, ev := range events {
		if ev.ObjectID == cardID {
			return last
		}
		if ev.ObjectID != sectionID {
			continue
		}
		switch {
		case ev.Type == store.EventEdit && ev.Actor == "human":
			last = ev.NewVersion
		case ev.Type == store.EventEdit, ev.Type == store.EventEditUndone:
			last = 0
		}
	}
	return 0
}

// CardUndo is «Fue un error · deshacer» on a card of your edit (B-78): the
// section and edit it names and whether the undo reverts that edit now.
// [EL DISEÑO NO LO DICE]: a card whose edit has a later edit of yours on top
// says so instead of reverting a different edit; undo that one first. A later
// save in the same edition (C.20) is not a later edit: the card's undo takes
// the whole edition, as «Deshacer» does.
type CardUndo struct {
	Section   string
	Version   int
	Available bool
	Reason    string
}

// UndoOfCard reads a card's CardUndo. The card keeps its section's id in
// section_id (R4, B-49).
func UndoOfCard(ctx context.Context, q store.Querier, projectID string, card store.Object) (CardUndo, error) {
	sec, err := q.GetObject(ctx, projectID, bodyString(card, "section_id"))
	if err != nil {
		return CardUndo{}, err
	}
	events, err := q.EventsSince(ctx, projectID, 0)
	if err != nil {
		return CardUndo{}, err
	}
	undo, err := SectionUndo(ctx, q, projectID, sec.ID)
	if err != nil {
		return CardUndo{}, err
	}
	out := CardUndo{Section: sec.Label, Version: EditThatOpened(events, sec.ID, card.ID)}
	switch {
	case !undo.Available:
		out.Reason = undo.Reason
	case out.Version == 0:
		out.Reason = UndoReasonNoEdit
	case !undo.Covers(out.Version):
		out.Reason = UndoReasonLaterEdit
	default:
		out.Available = true
	}
	return out, nil
}

// Why an edit of yours looks accidental (rule 6, B-80).
const (
	SuspiciousParagraphDeleted = "paragraph_deleted"
	SuspiciousOverHalf         = "over_half"
	SuspiciousFigureAnchor     = "figure_anchor"
	SuspiciousCitationAnchor   = "citation_anchor"
	SuspiciousDecisionText     = "decision_text"
	SuspiciousSignedText       = "signed_text"
	SuspiciousOtherSection     = "other_section"
	SuspiciousPasted           = "pasted"
)

// SuspiciousReason is one motive of a suspicious_edit_alert: the rule of B-80,
// its kind, what it is about (literal: an anchor, the title of a section or a
// source), and, when it holds a decision, its label and statement.
type SuspiciousReason struct {
	Rule      int    `json:"rule"`
	Kind      string `json:"kind"`
	About     string `json:"about,omitempty"`
	Decision  string `json:"decision,omitempty"`
	Statement string `json:"statement,omitempty"`
}

// EditAlert is an alert for the band under the title: unanswered, on an edit
// not undone. Seq is its event's, what POST /alerts/{n}/answer takes.
// Undoable says «Deshacer» reverts that edit now, or UndoReason why not.
// [EL DISEÑO NO LO DICE]: an alert leaves the band when you undo its edit, as
// when you answer it; both stay in the history.
type EditAlert struct {
	Seq        int                `json:"seq"`
	Version    int                `json:"version"`
	Reasons    []SuspiciousReason `json:"reasons"`
	Undoable   bool               `json:"undoable"`
	UndoReason string             `json:"undo_reason,omitempty"`
}

// EditAlertData is the data of an alert's event; an answer has answer_to.
type EditAlertData struct {
	Section    string             `json:"section"`
	Version    int                `json:"version"`
	Reasons    []SuspiciousReason `json:"reasons"`
	AnswerTo   int                `json:"answer_to,omitempty"`
	Deliberate bool               `json:"deliberate,omitempty"`
}

// EditAlerts lists a section's alerts for the band, oldest first.
func EditAlerts(ctx context.Context, q store.Querier, projectID, sectionID string) ([]EditAlert, error) {
	events, err := q.EventsSince(ctx, projectID, 0)
	if err != nil {
		return nil, err
	}
	undo, err := SectionUndo(ctx, q, projectID, sectionID)
	if err != nil {
		return nil, err
	}
	answered, undone := map[int]bool{}, map[int]bool{}
	var alerts []EditAlert
	for _, ev := range events {
		if ev.ObjectID != sectionID {
			continue
		}
		switch ev.Type {
		case store.EventEditUndone:
			for _, n := range UndoneVersions(ev) {
				undone[n] = true
			}
		case store.EventSuspiciousEditAlert:
			var data EditAlertData
			_ = json.Unmarshal(ev.Data, &data)
			if data.AnswerTo != 0 {
				answered[data.AnswerTo] = true
				continue
			}
			alerts = append(alerts, EditAlert{Seq: ev.Seq, Version: data.Version, Reasons: data.Reasons})
		}
	}
	out := []EditAlert{}
	for _, a := range alerts {
		if answered[a.Seq] || undone[a.Version] {
			continue
		}
		switch {
		case !undo.Available:
			a.UndoReason = undo.Reason
		case !undo.Covers(a.Version):
			a.UndoReason = UndoReasonLaterEdit
		default:
			a.Undoable = true
		}
		out = append(out, a)
	}
	return out, nil
}

// BlockDecisions are the current decisions a block's history shows (B-10): the
// ones that cover its figures and the ones the steps in it declared for the
// section. Rule 6 reads them to name what a removed block held (B-80).
func BlockDecisions(ctx context.Context, q store.Querier, projectID, sectionID, block string) ([]DecisionEntry, error) {
	events, err := q.EventsSince(ctx, projectID, 0)
	if err != nil {
		return nil, err
	}
	b := Compare(block)
	var steps []string
	for _, ev := range events {
		if ev.ObjectID != sectionID || ev.Actor != "agent" || ev.TurnID == "" || ev.NewVersion == 0 ||
			ev.Type != store.EventEdit && !(ev.Type == store.EventStateChanged && ev.NewVersion == 1) {
			continue
		}
		if stepInBlock(eventData(ev), b) {
			steps = append(steps, ev.TurnID)
		}
	}
	entries, err := blockDecisions(ctx, q, projectID, sectionID, b, events, steps)
	if err != nil {
		return nil, err
	}
	var out []DecisionEntry
	for _, e := range entries {
		if e.Decision != nil && !e.Decision.Discarded {
			out = append(out, *e.Decision)
		}
	}
	return out, nil
}
