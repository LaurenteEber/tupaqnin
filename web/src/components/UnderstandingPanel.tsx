import { useCallback, useEffect, useState } from "react";
import { confirmCase, correctCase, getUnderstanding, putSectionRole } from "../api/client";
import type { CaseView, SectionRoleView, UnderstandingView } from "../api/types";
import { translate, type Lang } from "../i18n";

// «Lo que entendí» as one card (U-6): (1) what was understood, with
// what was read from your material; (2) how it will be judged, only if
// there are criteria cases; (3) the plan, the understanding's sections
// with the mark «aquí se concluye» (R-8). «Confirm» confirms every case
// and every proposed mark; it does not hold the start, the chain has
// already started (B-17). Nothing here blocks the work (fichas §2).
export function UnderstandingPanel({ lang, projectID, refreshKey, onRoleChanged, onChanged }: {
  lang: Lang;
  projectID: string;
  refreshKey: number;
  onRoleChanged: () => void;
  // A case was confirmed or corrected: the frame shows its state.
  onChanged: () => void;
}) {
  const t = (key: string) => translate(lang, key);
  const [view, setView] = useState<UnderstandingView | null>(null);
  const [corrections, setCorrections] = useState<Record<string, string>>({});
  // Folded once confirmed; you can reopen it. It opens by itself while
  // something is still unconfirmed.
  const [folded, setFolded] = useState(true);
  const [correcting, setCorrecting] = useState(false);
  const [busy, setBusy] = useState(false);

  const refresh = useCallback(async () => {
    if (projectID) setView(await getUnderstanding(projectID));
  }, [projectID]);
  const changed = async () => {
    await refresh();
    onChanged();
  };

  useEffect(() => {
    void refresh();
  }, [refresh, refreshKey]);

  // R-8: the mark is the role; marked is `conclusion` (a stored
  // `recommendation` also shows marked), unmarked is `other`. Changing
  // or confirming a role makes a new section version (P-27).
  async function toggleMark(s: SectionRoleView) {
    await putSectionRole(projectID, s.label, marked(s) ? "other" : "conclusion");
    await refresh();
    onRoleChanged();
  }

  async function confirmAll() {
    if (!view) return;
    setBusy(true);
    try {
      for (const c of view.cases) {
        if (c.state === "unconfirmed") await confirmCase(projectID, c.label);
      }
      const proposed = view.sections.filter((s) => s.role_state === "proposed");
      for (const s of proposed) await putSectionRole(projectID, s.label, s.role);
      await changed();
      if (proposed.length > 0) onRoleChanged();
      setCorrecting(false);
      setFolded(true);
    } finally {
      setBusy(false);
    }
  }

  if (!projectID) return null;
  // P-43: a failed understanding shows as failed, also after a reload
  // (Go reads it from the events).
  const failed = view?.status === "agent_pending";
  const empty = !view || (view.understood === "" && view.cases.length === 0 && view.sections.length === 0);
  if (empty) {
    return (
      <div className="chat-card understanding" id="understanding">
        <h2>{t("understanding.title")}</h2>
        {failed ? <p className="turn-error">{t("understanding.pending")}</p> : <p className="meta">{t("understanding.empty")}</p>}
      </div>
    );
  }

  const unconfirmed =
    view.cases.some((c) => c.state === "unconfirmed") || view.sections.some((s) => s.role_state === "proposed");
  if (folded && !unconfirmed && !failed) {
    return (
      <div className="chat-card understanding folded" id="understanding">
        <span>
          <b>{t("understanding.title")}</b> · {t("understanding.confirmed")}
        </span>
        <button type="button" className="link" onClick={() => setFolded(false)}>
          {t("understanding.reopen")}
        </button>
      </div>
    );
  }

  const readings = view.cases.filter((c) => !c.criterion_label);
  const criteria = view.cases.filter((c) => c.criterion_label);
  const caseItem = (c: CaseView) => (
    <li key={c.label}>
      <span>
        {c.counts === undefined
          ? c.text
          : `${t(c.counts ? "understanding.counts" : "understanding.does_not_count")}: ${c.text}`}{" "}
        <span className={`st${c.state === "unconfirmed" ? " todo" : ""}`}>{t(`understanding.case_state.${c.state}`)}</span>
      </span>
      {correcting && (
        <span className="correct-row">
          <input
            aria-label={t("understanding.corrected")}
            placeholder={t("understanding.corrected")}
            value={corrections[c.label] ?? ""}
            onChange={(e) => setCorrections((prev) => ({ ...prev, [c.label]: e.target.value }))}
          />
          <button
            type="button"
            className="small"
            disabled={!corrections[c.label]}
            onClick={() => void correctCase(projectID, c.label, corrections[c.label]).then(changed)}
          >
            {t("understanding.save_correction")}
          </button>
        </span>
      )}
    </li>
  );

  return (
    <div className="chat-card understanding" id="understanding">
      <h2>{t("understanding.title")}</h2>
      {failed && <p className="turn-error">{t("understanding.pending")}</p>}

      {view.understood && <p>{view.understood}</p>}
      {readings.length > 0 && (
        <>
          <h3>{t("understanding.read")}</h3>
          <ul className="cases">{readings.map(caseItem)}</ul>
        </>
      )}

      {criteria.length > 0 && (
        <>
          <h3>{t("understanding.judged")}</h3>
          <ul className="cases">{criteria.map(caseItem)}</ul>
        </>
      )}

      {view.sections.length > 0 && (
        <>
          <h3>{t("understanding.plan")}</h3>
          <ul className="plan">
            {view.sections.map((s) => (
              <li key={s.label}>
                {correcting ? (
                  <label>
                    <input type="checkbox" checked={marked(s)} onChange={() => void toggleMark(s)} />
                    {s.title || t("section.untitled")} · {t("understanding.concludes")}
                  </label>
                ) : (
                  <span>
                    {s.title || t("section.untitled")}
                    {marked(s) && <>{" "}<span className="st mark">{t("understanding.concludes")}</span></>}
                  </span>
                )}
                {s.role_state === "proposed" && <span className="meta"> · {t("understanding.role_state.proposed")}</span>}
              </li>
            ))}
          </ul>
        </>
      )}

      <div className="row">
        <span />
        <span>
          <button type="button" className={correcting ? "active" : ""} onClick={() => setCorrecting((c) => !c)}>
            {t("understanding.correct")}
          </button>{" "}
          <button type="button" className="primary" disabled={busy} onClick={() => void confirmAll()}>
            {t("understanding.confirm")}
          </button>
        </span>
      </div>
    </div>
  );
}

function marked(s: SectionRoleView): boolean {
  return s.role === "conclusion" || s.role === "recommendation";
}
