// What the judge round's card decides from Go's fields (backlog-r3 3.6,
// B-67): pure, so the UI tests can run them. Go computes it all; this only
// picks what to show.

// «No aplica» counts until the batch starts (K-4).
export function offersNotApplicable(
  card: { state: string; batch_started?: boolean },
  flaw: { discarded?: boolean },
): boolean {
  return card.state === "open" && !card.batch_started && !flaw.discarded;
}

// The triage line (K-3): it warns the judge enters, without waiting.
export function triageKey(card: { reason?: string }): string {
  return card.reason === "audit" ? "judge.triage.audit" : "judge.triage.risk";
}

// How a flaw ended: what you did, or what the recheck said; "" while it
// waits.
export function flawOutcomeKey(flaw: { discarded?: boolean; verdict?: string }): string {
  if (flaw.discarded) return "judge.outcome.discarded";
  return flaw.verdict ? `judge.outcome.${flaw.verdict}` : "";
}
