// B-101: your messages wait in the conversation while the agent works, yours to
// edit or remove until they enter a step. Plain functions, for `node --test`.

export function isQueued(entry: { kind: string; queued?: boolean; n?: number }): boolean {
  return entry.kind === "message" && Boolean(entry.queued) && Boolean(entry.n);
}

export function queuedMessagePath(projectID: string, n: number): string {
  return `/api/projects/${projectID}/messages/${n}`;
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
