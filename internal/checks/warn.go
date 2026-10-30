package checks

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/LaurenteEber/tupaqnin/internal/graph"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// FindingUsageChange is rule 4: a change entered a section's list
// (A-1). Section is the section's label, About the label of what
// changed, Change the change's label (chg-n).
const FindingUsageChange = "usage_change"

// Results of a usage change (glossary: usage_change).
const (
	UsageEntered           = "entered"
	UsageUpdated           = "updated"
	UsageReviewedUnchanged = "reviewed_unchanged"
)

// Warn is rule 4 of flujos §1 (warnings to other sections, A-1, B-45),
// inside the transaction tx that changed the content of each object in
// changed (a decision, a figure or a source). By the edges it finds the
// sections that used another content: the sections a decision is
// used_in; for a figure, the sections where the decision that covers it
// is used_in, besides the figure's own; the sections that use a source.
// Each gets a change (chg-n) in its open_changes with usage_change
// entered; a signed section goes to changed_since_signoff, a draft to
// stale. The using edge then records what it now points at, so the
// same content never enters twice. exclude are section ids the change
// itself wrote: they are not «otras secciones» (A-1).
// [EL DISEÑO NO LO DICE]: whether the section the change wrote gets
// the warning; it does not, it was written with the new content.
func Warn(ctx context.Context, tx *store.Tx, actor, turnID string, changed []string, exclude map[string]bool) ([]Finding, error) {
	projectID := tx.ProjectID()
	var out []Finding
	for _, id := range changed {
		obj, err := tx.GetObject(ctx, projectID, id)
		if err != nil {
			return nil, err
		}
		users, err := usersOf(ctx, tx, obj)
		if err != nil {
			return nil, err
		}
		edgeEv := store.EventInput{Type: store.EventEdge, Actor: actor, TurnID: turnID}
		warned := map[string]bool{}
		another := map[string]bool{} // edge id → it used another content
		for _, u := range users {
			if _, seen := another[u.edge.ID]; !seen {
				another[u.edge.ID] = graph.UsedAnother(u.edge, obj)
				if another[u.edge.ID] {
					if _, err := tx.ChangeEdge(u.edge.ID, u.edge.ToID, u.edge.Anchor, graph.UsageOf(obj), edgeEv); err != nil {
						return nil, err
					}
				}
			}
			if !another[u.edge.ID] {
				continue
			}
			if exclude[u.section] || warned[u.section] {
				continue
			}
			warned[u.section] = true
			f, err := enter(ctx, tx, actor, turnID, u.section, obj)
			if err != nil {
				return nil, err
			}
			out = append(out, f)
		}
	}
	return out, nil
}

type user struct {
	edge    store.Edge // the using edge, whose data is compared
	section string
}

// usersOf lists the using edges of obj with the section that uses it.
func usersOf(ctx context.Context, tx *store.Tx, obj store.Object) ([]user, error) {
	projectID := tx.ProjectID()
	var out []user
	switch obj.Type {
	case store.ObjectDecision:
		edges, err := tx.EdgesFrom(ctx, projectID, obj.ID)
		if err != nil {
			return nil, err
		}
		for _, e := range edges {
			if e.Type == store.EdgeUsedIn {
				out = append(out, user{edge: e, section: e.ToID})
			}
		}
	case store.ObjectFigure:
		own := bodyString(obj, "section_id")
		covers, err := tx.EdgesFrom(ctx, projectID, obj.ID)
		if err != nil {
			return nil, err
		}
		for _, c := range covers {
			if c.Type != store.EdgeCovers {
				continue
			}
			used, err := tx.EdgesFrom(ctx, projectID, c.ToID)
			if err != nil {
				return nil, err
			}
			for _, e := range used {
				if e.Type == store.EdgeUsedIn && e.ToID != own {
					out = append(out, user{edge: c, section: e.ToID})
				}
			}
		}
	case store.ObjectSource:
		edges, err := tx.EdgesTo(ctx, projectID, obj.ID)
		if err != nil {
			return nil, err
		}
		for _, e := range edges {
			if e.Type == store.EdgeUses {
				out = append(out, user{edge: e, section: e.FromID})
			}
		}
	}
	return out, nil
}

// enter adds the change to the section's list and moves its state
// (estados §1): signed → changed_since_signoff, draft → stale; stale
// and changed_since_signoff stay.
func enter(ctx context.Context, tx *store.Tx, actor, turnID, sectionID string, obj store.Object) (Finding, error) {
	sec, err := tx.GetObject(ctx, tx.ProjectID(), sectionID)
	if err != nil {
		return Finding{}, err
	}
	label, err := tx.NewChangeLabel()
	if err != nil {
		return Finding{}, err
	}
	var body map[string]any
	_ = json.Unmarshal(sec.Body, &body)
	if body == nil {
		body = map[string]any{}
	}
	list := graph.OpenChanges(sec)
	list = append(list, graph.OpenChange{Label: label, Kind: string(obj.Type), About: obj.Label})
	body["open_changes"] = list
	state := sec.State
	switch state {
	case "signed":
		state = "changed_since_signoff"
	case "draft":
		state = "stale"
	}
	data := map[string]any{"kind": UsageEntered, "change": label, "section": sec.Label,
		"about": obj.Label, "about_kind": string(obj.Type)}
	updated, err := tx.UpdateObject(sec.ID, sec.CurrentVersion, state, body,
		store.EventInput{Type: store.EventUsageChange, Actor: actor, TurnID: turnID, Data: data})
	if err != nil && !errors.Is(err, store.ErrNoChange) {
		return Finding{}, err
	}
	if state != sec.State {
		if _, err := tx.AppendEvent(store.EventInput{Type: store.EventStateChanged, Actor: actor, TurnID: turnID,
			ObjectID: sec.ID, BaseVersion: sec.CurrentVersion, NewVersion: updated.CurrentVersion,
			Data: map[string]any{"label": sec.Label, "from": sec.State, "to": state,
				"why": "usage_change", "change": label}}); err != nil {
			return Finding{}, err
		}
	}
	return Finding{Rule: 4, Kind: FindingUsageChange, Section: sec.Label, About: obj.Label, Change: label}, nil
}
