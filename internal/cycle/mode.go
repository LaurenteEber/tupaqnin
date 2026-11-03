package cycle

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/LaurenteEber/tupaqnin/internal/graph"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// Piece 4.4: «continúa hasta terminar» (B-83, B-84, B-85). The mode lives in the
// history (mode_set), so every chain reads it before each step and turning it off
// in the bar takes effect at the next one. Your queued messages are read from the
// history; what lives here in memory, guarded by mu, only tells a chain to look.

const (
	// StopModeFinished: the mode ended; the report's cause says why (B-83).
	StopModeFinished StopReason = "mode_finished"
	// StopModeCap: the mode reached ModeMaxSteps.
	StopModeCap StopReason = "mode_cap"
	// StopProviderError: Meta failed during the mode (B-83).
	StopProviderError StopReason = "provider_error"
	// StopRestart: the app restarted with the mode on and nothing running (P-114).
	StopRestart StopReason = "restart"
	// ModeMaxSteps is the mode's cap (B-83: 60 steps [inicial]).
	ModeMaxSteps = 60
	// ModeMaxIdleSteps ends the mode after that many steps in a row with no new
	// text, no decisions and no proposals (P-114: 3 [inicial]), so a step that only
	// asks questions cannot burn the 60.
	ModeMaxIdleSteps = 3
)

var (
	// ErrSignoffQueued answers asking to sign while the mode's chain runs (B-83):
	// it opens when the running step ends. Asking again reads what happened.
	ErrSignoffQueued = errors.New("cycle: the signoff is queued until the running step ends")
	// ErrInvalidMode is a mode that is neither normal nor run_to_completion.
	ErrInvalidMode = errors.New("cycle: invalid mode")
	// ErrMessageNotQueued: the message already entered a step, or never waited (B-101).
	ErrMessageNotQueued = errors.New("cycle: the message is no longer queued")
	// ErrRunnerClosed is a turn asked for after Close.
	ErrRunnerClosed = errors.New("cycle: the runner is closed")
)

// modeQueue is what waits for a project's turn slot during the mode.
type modeQueue struct {
	// wake: a message was queued since the chain last looked.
	wake     bool
	signoffs []string
	// outcomes are what kept the queued signoffs from opening, by section, until
	// you ask again. One that opened leaves none: asking again finds it in
	// progress, or a new request if it already ended.
	outcomes map[string]signoffOutcome
	// wantsChain: the mode was activated, or a signoff closed, with the slot taken.
	wantsChain bool
	// turn is the chain's current turn, for the page and a queued message.
	turn string
	// applyWanted: an answer left decisions to apply with the slot taken (B-100).
	applyWanted bool
	// correction is the step of «Pedir corrección» running now (B-103).
	correction *runningCorrection
}

type signoffOutcome struct {
	open SignoffOpen
	err  error
}

// queueOf returns the project's queue; mu is held.
func (r *Runner) queueOf(projectID string) *modeQueue {
	q := r.queues[projectID]
	if q == nil {
		q = &modeQueue{outcomes: map[string]signoffOutcome{}}
		r.queues[projectID] = q
	}
	return q
}

func (r *Runner) mode(ctx context.Context, projectID string) (graph.Mode, error) {
	return graph.CurrentMode(ctx, r.db, projectID)
}

// RunningTurn is the turn of the chain running now, or "". The page follows it
// when it did not start it: after a reload, or a chain the app opened.
func (r *Runner) RunningTurn(projectID string) string {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.running[projectID] || r.queues[projectID] == nil {
		return ""
	}
	return r.queues[projectID].turn
}

// ModeWaitsForSignoff says the mode is on with nothing running because a signoff
// is in progress over the threshold (B-86): the bar says so (P-115).
func (r *Runner) ModeWaitsForSignoff(ctx context.Context, projectID string) (bool, error) {
	mode, err := r.mode(ctx, projectID)
	if err != nil || !mode.On() {
		return false, err
	}
	r.mu.Lock()
	busy := r.running[projectID]
	r.mu.Unlock()
	if busy {
		return false, nil
	}
	return r.cleanupWaitsForSignoff(ctx, projectID)
}

