package cycle

import (
	"context"
	"errors"
	"strings"
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
	// ErrOwnOptionNotOffered: only an objection's «por aclarar» takes your own option (B-107).
	ErrOwnOptionNotOffered = errors.New("cycle: this open question does not offer your own option")
	ErrSameStatement       = errors.New("cycle: your own option is the objected decision")
	// ErrEmptyStatement: your own option says nothing, by the «porque»'s rule (B-107).
	ErrEmptyStatement = errors.New("cycle: your own option has no statement")
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
	// TurnID is the one step your answer started to apply it to the text (B-100).
	TurnID string
	// Premise is the premise's label when the question kept or changed it.
	Premise string
	// DefensePreparing: the answer left a section ready whose defense is
	// now being prepared in the background (P-92).
	DefensePreparing bool
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
	return g.answerQuestion(ctx, projectID, label, option, nil, reason)
}

// AnswerObjectionWithYours is «Escribe la tuya» (B-107, extends B-81): only on an
// objection's «por aclarar», your statement, literal (rule 8), and your «porque» make
// a decision of yours that replaces the objected one, down the same path as «la
// otra». It joins the question's options as the one you chose, so what reads the
// chosen option keeps reading it.
// [EL DISEÑO NO LO DICE]: a statement that does not pass ValidReason, or equals the
// objected decision's after trimming and case, is refused like an answer with no
// option (act_timed, still open); spaces around the statement do not count.
func (g *Gate) AnswerObjectionWithYours(ctx context.Context, projectID, label, statement, reason string) (OpenQuestionAnswer, error) {
	return g.answerQuestion(ctx, projectID, label, nil, &statement, reason)
}

