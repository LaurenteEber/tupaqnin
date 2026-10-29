import { useCallback, useEffect, useState } from "react";
import { confirmCase, correctCase, getUnderstanding, putSectionRole } from "../api/client";
import type { UnderstandingView } from "../api/types";
import { translate, type Lang } from "../i18n";

// The right half of P2 (backlog 1.2): the principal's understanding,
// its cases and the proposed section roles. The human confirms or
// corrects; nothing here blocks the work (fichas §2).
export function UnderstandingPanel({ lang, projectID, refreshKey }: {
  lang: Lang;
  projectID: string;
  refreshKey: number;
}) {
  const t = (key: string) => translate(lang, key);
  const [view, setView] = useState<UnderstandingView | null>(null);
  const [corrections, setCorrections] = useState<Record<string, string>>({});

  const refresh = useCallback(async () => {
    if (projectID) setView(await getUnderstanding(projectID));
  }, [projectID]);

  useEffect(() => {
    void refresh();
  }, [refresh, refreshKey]);

  if (!projectID) return null;

  return (
    <div className="request-panel">
      <h2>{t("understanding.title")}</h2>
      {!view || (view.cases.length === 0 && view.sections.length === 0) ? (
        <p>{t("understanding.empty")}</p>
      ) : (
        <>
          {view.understood && <p>{view.understood}</p>}
          <ul>
            {view.cases.map((c) => (
              <li key={c.label}>
                <span>
                  {c.counts === undefined
                    ? c.text
                    : `${t(c.counts ? "understanding.counts" : "understanding.does_not_count")}: ${c.text}`}
                  {" · "}{c.state}
                </span>
                <button type="button" onClick={() => void confirmCase(projectID, c.label).then(refresh)}>
                  {t("understanding.confirm")}
                </button>
                <input
                  aria-label={t("understanding.corrected")}
                  value={corrections[c.label] ?? ""}
                  onChange={(e) => setCorrections((prev) => ({ ...prev, [c.label]: e.target.value }))}
                />
                <button
                  type="button"
                  disabled={!corrections[c.label]}
                  onClick={() => void correctCase(projectID, c.label, corrections[c.label]).then(refresh)}
                >
                  {t("understanding.correct")}
                </button>
              </li>
            ))}
          </ul>
          <ul>
            {view.sections.map((s) => (
              <li key={s.label}>
                {s.title || s.label} · {t("understanding.role")}:{" "}
                <select
                  value={s.role}
                  onChange={(e) => void putSectionRole(projectID, s.label, e.target.value).then(refresh)}
                >
                  {["conclusion", "recommendation", "other"].map((role) => (
                    <option key={role} value={role}>{t(`understanding.role.${role}`)}</option>
                  ))}
                </select>
                {" · "}{s.role_state}
              </li>
            ))}
          </ul>
        </>
      )}
    </div>
  );
}
