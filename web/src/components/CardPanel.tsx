import { useCallback, useEffect, useRef, useState } from "react";
import { answerCard, answerOpenQuestion, answerOpenQuestionWithYours, getTodo } from "../api/client";
import type { CardView, OpenQuestionView } from "../api/types";
import { anchoredScroll, canApplyQuestion, draftHasText, draftsOf, dropDraft, holdOrder, isStatementError, OWN_OPTION, questionErrorCode, setDraft, type CardDraft } from "../cardDrafts";
import { translate, type Lang } from "../i18n";
import { cardCounter } from "../noise";
import { undoReasonKey } from "../undo";
import { DecisionActions } from "./DecisionActions";

// P4 (backlog-r1 1.4): the open cards, what is yours to do. Each one
// reads its decision, why it is critical and the sections that use it;
// you choose an option and write a «porque». Go decides whether the
// «porque» counts: «ok» does not (fichas §3). Since slice 2 it also
// shows each open «por aclarar» (P4 [2], backlog-r2 2.4): the question,
// each option with its contrast, consequence and risk, what it blocks,
// the variants folded, and option + «porque» + «Aplicar».
export function CardPanel({ lang, projectID, refreshKey, onAnswered, onUndo, onDecisionActed }: {
  lang: Lang;
  projectID: string;
  refreshKey: number | string;
  // P-92: whether your answer left a defense being prepared.
  onAnswered: (defensePreparing: boolean, turnID?: string) => void;
  // «Fue un error · deshacer» on the card of your critical edit (B-78).
  onUndo: (section: string, editVersion: number) => Promise<void>;
  // «Objetar» and «Revocar» did something (E-1, B-82); absent in a read-only tab.
  onDecisionActed?: () => void;
}) {
  const t = (key: string, params?: Record<string, string>) => translate(lang, key, params);
  const [cards, setCards] = useState<CardView[]>([]);
  const [questions, setQuestions] = useState<OpenQuestionView[]>([]);
  const [drafts, setDrafts] = useState<Record<string, CardDraft>>(() => draftsOf(projectID));
  const [errors, setErrors] = useState<Record<string, string>>({});
  const shownOrder = useRef<string[]>([]);
  const panel = useRef<HTMLDivElement>(null);

  const write = (label: string, patch: CardDraft) => setDrafts(setDraft(projectID, label, patch));

  // Another project's drafts, under the same labels, are not this one's.
  useEffect(() => {
    setDrafts(draftsOf(projectID));
    setErrors({});
    shownOrder.current = [];
  }, [projectID]);

  const refresh = useCallback(async () => {
    if (!projectID) return;
    const todo = await getTodo(projectID);
    setCards(todo.cards);
    setQuestions(todo.open_questions ?? []);
  }, [projectID]);

  useEffect(() => {
    void refresh();
  }, [refresh, refreshKey]);

  async function apply(card: CardView) {
    // The card of your critical edit asks only for the «porque» (B-49).
    const option = card.own_edit ? null : drafts[card.label]?.option;
    if (!card.own_edit && !option) return;
    try {
      const res = await answerCard(projectID, card.label, option ?? null, drafts[card.label]?.reason ?? "");
      setErrors((prev) => ({ ...prev, [card.label]: "" }));
      setDrafts(dropDraft(projectID, card.label));
      await refresh();
      onAnswered(res.defense_preparing === true, res.turn_id);
    } catch (e) {
      const code = e instanceof Error && e.message === "http_422" ? "invalid_reason"
        : e instanceof Error && e.message === "http_409" ? "card_closed" : "failed";
      setErrors((prev) => ({ ...prev, [card.label]: code }));
      await refresh();
    }
  }

  async function applyQuestion(q: OpenQuestionView) {
    const draft = drafts[q.label];
    if (!canApplyQuestion(draft) || draft?.chosen === undefined) return;
    try {
      const res = draft.chosen === OWN_OPTION
        ? await answerOpenQuestionWithYours(projectID, q.label, draft.own ?? "", draft.reason ?? "")
        : await answerOpenQuestion(projectID, q.label, draft.chosen, draft.reason ?? "");
      setErrors((prev) => ({ ...prev, [q.label]: "" }));
      setDrafts(dropDraft(projectID, q.label));
      await refresh();
      onAnswered(res.defense_preparing === true, res.turn_id);
    } catch (e) {
      const code = questionErrorCode(e instanceof Error ? e.message : "");
      setErrors((prev) => ({ ...prev, [q.label]: code }));
      await refresh();
    }
  }

  // A card with text written stays where it is: what grows above it, such as the
  // conversation's stream, moves the scroll by as much (C.11).
  const held = [...cards.map((c) => c.label), ...questions.map((q) => q.label)].find((label) => draftHasText(drafts[label]));
  const heldID = held === undefined ? "" : cards.some((c) => c.label === held) ? `card-${held}` : `oq-${held}`;
  useEffect(() => {
    const card = heldID ? document.getElementById(heldID) : null;
    const scroller = panel.current?.closest<HTMLElement>(".center-view");
    const content = panel.current?.parentElement;
    if (!card || !scroller || !content) return;
    let top = card.getBoundingClientRect().top;
    const hold = () => {
      const now = card.getBoundingClientRect().top;
      if (now !== top) scroller.scrollTop = anchoredScroll(scroller.scrollTop, top, now);
      top = card.getBoundingClientRect().top;
    };
    const follow = () => {
      top = card.getBoundingClientRect().top;
    };
    const watcher = new ResizeObserver(hold);
    for (const child of Array.from(content.children)) watcher.observe(child);
    scroller.addEventListener("scroll", follow, { passive: true });
    return () => {
      watcher.disconnect();
      scroller.removeEventListener("scroll", follow);
    };
  }, [heldID]);

  const sent = [...cards.map((c) => c.label), ...questions.map((q) => q.label)];
  const order = holdOrder(shownOrder.current, sent, held !== undefined);
  shownOrder.current = order;
  // A card closed before your answer leaves with its button: its error stays here.
  const closed = Object.entries(errors).filter(([label, code]) => code && !sent.includes(label));

  if (sent.length === 0 && closed.length === 0) return null;

  const counter = (label: string) => (
    <p className="meta card-counter">{t("card.counter", cardCounter(order.indexOf(label), order.length))}</p>
  );

  const cardView = (card: CardView) => (
        <div key={card.label} id={`card-${card.label}`} className="card chat-card todo">
          {counter(card.label)}
          <h3>
            {/* The decision by its statement, never its label (rule 7, P-59). */}
            {t("card.title", { statement: card.statement })} · {t("card.critical")}:{" "}
            {card.reasons.map((r) => t(`card.reason.${r}`)).join(", ")}
          </h3>
          <p>{t("card.instead_of")}: {card.instead_of}</p>
          {card.would_choose_if && <p>{t("card.would_choose_if")}: {card.would_choose_if}</p>}
          <p>{t("card.sections")}: {card.sections.map((s) => s.title || t("section.untitled")).join(", ")}</p>
          {onDecisionActed && (
            <DecisionActions lang={lang} projectID={projectID} decision={card.decision} canObject={card.can_object}
              canRevoke={card.can_revoke} presetReason={drafts[card.label]?.reason} onActed={onDecisionActed} />
          )}
          {card.own_edit ? (
            <>
              <p>{t("card.own_edit")}</p>
              {/* B-78: it closes the card as obsolete; Go says when that edit can no longer be undone. */}
              <button
                type="button"
                className="small"
                disabled={!card.undo_available}
                title={card.undo_reason ? t(undoReasonKey(card.undo_reason)) : undefined}
                onClick={() => void onUndo(card.edit_section ?? "", card.edit_version ?? 0).finally(refresh)}
              >
                {t("card.undo")}
              </button>
              {!card.undo_available && card.undo_reason && <p className="meta">{t(undoReasonKey(card.undo_reason))}</p>}
            </>
          ) : (
            <>
              <label>
                <input
                  type="radio"
                  name={`option-${card.label}`}
                  checked={drafts[card.label]?.option === "keep"}
                  onChange={() => write(card.label, { option: "keep" })}
                />
                {t("card.keep", { what: card.statement })}
              </label>
              <label>
                <input
                  type="radio"
                  name={`option-${card.label}`}
                  checked={drafts[card.label]?.option === "other"}
                  onChange={() => write(card.label, { option: "other" })}
                />
                {t("card.other", { what: card.instead_of })}
              </label>
            </>
          )}
          <label>
            {t("card.reason")}
            <input
              value={drafts[card.label]?.reason ?? ""}
              onChange={(e) => write(card.label, { reason: e.target.value })}
            />
          </label>
          <div className="card-apply">
            <button type="button" disabled={!card.own_edit && !drafts[card.label]?.option} onClick={() => void apply(card)}>
              {t("card.apply")}
            </button>
            {errors[card.label] && <span className="turn-error" role="alert">{t(`card.error.${errors[card.label]}`)}</span>}
          </div>
        </div>
  );

  const questionView = (q: OpenQuestionView) => (
        <div key={q.label} id={`oq-${q.label}`} className="card chat-card todo">
          {counter(q.label)}
          <h3>
            {t("oq.title")} ·{" "}
            {/* What it blocks, by title or statement, never by label (rule 7). */}
            {t("oq.blocks", {
              what: [...q.sections.map((s) => s.title || t("section.untitled")), ...q.decisions].join(", "),
            })}
          </h3>
          {q.objection_reason ? (
            <>
              <p>{t("oq.objection.question", { statement: q.question })}</p>
              <p className="meta">{t("oq.objection.reason")}: {q.objection_reason}</p>
            </>
          ) : (
            <p>{q.question}</p>
          )}
          {q.options.map((o, i) => (
            <label key={i} className="oq-option">
              <input
                type="radio"
                name={`option-${q.label}`}
                checked={drafts[q.label]?.chosen === i}
                onChange={() => write(q.label, { chosen: i })}
              />
              <span className="oq-option-body">
                <span>{o.keep ? t("oq.keep", { what: o.option }) : o.other ? t("oq.other", { what: o.option }) : o.code ? t(`oq.scope.${o.code}`) : o.option}</span>
                {o.contrast && <span className="meta">{t("oq.contrast")}: {o.contrast}</span>}
                {o.consequence && <span className="meta">{t("oq.consequence")}: {o.consequence}</span>}
                {o.risk && <span className="meta">{t("oq.risk")}: {o.risk}</span>}
                {o.risk_reasons && o.risk_reasons.length > 0 && (
                  <span className="meta">{t("oq.risk")}: {o.risk_reasons.map((r) => t(`card.reason.${r}`)).join(", ")}</span>
                )}
              </span>
            </label>
          ))}
          {/* B-107: Go offers it, on an objection's question. Your words go as you wrote them. */}
          {q.offers_own && (
            <>
              <label className="oq-option">
                <input
                  type="radio"
                  name={`option-${q.label}`}
                  checked={drafts[q.label]?.chosen === OWN_OPTION}
                  onChange={() => write(q.label, { chosen: OWN_OPTION })}
                />
                {t("oq.own")}
              </label>
              {drafts[q.label]?.chosen === OWN_OPTION && (
                <label>
                  {t("oq.own.statement")}
                  <input
                    className="oq-own-statement"
                    value={drafts[q.label]?.own ?? ""}
                    onChange={(e) => write(q.label, { own: e.target.value })}
                  />
                  {isStatementError(errors[q.label]) && (
                    <span className="turn-error" role="alert"> {t(`card.error.${errors[q.label]}`)}</span>
                  )}
                </label>
              )}
            </>
          )}
          {q.variants.length > 0 && (
            <details>
              <summary>{t("oq.variants")}</summary>
              <ul>
                {q.variants.map((v, i) => (
                  <li key={i}>{v}</li>
                ))}
              </ul>
            </details>
          )}
          {q.new_passage && (
            <p className="meta">
              {t("oq.passages")}: «{q.new_passage}»{q.clashing_passage && <> · «{q.clashing_passage}»</>}
            </p>
          )}
          {q.undeclared_by_agent && <p className="meta">{t("oq.undeclared")}</p>}
          <label>
            {t("card.reason")}
            <input
              value={drafts[q.label]?.reason ?? ""}
              onChange={(e) => write(q.label, { reason: e.target.value })}
            />
          </label>
          <div className="card-apply">
            <button type="button" disabled={!canApplyQuestion(drafts[q.label])} onClick={() => void applyQuestion(q)}>
              {t("card.apply")}
            </button>
            {errors[q.label] && !isStatementError(errors[q.label]) && <span className="turn-error" role="alert">{t(`card.error.${errors[q.label]}`)}</span>}
          </div>
        </div>
  );

  return (
    <div className="request-panel cards" ref={panel}>
      <h2>{t("card.todo")}</h2>
      {closed.map(([label, code]) => (
        <p key={label} className="turn-error" role="alert">{t(`card.error.${code}`)}</p>
      ))}
      {order.map((label) => {
        const card = cards.find((c) => c.label === label);
        if (card) return cardView(card);
        const question = questions.find((q) => q.label === label);
        return question ? questionView(question) : null;
      })}
    </div>
  );
}
