package cycle

import "context"

// HoldTurnForTest takes the project's turn slot, as a running turn
// does, and returns the turn's end (endTurn).
func HoldTurnForTest(r *Runner, projectID string) (end func()) {
	if !r.tryAcquireTurn(projectID) {
		panic("turn slot taken")
	}
	return func() { r.endTurn(context.Background(), projectID, "turn-held") }
}

// WaitAfterAnswersForTest waits for the defenses after your answers
// running in the background (P-90).
func WaitAfterAnswersForTest(r *Runner) { r.answers.Wait() }
