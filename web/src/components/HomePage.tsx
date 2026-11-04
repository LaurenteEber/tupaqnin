import { useState, type FormEvent } from "react";
import { AccountError, login, register } from "../api/client";
import type { ProjectLine, SessionView } from "../api/types";
import { accountErrorText, createErrorKey, homeNoticeKeys, homeView, homeWelcomeKey, loginNoticeKey, nextPasswordShown, passwordFieldType, passwordToggleKey } from "../home";
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
  const welcome = homeWelcomeKey(session);
  const refused = createErrorKey(createError, session);
  const loginNotice = loginNoticeKey(projects);

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

  return (
    <div className="home">
      <section className="home-notice" aria-label={t("home.notice_title")}>
        {welcome && <p className="home-welcome">{t(welcome)}</p>}
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

      <div className="home-account">
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
