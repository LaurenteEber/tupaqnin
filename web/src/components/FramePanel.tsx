import { countKey } from "../noise";
import { useCallback, useEffect, useState } from "react";
import { getFrame, markDecisionSeen } from "../api/client";
import type { Frame, FrameDecision, FrameSection, ReturnLine } from "../api/graph";
import { SearchedName } from "./SearchedName";
import { DecisionActions } from "./DecisionActions";
import { candidateName } from "../provenance";
import { translate, type Lang } from "../i18n";

import { proposalsTarget } from "../proposals";

// What «Te toca» leads to: a card, the understanding's cases or a
// section to sign.
export type TodoTarget =
  | { kind: "card"; label: string }
  | { kind: "open_question"; label: string }
  | { kind: "flaws"; round: string }
  | { kind: "cases" }
  | { kind: "stop_report" }
  | { kind: "sign"; section: string }
  | { kind: "proposals"; section: string };

// The frame (backlog-r1 1.5b, specification §9): what the document is
// for, how it is judged, its skeleton with the critical decisions and
// what is yours to do. Go computes all of it; this panel only shows it.
// Expanding a decision records decision_seen (frame); scrolling does not.
// Slice 2 (backlog-r2 2.7): the return line above it after 3 days or
// more (M-2), the exact «sin ver» dot and the filter «solo lo que no
// viste», all computed in Go.
export function FramePanel({ lang, projectID, refreshKey, selected, onSection, onHistory, onDecisionHistory, onTodo, onReorder, onSearchCandidate, onDecisionActed, onTodoCount }: {
  // C.2: how many lines «Lo que te toca» has, for the header's status line.
  onTodoCount?: (count: number) => void;
  lang: Lang;
  projectID: string;
  refreshKey: string;
  // U-3: the section the document column shows.
  selected: string;
  onSection: (section: string) => void;
  onHistory: (section: string, block: number) => void;
  // A decision that covers no figure opens its own history (P-41).
  onDecisionHistory: (section: string, decision: string) => void;
  onTodo: (target: TodoTarget) => void;
  // P-49: «subir» and «bajar» send the whole new order; absent in a
  // read-only tab.
  onReorder?: (labels: string[]) => void;
  // «Buscarla» (B-69): you start a round on the candidate; absent in a
  // read-only tab.
  onSearchCandidate?: (n: number) => void;
  // «Objetar» and «Revocar» did something (E-1, B-82); absent in a read-only tab.
  onDecisionActed?: () => void;
}) {
  const t = (key: string, params?: Record<string, string>) => translate(lang, key, params);
  const [frame, setFrame] = useState<Frame | null>(null);
  const [unseenOnly, setUnseenOnly] = useState(false);

  const refresh = useCallback(async () => {
    if (!projectID) return;
    const next = await getFrame(projectID);
    setFrame(next);
    onTodoCount?.(next.todo.count);
  }, [projectID, onTodoCount]);

  useEffect(() => {
    setFrame(null);
    setUnseenOnly(false);
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
      tag: t("frame.todo.card_tag"), target: { kind: "card", label: c.label } });
  }
  for (const q of frame.todo.open_questions ?? []) {
    todos.push({ key: q.label, text: t("frame.todo.open_question", { question: q.question }),
      tag: t("oq.tag"), target: { kind: "open_question", label: q.label } });
  }
  for (const f of frame.todo.flaws_to_review ?? []) {
    // «Fallas por revisar» (pantallas, backlog-r3 3.6).
    todos.push({ key: f.round, text: t("frame.todo.flaws", { section: titleOf(f.section), n: String(f.flaws) }),
      tag: t("judge.tag"), target: { kind: "flaws", round: f.round } });
  }
  for (const s of frame.todo.ready_to_sign) {
    todos.push({ key: `sign-${s}`, text: t("frame.todo.sign", { section: titleOf(s) }),
      tag: t("section.status.ready"), target: { kind: "sign", section: s } });
  }
  // One line for all of them (pantallas: «Propuestas (n)»); it leads to the
  // first section with one, or to the proposed sections.
  const proposals = frame.todo.proposals ?? [];
  if (proposals.length > 0) {
    todos.push({ key: "proposals", text: t("frame.todo.proposals", { n: String(proposals.length) }), tag: "",
      target: { kind: "proposals", section: proposalsTarget(proposals) } });
  }
  if (frame.todo.stop_report_unseen) {
    // «Informe sin ver» until it is opened (backlog-r2 2.5).
    todos.push({ key: "stop", text: t("frame.todo.stop_report"), tag: "", target: { kind: "stop_report" } });
  }

  const decisionProps = { lang, projectID, titleOf, onSection, onHistory, onDecisionHistory, onTodo, onExpand: expanded, onDecisionActed };

  return (
    <div className="frame">
      {frame.return_line && (
        <ReturnLineBox lang={lang} line={frame.return_line} titleOf={titleOf} onSection={onSection} />
      )}
      <div className="fsec">
        <h4>{t("frame.purpose")}</h4>
        <p className="lit">«{frame.purpose.request_text}»</p>
        <p className="lit meta">
          {frame.purpose.audience && <>{t("frame.for")}: {frame.purpose.audience}</>}
          {/* The separator only between two parts (P-61). */}
          {frame.purpose.audience && (frame.purpose.premise_exploratory || frame.purpose.premise_text) && " · "}
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
        {frame.sections.length > 0 && (
          <button type="button" className="link" aria-pressed={unseenOnly} onClick={() => setUnseenOnly(!unseenOnly)}>
            {unseenOnly ? t("frame.unseen_all") : t("frame.unseen_only")}
          </button>
        )}
        {frame.sections.length === 0 ? (
          <p className="meta">{t("frame.skeleton_empty")}</p>
        ) : unseenOnly ? (
          // The filter (§9): Go orders it, more sections first.
          frame.unseen.length === 0 ? (
            <p className="meta">{t("frame.unseen_none")}</p>
          ) : (
            <ul className="skel">
              {frame.unseen.map((u) => (
                <li key={u.decision.label} className="sec">
                  <button type="button" className="t" onClick={() => onSection(u.section)}>{titleOf(u.section)}</button>
                  <DecisionItem {...decisionProps} section={u.section} decision={u.decision} />
                </li>
              ))}
            </ul>
          )
        ) : (
          <ul className="skel">
            {frame.sections.map((s, i) => (
              <SkeletonSection key={s.label} {...decisionProps} section={s} active={s.label === selected}
                onMove={onReorder && ((dir) => {
                  const labels = frame.sections.map((x) => x.label);
                  [labels[i], labels[i + dir]] = [labels[i + dir], labels[i]];
                  onReorder(labels);
                })}
                first={i === 0} last={i === frame.sections.length - 1} />
            ))}
          </ul>
        )}
      </div>

      {frame.candidates.length > 0 && (
        // R-2, B-69: what the agent named without a citation; never sources.
        <div className="fsec" id="frame-candidates">
          <h4>{t("frame.candidates")}</h4>
          <ul className="tlist">
            {frame.candidates.map((c) => (
              <li key={c.n}>
                {candidateName(c)} · <span className="meta">{t("provenance.candidate")}</span>{" "}
                {onSearchCandidate && (
                  <button type="button" className="small" onClick={() => onSearchCandidate(c.n)}>
                    {t("provenance.search_candidate")}
                  </button>
                )}
              </li>
            ))}
          </ul>
        </div>
      )}

      <div className="fsec" id="frame-todo">
        <h4>
          {t("frame.todo")} <span>{frame.todo.count > 0 ? frame.todo.count : ""}</span>
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

type DecisionProps = {
  lang: Lang;
  projectID: string;
  onDecisionActed?: () => void;
  titleOf: (label: string) => string;
  onSection: (section: string) => void;
  onHistory: (section: string, block: number) => void;
  onDecisionHistory: (section: string, decision: string) => void;
  onTodo: (target: TodoTarget) => void;
  onExpand: (d: FrameDecision) => void;
};

function SkeletonSection({ section, active, onMove, first, last, ...props }: DecisionProps & {
  section: FrameSection;
  onMove?: (dir: -1 | 1) => void;
  first: boolean;
  last: boolean;
  active: boolean;
}) {
  const { lang, titleOf, onSection } = props;
  const t = (key: string, params?: Record<string, string>) => translate(lang, key, params);
  return (
    <li className={`sec${active ? " active" : ""}`}>
      <button type="button" className="t" aria-current={active || undefined} onClick={() => onSection(section.label)}>
        {titleOf(section.label)}{" "}
        <span className={`st ${section.pendings > 0 ? "todo" : section.status === "signed" ? "signed" : "draft"}`}>
          {section.pendings > 0
            ? t(countKey("section.pendings", section.pendings), { n: String(section.pendings) })
            : t(`section.status.${section.status}`)}
        </span>
      </button>
      {onMove && active && (
        <span className="move">
          <button type="button" className="link" disabled={first} onClick={() => onMove(-1)}>
            {t("frame.move_up")}
          </button>
          <button type="button" className="link" disabled={last} onClick={() => onMove(1)}>
            {t("frame.move_down")}
          </button>
        </span>
      )}
      {section.unverified > 0 && (
        // R-2: how many paragraphs have no verified origin; each one's
        // history says where it comes from.
        <p className="meta">{t(countKey("frame.unverified", section.unverified), { n: String(section.unverified) })}</p>
      )}
      {section.first_line && <p className="first">{section.first_line}</p>}
      {section.decisions.map((d) => (
        <DecisionItem key={d.label} {...props} section={section.label} decision={d} />
      ))}
    </li>
  );
}

// A critical decision of the skeleton: the dot when Go says it is not seen
// (B3-9, P-77), and its card when expanded (§9). Its link opens
// its paragraph's history (P5) or, when it covers no figure, its own
// (GET /decisions/{label}/history, P-41).
function DecisionItem({ lang, projectID, section, decision: d, titleOf, onSection, onHistory, onDecisionHistory, onTodo, onExpand, onDecisionActed }:
  DecisionProps & { section: string; decision: FrameDecision }) {
  const t = (key: string, params?: Record<string, string>) => translate(lang, key, params);
  return (
    <details
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
        {d.sources.length > 0 && (
          <div>
            <b>{t("frame.searched_sources")}:</b>{" "}
            {d.sources.map((src) => (
              <span key={src.label}>
                <SearchedName lang={lang} source={src} />{" "}
              </span>
            ))}
          </div>
        )}
        <div className="links">
          {d.block > 0 ? (
            <button type="button" className="link" onClick={() => onHistory(section, d.block)}>
              {t("frame.history")}
            </button>
          ) : (
            <button type="button" className="link" onClick={() => onDecisionHistory(section, d.label)}>
              {t("frame.decision_history")}
            </button>
          )}
          <button type="button" className="link" onClick={() => onSection(section)}>
            {t("frame.to_text")}
          </button>
          {d.card && (
            <button type="button" className="link" onClick={() => onTodo({ kind: "card", label: d.card! })}>
              {t("frame.answer")}
            </button>
          )}
        </div>
        {onDecisionActed && (
          <DecisionActions lang={lang} projectID={projectID} decision={d.label} canObject={d.can_object}
            canRevoke={d.can_revoke} onActed={onDecisionActed} />
        )}
      </div>
    </details>
  );
}

// The return line (M-2), above the frame after 3 days or more: where
// you left off, what changed (with the history of the sections that
// changed) and what is yours. Go computes the numbers; it never blocks.
function ReturnLineBox({ lang, line, titleOf, onSection }: {
  lang: Lang;
  line: ReturnLine;
  titleOf: (label: string) => string;
  onSection: (section: string) => void;
}) {
  const t = (key: string, params?: Record<string, string>) => translate(lang, key, params);
  const act = t(`return.act.${line.act}`);
  const what = line.act_about ? t("return.act_about", { act, about: line.act_about }) : act;
  return (
    <div className="fsec return-line">
      <h4>{line.mode_finished && line.days < 1 ? t("return.title_mode") : t("return.title", { n: String(line.days) })}</h4>
      <p>{line.section ? t("return.left_in", { section: titleOf(line.section), act: what }) : t("return.left", { act: what })}</p>
      <p>
        {t("return.changed", { n: String(line.changes) })}
        {line.changed_sections.length > 0 && (
          <>
            {" → "}
            <button type="button" className="link" onClick={() => onSection(line.changed_sections[0])}>
              {t("return.to_history")}
            </button>
          </>
        )}
      </p>
      <p>
        {t("return.todo", { n: String(line.todo) })}
        {line.todo > 0 && (
          <>
            {" → "}
            <button type="button" className="link"
              onClick={() => document.getElementById("frame-todo")?.scrollIntoView({ block: "nearest", behavior: "smooth" })}>
              {t("return.to_todo")}
            </button>
          </>
        )}
      </p>
    </div>
  );
}
