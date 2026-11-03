package cycle

import (
	"context"
	"errors"
	"fmt"
	"slices"
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
func appliesData(data map[string]any, sections []string, text, reason, decision, during string) map[string]any {
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
	if during != "" {
		data["applies_during"] = during
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
		"so it does, close the changes it opened, and say in `triage_line`, in words, which decision you applied:\n" +
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
	r.runOneStep(projectID, turnID, &chainState{judgeFailed: map[string]bool{}, oneStep: true, task: task, oneStepReason: StopDecisionApplied})
}

// runOneStep is one step with its task, in the slot the caller holds.
func (r *Runner) runOneStep(projectID, turnID string, state *chainState) {
	defer r.endTurn(r.ctx, projectID, turnID)
	r.setChainTurn(projectID, turnID)
	next, err := r.chainTurn(r.ctx, projectID, turnID, "", false, state)
	r.correctionEnded(projectID)
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

// B-103 (c): «Pedir corrección al agente». One step on one pending of one section,
// the same function as above, with the caps of any step (B-89).

// StopCorrectionApplied: the one step of «Pedir corrección» ended (B-103).
const StopCorrectionApplied StopReason = "correction_applied"

// StopCorrectionStillOpen: that step ended and what you asked to correct is still
// open (B-103): only a pending really closed earns StopCorrectionApplied.
const StopCorrectionStillOpen StopReason = "correction_still_open"

// CorrectionOpenChange is the kind that names an open change: not a pending, but it
// keeps a section from being signed and the agent closes it (A-1).
const CorrectionOpenChange = "open_change"

var (
	// ErrPendingNotOpen: what you asked to correct is no longer open on the section.
	ErrPendingNotOpen = errors.New("cycle: the pending is not open on the section")
	// ErrPendingYours: an open card or «por aclarar» is yours, not the agent's.
	ErrPendingYours = errors.New("cycle: the pending is yours to close")
)

// runningCorrection is what the step of «Pedir corrección» running now corrects.
type runningCorrection struct {
	label, kind, about string
	turn               string
}

// correctionTask is the step's task for one thing left open on a section, or an
// error when it is not open: by label, whatever the path named it by (rule 7), its
// literal text (rule 8) and what closes it, as the agent reads its pendings.
func (r *Runner) correctionTask(ctx context.Context, projectID, section, kind, about string) (task, sectionLabel string, err error) {
	sec, err := r.db.GetObject(ctx, projectID, section)
	if err != nil {
		return "", "", err
	}
	if sec.Type != store.ObjectSection {
		return "", "", store.ErrNotFound
	}
	task, err = r.correctionOf(ctx, projectID, sec, kind, about)
	return task, sec.Label, err
}

func (r *Runner) correctionOf(ctx context.Context, projectID string, sec store.Object, kind, about string) (string, error) {
	label := sec.Label
	title := bodyStr(bodyOf(sec), "title")
	if kind == CorrectionOpenChange {
		for _, c := range graph.OpenChanges(sec) {
			if c.Label == about {
				return correctionWords(fmt.Sprintf("%s «%s» · open change %s, of %s %s · close it in handled_changes", label, title, c.Label, c.Kind, c.About)), nil
			}
		}
		return "", ErrPendingNotOpen
	}
	if (graph.Pending{Kind: kind}).IsYours() {
		return "", ErrPendingYours
	}
	_, pendings, err := graph.SectionStatus(ctx, r.db, projectID, label)
	if err != nil {
		return "", err
	}
	if !slices.ContainsFunc(pendings, func(p graph.Pending) bool { return p.Kind == kind && p.About == about }) {
		return "", ErrPendingNotOpen
	}
	lines, err := r.memory.AgentPendings(ctx, projectID)
	if err != nil {
		return "", err
	}
	return correctionWords(correctionLine(lines, label, title, kind, about)), nil
}

// correctionLine is the agent's own line for the pending, with its passage and what
// closes it; the short form for one its list leaves out (a malformed output).
func correctionLine(lines []string, label, title, kind, about string) string {
	line := fmt.Sprintf("%s «%s» · %s", label, title, kind)
	if about != "" {
		line += ": " + about
	}
	for _, l := range lines {
		if l == line || strings.HasPrefix(l, line+" ") {
			return l
		}
	}
	return line
}

func correctionWords(line string) string {
	return "The person asked you to correct this. In this one step, correct it in the text " +
		"and say in `triage_line`, in words, what you corrected:\n- " + line
}

// correctionStillOpen says, after the step, whether what you asked for is still open.
func (r *Runner) correctionStillOpen(ctx context.Context, projectID, label, kind, about string) (bool, error) {
	_, _, err := r.correctionTask(ctx, projectID, label, kind, about)
	if errors.Is(err, ErrPendingNotOpen) || errors.Is(err, store.ErrNotFound) {
		return false, nil
	}
	return err == nil, err
}

// acquireCorrection takes the slot for this correction, as turnID. With the slot
// taken, it returns the turn already correcting the same thing, if one is.
func (r *Runner) acquireCorrection(projectID string, want runningCorrection) (running string, acquired bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	q := r.queueOf(projectID)
	if r.running[projectID] {
		if c := q.correction; c != nil && c.label == want.label && c.kind == want.kind && c.about == want.about {
			return c.turn, false
		}
		return "", false
	}
	r.running[projectID] = true
	q.correction = &want
	return "", true
}

func (r *Runner) correctionEnded(projectID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if q := r.queues[projectID]; q != nil {
		q.correction = nil
	}
}

// queueCorrection queues your message, once: the same correction asked again while
// it waits writes nothing.
func (r *Runner) queueCorrection(ctx context.Context, projectID, message string) error {
	waiting, err := r.QueuedMessages(ctx, projectID)
	if err != nil {
		return err
	}
	if slices.ContainsFunc(waiting, func(m QueuedMessage) bool { return m.Text == message }) {
		return nil
	}
	return r.queueMessage(projectID, message)
}

// RequestCorrection runs one step of the agent on a pending of a section and returns
// its turn. With a turn running, the mode on or a signoff holding the steps back it
// queues message, the page's text in your language, as any message of yours (C.4):
// visible, editable, removable, and the turn is "". Then it is the queue's reader
// that answers it, not this one step. Asked again while that step runs, it answers
// with the same turn and writes nothing.
func (r *Runner) RequestCorrection(ctx context.Context, projectID, section, kind, about, message string) (string, error) {
	task, label, err := r.correctionTask(ctx, projectID, section, kind, about)
	if err != nil {
		return "", err
	}
	mode, err := r.mode(ctx, projectID)
	if err != nil {
		return "", err
	}
	if mode.On() {
		return "", r.queueCorrection(ctx, projectID, message)
	}
	turnID, err := newTurnID()
	if err != nil {
		return "", err
	}
	running, acquired := r.acquireCorrection(projectID, runningCorrection{label: label, kind: kind, about: about, turn: turnID})
	if running != "" {
		return running, nil
	}
	if !acquired {
		return "", r.queueCorrection(ctx, projectID, message)
	}
	if !r.trackTurn() {
		r.releaseTurn(projectID)
		return "", ErrRunnerClosed
	}
	waits, err := r.cleanupWaitsForSignoff(ctx, projectID)
	if err == nil && waits { // as StartTurn: the signoff's close answers it (P-114)
		err = r.writeQueuedMessage(projectID, "", message)
		r.turns.Done()
		r.releaseTurn(projectID)
		if err == nil {
			r.SignoffClosed(r.ctx, projectID) // it may have closed meanwhile
		}
		return "", err
	}
	if err != nil {
		r.turns.Done()
		r.releaseTurn(projectID)
		return "", err
	}
	r.beginChainTurn(projectID, turnID)
	state := &chainState{judgeFailed: map[string]bool{}, oneStep: true, task: task, oneStepReason: StopCorrectionApplied, taskSection: label,
		stillOpen: func(ctx context.Context) (bool, error) {
			return r.correctionStillOpen(ctx, projectID, label, kind, about)
		}}
	go func() {
		defer r.turns.Done()
		r.runOneStep(projectID, turnID, state)
	}()
	return turnID, nil
}
