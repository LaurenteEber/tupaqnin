package cycle

import (
	"context"
	"errors"
	"time"

	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// Card options (B-16): keep the decision, or take the other option.
const (
	OptionKeep  = "keep"
	OptionOther = "other"
)

var (
	ErrCardNotFound  = errors.New("cycle: card not found")
	ErrCardNotOpen   = errors.New("cycle: card is not open")
	ErrCardObsolete  = errors.New("cycle: the card's decision changed")
	ErrInvalidOption = errors.New("cycle: invalid card option")
	// ErrInvalidReason: the «porque» is empty, only signs or an assent;
	// answer_without_reason is recorded and the card stays open.
	ErrInvalidReason = errors.New("cycle: the reason does not count")
)

// CardAnswer is what answering a card did.
type CardAnswer struct {
	Card        string // label
	Decision    string // label of the card's decision
	NewDecision string // label of the human decision born with "other" (B-16)
}

// AnswerCard answers an open card (estados §2 and §3, fichas §3
// «Tarjeta válida»). keep leaves the decision confirmed; other leaves it
// superseded and gives birth to the person's decision (B-16). Every
// answer records act_timed (K-11); a valid one records decision_seen
// with card (M-1); one without a valid «porque» records
// answer_without_reason and leaves the card open.
func (g *Gate) AnswerCard(ctx context.Context, projectID, cardLabel, option, reason string) (CardAnswer, error) {
	card, err := g.db.GetObject(ctx, projectID, cardLabel)
	if err != nil || card.Type != store.ObjectCard {
		return CardAnswer{}, ErrCardNotFound
	}
	if card.State != "open" {
		return CardAnswer{}, ErrCardNotOpen
	}
	if option != OptionKeep && option != OptionOther {
		return CardAnswer{}, ErrInvalidOption
	}
	opened, err := g.cardOpenedAt(ctx, projectID, card.ID)
	if err != nil {
		return CardAnswer{}, err
	}
	actTimed := store.EventInput{Type: store.EventActTimed, Actor: "human", ObjectID: card.ID,
		Data: map[string]any{"act": "card", "label": card.Label,
			"duration_ms": time.Since(opened).Milliseconds()}}

	if !ValidReason(reason) {
		err := g.db.Write(ctx, projectID, func(tx *store.Tx) error {
			if _, err := tx.AppendEvent(store.EventInput{Type: store.EventAnswerWithoutReason,
				Actor: "human", ObjectID: card.ID, Data: map[string]any{"label": card.Label, "option": option}}); err != nil {
				return err
			}
			_, err := tx.AppendEvent(actTimed)
			return err
		})
		if err != nil {
			return CardAnswer{}, err
		}
		return CardAnswer{}, ErrInvalidReason
	}

	var answer CardAnswer
	err = g.db.Write(ctx, projectID, func(tx *store.Tx) error {
		return TrackCriticality(ctx, tx, "", func() error {
			card, err := tx.GetObject(ctx, projectID, card.ID)
			if err != nil {
				return err
			}
			if card.State != "open" {
				return ErrCardNotOpen
			}
			cardBody := bodyOf(card)
			dec, err := tx.GetObject(ctx, projectID, bodyStr(cardBody, "decision_id"))
			if err != nil {
				return err
			}
			answer = CardAnswer{Card: card.Label, Decision: dec.Label}
			if dec.State != "agent" {
				return ErrCardObsolete
			}
			human := func(t store.EventType, data map[string]any) store.EventInput {
				return store.EventInput{Type: t, Actor: "human", Reason: reason, Data: data}
			}
			if option == OptionKeep {
				if _, err := tx.UpdateObject(dec.ID, dec.CurrentVersion, "confirmed", bodyOf(dec),
					human(store.EventStateChanged, map[string]any{"label": dec.Label, "from": "agent", "to": "confirmed", "via": "card"})); err != nil {
					return err
				}
			} else {
				born, err := chooseOther(ctx, tx, dec, reason)
				if err != nil {
					return err
				}
				answer.NewDecision = born.Label
			}
			cardBody["chosen_option"], cardBody["reason"] = option, reason
			if _, err := tx.UpdateObject(card.ID, card.CurrentVersion, "answered", cardBody,
				human(store.EventStateChanged, map[string]any{"label": card.Label, "from": "open", "to": "answered"})); err != nil {
				return err
			}
			if _, err := tx.AppendEvent(store.EventInput{Type: store.EventDecisionSeen, Actor: "human",
				ObjectID: dec.ID, Data: map[string]any{"label": dec.Label, "seen_via": "card"}}); err != nil {
				return err
			}
			_, err = tx.AppendEvent(actTimed)
			return err
		})
	})
	if errors.Is(err, ErrCardObsolete) {
		// The decision changed before the answer: the card closes as
		// obsolete (estados §3).
		werr := g.db.Write(ctx, projectID, func(tx *store.Tx) error {
			return TrackCriticality(ctx, tx, "", func() error { return nil })
		})
		if werr != nil {
			return CardAnswer{}, werr
		}
		return answer, ErrCardObsolete
	}
	if err != nil {
		return CardAnswer{}, err
	}
	return answer, nil
}

// chooseOther is B-16: the person's decision is born from the card's;
// its statement is the previous «en vez de», its «en vez de» the
// previous statement, its «porque» the person's, and its «elegiría si»
// the previous one when there was one. covers and used_in move to it
// (B-8: covers changes its end; used_in leaves from the new decision, so
// the old edge ends and a new one is created).
func chooseOther(ctx context.Context, tx *store.Tx, old store.Object, reason string) (store.Object, error) {
	ob := bodyOf(old)
	assumption, _ := ob["assumption"].(bool)
	// [EL DISEÑO NO LO DICE]: whether the person's decision keeps
	// «supuesto: sí»; it keeps the previous value.
	born, err := tx.CreateObject(store.ObjectDecision, "human", map[string]any{
		"statement": bodyStr(ob, "instead_of"), "instead_of": bodyStr(ob, "statement"),
		"would_choose_if": bodyStr(ob, "would_choose_if"), "assumption": assumption,
		"origin": "human", "human_marked_critical": false, "reason": reason,
	}, store.EventInput{Type: store.EventStateChanged, Actor: "human", Reason: reason,
		Data: map[string]any{"via": "card", "replaces": old.Label}})
	if err != nil {
		return store.Object{}, err
	}
	if _, err := tx.UpdateObject(old.ID, old.CurrentVersion, "superseded", ob,
		store.EventInput{Type: store.EventStateChanged, Actor: "human", Reason: reason,
			Data: map[string]any{"label": old.Label, "from": "agent", "to": "superseded", "by": born.Label}}); err != nil {
		return store.Object{}, err
	}
	edge := store.EventInput{Type: store.EventEdge, Actor: "human"}
	if _, err := tx.AddEdge(store.EdgeSupersedes, born.ID, old.ID, "", edge); err != nil {
		return store.Object{}, err
	}
	in, err := tx.EdgesTo(ctx, tx.ProjectID(), old.ID)
	if err != nil {
		return store.Object{}, err
	}
	for _, e := range in {
		if e.Type == store.EdgeCovers {
			if _, err := tx.ChangeEdge(e.ID, born.ID, e.Anchor, edge); err != nil {
				return store.Object{}, err
			}
		}
	}
	out, err := tx.EdgesFrom(ctx, tx.ProjectID(), old.ID)
	if err != nil {
		return store.Object{}, err
	}
	for _, e := range out {
		if e.Type != store.EdgeUsedIn {
			continue
		}
		if err := tx.EndEdge(e.ID, edge); err != nil {
			return store.Object{}, err
		}
		if _, err := tx.AddEdge(store.EdgeUsedIn, born.ID, e.ToID, e.Anchor, edge); err != nil {
			return store.Object{}, err
		}
	}
	return born, nil
}

// cardOpenedAt is when the card was opened: the time of its first event.
func (g *Gate) cardOpenedAt(ctx context.Context, projectID, cardID string) (time.Time, error) {
	events, err := g.db.EventsSince(ctx, projectID, 0)
	if err != nil {
		return time.Time{}, err
	}
	for _, ev := range events {
		if ev.ObjectID == cardID {
			return parseStoredTime(ev.CreatedAt), nil
		}
	}
	return time.Now(), nil
}
