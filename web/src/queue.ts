// B-101: your messages wait in the conversation while the agent works, yours to
// edit or remove until they enter a step. Plain functions, for `node --test`.

export function isQueued(entry: { kind: string; queued?: boolean; n?: number }): boolean {
  return entry.kind === "message" && Boolean(entry.queued) && Boolean(entry.n);
}

export function queuedMessagePath(projectID: string, n: number): string {
  return `/api/projects/${projectID}/messages/${n}`;
}

// «Reintentar»: a message that was not sent goes now.
export function queuedRetryPath(projectID: string, n: number): string {
  return `${queuedMessagePath(projectID, n)}/retry`;
}

// The mark's text and the retry it offers, from what Go said about the message: at
// a spend cap none, since it would stop there; with several waiting, how many go.
export function queuedMark(entry: { not_sent?: boolean; not_sent_cap?: boolean; retry_sends?: number }): {
  key: string;
  retry: string | null;
  n: string;
} {
  const n = String(entry.retry_sends ?? 0);
  if (!entry.not_sent) return { key: "message.queued", retry: null, n };
  if (entry.not_sent_cap) return { key: "message.not_sent_cap", retry: null, n };
  return { key: "message.not_sent", retry: (entry.retry_sends ?? 0) > 1 ? "message.queued_retry_many" : "message.queued_retry", n };
}

export function queuedEditRequest(text: string) {
  return { method: "PUT", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ text }) };
}

export function queuedRemoveRequest() {
  return { method: "DELETE" };
}

// An edit can go only with some text; the page refuses a blank one as Go does.
export function canSaveEdit(text: string): boolean {
  return text.trim() !== "";
}
