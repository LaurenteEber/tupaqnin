import { useEffect, useState } from "react";
import { getBlockHistory } from "../api/client";
import type { Entry } from "../api/graph";
import { translate, type Lang } from "../i18n";

// P5 (backlog-r1 1.6): how a block came to be, newest first: who, when,
// the decision with its contrast, your «porque» and what was discarded.
// Go computes it from the history and the graph; no model narrates it.
export function HistoryPanel({ lang, projectID, label, name, block, onClose }: {
  lang: Lang;
  projectID: string;
  label: string;
  name: string;
  block: number;
  onClose: () => void;
}) {
  const t = (key: string, params?: Record<string, string>) => translate(lang, key, params);
  const [entries, setEntries] = useState<Entry[] | null>(null);

  useEffect(() => {
    setEntries(null);
    void getBlockHistory(projectID, label, block).then((v) => setEntries(v.entries), () => setEntries([]));
  }, [projectID, label, block]);

  return (
    <div className="request-panel">
      <h2>{t("history.title", { section: name, block: String(block) })}</h2>
      <button type="button" onClick={onClose}>{t("history.close")}</button>
      {entries && entries.length === 0 && <p>{t("history.empty")}</p>}
      <ul>
        {entries?.map((e, i) => (
          <li key={i}>
            {e.kind === "text_change" ? (
              <span>
                v{e.version} · {t(`history.actor.${e.actor}`)} · {e.at} · {t("history.text_change")}
              </span>
            ) : (
              e.decision && (
                <span>
                  {e.decision.discarded && <strong>{t("history.discarded")}: </strong>}
                  {t(`history.actor.${e.actor}`)} · {e.at} · {e.decision.label} «{e.decision.statement}»
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
