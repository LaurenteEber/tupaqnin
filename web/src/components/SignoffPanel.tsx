import { useCallback, useEffect, useState } from "react";
import {
  askFromSignoff,
  getSignoff,
  leaveSignoff,
  postSignoffAnswers,
  postSignoffExplanation,
  signSignoff,
  subscribeTurn,
} from "../api/client";
import type { SignoffView } from "../api/graph";
import { translate, type Lang } from "../i18n";
import { HistoryPanel } from "./HistoryPanel";

// P7 (backlog-r1 1.7): the signoff, full screen, without the frame and
// without the text: you explain without looking (flujos §3). Go decides
// what is missing, whether it coincides and whether you can sign; this
// screen only shows it and sends your answers.
export function SignoffPanel({ lang, projectID, label, onClose }: {
  lang: Lang;
  projectID: string;
  label: string;
  onClose: () => void;
}) {
  const t = (key: string, params?: Record<string, string>) => translate(lang, key, params);
  const [view, setView] = useState<SignoffView | null>(null);
  const [explanation, setExplanation] = useState("");
  const [originAnswer, setOriginAnswer] = useState("");
  const [caseAnswer, setCaseAnswer] = useState("");
  const [contrasting, setContrasting] = useState(false);
  const [errorCode, setErrorCode] = useState("");
  const [historyBlock, setHistoryBlock] = useState(0);
  const [question, setQuestion] = useState("");
  const [agentAnswer, setAgentAnswer] = useState("");
  const [asking, setAsking] = useState(false);

  const reload = useCallback(async () => {
    try {
      setView(await getSignoff(projectID, label));
    } catch (e) {
      setErrorCode(e instanceof Error ? e.message : "internal");
    }
  }, [projectID, label]);

  useEffect(() => {
    void reload();
  }, [reload]);

  function watch(turnID: string) {
    if (!turnID) {
      void reload(); // a brief signoff calls no model (A-4)
      return;
    }
    setContrasting(true);
    subscribeTurn(projectID, turnID, {
      onDelta: () => {},
      onStep: () => {
        setContrasting(false);
        setCaseAnswer("");
        void reload();
      },
      onError: (code) => {
        setContrasting(false);
        setErrorCode(code);
        void reload();
      },
    });
  }

  async function run(action: () => Promise<{ turn_id: string }>) {
    setErrorCode("");
    try {
      const { turn_id } = await action();
      watch(turn_id);
    } catch (e) {
      setErrorCode(e instanceof Error ? e.message : "internal");
    }
  }

  async function sign() {
    setErrorCode("");
    try {
      setView(await signSignoff(projectID, label));
    } catch (e) {
      setErrorCode(e instanceof Error ? e.message : "internal");
      void reload();
    }
  }

  async function leave() {
    try {
      await leaveSignoff(projectID, label);
    } catch {
      // Leaving a signoff that already ended changes nothing.
    }
    onClose();
  }

  async function ask() {
    setAgentAnswer("");
    setErrorCode("");
    try {
      const { turn_id } = await askFromSignoff(projectID, label, question);
      setAsking(true);
      subscribeTurn(projectID, turn_id, {
        onDelta: (delta) => {
          if (delta.field === "note" || delta.field === "text") setAgentAnswer((prev) => prev + delta.text);
        },
        onStep: (step) => {
          setAsking(false);
          if (step.note) setAgentAnswer(step.note);
        },
        onError: (code) => {
          setAsking(false);
          setErrorCode(code);
        },
      });
    } catch (e) {
      setErrorCode(e instanceof Error ? e.message : "internal");
    }
  }

  if (!view) {
    return (
      <section className="signoff" aria-label={t("signoff.title_plain")}>
        {errorCode ? <p className="turn-error">{t(`signoff.error.${errorCode}`)}</p> : <p className="working">{t("signoff.loading")}</p>}
        <button type="button" onClick={onClose}>{t("signoff.back")}</button>
      </section>
    );
  }

  const title = view.section_title || t("section.untitled");
  const active = view.phase === "in_progress";
  const explained = view.rounds > 0 || (view.brief && view.explanation !== "");
  const contrastedBy = view.wonder_unavailable ? t("signoff.contrasted_spark") : t("signoff.contrasted_wonder");

  return (
    <section className="signoff" aria-label={t("signoff.title", { section: title })}>
      <div className="signoff-head">
        <h2>{t("signoff.title", { section: title })}</h2>
        {active ? (
          <button type="button" onClick={() => void leave()}>{t("signoff.leave")}</button>
        ) : (
          <button type="button" onClick={onClose}>{t("signoff.back")}</button>
        )}
      </div>

      {!active && <p className="signoff-phase">{t(`signoff.phase.${view.phase}${view.cancelled ? `.${view.cancelled}` : ""}`)}</p>}

      <ol className="signoff-steps">
        <li>
          <b>{t("signoff.before")}</b>{" "}
          {view.pendings.length === 0 ? (
            <span>{t("signoff.before_none")} ✓</span>
          ) : (
            <ul>
              {view.pendings.map((p, i) => (
                <li key={i}>
                  {t(`pending.${p.kind}`)}
                  {p.about && <> · «{p.about}»</>}
                </li>
              ))}
            </ul>
          )}
        </li>

        <li>
          <b>{view.brief ? t("signoff.explain_brief") : t("signoff.explain", { n: String(view.critical_decisions.length) })}</b>
          {explained ? (
            <blockquote className="signoff-quote">{view.explanation}</blockquote>
          ) : (
            <>
              <textarea
                rows={5}
                value={explanation}
                disabled={!active || contrasting}
                onChange={(e) => setExplanation(e.target.value)}
                placeholder={t("signoff.explain_placeholder")}
              />
              <button
                type="button"
                className="primary small"
                disabled={!active || contrasting || explanation.trim() === ""}
                onClick={() => void run(() => postSignoffExplanation(projectID, label, explanation))}
              >
                {t("signoff.send_explanation")}
              </button>
            </>
          )}
          {view.origin_question && (
            <div className="signoff-origin">
              <p>
                {t("signoff.origin_question")}: «{view.origin_question}»
              </p>
              {view.origin_answer && <blockquote className="signoff-quote">{view.origin_answer}</blockquote>}
              {active && !view.coincides && (
                <input
                  type="text"
                  value={originAnswer}
                  disabled={contrasting}
                  onChange={(e) => setOriginAnswer(e.target.value)}
                  placeholder={t("signoff.origin_placeholder")}
                />
              )}
            </div>
          )}
        </li>

        {!view.brief && (
          <li>
            <b>{t("signoff.result", { model: contrastedBy })}</b>
            {contrasting && <p className="working">{t("signoff.contrasting")}</p>}
            {view.wonder_unavailable && <p className="meta">{t("signoff.wonder_unavailable")}</p>}
            {view.rounds === 0 && !contrasting && <p className="meta">{t("signoff.no_result")}</p>}
            <ul className="signoff-verdicts">
              {view.verdicts.map((v) => (
                <li key={v.decision}>
                  {v.decision} · {t(`signoff.verdict.${v.verdict}`)}
                  {v.missing && <> · {v.missing}</>}
                  {v.passage && <> · «{v.passage}»</>}
                  {v.verdict !== "match" && v.block > 0 && (
                    <button type="button" className="small" onClick={() => setHistoryBlock(v.block)}>
                      {t("signoff.history")}
                    </button>
                  )}
                </li>
              ))}
            </ul>
            {view.cases.map((c, i) => (
              <p key={i} className="meta">
                {t("signoff.case")}: «{c.case}» · «{c.answer}» · {t(`signoff.case_result.${c.result}`)}
              </p>
            ))}
            {view.open_case && (
              <div className="signoff-case">
                <p>
                  {t("signoff.new_case")}: «{view.open_case}»
                </p>
                {active && (
                  <input
                    type="text"
                    value={caseAnswer}
                    disabled={contrasting}
                    onChange={(e) => setCaseAnswer(e.target.value)}
                    placeholder={t("signoff.case_placeholder")}
                  />
                )}
              </div>
            )}
            <p className="meta">
              {t("signoff.failed_cases", { n: String(view.failed_cases), max: String(view.max_failed_cases) })}
            </p>
            {active && view.rounds > 0 && !view.coincides && (
              <button
                type="button"
                className="primary small"
                disabled={contrasting || (view.open_case !== undefined && view.open_case !== "" && caseAnswer.trim() === "") ||
                  (originAnswer.trim() === "" && caseAnswer.trim() === "")}
                onClick={() => void run(() => postSignoffAnswers(projectID, label, originAnswer, caseAnswer))}
              >
                {t("signoff.send_answers")}
              </button>
            )}
            {active && view.rounds > 0 && (
              <div className="signoff-ask">
                <input
                  type="text"
                  value={question}
                  disabled={asking}
                  onChange={(e) => setQuestion(e.target.value)}
                  placeholder={t("signoff.ask_placeholder")}
                />
                <button type="button" className="small" disabled={asking || question.trim() === ""} onClick={() => void ask()}>
                  {t("signoff.ask")}
                </button>
                {asking && <p className="working">{t("turn.working")}</p>}
                {agentAnswer && <p className="meta">{agentAnswer}</p>}
              </div>
            )}
          </li>
        )}
      </ol>

      {historyBlock > 0 && (
        <HistoryPanel
          lang={lang}
          projectID={projectID}
          label={view.section}
          name={title}
          block={historyBlock}
          onClose={() => setHistoryBlock(0)}
        />
      )}

      {errorCode && <p className="turn-error">{t(`signoff.error.${errorCode}`)}</p>}

      <div className="signoff-actions">
        <button type="button" className="primary" disabled={!view.can_sign} onClick={() => void sign()}>
          {t("signoff.sign")}
        </button>
        <span className="hint">{t("signoff.sign_hint")}</span>
      </div>
    </section>
  );
}
