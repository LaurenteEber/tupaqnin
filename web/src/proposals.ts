import type { FrameProposal, ProposalView } from "./api/graph";
import type { ProposalActResponse } from "./api/types";

// Where «Propuestas (n)» leads: the first section with one, or "" for the
// proposed sections at the end of the document.
export function proposalsTarget(lines: FrameProposal[]): string {
  return lines.find((p) => p.section)?.section ?? "";
}

// What a section's panel shows: its proposals; without a section, the
// proposed sections still open. Go computes them all.
export function proposalsToShow(all: ProposalView[], section?: string): ProposalView[] {
  return section ? all.filter((p) => p.section === section) : all.filter((p) => p.new_section && p.state === "open");
}

// B-99: why the app held back an open proposal instead of applying it; "" otherwise.
export function heldBackKey(p: { why?: string; state: string }): string {
  const held = ["removes_half", "removes_support", "breaks_sentence", "rewrites_sentence_end"];
  return p.state === "open" && p.why && held.includes(p.why) ? `proposal.state.${p.why}` : "";
}

// C.22: what a proposal's contrast lists. A step that deletes text carries one
// empty entry per paragraph; they are not lines. Each removed paragraph is listed
// once, and when nothing enters the panel says so, once.
export function contrastLines(c: { removed: string[]; added: string[] }): { removed: string[]; added: string[]; entersNothing: boolean } {
  const removed = [...new Set(c.removed.filter((text) => text.trim() !== ""))];
  const added = c.added.filter((text) => text.trim() !== "");
  return { removed, added, entersNothing: removed.length > 0 && added.length === 0 };
}

// The key of a closed proposal's line: its reason, or its state.
export function closedKey(p: ProposalView): string {
  return p.why && p.why !== "discarded" ? `proposal.state.${p.why}` : `proposal.state.${p.state}`;
}

// The turn Go opened for the new step (B-75), or "": the page watches it.
export function turnToWatch(res: ProposalActResponse): string {
  return res.turn_id ?? "";
}
