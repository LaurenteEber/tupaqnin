import { useState } from "react";
import { closeChange } from "../api/client";
import type { Warning } from "../api/graph";
import { translate, type Lang } from "../i18n";

// The band under a section's title and «qué debe abordar» (backlog-r2
// 2.9, pantallas «Avisos en el texto»): the open changes with «Actualizado»
// and «Revisado, no requiere cambio» (this one asks for a «porque»), then
// the open «por aclarar» and the agent's pendings, each with its link.
// Go computes every warning, its words and its place; this only presents
// them. No id is shown: labels go only into the links.
export function SectionWarnings({
  lang,
  projectID,
  warnings,
  readOnly,
  onClosed,
  onQuestion,
  onHistory,
}: {
  lang: Lang;
  projectID: string;
  warnings: Warning[];
  readOnly: boolean;
  onClosed: () => void;
  onQuestion: (label: string) => void;
  onHistory: (block: number) => void;
}) {
  const t = (key: string, params?: Record<string, string>) => translate(lang, key, params);
  const [reviewing, setReviewing] = useState("");
  const [reason, setReason] = useState("");
  const [error, setError] = useState<{ change: string; code: string } | null>(null);
  const [busy, setBusy] = useState(false);

  if (warnings.length === 0) return null;
  const changes = warnings.filter((w) => w.kind === "changed" || w.kind === "stale");
  const others = warnings.filter((w) => w.kind === "open_question" || w.kind === "agent_pending");
  const flaws = warnings.filter((w) => w.kind === "flaw");

  async function close(change: string, result: "updated" | "reviewed_unchanged") {
    setBusy(true);
    setError(null);
    try {
      await closeChange(projectID, change, result, result === "reviewed_unchanged" ? reason : "");
      setReviewing("");
      setReason("");
      onClosed();
    } catch (e) {
      setError({ change, code: e instanceof Error ? e.message : "internal" });
    } finally {
      setBusy(false);
    }
  }

  function changeLine(w: Warning) {
    const kind = w.change_kind || "other";
    return w.what ? t(`warnings.change.${kind}`, { what: w.what }) : t("warnings.change.unnamed");
  }

  return (
    <div className="warnings-band" role="region" aria-label={t("warnings.title")}>
      {changes.length > 0 && (
        <>
          <b>{t("warnings.changes", { n: String(changes.length) })}</b>
          <ul>
            {changes.map((w) => (
              <li key={w.target}>
                <span>{changeLine(w)}</span>
                {!readOnly && (
                  <span className="warnings-actions">
                    <button type="button" className="small" disabled={busy} onClick={() => void close(w.target, "updated")}>
                      {t("warnings.updated")}
                    </button>
                    <button
                      type="button"
                      className="small"
                      disabled={busy}
                      onClick={() => {
                        setReviewing(reviewing === w.target ? "" : w.target);
                        setReason("");
                        setError(null);
                      }}
                    >
                      {t("warnings.reviewed")}
                    </button>
                  </span>
                )}
                {reviewing === w.target && (
                  <span className="warnings-reason">
                    <input
                      autoFocus
                      value={reason}
                      placeholder={t("warnings.reason")}
                      onChange={(e) => setReason(e.target.value)}
                    />
                    <button
                      type="button"
                      className="small primary"
                      disabled={busy || reason.trim() === ""}
                      onClick={() => void close(w.target, "reviewed_unchanged")}
                    >
                      {t("warnings.save_reason")}
                    </button>
                  </span>
                )}
                {error?.change === w.target && <span className="turn-error">{t(`warnings.error.${error.code}`)}</span>}
              </li>
            ))}
          </ul>
        </>
      )}
      {others.length > 0 && (
        <ul>
          {others.map((w, i) =>
            w.kind === "open_question" ? (
              <li key={`oq-${w.target}`}>
                {t("warnings.open_question")}
                {w.what && <> «{w.what}»</>}{" "}
                <button type="button" className="link" onClick={() => onQuestion(w.target)}>
                  {t("warnings.go_question")}
                </button>
              </li>
            ) : (
              <li key={`p-${i}`}>
                {t("warnings.agent_pending", { pending: t(`pending.${w.pending}`) })}
                {w.cause && <> · {t(`pending.cause.${w.cause}`)}</>}
                {w.passage && <> · «{w.passage}»</>}{" "}
                {w.block ? (
                  <button type="button" className="link" onClick={() => onHistory(w.block ?? 0)}>
                    {t("warnings.go_history")}
                  </button>
                ) : null}
              </li>
            ),
          )}
        </ul>
      )}
      {flaws.length > 0 && (
        <ul>
          {flaws.map((w, i) => (
            <li key={`f-${i}`}>
              {t("warnings.flaw")}
              {w.passage && <> «{w.passage}»</>}{" "}
              <button type="button" className="link" onClick={() => onQuestion(w.target)}>
                {t("warnings.go_flaw")}
              </button>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
