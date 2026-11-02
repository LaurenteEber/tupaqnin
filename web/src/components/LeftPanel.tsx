import type { ReactNode } from "react";
import type { ProjectLine } from "../api/types";
import { translate, type Lang } from "../i18n";

// The left zone. U.1: what the «Tus proyectos ▾» menu held, in place.
export function LeftPanel({
  lang,
  off,
  onToggle,
  projects,
  projectID,
  onOpen,
  onNew,
  footer,
}: {
  lang: Lang;
  off: boolean;
  onToggle: () => void;
  projects: ProjectLine[];
  projectID: string;
  onOpen: (id: string) => void;
  onNew: (template?: string) => void;
  footer: ReactNode;
}) {
  const t = (key: string) => translate(lang, key);
  return (
    <aside className="left" aria-label={t("layout.projects")}>
      <div className="left-top">
        <button type="button" className="icon-btn" title={t("layout.left_toggle")} aria-label={t("layout.left_toggle")} aria-expanded={!off} onClick={onToggle}>
          ☰
        </button>
        {!off && (
          <div className="brand">
            <h1>{t("app.title")}</h1>
            <small>{t("app.tagline")}</small>
          </div>
        )}
      </div>
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
      {!off && (
        <>
          <div className="section-label">{t("project.list")}</div>
          <nav className="projects" aria-label={t("project.list")}>
            {projects.map((p) => (
              <button
                key={p.project_id}
                type="button"
                className={`project-row${p.project_id === projectID ? " active" : ""}`}
                aria-current={p.project_id === projectID ? "true" : undefined}
                onClick={() => onOpen(p.project_id)}
              >
                {p.name || t("project.untitled")}
              </button>
            ))}
          </nav>
          <div className="left-bottom">{footer}</div>
        </>
      )}
    </aside>
  );
}
