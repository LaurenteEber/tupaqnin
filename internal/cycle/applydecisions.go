package cycle

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/LaurenteEber/tupaqnin/internal/graph"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// B-100: your decisions reach the text by themselves. An answer that changes the
// text records which sections it touches; when one has nothing of yours left open,
// the app runs one step of the agent to apply them.

// StopDecisionApplied: the one step that applies your decisions ended (B-100).
const StopDecisionApplied StopReason = "decision_applied"

// sectionsUsing are the sections a decision is used in, by internal id.
func sectionsUsing(ctx context.Context, tx *store.Tx, decisionID string) ([]string, error) {
	edges, err := tx.EdgesFrom(ctx, tx.ProjectID(), decisionID)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, e := range edges {
		if e.Type == store.EdgeUsedIn {
			out = append(out, e.ToID)
		}
	}
	return out, nil
}

// sectionsTouchedBy are the sections a replacement reached: where the new decision
// is used, and those whose list of open changes now names the replaced one (a
// figure it covers, rule 4).
func sectionsTouchedBy(ctx context.Context, tx *store.Tx, born, replaced store.Object) ([]string, error) {
	out, err := sectionsUsing(ctx, tx, born.ID)
	if err != nil {
		return nil, err
	}
	sections, err := tx.ListObjects(ctx, tx.ProjectID(), store.ObjectSection)
	if err != nil {
		return nil, err
	}
	for _, sec := range sections {
		for _, c := range graph.OpenChanges(sec) {
			if c.About == replaced.Label {
				out = append(out, sec.ID)
				break
			}
		}
	}
	return out, nil
}

// appliesData is what the answer's event says it changes (B-100); nil when the
// answer changes no text, as «mantener».
func appliesData(data map[string]any, sections []string, text, reason, decision string) map[string]any {
	if len(sections) == 0 {
		return data
	}
	seen := map[string]bool{}
	unique := make([]string, 0, len(sections))
	for _, s := range sections {
		if !seen[s] {
			seen[s] = true
			unique = append(unique, s)
		}
	}
	data["applies_to"], data["applies_text"], data["applies_reason"] = unique, text, reason
	if decision != "" {
		data["applies_decision"] = decision
	}
	return data
}

// applyTask is the step's task for the sections ready: it names decisions and
// sections by label (rule 7) and your words in literal (rule 8). "" with none.
func applyTask(unapplied []graph.UnappliedDecision) string {
	var lines []string
	for _, u := range unapplied {
		if !u.Ready {
			continue
		}
		if u.Decision != "" {
			lines = append(lines, fmt.Sprintf("- %s in %s: «%s», because «%s»", u.Decision, u.Section, u.Text, u.Reason))
		} else {
			lines = append(lines, fmt.Sprintf("- the person's answer for %s: «%s», because «%s»", u.Section, u.Text, u.Reason))
		}
	}
	if len(lines) == 0 {
		return ""
	}
	return "The person decided and the text does not say it yet. In this one step, update the text " +
		"so it does, close the changes it opened, and say in `triage_line` which decision you applied:\n" +
		strings.Join(lines, "\n")
}

// applyAfterAnswer takes the turn and runs the one step when an answer left
// decisions ready to apply; it returns that turn's id. With the slot taken it stays
// wanted for the holder's release, which looks again: the running chain may apply
// them first. During the mode its chain does it (B-83). It never fails your answer.
func (r *Runner) applyAfterAnswer(ctx context.Context, projectID string) string {
	unapplied, err := graph.UnappliedDecisions(ctx, r.db, projectID)
	if err != nil {
		r.logAfterAnswer(ctx, "unapplied_decisions_error")
		return ""
	}
	task := applyTask(unapplied)
	if task == "" {
		return ""
	}
	mode, err := r.mode(ctx, projectID)
	if err != nil {
		r.logAfterAnswer(ctx, "unapplied_decisions_error")
		return ""
	}
	if mode.On() {
		turn, err := r.openChain(projectID)
		if err != nil && !errors.Is(err, ErrRunnerClosed) {
			r.logAfterAnswer(ctx, "unapplied_decisions_error")
		}
		return turn
	}
	if !r.tryAcquireTurn(projectID) {
		r.mu.Lock()
		r.queueOf(projectID).applyWanted = true
		free := !r.running[projectID] && !r.closed
		if free { // the holder released meanwhile: nobody would look
			r.running[projectID] = true
			r.turns.Add(1)
		}
		r.mu.Unlock()
		if free {
			go r.runHandoff(projectID)
		}
		return ""
	}
	if !r.trackTurn() {
		r.releaseTurn(projectID)
		return ""
	}
	turnID, err := newTurnID()
	if err != nil {
		r.turns.Done()
		r.releaseTurn(projectID)
		r.logAfterAnswer(ctx, "unapplied_decisions_error")
		return ""
	}
	r.beginChainTurn(projectID, turnID)
	go func() {
		defer r.turns.Done()
		r.runApplyStep(projectID, turnID, task)
	}()
	return turnID
}

// runApplyStep is the one step, in the slot the caller holds. A message of yours
// that arrived during it opens its own chain, as at any stop. No chain follows it:
// what a judge finds waits for the next one.
func (r *Runner) runApplyStep(projectID, turnID, task string) {
	defer r.endTurn(r.ctx, projectID, turnID)
	state := &chainState{judgeFailed: map[string]bool{}, oneStep: true, task: task}
	r.setChainTurn(projectID, turnID)
	next, err := r.chainTurn(r.ctx, projectID, turnID, "", false, state)
	if err != nil {
		r.logTurnFailure(err)
		return
	}
	if next != "" {
		if err := r.runChain(r.ctx, projectID, next, "", false); err != nil {
			r.logTurnFailure(err)
		}
	}
}