func (r *Runner) setChainTurn(projectID, turnID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.queueOf(projectID).turn = turnID
}

// trackTurn counts a background turn so Close waits for it; false after Close.
func (r *Runner) trackTurn() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return false
	}
	r.turns.Add(1)
	return true
}

// SetMode is PUT /mode (B-83): mode_set with your message in literal, on the mode
// read in its transaction. Activating with the agent stopped starts a turn and
// returns its id; with the slot taken, the running chain takes the mode at its
// next step, or the end of a turn that is no chain opens one (only one chain per
// project), and the id is empty.
// [EL DISEÑO NO LO DICE]: asking for the mode already set writes nothing and
// starts nothing; a message of yours that activates it again goes as a message.
func (r *Runner) SetMode(ctx context.Context, projectID, mode, message string) (string, error) {
	if mode != graph.ModeNormal && mode != graph.ModeRunToCompletion {
		return "", ErrInvalidMode
	}
	changed := false
	err := r.db.Write(ctx, projectID, func(tx *store.Tx) error {
		current, err := graph.CurrentMode(ctx, tx, projectID)
		if err != nil || current.Mode == mode {
			return err
		}
		changed = true
		data := map[string]any{"mode": mode}
		if mode == graph.ModeRunToCompletion {
			data["message"] = message
		}
		_, err = tx.AppendEvent(store.EventInput{Type: store.EventModeSet, Actor: "human", Data: data})
		return err
	})
	if err != nil || !changed {
		return "", err
	}
	if mode == graph.ModeNormal {
		r.mu.Lock()
		r.queueOf(projectID).wantsChain = false
		r.mu.Unlock()
		return "", nil
	}
	return r.openChain(projectID)
}

// openChain starts the mode's chain when the slot is free, or leaves it wanted
// for the holder's release. It returns the new turn's id, "" when it left it.
func (r *Runner) openChain(projectID string) (string, error) {
	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		return "", ErrRunnerClosed
	}
	if r.running[projectID] {
		r.queueOf(projectID).wantsChain = true
		r.mu.Unlock()
		return "", nil
	}
	r.running[projectID] = true
	r.turns.Add(1)
	r.mu.Unlock()
	turnID, err := newTurnID()
	if err != nil {
		r.turns.Done()
		r.releaseTurn(projectID)
		return "", err
	}
	r.beginChainTurn(projectID, turnID)
	go func() {
		defer r.turns.Done()
		defer r.endTurn(r.ctx, projectID, turnID)
		if err := r.runChain(r.ctx, projectID, turnID, "", false); err != nil {
			r.logTurnFailure(err)
		}
	}()
	return turnID, nil
}

func (r *Runner) beginChainTurn(projectID, turnID string) {
	r.setChainTurn(projectID, turnID)
	if r.pub != nil {
		r.pub.BeginTurn(projectID, turnID)
	}
}

func (r *Runner) logTurnFailure(err error) {
	if r.ctx.Err() == nil && r.Logger != nil {
		r.Logger.Error("turn failed", "error", err.Error())
	}
}

// queueMessage queues a message sent while the slot is taken (B-83, and B-101 in
// the normal mode too): human_message now, in literal, so you see it and it
// survives, and into the next step; if the chain ends first, its release opens
// a turn with it.
func (r *Runner) queueMessage(projectID, text string) error {
	r.mu.Lock()
	turnID := ""
	if q := r.queues[projectID]; q != nil {
		turnID = q.turn
	}
	r.mu.Unlock()
	if err := r.writeQueuedMessage(projectID, turnID, text); err != nil {
		return err
	}
	r.mu.Lock()
	r.queueOf(projectID).wake = true
	free := !r.running[projectID] && !r.closed
	if free { // the holder released meanwhile: nobody would read the queue
		r.running[projectID] = true
		r.turns.Add(1)
	}
	r.mu.Unlock()
	if free {
		go r.runHandoff(projectID)
	}
	return nil
}

