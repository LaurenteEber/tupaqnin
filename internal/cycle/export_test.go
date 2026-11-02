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

// ValidateReaderOutput checks a hand-made reader output against
// salida-lector, as the app does.
func ValidateReaderOutput(text string) []string { return validateReaderOutput(text) }

// ValidateDigestOutput checks a digest output against salida-digesto
// only (backlog-r3 3.4).
func ValidateDigestOutput(text string) []string {
	errs, _ := validateDigestSchema(text)
	return errs
}

// UnderstandingInstructions and CheckerInstructions are the embedded drafts.
func UnderstandingInstructions() string { return understandingInstructions }
func CheckerInstructions() string       { return checkerInstructions }
func PrincipalInstructions() string     { return principalInstructions }

// SetBeforeLeaveOpenWrite runs f between LeaveRoundOpen's read and its
// write, and returns the restore.
func SetBeforeLeaveOpenWrite(f func()) (restore func()) {
	beforeLeaveOpenWrite = f
	return func() { beforeLeaveOpenWrite = func() {} }
}

// SetContextWindowForTest fixes the principal's window with a small value
// (backlog-r4 4.5): only tests can set it, never the environment (T-9).
func SetContextWindowForTest(r *Runner, tokens int) { r.contextWindow = tokens }
