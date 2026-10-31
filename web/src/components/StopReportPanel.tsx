import { useCallback, useEffect, useState } from "react";
import { getStopReport, markStopReportSeen } from "../api/client";
import type { StopReportView } from "../api/types";
import { translate, type Lang } from "../i18n";

// P6 (backlog-r2 2.5): the last stop report, in the conversation, folded
// under its first line. Everything comes from Go: why it stopped, the
// section and whether to propose signing it, what is verified, what is
// yours. Opening it counts as seen (B-40); folded does not. Never a
// label on screen: sections by title, decisions by statement.
export function StopReportPanel({ lang, projectID, refreshKey, disabled, onContinue, onSign, onSeen }: {
  lang: Lang;
  projectID: string;
  refreshKey: string | number;
  disabled: boolean;
  // «Sigue» and «Abordémoslo» send the button's text as your message.
  onContinue: (text: string) => void;
  // «Firmar» opens P7 for the report's section.
  onSign: (section: string) => void;
  onSeen: () => void;
}) {
  const t = (key: string, params?: Record<string, string>) => translate(lang, key, params);
  const [report, setReport] = useState<StopReportView | null>(null);

  const load = useCallback(async () => {
    try {
      setReport(await getStopReport(projectID));
    } catch {
      setReport(null);
    }
  }, [projectID]);
  useEffect(() => {
    void load();
  }, [load, refreshKey]);

  if (!report) return null;

  async function opened(open: boolean) {
    if (!open || !report || report.seen) return;
    try {
      await markStopReportSeen(projectID);
      setReport({ ...report, seen: true });
      onSeen();
    } catch {
      // Not recorded: it stays unseen and opening it again retries.
    }
  }

  const title = report.section ? report.section.title || t("section.untitled") : "";
  const first = report.answer || t("stop_report.no_answer");
  const continueText = t("stop_report.continue");
  const addressText = t("stop_report.address");

  return (
    <details id="stop-report" className="agent-step" onToggle={(e) => void opened(e.currentTarget.open)}>
      <summary>
        <b>{title ? t("stop_report.title_section", { section: title }) : t("stop_report.title")}</b>
        {!report.seen && <> · <span className="st todo">{t("stop_report.unseen")}</span></>}
        <span className="reply" style={{ display: "block" }}>{first}</span>
      </summary>
      <p className="meta">{t(`turn.stop.${report.stop_reason}`)}</p>
      <details className="note">
        <summary>{t("stop_report.done", { n: String(report.done.length) })}</summary>
        <ul>{report.done.map((d, i) => <li key={i}>{d}</li>)}</ul>
      </details>
      <details className="note">
        <summary>{t("stop_report.verified", { n: String(report.verified.length) })}</summary>
        <ul>
          {report.verified.map((c, i) => (
            <li key={i}>«{c.claim}» · {c.section.title || t("section.untitled")}</li>
          ))}
        </ul>
      </details>
      <details className="note">
        <summary>{t("stop_report.not_verified", { n: String(report.not_verified.length) })}</summary>
        <ul>
          {report.not_verified.map((c, i) => (
            <li key={i}>«{c.claim}» · {c.section.title || t("section.untitled")}</li>
          ))}
        </ul>
      </details>
      <details className="note">
        <summary>{t("stop_report.open", { n: String(report.open.length) })}</summary>
        <ul>
          {report.open.map((o) => (
            <li key={o.label}>{t(`stop_report.open.${o.kind}`)}: «{o.text}»</li>
          ))}
        </ul>
      </details>
      {report.decisions.length > 0 && (
        <details className="note">
          <summary>{t("stop_report.unseen_decisions", { n: String(report.decisions.length) })}</summary>
          <ul>
            {report.decisions.map((d) => (
              <li key={d.label}>
                {d.gist || d.statement}
                {d.sections.length > 0 && (
                  <span className="meta"> · {d.sections.map((s) => s.title || t("section.untitled")).join(" · ")}</span>
                )}
              </li>
            ))}
          </ul>
        </details>
      )}
      {report.next && <p className="triage">{t("stop_report.next")}: {report.next}</p>}
      <div className="empty-actions">
        <button type="button" disabled={disabled} onClick={() => onContinue(continueText)}>{continueText}</button>
        <button type="button" disabled={disabled} onClick={() => onContinue(addressText)}>{addressText}</button>
        {report.propose_signoff && report.section && (
          <button type="button" className="primary small" disabled={disabled}
            onClick={() => report.section && onSign(report.section.label)}>
            {t("stop_report.sign", { section: title })}
          </button>
        )}
      </div>
    </details>
  );
}
