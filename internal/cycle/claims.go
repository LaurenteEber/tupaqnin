package cycle

import (
	"context"
	"errors"
	"strings"

	"github.com/LaurenteEber/tupaqnin/internal/graph"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// ErrPassageNotInSection: the accepted passage is not a paragraph of
// the section's current text.
var ErrPassageNotInSection = errors.New("cycle: the passage is not a paragraph of the section")

// AcceptClaim is you accepting a paragraph without a source as general
// knowledge (R-1, backlog-r2 2.11): with a valid «porque» (T-1) it
// records claim_accepted on the section, with the literal passage and
// your «porque», in the transaction that reads the section's current
// text. It holds while the passage stays literal in the section
// (graph.UnsourcedClaims).
// [EL DISEÑO NO LO DICE]: what passage counts; a whole paragraph of the
// current text (a block, B-10), so a short fragment cannot accept a
// paragraph that holds it.
func (g *Gate) AcceptClaim(ctx context.Context, projectID, sectionLabel, passage, reason string) error {
	if !ValidReason(reason) {
		return ErrInvalidReason
	}
	return g.db.Write(ctx, projectID, func(tx *store.Tx) error {
		sec, err := tx.GetObject(ctx, projectID, sectionLabel)
		if errors.Is(err, store.ErrNotFound) || (err == nil && sec.Type != store.ObjectSection) {
			return ErrSectionNotFound
		}
		if err != nil {
			return err
		}
		want := graph.CompareString(passage)
		found := false
		for _, block := range graph.Blocks(bodyStr(bodyOf(sec), "text")) {
			if strings.TrimSpace(want) != "" && graph.CompareString(block) == want {
				found = true
				break
			}
		}
		if !found {
			return ErrPassageNotInSection
		}
		_, err = tx.AppendEvent(store.EventInput{Type: store.EventClaimAccepted, Actor: "human", Reason: reason,
			ObjectID: sec.ID, BaseVersion: sec.CurrentVersion,
			Data: map[string]any{"label": sec.Label, "passage": passage}})
		return err
	})
}
