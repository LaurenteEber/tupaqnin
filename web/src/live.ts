// The page's live updates with no turn of their own (P-92, P-93,
// P-94): plain functions, without React, so `node --test` checks them.

// What a failed turn leaves to read: its message comes back (B-101), and a step that
// failed right after the understanding leaves sections the page has not read, so
// «Document» would stay off until a reload.
export function afterFailedTurn(read: { sections: () => unknown; conversation: () => unknown }): void {
  void read.sections();
  void read.conversation();
}

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

// Backlog-r3 3.6: the catalog code of a refused «Auditar sección». Its call
// throws the API's code; each refusal has its own message.
export function auditErrorCode(error: unknown, defensePreparing: boolean): string {
  const code = error instanceof Error ? error.message : "";
  if (code === "turn_running") return defensePreparing ? "turn_running_defense" : "turn_running";
  return code === "round_open" || code === "section_empty" ? code : "turn_failed";
}

// P-100: the catalog code of a refused «Buscar evidencia» or «Buscarla». Its
// call throws the API's code; each refusal has its own message.
export function roundErrorCode(error: unknown, defensePreparing: boolean): string {
  const code = error instanceof Error ? error.message : "";
  if (code === "turn_running") return defensePreparing ? "turn_running_defense" : "turn_running";
  return code === "request_required" || code === "candidate_not_found" ? code : "turn_failed";
}

// Whether the message invites you to write to the agent: not when writing
// again cannot help.
export function invitesRetry(code: string): boolean {
  return !["turn_running", "turn_running_defense", "round_open", "section_empty", "request_required", "candidate_not_found"].includes(code);
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

export function roundKeepsPolling(card: {
  running: boolean;
  parts: { status: string }[];
  kind?: string;
  state?: string;
}): boolean {
  // A judge round's batch and recheck run in the turn (backlog-r3 3.6).
  if (card.running || card.parts.some((p) => p.status === "running") || (card.kind === "judge" && card.state === "open")) return true;
  // C.2: a round the principal asked for exists before its readers start (its
  // step's checker runs in between): a part still pending has not started yet,
  // and a card read in that window would otherwise never ask again.
  return card.state !== "closed" && card.parts.some((p) => p.status === "pending");
}

// pollRound reads a round until it stops running, whoever opened it (a click or
// a step, C.2). A failed read is asked again.
export async function pollRound<C extends Parameters<typeof roundKeepsPolling>[0]>(p: {
  read: () => Promise<C>;
  onCard: (card: C) => void;
  sleep: (ms: number) => Promise<void>;
  cancelled: () => boolean;
}): Promise<void> {
  for (;;) {
    try {
      const card = await p.read();
      if (p.cancelled()) return;
      p.onCard(card);
      if (!roundKeepsPolling(card)) return;
    } catch {
      if (p.cancelled()) return;
    }
    await p.sleep(ROUND_POLL_MS);
    if (p.cancelled()) return;
  }
}

// C.2: how often the header's spend is read again while a turn runs.
// [inicial] in the backlog: 10 s.
export const SUMMARY_POLL_MS = 10_000;

// C.2: what the agent is doing, from the stream's pieces.
export interface AgentActivity {
  kind: "working" | "writing" | "reading" | "judging";
  // The label of the section the step writes, when the stream named it.
  section: string;
}

export const IDLE_ACTIVITY: AgentActivity = { kind: "working", section: "" };

export function nextActivity(prev: AgentActivity, delta: { field?: string; text?: string; start?: boolean }): AgentActivity {
  switch (delta.field) {
    case "triage_line":
      return delta.start ? IDLE_ACTIVITY : prev;
    case "section_id":
      return { ...prev, section: delta.start ? (delta.text ?? "") : prev.section + (delta.text ?? "") };
    case "text":
      return { ...prev, kind: "writing" };
    case "round":
      return { kind: "reading", section: "" };
    default:
      return prev;
  }
}

// C.2: the header's line, computed from what already arrives: what the agent is
// doing, or how many things are yours, or nothing. It returns a catalog key.
export function statusLine(s: {
  working: boolean;
  defensePreparing: boolean;
  activity: AgentActivity;
  sectionTitle: (label: string) => string;
  todo: number;
}): { key: string; params?: Record<string, string> } | null {
  if (s.working) {
    if (s.activity.kind === "reading") return { key: "status.agent.reading" };
    if (s.activity.kind === "judging") return { key: "status.agent.judging" };
    if (s.activity.kind === "writing") {
      const section = s.sectionTitle(s.activity.section);
      return section ? { key: "status.agent.writing_section", params: { section } } : { key: "status.agent.writing" };
    }
    return { key: "status.agent.working" };
  }
  if (s.defensePreparing) return { key: "status.agent.defense" };
  if (s.todo === 1) return { key: "status.yours.one" };
  if (s.todo > 1) return { key: "status.yours.many", params: { n: String(s.todo) } };
  return null;
}

// The bar's word for the mode on (P-115): Go says when it waits for your signoff.
export function modeBarKey(waitsSignoff?: boolean): string {
  return waitsSignoff ? "mode.waits_signoff" : "mode.run_to_completion";
}

// The key of why a chain stopped: the mode's last stop says it by its cause
// (B-83), computed in Go; only the wording is the page's.
export function stopKey(reason: string, cause?: string): string {
  return reason === "mode_finished" && cause ? `turn.stop.mode_finished.${cause}` : `turn.stop.${reason}`;
}

// B-83: the turn the page must attach to after reading the summary: the chain
// running now when it does not follow it yet (one the app opened, or a reload).
export function turnToFollow(summary: { running_turn?: string }, watched: string): string {
  return summary.running_turn && summary.running_turn !== watched ? summary.running_turn : "";
}

// B-83: a queued signoff is asked for again while the page is on its project.
export function keepsAskingToSign(result: { kind: string }, askedProject: string, currentProject: string): boolean {
  return result.kind === "queued" && askedProject === currentProject;
}