// writeQueuedMessage is your queued message in the history, in literal. Over the
// threshold with a signoff in progress it says so, for the conversation's line
// that the agent answers when the signoff ends (P-114).
func (r *Runner) writeQueuedMessage(projectID, turnID, text string) error {
	data := map[string]any{"text": text, "queued": true}
	waits, err := r.cleanupWaitsForSignoff(r.ctx, projectID)
	if err != nil {
		return err
	}
	if waits {
		data["waits_signoff"] = true
	}
	return r.db.Write(r.ctx, projectID, func(tx *store.Tx) error {
		_, err := tx.AppendEvent(store.EventInput{Type: store.EventHumanMessage, Actor: "human", TurnID: turnID, Data: data})
		return err
	})
}

// undeliveredMessages are the queued messages that never entered a call of the
// principal, in order, derived from the history: human_message against the items
// of the conversation (the last model_call), so a step that failed or a restart
// loses none and none enters twice. A startup's «Your unanswered message» lines
// count as delivered; that it repeats the first as its task does not.
func (r *Runner) undeliveredMessages(ctx context.Context, projectID string) ([]string, error) {
	queued, err := r.undelivered(ctx, projectID)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, m := range queued {
		out = append(out, m.Text)
	}
	return out, nil
}

// QueuedMessage is a message of yours waiting for the next step (B-101): N is
// its place in the history, which the page uses to edit or remove it. NotSent: no
// step will read it unless you send something or retry (queueStranded); NotSentCap:
// a spend cap is why, and a retry would stop there; RetrySends: how many a retry
// sends, since a step takes the queue whole.
type QueuedMessage struct {
	N          int
	Text       string
	NotSent    bool
	NotSentCap bool
	RetrySends int
}

func (r *Runner) undelivered(ctx context.Context, projectID string) ([]QueuedMessage, error) {
	events, err := r.db.EventsSince(ctx, projectID, 0)
	if err != nil {
		return nil, err
	}
	var queued []QueuedMessage
	sent := map[string]int{} // every unanswered message of yours, by text
	withdrawn := store.WithdrawnMessages(events)
	for _, ev := range events {
		if ev.Type != store.EventHumanMessage || withdrawn[ev.Seq] {
			continue
		}
		var data struct {
			Text     string `json:"text"`
			Response string `json:"response"`
			Queued   bool   `json:"queued"`
		}
		if json.Unmarshal(ev.Data, &data) != nil || data.Text == "" || data.Response != "" {
			continue
		}
		sent[data.Text]++
		if data.Queued {
			queued = append(queued, QueuedMessage{N: ev.Seq, Text: data.Text})
		}
	}
	if len(queued) == 0 {
		return nil, nil
	}
	inst, err := r.readInstance(ctx, projectID)
	if err != nil {
		return nil, err
	}
	var items []json.RawMessage
	if inst.id != "" {
		if items, err = r.conversation(ctx, projectID, inst.id); err != nil {
			return nil, err
		}
	}
	missing := map[string]int{}
	for _, m := range queued {
		missing[m.Text] = min(missing[m.Text]+1, sent[m.Text]-deliveredCount(items, m.Text))
	}
	var out []QueuedMessage
	for i := len(queued) - 1; i >= 0; i-- { // the last ones of an equal text are the new ones
		if missing[queued[i].Text] > 0 {
			missing[queued[i].Text]--
			out = append(out, queued[i])
		}
	}
	slices.Reverse(out)
	return out, nil
}

// takeQueued reads what enters the step about to run and marks it as entering:
// from here on it can no longer be edited or removed, though it only counts as
// delivered when the call is recorded, once the model answered.
func (r *Runner) takeQueued(ctx context.Context, projectID string) ([]string, error) {
	r.queueMu.Lock()
	defer r.queueMu.Unlock()
	queued, err := r.undelivered(ctx, projectID)
	if err != nil {
		return nil, err
	}
	entering := map[int]bool{}
	var out []string
	for _, m := range queued {
		entering[m.N] = true
		out = append(out, m.Text)
	}
	if r.entering == nil {
		r.entering = map[string]map[int]bool{}
	}
	r.entering[projectID] = entering
	return out, nil
}

