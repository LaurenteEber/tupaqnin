// The home (P1, E.6): Go computes the session and the project lines; this only
// picks what shows and which text says it.
export interface HomeSession {
  account: string;
  can_create: boolean;
  cannot_create: string;
}

export interface Text {
  key: string;
  params?: Record<string, string>;
}

// Why a new project does not open (GET /api/session).
export const CANNOT_CREATE = [
  "account_required",
  "account_limit",
  "link_full",
  "link_budget",
  "day_cap",
  "disk_full",
] as const;

export function cannotCreateKey(reason: string): string {
  return `home.cannot_create.${reason}`;
}

// What the screen holds. No session (null) is «Necesitas un link de invitación»
// and «Entrar»: only a link registers (B3-18), and entering asks for none (B-91).
export function homeView(session: HomeSession | null) {
  const signedIn = session !== null && session.account !== "";
  return {
    needsLink: session === null,
    start: session !== null,
    list: session !== null,
    register: session !== null && !signedIn,
    login: !signedIn,
    logout: signedIn,
  };
}

// The notices of the home (1d, 1f, 1g, 1h; B3-20).
export function homeNoticeKeys(session: HomeSession | null): string[] {
  if (session === null) return [];
  const keys: string[] = [];
  if (session.account === "") keys.push("home.notice.no_account");
  if (!session.can_create) keys.push(cannotCreateKey(session.cannot_create));
  return keys;
}

// The login form's notice while you hold a project without an account (B-91).
// It reads only what Go already sent about your session, never anything about
// the name typed, so it does not tell who exists or how many projects they hold.
export function loginNoticeKey(projects: { no_account: boolean }[]): string | null {
  return projects.some((p) => p.no_account) ? "home.login_moves_project" : null;
}

// A create refused after the screen was drawn: the notice's text, unless the
// fresh session already shows it. Null when there is nothing to add.
export function createErrorKey(code: string, session: HomeSession | null): string | null {
  if (!code) return null;
  if (code === "no_session") return session === null ? "home.no_link" : "home.error.internal";
  if (!(CANNOT_CREATE as readonly string[]).includes(code)) return "home.error.internal";
  return session?.cannot_create === code ? null : cannotCreateKey(code);
}

const ACCOUNT_ERRORS = [
  "username_taken",
  "link_accounts_full",
  "invalid_credentials",
  "already_registered",
  "invalid_username",
  "invalid_password",
  "too_many_attempts",
  "no_session",
];

// The text of a register or login refusal; a 429 says how long to wait.
export function accountErrorText(code: string, retryAfterSeconds = 0): Text {
  if (!ACCOUNT_ERRORS.includes(code)) return { key: "home.account.error.internal" };
  const key = `home.account.error.${code}`;
  return code === "too_many_attempts" ? { key, params: { seconds: String(retryAfterSeconds) } } : { key };
}

const MINUTE = 60_000;
const HOUR = 60 * MINUTE;
const DAY = 24 * HOUR;

// «hace 2 h», from the moment Go sent.
export function activityAgo(lastActivity: string, now: number): Text {
  const past = now - Date.parse(lastActivity);
  if (past < MINUTE) return { key: "home.ago.now" };
  if (past < HOUR) return { key: "home.ago.minutes", params: { n: String(Math.floor(past / MINUTE)) } };
  if (past < DAY) return { key: "home.ago.hours", params: { n: String(Math.floor(past / HOUR)) } };
  return { key: "home.ago.days", params: { n: String(Math.floor(past / DAY)) } };
}

// «1 de 3 secciones firmadas», as Go counted them.
export function projectProgress(p: { signed: number; sections: number }): Text {
  if (p.sections === 0) return { key: "home.project.no_sections" };
  return { key: "home.project.signed", params: { signed: String(p.signed), sections: String(p.sections) } };
}

// The left panel's line for a project (U.2), from what Go counted.
export function projectLineTexts(
  p: { last_activity: string; sections: number; signed: number; no_account: boolean },
  now: number,
): Text[] {
  const line = [activityAgo(p.last_activity, now)];
  if (p.sections === 0) {
    line.push({ key: "home.project.no_sections" });
  } else {
    line.push(p.sections === 1 ? { key: "left.project.section_one" } : { key: "left.project.sections", params: { n: String(p.sections) } });
    line.push(p.signed === 1 ? { key: "left.project.signed_one" } : { key: "left.project.signed", params: { n: String(p.signed) } });
  }
  if (p.no_account) line.push({ key: "home.project.no_account" });
  return line;
}

// A project's name, or the catalog's: never an internal id (rule 7).
export function projectNameText(p: { name: string }): string | Text {
  return p.name || { key: "project.untitled" };
}

// What the left panel offers (U.2): the same rules as the home, plus the notice
// of B3-18, whose «descárgalo» needs a project open.
export function leftBottom(session: HomeSession | null, projectOpen: boolean) {
  const view = homeView(session);
  const notice = session !== null && session.account === "";
  return {
    start: view.start,
    list: view.list,
    notice,
    noticeDownload: notice && projectOpen,
    register: view.register,
    login: view.login,
    logout: view.logout,
  };
}