func (g *Gate) answerQuestion(ctx context.Context, projectID, label string, option *int, own *string, reason string) (OpenQuestionAnswer, error) {
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
	if own != nil && bodyStr(body, "origin") != checks.OriginObjection {
		return OpenQuestionAnswer{}, ErrOwnOptionNotOffered
	}
	var ownRefused error
	if own != nil {
		if ownRefused, err = g.refuseOwnStatement(ctx, projectID, body, *own); err != nil {
			return OpenQuestionAnswer{}, err
		}
	}
	opened, err := g.cardOpenedAt(ctx, projectID, oq.ID)
	if err != nil {
		return OpenQuestionAnswer{}, err
	}
	actTimed := store.EventInput{Type: store.EventActTimed, Actor: "human", ObjectID: oq.ID,
		Data: map[string]any{"act": "open_question", "label": oq.Label,
			"duration_ms": time.Since(opened).Milliseconds()}}

	if !ValidReason(reason) || (option == nil && own == nil) || ownRefused != nil {
		err := g.db.Write(ctx, projectID, func(tx *store.Tx) error {
			if !ValidReason(reason) {
				data := map[string]any{"label": oq.Label, "option": nil}
				if option != nil {
					data["option"] = *option
				}
				if own != nil {
					data["own"] = true
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
		if ownRefused != nil {
			return OpenQuestionAnswer{}, ownRefused
		}
		return OpenQuestionAnswer{}, ErrNoOption
	}

	answer := OpenQuestionAnswer{OpenQuestion: oq.Label}
	during := g.runningTurn(projectID)
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
			var chosen map[string]any
			var index int
			if own != nil {
				chosen = map[string]any{"option": strings.TrimSpace(*own), "own": true, "contrast": "", "consequence": "", "risk": ""}
				index = len(options)
				body["options"] = append(options, chosen)
			} else {
				index = *option
				chosen, _ = options[index].(map[string]any)
			}
			chosenText := bodyStr(chosen, "option")
			keep, _ := chosen["keep"].(bool)
			if err := g.applyKept(ctx, tx, body, keep, chosenText, reason, &answer); err != nil {
				return err
			}
			body["chosen_option"], body["reason"] = index, reason
			variants, _ := body["variants"].([]any)
			if variants == nil {
				variants = []any{}
			}
			edges, err := tx.EdgesFrom(ctx, projectID, oq.ID)
			if err != nil {
				return err
			}
			resolved := map[string]any{"label": oq.Label, "from": "open", "to": "resolved",
				"option": index, "chosen": chosenText, "discarded_variants": variants}
			if own != nil {
				resolved["own"] = true
			}
			// B-100: an answer other than «mantener» changes the text of what the
			// question blocked and of where the decision it replaced was used.
			if !keep {
				var sections []string
				for _, e := range edges {
					if e.Type != store.EdgeBlocks {
						continue
					}
					if to, err := tx.GetObject(ctx, projectID, e.ToID); err == nil && to.Type == store.ObjectSection {
						sections = append(sections, to.ID)
					}
				}
				if answer.NewDecision != "" {
					born, err := tx.GetObject(ctx, projectID, answer.NewDecision)
					if err != nil {
						return err
					}
					keeps, _ := body["keeps"].(map[string]any)
					replaced, err := tx.GetObject(ctx, projectID, bodyStr(keeps, "id"))
					if err != nil {
						return err
					}
					used, err := sectionsTouchedBy(ctx, tx, born, replaced)
					if err != nil {
						return err
					}
					sections = append(sections, used...)
				}
				resolved = appliesData(resolved, sections, chosenText, reason, answer.NewDecision, during)
			}
			if _, err := tx.UpdateObject(oq.ID, oq.CurrentVersion, "resolved", body,
				store.EventInput{Type: store.EventStateChanged, Actor: "human", Reason: reason, Data: resolved}); err != nil {
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

// refuseOwnStatement is why your own option does not count (B-107), or nil: the
// «porque»'s rule, and it cannot be the decision you objected to.
func (g *Gate) refuseOwnStatement(ctx context.Context, projectID string, question map[string]any, statement string) (refusal, err error) {
	if !ValidReason(statement) {
		return ErrEmptyStatement, nil
	}
	keeps, _ := question["keeps"].(map[string]any)
	objected, err := g.db.GetObject(ctx, projectID, bodyStr(keeps, "id"))
	if err != nil {
		return nil, err
	}
	if strings.EqualFold(strings.TrimSpace(statement), strings.TrimSpace(bodyStr(bodyOf(objected), "statement"))) {
		return ErrSameStatement, nil
	}
	return nil, nil
}

// applyKept applies the answer to what the question's first option
// keeps (body keeps): a decision of yours or a confirmed one, replaced
// by yours when you choose another option, or the contradicted premise,
// back to current or in a new version. An objection's question has its own
// rule (answerObjection).
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
		if bodyStr(body, "origin") == checks.OriginObjection {
			return answerObjection(ctx, tx, obj, keep, chosen, reason, answer)
		}
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
// trigger (runQueuedProbes). Then, when the answer left a section ready
// (P-86: its last «por aclarar»), its defense, in the background
// (defendInBackground, P-90). The
// answer stands either way.
func (r *Runner) AnswerOpenQuestion(ctx context.Context, projectID, label string, option *int, reason string) (OpenQuestionAnswer, error) {
	before := r.notReadyBeforeYourAnswer(ctx, projectID)
	answer, err := r.gate.AnswerOpenQuestion(ctx, projectID, label, option, reason)
	return r.afterQuestionAnswer(ctx, projectID, before, answer, err)
}

// AnswerObjectionWithYours is the gate's, plus what any answer starts: B-100's step.
func (r *Runner) AnswerObjectionWithYours(ctx context.Context, projectID, label, statement, reason string) (OpenQuestionAnswer, error) {
	before := r.notReadyBeforeYourAnswer(ctx, projectID)
	answer, err := r.gate.AnswerObjectionWithYours(ctx, projectID, label, statement, reason)
	return r.afterQuestionAnswer(ctx, projectID, before, answer, err)
}

func (r *Runner) afterQuestionAnswer(ctx context.Context, projectID string, before map[string]bool, answer OpenQuestionAnswer, err error) (OpenQuestionAnswer, error) {
	if err != nil {
		return answer, err
	}
	defenses := r.defensesToPrepare(ctx, projectID, before)
	answer.DefensePreparing = len(defenses) > 0 // P-92
	answer.TurnID, err = r.afterYourAnswer(ctx, projectID, defenses, answer.PremiseChanged)
	return answer, err
}
