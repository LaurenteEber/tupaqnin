// The page's live updates with no turn of their own (P-92, P-93,
// P-94): plain functions, without React, so `node --test` checks them.

// P-92: while a defense after your answer is being prepared, the
// conversation asks again every few seconds, up to a cap.
// [EL DISEÑO NO LO DICE]: the interval and the cap; 3 s, and 3 min
// (a judge call takes ~30 s, its retry another, and it may wait for a
// running turn).
export const DEFENSE_POLL_MS = 3_000;
export const DEFENSE_POLL_CAP_MS = 180_000;

export interface DefensePoll<E> {
  // Whether a defense of the project is still queued or being prepared
  // (the summary's defense_preparing).
  preparing: () => Promise<boolean>;
  conversation: () => Promise<E[]>;
  onEntries: (entries: E[]) => void;
  sleep: (ms: number) => Promise<void>;
  now: () => number;
  cancelled: () => boolean;
  // P-95: when the wait ends (not when it is cancelled), the page reads
  // again what the header shows: the summary, with what the judge spent.
  onEnd?: () => void;
}

export type DefensePollEnd = "ended" | "cap" | "cancelled";

// pollDefense refreshes the conversation until the preparation ends (its
// questions are written before it does, so the last refresh, read after
// it ended, shows them) or the cap passes. A failed read is asked again.
export async function pollDefense<E>(p: DefensePoll<E>): Promise<DefensePollEnd> {
  const end = await waitDefense(p);
  if (end !== "cancelled") p.onEnd?.();
  return end;
}

async function waitDefense<E>(p: DefensePoll<E>): Promise<DefensePollEnd> {
  const deadline = p.now() + DEFENSE_POLL_CAP_MS;
  while (p.now() < deadline) {
    await p.sleep(DEFENSE_POLL_MS);
    if (p.cancelled()) return "cancelled";
    try {
      const still = await p.preparing();
      const entries = await p.conversation();
      if (p.cancelled()) return "cancelled";
      p.onEntries(entries);
      if (!still) return "ended";
    } catch {
      // asked again at the next interval
    }
  }
  return "cap";
}

// P-94: the catalog code of a message that did not go out. A turn
// running while the defense line shows is the defense's.
export function sendErrorCode(error: unknown, defensePreparing: boolean): string {
  if (error instanceof Error && error.message === "http_409") {
    return defensePreparing ? "turn_running_defense" : "turn_running";
  }
  return "turn_failed";
}

// P-94: the input keeps your text unless the message went out.
export function textAfterSend(text: string, sent: boolean): string {
  return sent ? "" : text;
}

// P-93: each principal step's output opens with its triage line, so
// the start of one means the step before it was applied: the page
// reads the sections again.
export function startsStep(delta: { field?: string; start?: boolean }): boolean {
  return delta.field === "triage_line" && delta.start === true;
}

// P-93: the editor takes a newer text of its section that someone else
// wrote (the agent's step) only while you have not typed since it last
// loaded or saved, so nothing of yours is overwritten.
export function shouldReloadEditor(
  base: { version: number; savedText: string },
  incoming: { version: number; text: string },
  editorText: string,
  shownText: string,
): boolean {
  return incoming.version > base.version && incoming.text !== base.savedText && editorText === shownText;
}

// Backlog-r3 3.2 (B-64): a round started in the turn being followed.
// The stream says so with a piece of field «round» carrying its label;
// the page reads the conversation again so the round's card shows.
export function roundStarted(delta: { field?: string; text?: string }): string {
  return delta.field === "round" ? (delta.text ?? "") : "";
}

// The round card asks again every few seconds while its readers run.
// [EL DISEÑO NO LO DICE]: the interval; 2 s.
export const ROUND_POLL_MS = 2_000;

export function roundKeepsPolling(card: { running: boolean; parts: { status: string }[] }): boolean {
  return card.running || card.parts.some((p) => p.status === "running");
}
