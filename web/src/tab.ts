// The tab's name for the editor lease (B-102): it survives a reload of that tab,
// and a new tab has its own. A duplicated tab carries the same one, and the first
// to renew wins, as before.
export const TAB_ID_KEY = "tq.tabId";

export interface TabStore {
  getItem(key: string): string | null;
  setItem(key: string, value: string): void;
}

// The getter is itself guarded: reading sessionStorage can throw when the browser
// blocks it, and the tab then lives with an id of this load only.
export function readTabId(store: () => TabStore, make: () => string): string {
  try {
    const s = store();
    const saved = s.getItem(TAB_ID_KEY);
    if (saved) return saved;
    const id = make();
    s.setItem(TAB_ID_KEY, id);
    return id;
  } catch {
    return make();
  }
}

// «Editar aquí»: what the notice says after asking to take the edit.
export type TakeOverResult = "" | "busy";

export function takeOverResult(editor: boolean): TakeOverResult {
  return editor ? "" : "busy";
}

// editorReleaseRequest frees the lease as the page goes. It is keepalive, not
// sendBeacon: a beacon can only POST, and the route is a DELETE (B-102).
export function editorReleaseRequest(tabID: string) {
  return {
    method: "DELETE",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ tab_id: tabID }),
    keepalive: true,
  };
}
