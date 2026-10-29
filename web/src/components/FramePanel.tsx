import { useCallback, useEffect, useState } from "react";
import { getFrame, markDecisionSeen } from "../api/client";
import type { Frame, FrameDecision, FrameSection } from "../api/graph";
import { translate, type Lang } from "../i18n";

// What «Te toca» leads to: a card, the understanding's cases or a
// section to sign.
export type TodoTarget =
  | { kind: "card"; label: string }
  | { kind: "cases" }
  | { kind: "sign"; section: string };

// The frame (backlog-r1 1.5b, specification §9): what the document is
// for, how it is judged, its skeleton with the critical decisions and
// what is yours to do. Go computes all of it; this panel only shows it.
// Expanding a decision records decision_seen (frame); scrolling does not.
export function FramePanel({ lang, projectID, refreshKey, onSection, onHistory, onTodo }: {
  lang: Lang;
  projectID: string;
  refreshKey: string;
  onSection: (section: string) => void;
  onHistory: (section: string, block: number) => void;
  onTodo: (target: TodoTarget) => void;
}) {
  const t = (key: string, params?: Record<string, string>) => translate(lang, key, params);
  const [frame, setFrame] = useState<Frame | null>(null);

  const refresh = useCallback(async () => {
    if (projectID) setFrame(await getFrame(projectID));
  }, [projectID]);

  useEffect(() => {
    setFrame(null);
  }, [projectID]);

  useEffect(() => {
    void refresh();
  }, [refresh, refreshKey]);

  if (!frame) return null;

  const titleOf = (label: string) =>
    frame.sections.find((s) => s.label === label)?.title || t("section.untitled");

  if (!frame.purpose.request_text) {
    return (
      <div className="frame-empty">
        <b>{t("frame.title")}</b>
        <span>{t("frame.empty")}</span>
      </div>
    );
  }

  async function expanded(d: FrameDecision) {
    try {
      await markDecisionSeen(projectID, d.label);
      await refresh();
    } catch {
      // [EL DISEÑO NO LO DICE]: a failed «seen» shows nothing; the
      // frame never blocks.
    }
  }

  const todos: { key: string; text: string; tag: string; target?: TodoTarget }[] = [];
  if (frame.todo.unconfirmed_cases > 0) {
    todos.push({ key: "cases", text: t("frame.todo.cases"),
      tag: t("frame.todo.cases_n", { n: String(frame.todo.unconfirmed_cases) }), target: { kind: "cases" } });
  }
  for (const c of frame.todo.cards) {
    todos.push({ key: c.label, text: t("frame.todo.card", { statement: c.statement }),
      tag: t("card.critical"), target: { kind: "card", label: c.label } });
  }
  for (const s of frame.todo.ready_to_sign) {
    todos.push({ key: `sign-${s}`, text: t("frame.todo.sign", { section: titleOf(s) }),
      tag: t("section.status.ready"), target: { kind: "sign", section: s } });
  }
  if (frame.todo.stop_report_unseen) {
    todos.push({ key: "stop", text: t("frame.todo.stop_report"), tag: "" });
  }

  return (
    <div className="frame">
      <div className="fsec">
        <h4>{t("frame.purpose")}</h4>
        <p className="lit">«{frame.purpose.request_text}»</p>
        <p className="lit meta">
          {frame.purpose.audience && <>{t("frame.for")}: {frame.purpose.audience} · </>}
          {frame.purpose.premise_exploratory
            ? t("frame.exploratory")
            : frame.purpose.premise_text && <>{t("frame.premise")}: «{frame.purpose.premise_text}»</>}
        </p>
      </div>

      {frame.criteria.length > 0 && (
        <div className="fsec">
          <h4>{t("frame.judged")}</h4>
          {frame.criteria.map((c) => (
            <div key={c.label} className="crit">
              <span>«{c.text}»</span>
              {c.cases.map((cs) => (
                <span key={cs.label}>
                  <span className={cs.counts ? "yes" : "no"}>
                    {t(cs.counts ? "understanding.counts" : "understanding.does_not_count")}:
                  </span>{" "}
                  {cs.text} <span className={`st ${cs.state === "unconfirmed" ? "todo" : "signed"}`}>
                    {t(`frame.case_state.${cs.state}`)}
                  </span>
                </span>
              ))}
            </div>
          ))}
        </div>
      )}

      <div className="fsec">
        <h4>{t("frame.skeleton")}</h4>
        {frame.sections.length === 0 ? (
          <p className="meta">{t("frame.skeleton_empty")}</p>
        ) : (
          <ul className="skel">
            {frame.sections.map((s) => (
              <SkeletonSection key={s.label} lang={lang} section={s} titleOf={titleOf}
                onSection={onSection} onHistory={onHistory} onTodo={onTodo} onExpand={expanded} />
            ))}
          </ul>
        )}
      </div>

      <div className="fsec">
        <h4>
          {t("frame.todo")} <span>{todos.length > 0 ? todos.length : ""}</span>
        </h4>
        <ul className="tlist">
          {todos.length === 0 && <li className="meta">{t("frame.todo_empty")}</li>}
          {todos.map((item) => (
            <li key={item.key}>
              <button type="button" disabled={!item.target} onClick={() => item.target && onTodo(item.target)}>
                {item.text} {item.tag && <span className="st todo">{item.tag}</span>}
              </button>
            </li>
          ))}
        </ul>
      </div>
    </div>
  );
}

