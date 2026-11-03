// C.3: reloading or sharing the address comes back to the same project and the
// same language. Plain functions, so `node --test` checks them.

const PROJECT_HASH = /^#\/p\/([A-Za-z0-9_-]+)$/;

// readProjectHash is the project the address names; "" when it names none.
export function readProjectHash(hash: string): string {
  return PROJECT_HASH.exec(hash)?.[1] ?? "";
}

// projectHash is the address's hash for an open project; "" with none open.
export function projectHash(projectID: string): string {
  return projectID ? `#/p/${projectID}` : "";
}

export const LANG_KEY = "tq.lang";

export type RouteLang = "en" | "es";

export interface LangStore {
  getItem(key: string): string | null;
  setItem(key: string, value: string): void;
}

// pickLang: `?lang=es` in the address first, then what you chose, then English.
// The getter is itself guarded: reading localStorage can throw when it is blocked.
export function pickLang(search: string, store: () => LangStore): RouteLang {
  const asked = new URLSearchParams(search).get("lang");
  if (asked === "es" || asked === "en") return asked;
  try {
    return store().getItem(LANG_KEY) === "es" ? "es" : "en";
  } catch {
    return "en";
  }
}

export function saveLang(lang: RouteLang, store: () => LangStore): void {
  try {
    store().setItem(LANG_KEY, lang);
  } catch {
    // The choice lasts until this page reloads.
  }
}