// doneQueued: the step's call ended. What it delivered no longer shows as queued;
// what a failed call did not deliver waits again.
func (r *Runner) doneQueued(projectID string) {
	r.queueMu.Lock()
	defer r.queueMu.Unlock()
	delete(r.entering, projectID)
}

// QueuedMessages are the messages still waiting: yours to edit or remove (B-101).
func (r *Runner) QueuedMessages(ctx context.Context, projectID string) ([]QueuedMessage, error) {
	r.queueMu.Lock()
	defer r.queueMu.Unlock()
	waiting, err := r.waitingMessages(ctx, projectID)
	if err != nil || len(waiting) == 0 {
		return waiting, err
	}
	stranded, err := r.queueStranded(ctx, projectID)
	if err != nil {
		return nil, err
	}
	if !stranded {
		return waiting, nil
	}
	stopped, err := r.StoppedBy(ctx, projectID)
	if err != nil {
		return nil, err
	}
	for i := range waiting {
		waiting[i].NotSent, waiting[i].NotSentCap, waiting[i].RetrySends = true, stopped != "", len(waiting)
	}
	return waiting, nil
}

// queueStranded says nobody will read the queue: the slot is free, no release is
// about to take it up, and no signoff over the threshold keeps it for its close. A
// failed step leaves it so, and so do a cap or a restart.
func (r *Runner) queueStranded(ctx context.Context, projectID string) (bool, error) {
	r.mu.Lock()
	q := r.queues[projectID]
	pending := r.running[projectID] || r.closed || (q != nil && q.wake)
	r.mu.Unlock()
	if pending {
		return false, nil
	}
	waits, err := r.cleanupWaitsForSignoff(ctx, projectID)
	return !waits, err
}

// RetryQueuedMessage is «Reintentar»: a message nobody would read opens a turn now,
// the turn's id to follow, "" when something already reads the queue. The step reads
// the whole queue in order, so every message waiting enters with it.
// [EL DISEÑO NO LO DICE]: one route for one message; a retry of one sends them all,
// since a step takes the queue whole (takeQueued).
func (r *Runner) RetryQueuedMessage(ctx context.Context, projectID string, n int) (string, error) {
	waiting, err := r.QueuedMessages(ctx, projectID)
	if err != nil {
		return "", err
	}
	i := slices.IndexFunc(waiting, func(m QueuedMessage) bool { return m.N == n })
	if i < 0 {
		return "", ErrMessageNotQueued
	}
	if !waiting[i].NotSent {
		return "", nil
	}
	r.mu.Lock()
	r.queueOf(projectID).wake = true
	r.mu.Unlock()
	return r.openChain(projectID)
}

// waitingMessages: queueMu is held.
func (r *Runner) waitingMessages(ctx context.Context, projectID string) ([]QueuedMessage, error) {
	queued, err := r.undelivered(ctx, projectID)
	if err != nil {
		return nil, err
	}
	entering := r.entering[projectID]
	return slices.DeleteFunc(queued, func(m QueuedMessage) bool { return entering[m.N] }), nil
}

// WithdrawQueuedMessage takes back a message still waiting: it never enters a
// step. ErrMessageNotQueued when it already did, or never waited.
func (r *Runner) WithdrawQueuedMessage(ctx context.Context, projectID string, n int) error {
	return r.changeQueued(ctx, projectID, n, func(tx *store.Tx) error { return nil })
}

// EditQueuedMessage replaces a message still waiting by your new text.
// [EL DISEÑO NO LO DICE]: history only grows, so an edit is the old one withdrawn
// and a new one queued, in one transaction; it moves to the end of the queue.
func (r *Runner) EditQueuedMessage(ctx context.Context, projectID string, n int, text string) error {
	return r.changeQueued(ctx, projectID, n, func(tx *store.Tx) error {
		_, err := tx.AppendEvent(store.EventInput{Type: store.EventHumanMessage, Actor: "human",
			Data: map[string]any{"text": text, "queued": true}})
		return err
	})
}