function SkeletonSection({ lang, section, titleOf, onSection, onHistory, onTodo, onExpand }: {
  lang: Lang;
  section: FrameSection;
  titleOf: (label: string) => string;
  onSection: (section: string) => void;
  onHistory: (section: string, block: number) => void;
  onTodo: (target: TodoTarget) => void;
  onExpand: (d: FrameDecision) => void;
}) {
  const t = (key: string, params?: Record<string, string>) => translate(lang, key, params);
  return (
    <li className="sec">
      <button type="button" className="t" onClick={() => onSection(section.label)}>
        {titleOf(section.label)}{" "}
        <span className={`st ${section.pendings > 0 ? "todo" : section.status === "signed" ? "signed" : "draft"}`}>
          {section.pendings > 0
            ? t("section.pendings", { n: String(section.pendings) })
            : t(`section.status.${section.status}`)}
        </span>
      </button>
      {section.first_line && <p className="first">{section.first_line}</p>}
      {section.decisions.map((d) => (
        <details
          key={d.label}
          className="dec"
          onToggle={(e) => {
            if ((e.currentTarget as HTMLDetailsElement).open) onExpand(d);
          }}
        >
          <summary>
            {!d.seen && <span className="dot" title={t("frame.unseen")} />}
            <span className="ico">{t(`frame.icon.${d.icon}`)}</span>
            <span>{d.statement}</span>
          </summary>
          <div className="decbody">
            <div><b>{t("card.instead_of")}:</b> {d.instead_of}</div>
            {d.would_choose_if && <div><b>{t("card.would_choose_if")}:</b> {d.would_choose_if}</div>}
            <div>
              <b>{t("frame.your_reason")}:</b>{" "}
              {d.reason ? <>«{d.reason}»</> : <span className="no">{t("frame.not_answered")}</span>}
            </div>
            {d.signoff_explanation && (
              <div><b>{t("frame.signoff_explanation")}:</b> «{d.signoff_explanation}»</div>
            )}
            <div><b>{t("card.sections")}:</b> {d.sections.map(titleOf).join(" · ")}</div>
            <div className="links">
              {d.block > 0 && (
                <button type="button" className="link" onClick={() => onHistory(section.label, d.block)}>
                  {t("frame.history")}
                </button>
              )}
              <button type="button" className="link" onClick={() => onSection(section.label)}>
                {t("frame.to_text")}
              </button>
              {d.card && (
                <button type="button" className="link" onClick={() => onTodo({ kind: "card", label: d.card! })}>
                  {t("frame.answer")}
                </button>
              )}
            </div>
          </div>
        </details>
      ))}
    </li>
  );
}
