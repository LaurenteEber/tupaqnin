import { FileButton } from "./FileButton";
import { evidenceFromKey } from "../plain";
import { useCallback, useEffect, useState } from "react";
import {
  changeSectionDetails,
  confirmCase,
  confirmPlan,
  correctCase,
  getUnderstanding,
  putEvidenceRequired,
  putSectionRole,
  uploadSource,
} from "../api/client";
import type { CaseView, SectionRoleView, UnderstandingView } from "../api/types";
import { translate, type Lang } from "../i18n";
import { understandingUnconfirmed } from "../understandingState";

// «Lo que entendí» as one card (U-6): (1) what was understood, with
// what was read from your material; (2) how it will be judged, only if
// there are criteria cases; (3) the plan, the understanding's sections
// with the mark «aquí se concluye» (R-8) and, from slice 2, what each
// section concludes or addresses and where its evidence comes from
// (R-6). «Confirm» confirms every case, every proposed mark and the plan
// (each line passes to its section, B-53); correcting a line or a mark
// also answers the plan. It does not hold the start, the chain has
// already started (B-17). Nothing here blocks the work (fichas §2).
export function UnderstandingPanel({ lang, projectID, refreshKey, onRoleChanged, onChanged, onSearchEvidence }: {
  lang: Lang;
  projectID: string;
  refreshKey: number;
  // A mark or a plan line changed: the section list reloads.
  onRoleChanged: () => void;
  // A case was confirmed or corrected: the frame shows its state.
  onChanged: () => void;
  // «Buscar evidencia» (B-72): you start a round; shown when the
  // document needs evidence.
  onSearchEvidence?: () => void;
}) {
  const t = (key: string) => translate(lang, key);
  const [view, setView] = useState<UnderstandingView | null>(null);
  const [corrections, setCorrections] = useState<Record<string, string>>({});
  // A plan line being corrected: its title and focus, by section label.
  const [lines, setLines] = useState<Record<string, { title: string; focus: string }>>({});
  const [lineError, setLineError] = useState("");
  // Folded once confirmed; you can reopen it. It opens by itself while
  // something is still unconfirmed.
  const [folded, setFolded] = useState(true);
  const [correcting, setCorrecting] = useState(false);
  const [busy, setBusy] = useState(false);
  const [uploadError, setUploadError] = useState("");

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

  // R-4: «subir material» on the card, as the request's upload does.
  async function uploadMaterial(file: File) {
    setUploadError("");
    try {
      await uploadSource(projectID, "file", file);
      await changed();
    } catch (e) {
      setUploadError(e instanceof Error ? e.message : "internal");
    }
  }

  // R-8: the mark is the role; marked is `conclusion` (a stored
  // `recommendation` also shows marked), unmarked is `other`. Changing
  // or confirming a role makes a new section version (P-27).
  async function toggleMark(s: SectionRoleView) {
    await putSectionRole(projectID, s.label, marked(s) ? "other" : "conclusion", true);
    await changed();
    onRoleChanged();
  }

  // Correcting a plan line: your title and focus, literal (B-53); the
  // first correction also answers the plan (R-6), Go records it.
  async function saveLine(s: SectionRoleView) {
    const line = lines[s.label];
    if (!line) return;
    setLineError("");
    try {
      await changeSectionDetails(projectID, s.label, { title: line.title, focus: line.focus, plan: true });
      setLines(({ [s.label]: _, ...rest }) => rest);
      await changed();
      onRoleChanged(); // the section list shows the new title and focus
    } catch (e) {
      setLineError((e as Error).message);
    }
  }

  async function confirmAll() {
    if (!view) return;
    setBusy(true);
    try {
      const answersPlan = !view.plan_confirmed && view.sections.length > 0;
      if (answersPlan) await confirmPlan(projectID);
      for (const c of view.cases) {
        if (c.state === "unconfirmed") await confirmCase(projectID, c.label);
      }
      const proposed = view.sections.filter((s) => s.role_state === "proposed");
      for (const s of proposed) await putSectionRole(projectID, s.label, s.role);
      await changed();
      // The plan's lines are now the sections' «qué debe abordar».
      if (proposed.length > 0 || answersPlan) onRoleChanged();
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
  async function toggleEvidence(required: boolean) {
    if (!projectID) return;
    setBusy(true);
    try {
      await putEvidenceRequired(projectID, required);
      await changed();
    } finally {
      setBusy(false);
    }
  }

  const empty = !view || (view.understood === "" && view.cases.length === 0 && view.sections.length === 0);
  if (empty) {
    return (
      <div className="chat-card understanding" id="understanding">
        <h2>{t("understanding.title")}</h2>
        {failed ? <p className="turn-error">{t("understanding.pending")}</p> : <p className="meta">{t("understanding.empty")}</p>}
      </div>
    );
  }

  const unconfirmed = understandingUnconfirmed(view);
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
      {/* C.19 (B-109). [EL DISEÑO NO LO DICE] With a failed understanding it shows too: it did start. */}
      {unconfirmed && <p className="meta agent-started">{t("understanding.agent_started")}</p>}

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
          <h3>
            {t("understanding.plan")}{" "}
            <span className="meta">
              · {t(view.plan_confirmed ? "understanding.plan.confirmed" : "understanding.plan.proposed")}
            </span>
          </h3>
          <ul className="plan">
            {view.sections.map((s) => {
              const line = lines[s.label];
              return (
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
                  {s.focus && (
                    <div className="meta">
                      {t("understanding.plan.addresses")}: {s.focus}
                    </div>
                  )}
                  {s.evidence_from && (
                    <div className="meta">
                      {t("understanding.plan.evidence_from")}: {evidenceFromKey(s.evidence_from) ? t(evidenceFromKey(s.evidence_from)) : s.evidence_from}
                    </div>
                  )}
                  {correcting && (
                    <span className="correct-row">
                      <input
                        aria-label={t("section.field.title")}
                        placeholder={t("section.field.title")}
                        value={line?.title ?? s.title}
                        onChange={(e) =>
                          setLines((prev) => ({
                            ...prev,
                            [s.label]: { title: e.target.value, focus: prev[s.label]?.focus ?? s.focus ?? "" },
                          }))
                        }
                      />
                      <input
                        aria-label={t("section.field.focus")}
                        placeholder={t("section.field.focus")}
                        value={line?.focus ?? s.focus ?? ""}
                        onChange={(e) =>
                          setLines((prev) => ({
                            ...prev,
                            [s.label]: { title: prev[s.label]?.title ?? s.title, focus: e.target.value },
                          }))
                        }
                      />
                      <button type="button" className="small" disabled={!line} onClick={() => void saveLine(s)}>
                        {t("understanding.save_correction")}
                      </button>
                    </span>
                  )}
                </li>
              );
            })}
          </ul>
          {lineError && <p className="turn-error">{t(`section.error.${lineError}`)}</p>}
        </>
      )}

      {view.evidence_required && (
        // «¿Pide evidencia?» (B-51): proposed by the «Entendí», corrected
        // here; with yes, a paragraph without a verified source blocks
        // the signoff (R-1).
        <p className="evidence">
          {correcting ? (
            <label>
              <input
                type="checkbox"
                checked={view.evidence_required.value}
                onChange={() => void toggleEvidence(!view.evidence_required!.value)}
              />
              {t("understanding.evidence_required")}
            </label>
          ) : (
            <>
              {t("understanding.evidence_required")}{" "}
              <b>{t(view.evidence_required.value ? "understanding.evidence.yes" : "understanding.evidence.no")}</b>
            </>
          )}
          {view.evidence_required.why && <> · {view.evidence_required.why}</>}
          {view.evidence_required.state === "proposed" && (
            <span className="meta"> · {t("understanding.role_state.proposed")}</span>
          )}
          {view.evidence_required.value && onSearchEvidence && (
            <>
              {" "}
              <button type="button" className="small" onClick={onSearchEvidence}>
                {t("understanding.search_evidence")}
              </button>
            </>
          )}
        </p>
      )}
      {view.evidence_required?.value && view.no_material && (
        // R-4: without material, its two ways on: this and «Buscar evidencia».
        <p className="evidence">
          {t("understanding.no_material")}{" "}
          <FileButton label={t("source.upload")} onFile={(f) => void uploadMaterial(f)} />
          {uploadError && <span className="turn-error"> {t(`source.error.${uploadError}`)}</span>}
        </p>
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