func (r *Runner) changeQueued(ctx context.Context, projectID string, n int, then func(*store.Tx) error) error {
	r.queueMu.Lock()
	defer r.queueMu.Unlock()
	waiting, err := r.waitingMessages(ctx, projectID)
	if err != nil {
		return err
	}
	if !slices.ContainsFunc(waiting, func(m QueuedMessage) bool { return m.N == n }) {
		return ErrMessageNotQueued
	}
	return r.db.Write(ctx, projectID, func(tx *store.Tx) error {
		if _, err := tx.AppendEvent(store.EventInput{Type: store.EventMessageWithdrawn, Actor: "human",
			Data: map[string]any{"message": n}}); err != nil {
			return err
		}
		return then(tx)
	})
}

// deliveredCount is how many times a message of yours entered the conversation:
// as an item of its own, or as a line of the startup's core.
func deliveredCount(items []json.RawMessage, text string) int {
	n := 0
	for _, raw := range items {
		var item struct {
			Role    string `json:"role"`
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
		}
		if json.Unmarshal(raw, &item) != nil || item.Role != "user" || len(item.Content) == 0 {
			continue
		}
		got := item.Content[0].Text
		switch {
		case got == text:
			n++
		case strings.HasPrefix(got, "Core\n"):
			n += strings.Count(got, "Your unanswered message: "+text+"\n")
		}
	}
	return n
}

// hasMessages reports a message queued since the chain last looked that still
// has to enter a step.
func (r *Runner) hasMessages(ctx context.Context, projectID string) bool {
	r.mu.Lock()
	q := r.queues[projectID]
	wake := q != nil && q.wake
	r.mu.Unlock()
	if !wake {
		return false
	}
	out, err := r.undeliveredMessages(ctx, projectID)
	if err != nil {
		r.logTurnFailure(err)
		return false
	}
	if len(out) == 0 {
		r.clearWake(projectID)
	}
	return len(out) > 0
}

// clearWake says the queue needs no chain now: the next step reads what is
// undelivered, and a stop that cannot answer it (a cap, Meta failing, a signoff
// over the threshold) leaves it for the next chain, in the history.
func (r *Runner) clearWake(projectID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if q := r.queues[projectID]; q != nil {
		q.wake = false
	}
}

