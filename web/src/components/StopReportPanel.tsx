import { useCallback, useEffect, useRef, useState } from "react";
import { answerAudienceQuestion, getStopReport, markStopReportSeen } from "../api/client";
import type { StopReportView } from "../api/types";
import { detectedLine } from "../detected";
import { translate, type Lang } from "../i18n";
import { stopKey } from "../live";
import { plainAgentText } from "../plain";

// P6 (backlog-r2 2.5): the last stop report, in the conversation, folded
// under its first line. Everything comes from Go: why it stopped, the
// section and whether to propose signing it, what is verified, what is
// yours. Opening it counts as seen (B-40); folded does not. Never a
// label on screen: sections by title, decisions by statement. The
// defense (backlog-r2 2.6, 5b) lists the open audience questions of the
// section with «Sí» and «No»; they never block «Firmar».
// The short return (backlog-r2 2.7, B-48 b): under the report, what the
// app detected since it, your edits included; when Go says so (unseen,
// no return line), the conversation opens with the report on top.
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
  const [answerError, setAnswerError] = useState(false);
  // The conversation opens with the report on top once per project.
  const broughtUp = useRef("");
  const box = useRef<HTMLDivElement>(null);

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

  useEffect(() => {
    if (!report?.opens_conversation || broughtUp.current === projectID) return;
    broughtUp.current = projectID;
    box.current?.scrollIntoView({ block: "start" });
  }, [report, projectID]);

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

  async function answer(label: string, value: "yes" | "no") {
    setAnswerError(false);
    try {
      await answerAudienceQuestion(projectID, label, value);
      await load();
    } catch {
      setAnswerError(true);
    }
  }

  const title = report.section ? report.section.title || t("section.untitled") : "";
  const first = plainAgentText(report.answer) || t("stop_report.no_answer");
  const continueText = t("stop_report.continue");
  const addressText = t("stop_report.address");

  return (
    <div ref={box}>
      <details id="stop-report" className="agent-step" onToggle={(e) => void opened(e.currentTarget.open)}>
        <summary>
          <b>{title ? t("stop_report.title_section", { section: title }) : t("stop_report.title")}</b>
          {!report.seen && <> · <span className="st todo">{t("stop_report.unseen")}</span></>}
          <span className="reply" style={{ display: "block" }}>{first}</span>
        </summary>
        <p className="meta">{t(stopKey(report.stop_reason, report.cause))}</p>
        {/* P-75 (c): Go says the principal's part failed; the app's part follows. */}
        {report.principal_failed && <p className="meta">{t("stop_report.incomplete")}</p>}
        {/* Backlog-r3 3.6 (fichas §7): visible, blocking nothing. */}
        {(report.judge_not_verified ?? []).length > 0 && (
          <p className="meta">
            {t("stop_report.judge_not_verified", {
              sections: report.judge_not_verified.map((s) => s.title || t("section.untitled")).join(", "),
            })}
          </p>
        )}
        <details className="note">
          <summary>{t("stop_report.done", { n: String(report.done.length) })}</summary>
          <ul>{report.done.map((d, i) => <li key={i}>{plainAgentText(d)}</li>)}</ul>
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
        {report.agent_pendings.length > 0 && (
          <details className="note">
            <summary>{t("stop_report.agent_pendings", { n: String(report.agent_pendings.length) })}</summary>
            <ul>
              {report.agent_pendings.map((p, i) => (
                <li key={i}>
                  {t(`pending.${p.kind}`)}
                  {p.cause && <> · {t(`pending.cause.${p.cause}`)}</>}
                  {p.passage && <>: «{p.passage}»</>} · {p.section.title || t("section.untitled")}
                </li>
              ))}
            </ul>
          </details>
        )}
        {report.decisions.length > 0 && (
          <details className="note">
            <summary>{t("stop_report.unseen_decisions", { n: String(report.decisions.length) })}</summary>
            <ul>
              {report.decisions.map((d) => (
                <li key={d.label}>
                  {plainAgentText(d.gist || d.statement)}
                  {d.sections.length > 0 && (
                    <span className="meta"> · {d.sections.map((s) => s.title || t("section.untitled")).join(" · ")}</span>
                  )}
                </li>
              ))}
            </ul>
          </details>
        )}
        {report.next && <p className="triage">{t("stop_report.next")}: {plainAgentText(report.next)}</p>}
        {report.audience_questions.length > 0 && (
          <div className="note">
            <p className="meta">{t("stop_report.defense")}</p>
            <ul>
              {report.audience_questions.map((q) => (
                <li key={q.label}>
                  {q.not_covered ? t("stop_report.defense.not_covered", { text: q.question }) : q.question}{" "}
                  <button type="button" className="small" disabled={disabled} onClick={() => void answer(q.label, "yes")}>
                    {t("stop_report.defense.yes")}
                  </button>{" "}
                  <button type="button" className="small" disabled={disabled} onClick={() => void answer(q.label, "no")}>
                    {t("stop_report.defense.no")}
                  </button>
                </li>
              ))}
            </ul>
            <p className="meta">{t("stop_report.defense.hint")}</p>
            {answerError && <p className="meta">{t("stop_report.defense.error")}</p>}
          </div>
        )}
        {report.defense_failed && <p className="meta">{t("stop_report.defense.failed")}</p>}
        {/* B-100: your decisions that have not reached the text, read now. */}
        {report.unapplied.length > 0 && (
          <div className="unapplied" role="status">
            <p className="meta">{t("stop_report.unapplied", { section: report.unapplied.join(", ") })}</p>
            <button type="button" className="small" disabled={disabled}
              onClick={() => onContinue(t("stop_report.unapplied_message", { section: report.unapplied.join(", ") }))}>
              {t("stop_report.unapplied_apply")}
            </button>
          </div>
        )}
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
      {report.detected.length > 0 && (
        <div className="note detected">
          <p className="meta">{t("detected.title")}</p>
          <ul>{report.detected.map((d, i) => <li key={i}>{detectedLine(t, d)}</li>)}</ul>
        </div>
      )}
    </div>
  );
}
