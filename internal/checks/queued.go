package checks

import (
	"context"
	"strings"

	"github.com/LaurenteEber/tupaqnin/internal/graph"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// QueueProbe queues, in the transaction that changed what is protected,
// the probe of protected against the current text of the change's
// sections (P-72): the probe of a new premise (B-46, estados §6, fichas
// §3) and, in 2.8, of a new criterion. It is a probe event with kind
// queued, by the app, with the labels of what is protected and of the
// sections; the passages are not kept, because the probe goes against
// the text current when it runs. A change with no work queues nothing.
// While it is pending its sections are «sin comprobar» (B-50,
// graph.SectionStatus); the runner sends it when its slot is free, when
// the turn ends or at the next B-36 trigger (QueuedProbes).
func QueueProbe(ctx context.Context, tx *store.Tx, protectedID string, change Change) error {
	if !change.HasWork() {
		return nil
	}
	prot, err := tx.GetObject(ctx, tx.ProjectID(), protectedID)
	if err != nil {
		return err
	}
	sections, err := sectionLabels(ctx, tx, tx.ProjectID(), change)
	if err != nil {
		return err
	}
	_, err = tx.AppendEvent(store.EventInput{Type: store.EventProbe, Actor: "app", ObjectID: prot.ID,
		Data: map[string]any{"kind": graph.ProbeKindQueued, "protected": prot.Label, "queued_sections": sections}})
	return err
}

// QueuedProbes are the changes of the queued probes still pending
// (P-72, graph.PendingProbes), in the order they were queued: each
// passage is the current text of one of its sections, as in
// PremiseProbe. A section that has no text now is left out.
// [EL DISEÑO NO LO DICE]: a queued probe whose sections all have no
// text has no work and is not sent; it stays pending until one of them
// has text again.
func QueuedProbes(ctx context.Context, q store.Querier, projectID string) ([]Change, error) {
	events, err := q.EventsSince(ctx, projectID, 0)
	if err != nil {
		return nil, err
	}
	var out []Change
	for _, qp := range graph.PendingProbes(events) {
		change := Change{Kind: YourEdit, Actor: "human", Queued: qp.Seq}
		for _, label := range qp.Sections {
			sec, err := q.GetObject(ctx, projectID, label)
			if err != nil {
				return nil, err
			}
			text := bodyText(sec, "text")
			if strings.TrimSpace(text) == "" {
				continue
			}
			change.Sections = append(change.Sections, sec.ID)
			change.Passages = append(change.Passages, Passage{Section: sec.ID, Text: text})
		}
		out = append(out, change)
	}
	return out, nil
}
