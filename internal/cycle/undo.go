package cycle

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/LaurenteEber/tupaqnin/internal/graph"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// Undo and the answer to a suspicious edit (backlog-r4 4.2, G-1, B-78 to B-80).
// An undo never edits nor deletes: it is a new version with its edit_undone
// event, conditioned on the version read, in one transaction (rule 6). Its
// decisions and the proposal its edit displaced do not move; no model runs.

var (
	// ErrAlertNotFound: that seq is not an alert of this project.
	ErrAlertNotFound = errors.New("cycle: alert not found")
	// ErrAlertAnswered: the alert already has its answer.
	ErrAlertAnswered = errors.New("cycle: alert already answered")
)

// UndoRefused is «Deshacer» off, with its reason (B-79).
type UndoRefused struct{ Reason string }

func (e UndoRefused) Error() string { return "cycle: undo unavailable: " + e.Reason }

// UndoOutcome is what an undo left. SignoffCancelled says it closed a signoff.
type UndoOutcome struct {
	Label            string
	Version          int
	Status           string
	RestoredVersion  int
	SignoffCancelled bool
}

// UndoEdit is «Deshacer» on a section: it restores the text your last edit
// reverts to (graph.SectionUndo, the one answer). A displaced edit goes back to
// the version your tab read, not to the agent's it was saved over: that text
// and its figures and citations stay in the open proposal, which accepting
// brings back (B-73 d, B-79). editVersion, when it is not 0, is the edit you
// mean (a card's, an alert's): under a later one it is refused.
func (g *Gate) UndoEdit(ctx context.Context, projectID, label string, editVersion int) (UndoOutcome, error) {
	var out UndoOutcome
	stoppedBy := ""
	if g.StoppedBy != nil {
		var err error
		if stoppedBy, err = g.StoppedBy(ctx, projectID); err != nil {
			return out, err
		}
	}
	err := g.db.Write(ctx, projectID, func(tx *store.Tx) error {
		sec, err := tx.GetObject(ctx, projectID, label)
		if errors.Is(err, store.ErrNotFound) || err == nil && sec.Type != store.ObjectSection {
			return ErrNotASection
		}
		if err != nil {
			return err
		}
		undo, err := graph.SectionUndo(ctx, tx, projectID, sec.ID)
		if err != nil {
			return err
		}
		if !undo.Available {
			return UndoRefused{Reason: undo.Reason}
		}
		if editVersion != 0 && editVersion != undo.Version {
			return UndoRefused{Reason: graph.UndoReasonLaterEdit}
		}
		restored, err := textAtVersion(ctx, tx, sec.ID, undo.Restores)
		if err != nil {
			return err
		}
		body := bodyOf(sec)
		before := bodyStr(body, "text")
		body["text"] = restored
		// An edit from before the signing: undoing it changes what you signed,
		// like any edit of yours (estados §1).
		state := sec.State
		if state == "signed" {
			state = "changed_since_signoff"
		}
		updated, err := tx.UpdateObject(sec.ID, sec.CurrentVersion, state, body, store.EventInput{
			Type: store.EventEditUndone, Actor: "human",
			Data: map[string]any{"section": sec.Label, "undone_version": undo.Version, "restored_version": undo.Restores}})
		if err != nil {
			return err
		}
		if err := obsoleteCardsOfEdit(ctx, tx, sec.ID, undo.Version); err != nil {
			return err
		}
		// The restored version passes rules 1 to 5 like any edit (flujos §1),
		// and not rule 6: that one is on your edits (B-80).
		if _, err := CheckYourEdit(ctx, tx, sec.ID, before); err != nil {
			return err
		}
		signing, inSignoff, err := graph.ActiveSignoff(ctx, tx, projectID, sec.ID)
		if err != nil {
			return err
		}
		if inSignoff {
			out.SignoffCancelled = true
			if err := CancelSignoff(tx, signing, graph.CancelEdited, "human", map[string]any{"section": sec.Label}); err != nil {
				return err
			}
		}
		// A-1: «Actualizado» rested on the edit you undid.
		if updated, err = reopenUndoneUpdates(ctx, tx, updated); err != nil {
			return err
		}
		if updated, err = backToSigned(ctx, tx, updated, stoppedBy); err != nil {
			return err
		}
		status, _, err := graph.SectionStatus(ctx, tx, projectID, sec.Label)
		if err != nil {
			return err
		}
		out.Label, out.Version, out.RestoredVersion, out.Status = sec.Label, updated.CurrentVersion, undo.Restores, string(status)
		return nil
	})
	return out, err
}

