// C.10, B-105: your explanation of a signoff stays in this tab while you write it,
// so leaving the signoff or failing an attempt does not lose it. Nothing goes to the
// server until you send it. The getter is itself guarded: reading sessionStorage can
// throw when the browser blocks it, and then nothing is kept.
export const SIGN_DRAFT_DELAY_MS = 1000;

export interface DraftStore {
  getItem(key: string): string | null;
  setItem(key: string, value: string): void;
  removeItem(key: string): void;
}

export function signDraftKey(projectID: string, section: string): string {
  return `tq.sign.${projectID}.${section}`;
}

export function loadSignDraft(store: () => DraftStore, projectID: string, section: string): string {
  try {
    return store().getItem(signDraftKey(projectID, section)) ?? "";
  } catch {
    return "";
  }
}

export function saveSignDraft(store: () => DraftStore, projectID: string, section: string, text: string): void {
  try {
    if (text === "") store().removeItem(signDraftKey(projectID, section));
    else store().setItem(signDraftKey(projectID, section), text);
  } catch {
    // Only this tab's convenience is lost.
  }
}

export function clearSignDraft(store: () => DraftStore, projectID: string, section: string): void {
  saveSignDraft(store, projectID, section, "");
}
