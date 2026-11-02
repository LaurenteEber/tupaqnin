import { answerAlert } from "../api/client";
import type { EditAlert } from "../api/graph";
import { translate, type Lang } from "../i18n";
import { alertReasonKey, alertReasonWhat, undoReasonKey } from "../undo";

// The alert of a suspicious edit (backlog-r4 4.2, pantallas: «la alerta como
// banda bajo el título: qué pasó, «Deshacer» y «Fue a propósito»»). It never
// blocks: unanswered, your edit rules and the alert stays. Go finds the motives;
// what they name is shown literal (rule 8).
export function EditAlerts({
  lang,
  projectID,
  alerts,
  readOnly,
  onUndo,
  onAnswered,
}: {
  lang: Lang;
  projectID: string;
  alerts: EditAlert[];
  readOnly: boolean;
  onUndo: (editVersion: number) => void;
  onAnswered: () => void;
}) {
  const t = (key: string, vars?: Record<string, string>) => translate(lang, key, vars);
  if (alerts.length === 0) return null;
  return (
    <div className="warnings-band edit-alerts" role="alert">
      {alerts.map((a) => (
        <div key={a.seq} className="edit-alert">
          <ul>
            {a.reasons.map((r, i) => (
              <li key={i}>{t(alertReasonKey(r), { what: alertReasonWhat(r) || t("section.untitled") })}</li>
            ))}
          </ul>
          <b>{t("alert.question")}</b>
          {!readOnly && (
            <span className="warnings-actions">
              <button
                type="button"
                className="small"
                disabled={!a.undoable}
                title={a.undo_reason ? t(undoReasonKey(a.undo_reason)) : undefined}
                onClick={() => onUndo(a.version)}
              >
                {t("undo.button")}
              </button>
              <button type="button" className="small" onClick={() => void answerAlert(projectID, a.seq).finally(onAnswered)}>
                {t("alert.deliberate")}
              </button>
            </span>
          )}
        </div>
      ))}
    </div>
  );
}
