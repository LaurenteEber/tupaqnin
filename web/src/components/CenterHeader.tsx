import type { ReactNode } from "react";
import type { CenterView } from "../layout";
import { translate, type Lang } from "../i18n";

// The center's header, for both views (U.1): the selector, the project's title,
// the mode and the right panel's toggle; under them, the model line.
export function CenterHeader({
  lang,
  view,
  hasDocument,
  conversationNews,
  onView,
  title,
  mode,
  model,
  status,
  onToggleRight,
}: {
  lang: Lang;
  view: CenterView;
  hasDocument: boolean;
  conversationNews: boolean;
  onView: (view: CenterView) => void;
  title: ReactNode;
  mode: ReactNode;
  model: ReactNode;
  status?: ReactNode;
  onToggleRight: () => void;
}) {
  const t = (key: string) => translate(lang, key);
  return (
    <div className="center-head">
      <div className="head-row">
        <span className="seg" role="tablist" aria-label={t("layout.center_view")}>
          <button type="button" role="tab" aria-selected={view === "conv"} className={view === "conv" ? "on" : ""} onClick={() => onView("conv")}>
            {t("chat.title")}
            {/* [EL DISEÑO NO LO DICE] From the document, a message or an audit runs
                out of sight: a dot says the conversation has something new. */}
            {conversationNews && view !== "conv" && <span className="dot news" role="img" aria-label={t("center.activity")} />}
          </button>
          <button type="button" role="tab" aria-selected={view === "doc"} className={view === "doc" ? "on" : ""} disabled={!hasDocument} onClick={() => onView("doc")}>
            {t("center.doc")}
          </button>
        </span>
        <div className="head-title">{title}</div>
        <span className="spacer" />
        {mode}
        <button type="button" className="icon-btn" title={t("layout.right_toggle")} aria-label={t("layout.right_toggle")} onClick={onToggleRight}>
          ▤
        </button>
      </div>
      {model}
      {status}
    </div>
  );
}
