import type { SuspiciousReason } from "./api/graph";

// Deshacer and the suspicious edit on the page (backlog-r4 4.2, B-79, B-80).
// Go says whether «Deshacer» is on and why not, and what each alert is; this
// only picks the words.

const reasons = ["no_edit", "agent_wrote", "proposal_accepted", "later_edit"];

// The key of why «Deshacer» is off, or of a failure that names no reason.
export function undoReasonKey(code: string): string {
  return reasons.includes(code) ? `undo.reason.${code}` : "undo.error";
}

// The line under the section's button: only when something could be undone
// and is not, since with no edit of yours there is nothing to explain.
export function undoLineKey(available: boolean, reason: string): string {
  return available || !reason ? "" : undoReasonKey(reason);
}

// The key of one motive of an alert.
export function alertReasonKey(r: SuspiciousReason): string {
  return `alert.reason.${r.kind}`;
}

// What a motive names: the literal anchor, section or source, or the decision's
// statement.
export function alertReasonWhat(r: SuspiciousReason): string {
  return r.statement || r.about || "";
}
