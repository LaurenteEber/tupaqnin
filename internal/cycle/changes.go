package cycle

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/LaurenteEber/tupaqnin/internal/checks"
	"github.com/LaurenteEber/tupaqnin/internal/graph"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

var (
	// ErrChangeNotFound: no section has that change in its list.
	ErrChangeNotFound = errors.New("cycle: change not found")
	// ErrInvalidResult: a close is updated or reviewed_unchanged.
	ErrInvalidResult = errors.New("cycle: invalid change result")
	// ErrSectionNotEdited: you mark a change updated only after editing
	// that section (fichas §3, «Cierre de un cambio»).
	ErrSectionNotEdited = errors.New("cycle: the section was not edited after the change")
)

// sectionOfChange finds the section whose list holds the change label.
func sectionOfChange(ctx context.Context, tx *store.Tx, label string) (store.Object, graph.OpenChange, bool, error) {
	sections, err := tx.ListObjects(ctx, tx.ProjectID(), store.ObjectSection)
	if err != nil {
		return store.Object{}, graph.OpenChange{}, false, err
	}
	for _, sec := range sections {
		for _, c := range graph.OpenChanges(sec) {
			if c.Label == label {
				return sec, c, true, nil
			}
		}
	}
	return store.Object{}, graph.OpenChange{}, false, nil
}

// closeChange removes the change from the section's list with
// usage_change (updated or reviewed_unchanged, with its «porque»); with
// the list empty, a stale section goes back to draft (estados §1). A
// section in changed_since_signoff stays there: it needs a new signoff.
func closeChange(ctx context.Context, tx *store.Tx, sec store.Object, change graph.OpenChange,
	result, reason, actor, turnID string) error {
	var body map[string]any
	_ = json.Unmarshal(sec.Body, &body)
	rest := []graph.OpenChange{}
	for _, c := range graph.OpenChanges(sec) {
		if c.Label != change.Label {
			rest = append(rest, c)
		}
	}
	body["open_changes"] = rest
	state := sec.State
	if state == "stale" && len(rest) == 0 {
		state = "draft"
	}
	data := map[string]any{"kind": result, "change": change.Label, "section": sec.Label, "about": change.About}
	if reason != "" {
		data["reason"] = reason
	}
	ev := store.EventInput{Type: store.EventUsageChange, Actor: actor, TurnID: turnID, Reason: reason, Data: data}
	if actor == "agent" {
		ev.AgentRole = "principal"
	}
	updated, err := tx.UpdateObject(sec.ID, sec.CurrentVersion, state, body, ev)
	if err != nil {
		return err
	}
	if state != sec.State {
		_, err = tx.AppendEvent(store.EventInput{Type: store.EventStateChanged, Actor: actor, TurnID: turnID,
			ObjectID: sec.ID, BaseVersion: sec.CurrentVersion, NewVersion: updated.CurrentVersion,
			Data: map[string]any{"label": sec.Label, "from": sec.State, "to": state,
				"why": "open_changes_closed"}})
	}
	return err
}

// CloseChange is your close of a change (POST /changes/{label}/close,
// fichas §3 «Cierre de un cambio»): updated after editing that section,
// or reviewed_unchanged with a valid «porque» (T-1, fichas §3 «Tarjeta
// válida»). An invalid one closes nothing.
// [EL DISEÑO NO LO DICE]: «después de editar» is read as an edit of
// yours on that section after the change entered its list.
func (g *Gate) CloseChange(ctx context.Context, projectID, label, result, reason string) error {
	switch result {
	case checks.UsageUpdated:
	case checks.UsageReviewedUnchanged:
		if !ValidReason(reason) {
			return ErrInvalidReason
		}
	default:
		return ErrInvalidResult
	}
	return g.db.Write(ctx, projectID, func(tx *store.Tx) error {
		sec, change, ok, err := sectionOfChange(ctx, tx, label)
		if err != nil {
			return err
		}
		if !ok {
			return ErrChangeNotFound
		}
		if result == checks.UsageUpdated {
			edited, err := editedAfterEntered(ctx, tx, sec.ID, label)
			if err != nil {
				return err
			}
			if !edited {
				return ErrSectionNotEdited
			}
		}
		if result == checks.UsageUpdated {
			reason = ""
		}
		return closeChange(ctx, tx, sec, change, result, reason, "human", "")
	})
}

// editedAfterEntered reports an edit of yours on the section after the
// usage_change that entered the change.
func editedAfterEntered(ctx context.Context, tx *store.Tx, sectionID, label string) (bool, error) {
	events, err := tx.EventsSince(ctx, tx.ProjectID(), 0)
	if err != nil {
		return false, err
	}
	entered := 0
	for _, ev := range events {
		var data map[string]any
		_ = json.Unmarshal(ev.Data, &data)
		switch {
		case ev.Type == store.EventUsageChange && data["kind"] == checks.UsageEntered && data["change"] == label:
			entered = ev.Seq
		case entered > 0 && ev.Type == store.EventEdit && ev.Actor == "human" && ev.ObjectID == sectionID:
			return true, nil
		}
	}
	return false, nil
}

// handledChange is one entry of the principal's handled_changes.
type handledChange struct {
	ChangeID string `json:"change_id"`
	Result   string `json:"result"`
	Reason   string `json:"reason"`
}

// applyHandledChanges is rule 7 of the gate (specification §3): each
// change the step declares handled is in the list of a section; updated
// needs the step to write that section, reviewed_unchanged a valid
// «porque». A close that does not comply is rejected alone, with a
// message for the agent; the rest of the step applies. written are the
// section ids the step wrote.
func applyHandledChanges(ctx context.Context, tx *store.Tx, turnID string, raw json.RawMessage,
	written map[string]bool) (rejected []map[string]any, err error) {
	var handled []handledChange
	_ = json.Unmarshal(raw, &handled)
	for _, h := range handled {
		reject := func(format string, args ...any) {
			rejected = append(rejected, map[string]any{"change_id": h.ChangeID, "result": h.Result,
				"message": fmt.Sprintf(format, args...)})
		}
		sec, change, ok, err := sectionOfChange(ctx, tx, h.ChangeID)
		if err != nil {
			return nil, err
		}
		switch {
		case !ok:
			reject("%s is not in the list of changes of any section: name a change from a section's open changes", h.ChangeID)
			continue
		case h.Result == checks.UsageUpdated && !written[sec.ID]:
			reject("%s was declared updated, but your step did not write %s: write in %s, or declare it reviewed_unchanged with a reason",
				h.ChangeID, sec.Label, sec.Label)
			continue
		case h.Result == checks.UsageReviewedUnchanged && !ValidReason(h.Reason):
			reject("%s was declared reviewed_unchanged without a valid reason: say why %s does not need a change",
				h.ChangeID, sec.Label)
			continue
		}
		reason := h.Reason
		if h.Result == checks.UsageUpdated {
			reason = ""
		}
		if err := closeChange(ctx, tx, sec, change, h.Result, reason, "agent", turnID); err != nil {
			return nil, err
		}
	}
	return rejected, nil
}
