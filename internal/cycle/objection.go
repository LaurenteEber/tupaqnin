package cycle

import (
	"context"
	"errors"

	"github.com/LaurenteEber/tupaqnin/internal/checks"
	"github.com/LaurenteEber/tupaqnin/internal/graph"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// ErrDecisionNotYours: only a decision of yours or a confirmed one is revoked
// (estados §2, B-82).
var ErrDecisionNotYours = errors.New("cycle: only a decision of yours or a confirmed one is revoked")

// Objection is what objecting did: the «por aclarar» it opened.
type Objection struct {
	OpenQuestion string // label
}

// ObjectDecision is objecting (E-1, B-81): the decision goes to
// in_open_question and a «por aclarar» opens with the app's two options, in one
// write. Nothing but you reaches it: no output field or tool of the agent does.
// A signed section that uses it goes to «cambió» (estados §1); an open card of
// the decision becomes obsolete (estados §3), through TrackCriticality.
// [EL DISEÑO NO LO DICE]: whether the motive must pass T-1 and what a refused
// one leaves: B-81 says «con tu motivo», so it is validated as B-82's «porque»
// and a refusal writes nothing.
// [EL DISEÑO NO LO DICE]: whether a signoff in progress closes. B-14 lists left,
// edited and missing_in_text, and an objection changes no text: it stays open
// and Sign refuses it while the question is pending, like a card answered during
// a signoff. It closes nothing, so SignoffClosed does not fire.
func (g *Gate) ObjectDecision(ctx context.Context, projectID, label, reason string) (Objection, error) {
	var out Objection
	err := g.db.Write(ctx, projectID, func(tx *store.Tx) error {
		dec, err := currentDecisionForAct(ctx, tx, label)
		if err != nil {
			return err
		}
		if !graph.CanObject(dec.State) {
			return ErrDecisionNotCurrent
		}
		if !ValidReason(reason) {
			return ErrInvalidReason
		}
		return TrackCriticality(ctx, tx, "", func() error {
			oq, err := checks.OpenObjection(ctx, tx, dec, reason)
			if err != nil {
				return err
			}
			out.OpenQuestion = oq.Label
			return nil
		})
	})
	if err != nil {
		return Objection{}, err
	}
	return out, nil
}

// RevokeDecision is revoking (B-82, H-3): only yours or a confirmed one; it stays
// revoked, without a successor, and the sections that use it get their notice
// (2.2: «cambió» or «escrita con lo anterior»). Without a valid «porque» nothing
// changes.
// [EL DISEÑO NO LO DICE]: what the edges of a revoked decision do. They stay: the
// notice is what asks you to rewrite, and a figure it covers still counts as
// covered until the text changes. The signoff in progress: as in ObjectDecision,
// Sign refuses it by its open change.
func (g *Gate) RevokeDecision(ctx context.Context, projectID, label, reason string) error {
	return g.db.Write(ctx, projectID, func(tx *store.Tx) error {
		dec, err := currentDecisionForAct(ctx, tx, label)
		if err != nil {
			return err
		}
		if !graph.CanRevoke(dec.State) {
			return ErrDecisionNotYours
		}
		if !ValidReason(reason) {
			return ErrInvalidReason
		}
		return TrackCriticality(ctx, tx, "", func() error {
			if _, err := tx.UpdateObject(dec.ID, dec.CurrentVersion, "revoked", bodyOf(dec),
				store.EventInput{Type: store.EventStateChanged, Actor: "human", Reason: reason,
					Data: map[string]any{"label": dec.Label, "from": dec.State, "to": "revoked"}}); err != nil {
				return err
			}
			_, err := checks.Warn(ctx, tx, "human", "", []string{dec.ID}, nil)
			return err
		})
	})
}

func currentDecisionForAct(ctx context.Context, tx *store.Tx, label string) (store.Object, error) {
	dec, err := tx.GetObject(ctx, tx.ProjectID(), label)
	if errors.Is(err, store.ErrNotFound) || err == nil && dec.Type != store.ObjectDecision {
		return store.Object{}, ErrDecisionNotFound
	}
	return dec, err
}

// answerObjection is the answer to an objection's «por aclarar» (B-81, estados
// §2): «mantener» leaves the decision yours, with your «porque»; «la otra» makes
// a decision of yours that replaces it, whoever's it was. Only this answer moves
// a decision out of in_open_question.
func answerObjection(ctx context.Context, tx *store.Tx, dec store.Object, keep bool, chosen, reason string, answer *OpenQuestionAnswer) error {
	if !keep {
		born, err := replaceByYours(ctx, tx, dec, chosen, reason, "open_question")
		if err != nil {
			return err
		}
		answer.NewDecision = born.Label
		return nil
	}
	body := bodyOf(dec)
	body["reason"], body[graph.KeptReasonField] = reason, true
	_, err := tx.UpdateObject(dec.ID, dec.CurrentVersion, "human", body,
		store.EventInput{Type: store.EventStateChanged, Actor: "human", Reason: reason,
			Data: map[string]any{"label": dec.Label, "from": dec.State, "to": "human", "via": "open_question"}})
	return err
}
