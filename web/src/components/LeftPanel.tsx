import type { ReactNode } from "react";
import type { ProjectLine, SessionView } from "../api/types";
import { leftBottom, projectLineTexts, projectNameText } from "../home";
import { translate, type Lang } from "../i18n";
import type { HomeForm } from "./HomePage";

// The left zone (U.2): start a project, your projects as a history, and at the
// bottom the notice without an account, the language and the account. The rules
// are the home's, read from what Go sent.
export function LeftPanel({
  lang,
  off,
  onToggle,
  projects,
  projectID,
  session,
  onOpen,
  onNew,
  onHome,
  onForm,
  onLogout,
  onDownload,
  langSwitch,
}: {
  lang: Lang;
  off: boolean;
  onToggle: () => void;
  projects: ProjectLine[];
  projectID: string;
  session: SessionView | null;
  onOpen: (id: string) => void;
  onNew: (template?: string) => void;
  onHome: () => void;
  onForm: (form: HomeForm) => void;
  onLogout: () => void;
  onDownload: () => void;
  langSwitch: ReactNode;
}) {
  const t = (key: string, params?: Record<string, string>) => translate(lang, key, params);
  const view = leftBottom(session, projectID !== "");
  const now = Date.now();
  return (
    <aside className="left" aria-label={t("layout.projects")}>
      <div className="left-top">
        <button type="button" className="icon-btn" title={t("layout.left_toggle")} aria-label={t("layout.left_toggle")} aria-expanded={!off} onClick={onToggle}>
          ☰
        </button>
        {!off && (
          // [EL DISEÑO NO LO DICE] «Inicio» leaves the header: the brand leads home.
          <button type="button" className="brand" title={t("home.back")} onClick={onHome}>
            <b>{t("app.title")}</b>
            <small>{t("app.tagline")}</small>
          </button>
        )}
      </div>
      {view.start && (
        <div className="left-actions">
          <button type="button" className="nav-item" title={t("left.new")} aria-label={t("left.new")} onClick={() => onNew()}>
            <span className="ic">＋</span>
            {!off && <span>{t("left.new")}</span>}
          </button>
          <button type="button" className="nav-item" title={t("project.demo")} aria-label={t("project.demo")} onClick={() => onNew("case-5")}>
            <span className="ic">▷</span>
            {!off && <span>{t("project.demo")}</span>}
          </button>
        </div>
      )}
      {!off && (
        <>
          {view.list && <div className="section-label">{t("project.list")}</div>}
          {view.list && (
            <nav className="projects" aria-label={t("project.list")}>
              {projects.map((p) => {
                const name = projectNameText(p);
                return (
                  <button
                    key={p.project_id}
                    type="button"
                    className={`project-row${p.project_id === projectID ? " active" : ""}`}
                    aria-current={p.project_id === projectID ? "true" : undefined}
                    onClick={() => onOpen(p.project_id)}
                  >
                    {typeof name === "string" ? name : t(name.key)}
                    <span className="meta">
                      {projectLineTexts(p, now)
                        .map((text) => t(text.key, text.params))
                        .join(" · ")}
                    </span>
                  </button>
                );
              })}
            </nav>
          )}
          <div className="left-bottom">
            {view.notice && (
              <div className="left-notice" role="status">
                {t("left.notice.lead")} ·{" "}
                {view.noticeDownload && (
                  <>
                    <button type="button" className="link" onClick={onDownload}>
                      {t("left.notice.download")}
                    </button>{" "}
                    {t("left.notice.or")}{" "}
                  </>
                )}
                <button type="button" className="link" onClick={() => onForm("register")}>
                  {t("left.notice.register")}
                </button>
              </div>
            )}
            <div className="left-row">
              <span>{t("language.label")}</span>
              {langSwitch}
            </div>
            <div className="left-row">
              {view.register && (
                <button type="button" className="link" onClick={() => onForm("register")}>
                  {t("home.register")}
                </button>
              )}
              {view.login && (
                <button type="button" className="link" onClick={() => onForm("login")}>
                  {t("home.login")}
                </button>
              )}
              {view.logout && (
                <>
                  <span className="left-account">{t("home.signed_in", { account: session?.account ?? "" })}</span>
                  <button type="button" className="link" onClick={onLogout}>
                    {t("home.logout")}
                  </button>
                </>
              )}
            </div>
          </div>
        </>
      )}
    </aside>
  );
}
