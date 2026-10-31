package cycle

import (
	"context"
	"errors"
	"time"

	"github.com/LaurenteEber/tupaqnin/internal/checks"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

var (
	ErrOpenQuestionNotFound = errors.New("cycle: open question not found")
	ErrOpenQuestionNotOpen  = errors.New("cycle: open question is not open")
	// ErrNoOption: the answer chooses no option (estados §4: «una
	// respuesta que no elige»); act_timed is recorded and it stays open.
	ErrNoOption = errors.New("cycle: the answer chooses no option")
)

// OpenQuestionAnswer is what answering a «por aclarar» did.
type OpenQuestionAnswer struct {
	OpenQuestion string // label
	// NewDecision is the label of your decision born when the question
	// blocked a decision of yours or a confirmed one and you chose
	// another option (estados §2, as B-16).
	NewDecision string
	// PremiseChanged: you changed the contradicted premise; its new
	// version is probed against the current text (B-46).
	PremiseChanged bool
	// Premise is the premise's label when the question kept or changed it.
	Premise string
}

// AnswerOpenQuestion answers an open «por aclarar» (estados §4, backlog-r2
// 2.4): option is the index in its list of options, from 0, and the
// «porque» must be valid (T-1, B-20 a, P-66). Every answer records
// act_timed (K-11); one without a valid «porque» records
// answer_without_reason and one without an option stays open too. A
// valid answer resolves it: the variants go to the history as
// discarded, its blocks edges end, and what depended on it is back to
// what SectionStatus computes (draft, or still stale or
// changed_since_signoff). The app creates no decision, except when its
// first option keeps a decision of yours or a confirmed one: choosing
// another replaces it by yours (estados §2). On the contradicted
// premise, the first option keeps it (back to current) and another
// creates its new version with the chosen option as its text (estados
// §6).
// [EL DISEÑO NO LO DICE]: which variants are «no elegidas»; variants
// are labelled strings with no link to an option, so all of them go to
// the resolved event as discarded_variants. The premise's new text is
// the chosen option, as a replaced decision's statement is.
func (g *Gate) AnswerOpenQuestion(ctx context.Context, projectID, label string, option *int, reason string) (OpenQuestionAnswer, error) {
	oq, err := g.db.GetObject(ctx, projectID, label)
	if err != nil || oq.Type != store.ObjectOpenQuestion {
		return OpenQuestionAnswer{}, ErrOpenQuestionNotFound
	}
	if oq.State != "open" {
		return OpenQuestionAnswer{}, ErrOpenQuestionNotOpen
	}
	body := bodyOf(oq)
	options, _ := body["options"].([]any)
	if option != nil && (*option < 0 || *option >= len(options)) {
		return OpenQuestionAnswer{}, ErrInvalidOption
	}
	opened, err := g.cardOpenedAt(ctx, projectID, oq.ID)
	if err != nil {
		return OpenQuestionAnswer{}, err
	}
	actTimed := store.EventInput{Type: store.EventActTimed, Actor: "human", ObjectID: oq.ID,
		Data: map[string]any{"act": "open_question", "label": oq.Label,
			"duration_ms": time.Since(opened).Milliseconds()}}

	if !ValidReason(reason) || option == nil {
		err := g.db.Write(ctx, projectID, func(tx *store.Tx) error {
			if !ValidReason(reason) {
				data := map[string]any{"label": oq.Label, "option": nil}
				if option != nil {
					data["option"] = *option
				}
				if _, err := tx.AppendEvent(store.EventInput{Type: store.EventAnswerWithoutReason,
					Actor: "human", ObjectID: oq.ID, Data: data}); err != nil {
					return err
				}
			}
			_, err := tx.AppendEvent(actTimed)
			return err
		})
		if err != nil {
			return OpenQuestionAnswer{}, err
		}
		if !ValidReason(reason) {
			return OpenQuestionAnswer{}, ErrInvalidReason
		}
		return OpenQuestionAnswer{}, ErrNoOption
	}

	answer := OpenQuestionAnswer{OpenQuestion: oq.Label}
	err = g.db.Write(ctx, projectID, func(tx *store.Tx) error {
		return TrackCriticality(ctx, tx, "", func() error {
			oq, err := tx.GetObject(ctx, projectID, oq.ID)
			if err != nil {
				return err
			}
			if oq.State != "open" {
				return ErrOpenQuestionNotOpen
			}
			body := bodyOf(oq)
			options, _ := body["options"].([]any)
			chosen, _ := options[*option].(map[string]any)
			chosenText := bodyStr(chosen, "option")
			keep, _ := chosen["keep"].(bool)
			if err := g.applyKept(ctx, tx, body, keep, chosenText, reason, &answer); err != nil {
				return err
			}
			body["chosen_option"], body["reason"] = *option, reason
			variants, _ := body["variants"].([]any)
			if variants == nil {
				variants = []any{}
			}
			if _, err := tx.UpdateObject(oq.ID, oq.CurrentVersion, "resolved", body,
				store.EventInput{Type: store.EventStateChanged, Actor: "human", Reason: reason,
					Data: map[string]any{"label": oq.Label, "from": "open", "to": "resolved",
						"option": *option, "chosen": chosenText, "discarded_variants": variants}}); err != nil {
				return err
			}
			edges, err := tx.EdgesFrom(ctx, projectID, oq.ID)
			if err != nil {
				return err
			}
			for _, e := range edges {
				if e.Type != store.EdgeBlocks {
					continue
				}
				if err := tx.EndEdge(e.ID, store.EventInput{Type: store.EventEdge, Actor: "app"}); err != nil {
					return err
				}
			}
			_, err = tx.AppendEvent(actTimed)
			return err
		})
	})
	if err != nil {
		return OpenQuestionAnswer{}, err
	}
	return answer, nil
}

// applyKept applies the answer to what the question's first option
// keeps (body keeps): a decision of yours or a confirmed one, replaced
// by yours when you choose another option, or the contradicted premise,
// back to current or in a new version.
func (g *Gate) applyKept(ctx context.Context, tx *store.Tx, body map[string]any, keep bool, chosen, reason string, answer *OpenQuestionAnswer) error {
	keeps, _ := body["keeps"].(map[string]any)
	if keeps == nil {
		return nil
	}
	obj, err := tx.GetObject(ctx, tx.ProjectID(), bodyStr(keeps, "id"))
	if err != nil {
		return err
	}
	human := func(data map[string]any) store.EventInput {
		return store.EventInput{Type: store.EventStateChanged, Actor: "human", Reason: reason, Data: data}
	}
	switch bodyStr(keeps, "kind") {
	case checks.KeepDecision:
		// [EL DISEÑO NO LO DICE]: a kept decision that is no longer yours
		// or confirmed (superseded meanwhile) is left as it is.
		if keep || obj.State != "human" && obj.State != "confirmed" {
			return nil
		}
		born, err := replaceByYours(ctx, tx, obj, chosen, reason, "open_question")
		if err != nil {
			return err
		}
		answer.NewDecision = born.Label
	case checks.KeepPremise:
		answer.Premise = obj.Label
		pb := bodyOf(obj)
		data := map[string]any{"label": obj.Label, "from": obj.State, "to": "current", "via": "open_question"}
		if !keep {
			pb["text"] = chosen
			answer.PremiseChanged = true
		} else if obj.State != "contradicted" {
			return nil
		}
		if _, err := tx.UpdateObject(obj.ID, obj.CurrentVersion, "current", pb, human(data)); err != nil &&
			!errors.Is(err, store.ErrNoChange) {
			return err
		}
		if answer.PremiseChanged {
			// B-46, P-72: its new version's probe is queued in the same
			// transaction; the runner sends it.
			change, err := checks.PremiseProbe(ctx, tx, tx.ProjectID(), obj.ID)
			if err != nil {
				return err
			}
			return checks.QueueProbe(ctx, tx, obj.ID, change)
		}
	}
	return nil
}

// AnswerOpenQuestion is the gate's answer plus, when you changed the
// premise, the probe of its new version against the current text in
// one checker call (B-46, backlog-r2 2.4), in the project's turn slot.
// The gate queued it in the history (P-72): with the slot free it runs
// now; with a turn of the project running it stays queued, its sections
// «sin comprobar», and runs when that turn ends or at the next B-36
// trigger (runQueuedProbes). The answer stands either way.
func (r *Runner) AnswerOpenQuestion(ctx context.Context, projectID, label string, option *int, reason string) (OpenQuestionAnswer, error) {
	answer, err := r.gate.AnswerOpenQuestion(ctx, projectID, label, option, reason)
	if err != nil || !answer.PremiseChanged {
		return answer, err
	}
	if !r.tryAcquireTurn(projectID) {
		return answer, nil
	}
	defer r.releaseTurn(projectID)
	turnID, err := newTurnID()
	if err != nil {
		return answer, err
	}
	return answer, r.runQueuedProbes(ctx, projectID, turnID)
}
