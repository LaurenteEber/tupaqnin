import type { ReactNode } from "react";
import type { RightTab } from "../layout";
import { translate, type Lang } from "../i18n";

// The right zone's tabs. Every pane stays mounted while its tab is away, so a
// decision left open in the frame is still open when you come back.
export function RightPanel({
  lang,
  tab,
  historyOpen,
  onTab,
  onClose,
  frame,
  history,
  markdown,
}: {
  lang: Lang;
  tab: RightTab;
  historyOpen: boolean;
  onTab: (tab: RightTab) => void;
  onClose: () => void;
  frame: ReactNode;
  history: ReactNode;
  markdown: ReactNode;
}) {
  const t = (key: string) => translate(lang, key);
  const tabButton = (id: RightTab, label: string) => (
    <button
      type="button"
      role="tab"
      aria-selected={tab === id}
      className={`tab${tab === id ? " on" : ""}`}
      onClick={() => onTab(id)}
    >
      {label}
    </button>
  );
  return (
    <>
      <div className="right-head">
        <div className="tabs" role="tablist" aria-label={t("layout.right_view")}>
          {tabButton("frame", t("frame.title"))}
          {historyOpen && tabButton("history", t("history.tab"))}
          {tabButton("markdown", t("markdown.title"))}
        </div>
        <span className="spacer" />
        <button type="button" className="icon-btn" title={t("layout.right_close")} aria-label={t("layout.right_close")} onClick={onClose}>
          ✕
        </button>
      </div>
      <div className="right-body">
        <section className={`right-pane${tab === "frame" ? "" : " off"}`} role="tabpanel" aria-label={t("frame.title")}>
          {frame}
        </section>
        {historyOpen && (
          <section className={`right-pane${tab === "history" ? "" : " off"}`} role="tabpanel" aria-label={t("history.tab")}>
            {history}
          </section>
        )}
        <section className={`right-pane${tab === "markdown" ? "" : " off"}`} role="tabpanel" aria-label={t("markdown.title")}>
          {markdown}
        </section>
      </div>
    </>
  );
}
