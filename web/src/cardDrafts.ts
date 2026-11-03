// C.11: what you wrote on a card lives outside the component, by project and card,
// so nothing that redraws or unmounts the panel loses it; and a card with text keeps
// its place. Only this page's memory: nothing goes out until you press «Apply».
export interface CardDraft {
  reason?: string;
  option?: "keep" | "other";
  chosen?: number;
}

const drafts = new Map<string, Record<string, CardDraft>>();

export function draftsOf(projectID: string): Record<string, CardDraft> {
  return drafts.get(projectID) ?? {};
}

export function setDraft(projectID: string, label: string, patch: CardDraft): Record<string, CardDraft> {
  const next = { ...draftsOf(projectID) };
  next[label] = { ...next[label], ...patch };
  drafts.set(projectID, next);
  return next;
}

export function dropDraft(projectID: string, label: string): Record<string, CardDraft> {
  const next = { ...draftsOf(projectID) };
  delete next[label];
  drafts.set(projectID, next);
  return next;
}

export function draftHasText(draft: CardDraft | undefined): boolean {
  return (draft?.reason ?? "").trim() !== "";
}

// The cards' order: as Go sent it, unless a card has text written; then the ones
// you see stay in their place and the new ones go last.
export function holdOrder(shown: string[], sent: string[], holding: boolean): string[] {
  if (!holding) return sent;
  const kept = shown.filter((label) => sent.includes(label));
  return [...kept, ...sent.filter((label) => !kept.includes(label))];
}

// The scroll that leaves the card where it was after what is above it changed.
export function anchoredScroll(scrollTop: number, topBefore: number, topNow: number): number {
  return scrollTop + (topNow - topBefore);
}