func textAtVersion(ctx context.Context, tx *store.Tx, objectID string, version int) (string, error) {
	versions, err := tx.Versions(ctx, tx.ProjectID(), objectID)
	if err != nil {
		return "", err
	}
	for _, v := range versions {
		if v.Version == version {
			var b map[string]any
			if err := json.Unmarshal(v.Body, &b); err != nil {
				return "", err
			}
			return bodyStr(b, "text"), nil
		}
	}
	return "", store.ErrNotFound
}

// obsoleteCardsOfEdit: the open R4 card of that edit goes to obsolete; its
// decision did not change (B-49, estados §3).
func obsoleteCardsOfEdit(ctx context.Context, tx *store.Tx, sectionID string, version int) error {
	cards, err := tx.ListObjects(ctx, tx.ProjectID(), store.ObjectCard)
	if err != nil {
		return err
	}
	events, err := tx.EventsSince(ctx, tx.ProjectID(), 0)
	if err != nil {
		return err
	}
	for _, card := range cards {
		b := bodyOf(card)
		if card.State != "open" || bodyStr(b, "kind") != CardOwnEdit || bodyStr(b, "section_id") != sectionID ||
			graph.EditThatOpened(events, sectionID, card.ID) != version {
			continue
		}
		if _, err := tx.UpdateObject(card.ID, card.CurrentVersion, "obsolete", b, store.EventInput{
			Type: store.EventStateChanged, Actor: "app",
			Data: map[string]any{"label": card.Label, "from": "open", "to": "obsolete", "why": "edit_undone"}}); err != nil {
			return err
		}
	}
	return nil
}

// backToSigned is B-78: the section equals its current signature again and
// nothing is pending nor open, so it is signed without signing anew. Equal is by
// components (specification §4): text, critical decisions, criteria and sources;
// anything else stays in «cambió», which needs a new signoff. Pendings are the
// signoff's own: at a cap, the agent's do not count (B3-20).
func backToSigned(ctx context.Context, tx *store.Tx, sec store.Object, stoppedBy string) (store.Object, error) {
	if sec.State != "changed_since_signoff" || len(graph.OpenChanges(sec)) > 0 {
		return sec, nil
	}
	signed, ok, err := graph.CurrentSignoff(ctx, tx, tx.ProjectID(), sec.ID)
	if err != nil || !ok {
		return sec, err
	}
	same, err := graph.EqualsSignature(ctx, tx, tx.ProjectID(), sec, signed)
	if err != nil || !same {
		return sec, err
	}
	_, pendings, err := graph.SectionStatus(ctx, tx, tx.ProjectID(), sec.Label)
	if err != nil {
		return sec, err
	}
	if blocking, _ := graph.PendingsAtCap(pendings, stoppedBy); len(blocking) > 0 {
		return sec, nil
	}
	return tx.UpdateObject(sec.ID, sec.CurrentVersion, "signed", bodyOf(sec), store.EventInput{
		Type: store.EventStateChanged, Actor: "app",
		Data: map[string]any{"label": sec.Label, "from": sec.State, "to": "signed", "via": "edit_undone"}})
}

// AnswerAlert is «Fue a propósito»: another suspicious_edit_alert with
// answer_to and deliberate; the alert's event stays (history only grows). seq
// is the alert's event in this project; anything else is not found.
func (g *Gate) AnswerAlert(ctx context.Context, projectID string, seq int) error {
	return g.db.Write(ctx, projectID, func(tx *store.Tx) error {
		if seq < 1 {
			return ErrAlertNotFound
		}
		events, err := tx.EventsSince(ctx, projectID, seq-1)
		if err != nil {
			return err
		}
		if len(events) == 0 || events[0].Seq != seq || events[0].Type != store.EventSuspiciousEditAlert {
			return ErrAlertNotFound
		}
		alert := events[0]
		var data graph.EditAlertData
		if err := json.Unmarshal(alert.Data, &data); err != nil {
			return err
		}
		if data.AnswerTo != 0 {
			return ErrAlertNotFound
		}
		for _, ev := range events[1:] {
			if ev.Type != store.EventSuspiciousEditAlert {
				continue
			}
			var later graph.EditAlertData
			if err := json.Unmarshal(ev.Data, &later); err != nil {
				return err
			}
			if later.AnswerTo == seq {
				return ErrAlertAnswered
			}
		}
		_, err = tx.AppendEvent(store.EventInput{Type: store.EventSuspiciousEditAlert, Actor: "human", ObjectID: alert.ObjectID,
			Data: graph.EditAlertData{Section: data.Section, Version: data.Version, AnswerTo: seq, Deliberate: true}})
		return err
	})
}