// queueSignoff answers asking to sign with the slot taken and the mode on (B-83):
// what a queued request left, or ErrSignoffQueued. done is false when the normal
// path applies (mode off, or the slot free).
func (r *Runner) queueSignoff(ctx context.Context, projectID, label string) (open SignoffOpen, done bool, err error) {
	r.mu.Lock()
	if q := r.queues[projectID]; q != nil {
		if out, ok := q.outcomes[label]; ok {
			delete(q.outcomes, label)
			r.mu.Unlock()
			return out.open, true, out.err
		}
	}
	busy := r.running[projectID]
	r.mu.Unlock()
	if !busy {
		return SignoffOpen{}, false, nil
	}
	mode, err := r.mode(ctx, projectID)
	if err != nil || !mode.On() {
		return SignoffOpen{}, false, err
	}
	// A signoff already in progress: the same one, as outside the mode (B-14).
	if section, err := r.db.GetObject(ctx, projectID, label); err == nil {
		if _, active, err := graph.ActiveSignoff(ctx, r.db, projectID, section.ID); err == nil && active {
			open, err := r.gate.RequestSignoff(ctx, projectID, label)
			return open, true, err
		}
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.running[projectID] {
		return SignoffOpen{}, false, nil
	}
	q := r.queueOf(projectID)
	if !slices.Contains(q.signoffs, label) {
		q.signoffs = append(q.signoffs, label)
	}
	return SignoffOpen{}, true, ErrSignoffQueued
}

// openQueuedSignoffs opens what you asked to sign during the running step, with
// the rules of any request: the checker first and the same gate (B-36, P-113).
// The caller holds the project's slot.
func (r *Runner) openQueuedSignoffs(ctx context.Context, projectID, turnID string) {
	r.mu.Lock()
	var labels []string
	if q := r.queues[projectID]; q != nil {
		labels, q.signoffs = q.signoffs, nil
	}
	r.mu.Unlock()
	for _, label := range labels {
		open, err := r.openSignoffHolding(withSignoffBalance(ctx), projectID, label, turnID)
		if err == nil && open.Opened {
			continue
		}
		r.mu.Lock()
		r.queueOf(projectID).outcomes[label] = signoffOutcome{open: open, err: err}
		r.mu.Unlock()
	}
}

// runHandoff is what a released slot leaves to do during the mode: the queued
// signoffs, then a chain for the queued messages or the wanted mode. The caller
// took the slot and counted it in turns.
func (r *Runner) runHandoff(projectID string) {
	defer r.turns.Done()
	turnID, err := newTurnID()
	if err != nil {
		r.releaseTurn(projectID)
		return
	}
	r.openQueuedSignoffs(r.ctx, projectID, turnID)
	r.mu.Lock()
	q := r.queueOf(projectID)
	wanted := q.wantsChain
	q.wantsChain = false
	apply := q.applyWanted
	q.applyWanted = false
	r.mu.Unlock()
	messages := r.hasMessages(r.ctx, projectID)
	if wanted && !messages {
		mode, err := r.mode(r.ctx, projectID)
		wanted = err == nil && mode.On()
	}
	if !messages && !wanted {
		// B-100: the answers that waited for the slot, if the holder did not apply them.
		if apply {
			if unapplied, err := graph.UnappliedDecisions(r.ctx, r.db, projectID); err == nil {
				if task := applyTask(unapplied); task != "" {
					r.beginChainTurn(projectID, turnID)
					r.runApplyStep(projectID, turnID, task)
					return
				}
			}
		}
		r.releaseTurn(projectID)
		return
	}
	if apply { // this chain's release looks again (B-100)
		r.mu.Lock()
		r.queueOf(projectID).applyWanted = true
		r.mu.Unlock()
	}
	r.beginChainTurn(projectID, turnID)
	defer r.endTurn(r.ctx, projectID, turnID)
	if err := r.runChain(r.ctx, projectID, turnID, "", false); err != nil {
		r.logTurnFailure(err)
	}
}

// SignoffClosed is B-86's «al cerrarse la firma, la app limpia y, si el modo
// sigue, reabre la cadena», told by every place a signoff closes. The cleanup
// needs nothing here: the next ensureInstance does it. It opens the chain when
// the mode goes on and the chain ended waiting for this (over the threshold), or
// when a message of yours waits for an answer (P-114), and no other signoff is in
// progress.
func (r *Runner) SignoffClosed(ctx context.Context, projectID string) {
	mode, err := r.mode(ctx, projectID)
	if err != nil {
		return
	}
	messages, err := r.undeliveredMessages(ctx, projectID)
	if err != nil {
		return
	}
	if len(messages) == 0 && (!mode.On() || !r.NeedsReset(ctx, projectID)) {
		return
	}
	if waits, err := r.signoffInProgress(ctx, projectID); err != nil || waits {
		return
	}
	if len(messages) > 0 {
		r.mu.Lock()
		r.queueOf(projectID).wake = true
		r.mu.Unlock()
	}
	if _, err := r.openChain(projectID); err != nil && !errors.Is(err, ErrRunnerClosed) {
		r.logTurnFailure(err)
	}
}

// modeEnd says a report is the mode's last (B-83).
type modeEnd struct {
	cause StopReason
	// noCall: the report keeps the app's part, without calling a model.
	noCall bool
}

// finishMode is the mode's final stop (B-83, B-85): the report with its cause,
// the mode back to normal in the same transaction, and then the defense of each
// section that needs it (K-7), unless a cap or a failing provider left no room.
func (r *Runner) finishMode(ctx context.Context, projectID, turnID string, cause StopReason) (StopReason, string, error) {
	end := &modeEnd{cause: cause, noCall: cause == StopProviderError}
	capErr, err := r.stopReport(ctx, projectID, turnID, cause, nil, end)
	if err != nil {
		return "", "", err
	}
	if capErr != nil {
		r.clearWake(projectID)
		return StopReason(capErr.Reason), capErr.Cap, nil
	}
	if !end.noCall {
		if err := r.defendEverySection(ctx, projectID, turnID); err != nil {
			return "", "", err
		}
	} else {
		r.clearWake(projectID)
	}
	return cause, "", nil
}

// defendEverySection prepares the defense of each section with text that needs
// it (K-7), once: prepareDefense does not repeat a text it already prepared
// (P-76), and a judge that fails or a cap leave what they leave at any stop.
// [EL DISEÑO NO LO DICE]: a section with cards or «por aclarar» counts: K-7 says
// «cada sección que la necesita», and the stop is exactly when only those remain;
// a signed section is done and does not.
func (r *Runner) defendEverySection(ctx context.Context, projectID, turnID string) error {
	sections, err := graph.OrderedSections(ctx, r.db, projectID)
	if err != nil {
		return err
	}
	for _, sec := range sections {
		if sec.State == "signed" || strings.TrimSpace(bodyStr(bodyOf(sec), "text")) == "" {
			continue
		}
		if err := r.prepareDefense(ctx, projectID, turnID, sec.Label); err != nil {
			return err
		}
	}
	return nil
}

// endModeAfterFailure ends the mode when a chain failed inside the app: nothing
// would run it, and the bar would say it runs. The stream already said turn_failed.
// [EL DISEÑO NO LO DICE]: the cause; agent_pending, «un fallo del agente», the
// nearest of B-83's list, with the app's part of the report and no call.
func (r *Runner) endModeAfterFailure(ctx context.Context, projectID, turnID string) {
	if ctx.Err() != nil {
		return // Close: the mode stays on in the history
	}
	mode, err := r.mode(ctx, projectID)
	if err != nil || !mode.On() {
		return
	}
	end := &modeEnd{cause: StopAgentPending, noCall: true}
	if _, err := r.stopReport(ctx, projectID, turnID, StopAgentPending, nil, end); err != nil {
		r.logTurnFailure(err)
	}
	r.clearWake(projectID)
}

// EndModesLeftRunning ends each mode a restart left on, with nothing running
// (P-114): mode_finished with cause restart, the app's part of the report with no
// call, and the mode back to normal, so the bar never says it runs and your next
// visit has its return line (B-85). The app calls it once, before it serves.
// [EL DISEÑO NO LO DICE]: what you queued stays in the history for your next
// message or a signoff's close, as any queued message of a mode that ended; the
// sweep opens no chain.
func (r *Runner) EndModesLeftRunning(ctx context.Context) error {
	projects, err := r.db.ProjectsWithMode(ctx)
	if err != nil {
		return err
	}
	// One project that fails does not leave the others saying the mode runs.
	var failed []error
	for _, projectID := range projects {
		mode, err := r.mode(ctx, projectID)
		if err != nil {
			failed = append(failed, err)
			continue
		}
		if !mode.On() || !r.tryAcquireTurn(projectID) {
			continue
		}
		if err := r.endModeLeftRunning(ctx, projectID); err != nil {
			failed = append(failed, err)
		}
		r.releaseTurn(projectID)
	}
	return errors.Join(failed...)
}

func (r *Runner) endModeLeftRunning(ctx context.Context, projectID string) error {
	turnID, err := newTurnID()
	if err != nil {
		return err
	}
	_, err = r.stopReport(ctx, projectID, turnID, StopRestart, nil, &modeEnd{cause: StopRestart, noCall: true})
	return err
}

// modeLine is the mode as layer 3 says it (block 2 §2), for a live conversation,
// which has no startup to read it from.
func modeLine(mode graph.Mode) string {
	line := fmt.Sprintf("Mode: %s\n", mode.Mode)
	if mode.Message != "" {
		line += fmt.Sprintf("Mode message: %s\n", mode.Message)
	}
	return line
}
