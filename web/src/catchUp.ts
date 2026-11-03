// C.9 (B-103): what each thing that keeps a section from being signed offers. Go says
// what is open; a label reaches the page only to route, never as text (rule 7).

interface PendingLike {
  kind: string;
  about?: string;
}

// An open change is not a pending (A-1): the agent closes it, as a correction.
export const OPEN_CHANGE = "open_change";

const isYours = (kind: string) => kind === "open_card" || kind === "open_question";

export type PendingRoute = { to: "reach"; target: string } | { to: "correct" };

// A card and a «por aclarar» are yours: their button leads to them. Everything else
// is the agent's to close: «Pedir corrección al agente».
export function pendingRoute(p: PendingLike): PendingRoute {
  if (p.kind === "open_card") return { to: "reach", target: `card-${p.about ?? ""}` };
  if (p.kind === "open_question") return { to: "reach", target: `oq-${p.about ?? ""}` };
  return { to: "correct" };
}

// What the line says about the pending: a passage or a figure, never a label.
export function shownAbout(p: PendingLike): string {
  return isYours(p.kind) || p.kind === OPEN_CHANGE ? "" : (p.about ?? "");
}

// The message that stands for the correction in the conversation if it has to wait
// for a turn: yours, in the page's language.
export function correctionMessage(
  t: (key: string, params?: Record<string, string>) => string,
  p: PendingLike,
  section: string,
): string {
  const base = p.kind === OPEN_CHANGE ? t("signoff.open_change") : t(`pending.${p.kind}`);
  const about = shownAbout(p);
  return t("signoff.correction.message", { pending: about ? `${base} · ${about}` : base, section });
}

// A double click asks once (B-103): one run at a time on this flag.
export async function oneAtATime(busy: { current: boolean }, run: () => Promise<void>): Promise<void> {
  if (busy.current) return;
  busy.current = true;
  try {
    await run();
  } finally {
    busy.current = false;
  }
}
