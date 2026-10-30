import { useEffect, useRef, useState } from "react";
import { getBlockHistory, getDecisionHistory } from "../api/client";
import type { Entry } from "../api/graph";
import { translate, type Lang } from "../i18n";

// P5 (backlog-r1 1.6): how a block came to be, newest first: who, when,
// the decision with its contrast, your «porque» and what was discarded.
// Go computes it from the history and the graph; no model narrates it.
// With decision set (P-41, 30-oct), it is that decision's own history:
// the signoff opens it for a decision with no point in the text.
export function HistoryPanel({ lang, projectID, label, name, block, decision, onClose }: {
  lang: Lang;
  projectID: string;
  label: string;
  name: string;
  block: number;
  decision?: string;
  onClose: () => void;
}) {
  const t = (key: string, params?: Record<string, string>) => translate(lang, key, params);
  const [entries, setEntries] = useState<Entry[] | null>(null);
  const panel = useRef<HTMLDivElement>(null);

  // The panel opens below the editor, inside a scrolling column: bring
  // it into view, or the click seems to do nothing (P-34).
  useEffect(() => {
    panel.current?.scrollIntoView({ behavior: "smooth", block: "nearest" });
  }, [block, label, decision]);

  useEffect(() => {
    setEntries(null);
    const load = decision ? getDecisionHistory(projectID, decision) : getBlockHistory(projectID, label, block);
    void load.then((v) => setEntries(v.entries), () => setEntries([]));
  }, [projectID, label, block, decision]);

  return (
    <div className="request-panel" ref={panel}>
      <h2>
        {decision
          ? t("history.decision_title", { section: name })
          : t("history.title", { section: name, block: String(block) })}
      </h2>
      <button type="button" onClick={onClose}>{t("history.close")}</button>
      {entries && entries.length === 0 && <p>{t("history.empty")}</p>}
      <ul>
        {entries?.map((e, i) => (
          <li key={i}>
            {e.kind === "text_change" ? (
              <span>
                {localTime(e.at, lang)} · {t(`history.text_change.${e.actor}`)}
              </span>
            ) : (
              e.decision && (
                <span>
                  {e.decision.discarded && <strong>{t("history.discarded")}: </strong>}
                  {localTime(e.at, lang)} · {t(e.decision.origin === "human" ? "history.decision.human" : "history.decision.agent")}{" "}
                  «{e.decision.statement}»
                  {" · "}{t("card.instead_of")} «{e.decision.instead_of}»
                  {e.decision.would_choose_if && <> · {t("card.would_choose_if")} «{e.decision.would_choose_if}»</>}
                  {e.decision.reason && <> · {t("history.your_reason")}: «{e.decision.reason}»</>}
                  {e.decision.figure && <> · {t("history.figure")}: {e.decision.figure}</>}
                </span>
              )
            )}
          </li>
        ))}
      </ul>
    </div>
  );
}

// localTime shows a stored UTC time in the browser's time zone (T-7).
function localTime(at: string, lang: Lang): string {
  const d = new Date(at);
  return Number.isNaN(d.getTime()) ? "" : d.toLocaleString(lang, { dateStyle: "medium", timeStyle: "short" });
}
