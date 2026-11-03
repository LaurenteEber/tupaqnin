import { useState, type FormEvent } from "react";
import { AccountError, login, logout, register } from "../api/client";
import type { ProjectLine, SessionView } from "../api/types";
import { accountErrorText, activityAgo, createErrorKey, homeNoticeKeys, homeView, loginNoticeKey, nextPasswordShown, passwordFieldType, passwordToggleKey, projectProgress } from "../home";
import { translate, type Lang } from "../i18n";

export type HomeForm = "" | "register" | "login";
type Form = HomeForm;

// P1: the screen when no project is open. Go says who you are and what opens;
// this shows it and asks for the two forms (1a, 1c, 1d, 1e, 1f, 1g, 1h).
export function HomePage({
  lang,
  session,
  sessionFailed,
  projects,
  createError,
  openForm: askedForm = "",
  onOpen,
  onNew,
  onSessionChanged,
}: {
  lang: Lang;
  session: SessionView | null;
  sessionFailed: boolean;
  projects: ProjectLine[];
  createError: string;
  // The form the left panel asked for (U.2).
  openForm?: HomeForm;
  onOpen: (id: string) => void;
  onNew: (template?: string) => void;
  onSessionChanged: () => Promise<void>;
}) {
  const t = (key: string, params?: Record<string, string>) => translate(lang, key, params);
  const say = (text: { key: string; params?: Record<string, string> }) => t(text.key, text.params);
  const [form, setForm] = useState<Form>(askedForm);
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [passwordShown, setPasswordShown] = useState(false);
  const [formError, setFormError] = useState("");
  const [sending, setSending] = useState(false);

  const view = homeView(session);
  const refused = createErrorKey(createError, session);
  const loginNotice = loginNoticeKey(projects);
  const now = Date.now();

  function openForm(next: Form) {
    setForm(next);
    setPassword("");
    setPasswordShown((shown) => nextPasswordShown(shown, "change_form"));
    setFormError("");
  }

  function showError(e: unknown) {
    const retryAfter = e instanceof AccountError ? e.retryAfterSeconds : 0;
    setFormError(say(accountErrorText(e instanceof Error ? e.message : "internal", retryAfter)));
  }

  async function submit(event: FormEvent) {
    event.preventDefault();
    setPasswordShown((shown) => nextPasswordShown(shown, "submit"));
    setSending(true);
    setFormError("");
    try {
      await (form === "register" ? register : login)(username, password);
      setForm("");
      setUsername("");
      setPassword("");
      await onSessionChanged();
    } catch (e) {
      showError(e);
    } finally {
      setSending(false);
    }
  }

  async function leave() {
    setSending(true);
    setFormError("");
    try {
      await logout();
      await onSessionChanged();
    } catch (e) {
      showError(e);
    } finally {
      setSending(false);
    }
  }

  return (
    <div className="home">
      <section className="home-notice" aria-label={t("home.notice_title")}>
        <p>{t("home.what")}</p>
        <p>{t("home.sensitive")}</p>
        {view.needsLink && !sessionFailed && <p className="home-warn">{t("home.no_link")}</p>}
        {sessionFailed && <p className="turn-error">{t("home.error.internal")}</p>}
        {homeNoticeKeys(session).map((key) => (
          <p className="home-warn" key={key}>
            {t(key)}
          </p>
        ))}
        {refused && <p className="turn-error">{t(refused)}</p>}
      </section>

      {view.start && (
        <div className="empty-actions">
          <button type="button" className="primary" disabled={!session?.can_create} onClick={() => onNew("case-5")}>
            {t("project.demo")}
          </button>
          <button type="button" disabled={!session?.can_create} onClick={() => onNew()}>
            {t("project.blank")}
          </button>
        </div>
      )}

      {view.list && (
        <section className="home-list" aria-label={t("project.list")}>
          <h2>{t("project.list")}</h2>
          {projects.length === 0 && <p className="meta">{t("home.no_projects")}</p>}
          <ul>
            {projects.map((p) => (
              <li key={p.project_id}>
                <button type="button" onClick={() => onOpen(p.project_id)}>
                  {p.name || t("project.untitled")}
                </button>
                <span className="meta">
                  {say(activityAgo(p.last_activity, now))} · {say(projectProgress(p))}
                  {p.no_account && <> · {t("home.project.no_account")}</>}
                </span>
              </li>
            ))}
          </ul>
        </section>
      )}

      <div className="home-account">
        {view.logout && (
          <>
            <span className="meta">{t("home.signed_in", { account: session?.account ?? "" })}</span>
            <button type="button" disabled={sending} onClick={() => void leave()}>
              {t("home.logout")}
            </button>
          </>
        )}
        {view.register && (
          <button type="button" aria-expanded={form === "register"} onClick={() => openForm("register")}>
            {t("home.register")}
          </button>
        )}
        {view.login && (
          <button type="button" aria-expanded={form === "login"} onClick={() => openForm("login")}>
            {t("home.login")}
          </button>
        )}
      </div>

      {form !== "" && !view.logout && (
        <form className="home-form" onSubmit={(event) => void submit(event)}>
          <label>
            {t("home.username")}
            <input
              name="username"
              autoComplete="username"
              required
              value={username}
              onChange={(e) => setUsername(e.target.value)}
            />
          </label>
          <div className="home-password">
            <label>
              {t("home.password")}
              <input
                id="home-password"
                name="password"
                type={passwordFieldType(passwordShown)}
                autoComplete={form === "register" ? "new-password" : "current-password"}
                required
                value={password}
                onChange={(e) => setPassword(e.target.value)}
              />
            </label>
            <button
              type="button"
              aria-pressed={passwordShown}
              aria-controls="home-password"
              onClick={() => setPasswordShown((shown) => nextPasswordShown(shown, "toggle"))}
            >
              {t(passwordToggleKey(passwordShown))}
            </button>
          </div>
          {form === "register" && <p className="hint">{t("home.register_hint")}</p>}
          {form === "login" && loginNotice && <p className="home-warn">{t(loginNotice)}</p>}
          <button type="submit" className="primary" disabled={sending}>
            {form === "register" ? t("home.register") : t("home.login")}
          </button>
        </form>
      )}
      {formError && (
        <p className="turn-error" role="alert">
          {formError}
        </p>
      )}
    </div>
  );
}
