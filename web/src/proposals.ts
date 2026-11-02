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

// The key of a closed proposal's line: its reason, or its state.
export function closedKey(p: ProposalView): string {
  return p.why && p.why !== "discarded" ? `proposal.state.${p.why}` : `proposal.state.${p.state}`;
}

// The turn Go opened for the new step (B-75), or "": the page watches it.
export function turnToWatch(res: ProposalActResponse): string {
  return res.turn_id ?? "";
}
