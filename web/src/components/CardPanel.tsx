import { useCallback, useEffect, useState } from "react";
import { answerCard, answerOpenQuestion, getTodo } from "../api/client";
import type { CardView, OpenQuestionView } from "../api/types";
import { translate, type Lang } from "../i18n";

// P4 (backlog-r1 1.4): the open cards, what is yours to do. Each one
// reads its decision, why it is critical and the sections that use it;
// you choose an option and write a «porque». Go decides whether the
// «porque» counts: «ok» does not (fichas §3). Since slice 2 it also
// shows each open «por aclarar» (P4 [2], backlog-r2 2.4): the question,
// each option with its contrast, consequence and risk, what it blocks,
// the variants folded, and option + «porque» + «Aplicar».
export function CardPanel({ lang, projectID, refreshKey, onAnswered }: {
  lang: Lang;
  projectID: string;
  refreshKey: number | string;
  onAnswered: () => void;
}) {
  const t = (key: string, params?: Record<string, string>) => translate(lang, key, params);
  const [cards, setCards] = useState<CardView[]>([]);
  const [questions, setQuestions] = useState<OpenQuestionView[]>([]);
  const [chosen, setChosen] = useState<Record<string, number>>({});
  const [options, setOptions] = useState<Record<string, "keep" | "other">>({});
  const [reasons, setReasons] = useState<Record<string, string>>({});
  const [errors, setErrors] = useState<Record<string, string>>({});

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
    const option = card.own_edit ? null : options[card.label];
    if (!card.own_edit && !option) return;
    try {
      await answerCard(projectID, card.label, option ?? null, reasons[card.label] ?? "");
      setErrors((prev) => ({ ...prev, [card.label]: "" }));
      await refresh();
      onAnswered();
    } catch (e) {
      const code = e instanceof Error && e.message === "http_422" ? "invalid_reason"
        : e instanceof Error && e.message === "http_409" ? "card_closed" : "failed";
      setErrors((prev) => ({ ...prev, [card.label]: code }));
      await refresh();
    }
  }

  async function applyQuestion(q: OpenQuestionView) {
    const option = chosen[q.label];
    if (option === undefined) return;
    try {
      await answerOpenQuestion(projectID, q.label, option, reasons[q.label] ?? "");
      setErrors((prev) => ({ ...prev, [q.label]: "" }));
      await refresh();
      onAnswered();
    } catch (e) {
      const code = e instanceof Error && e.message === "http_422" ? "invalid_reason"
        : e instanceof Error && e.message === "http_409" ? "question_closed" : "failed";
      setErrors((prev) => ({ ...prev, [q.label]: code }));
      await refresh();
    }
  }

  if (cards.length === 0 && questions.length === 0) return null;

  return (
    <div className="request-panel cards">
      <h2>{t("card.todo")}</h2>
      {cards.map((card) => (
        <div key={card.label} id={`card-${card.label}`} className="card chat-card todo">
          <h3>
            {/* The decision by its statement, never its label (rule 7, P-59). */}
            {t("card.title", { statement: card.statement })} · {t("card.critical")}:{" "}
            {card.reasons.map((r) => t(`card.reason.${r}`)).join(", ")}
          </h3>
          <p>{t("card.instead_of")}: {card.instead_of}</p>
          {card.would_choose_if && <p>{t("card.would_choose_if")}: {card.would_choose_if}</p>}
          <p>{t("card.sections")}: {card.sections.map((s) => s.title || t("section.untitled")).join(", ")}</p>
          {card.own_edit ? (
            <p>{t("card.own_edit")}</p>
          ) : (
            <>
              <label>
                <input
                  type="radio"
                  name={`option-${card.label}`}
                  checked={options[card.label] === "keep"}
                  onChange={() => setOptions((prev) => ({ ...prev, [card.label]: "keep" }))}
                />
                {t("card.keep")}
              </label>
              <label>
                <input
                  type="radio"
                  name={`option-${card.label}`}
                  checked={options[card.label] === "other"}
                  onChange={() => setOptions((prev) => ({ ...prev, [card.label]: "other" }))}
                />
                {t("card.other")}
              </label>
            </>
          )}
          <label>
            {t("card.reason")}
            <input
              value={reasons[card.label] ?? ""}
              onChange={(e) => setReasons((prev) => ({ ...prev, [card.label]: e.target.value }))}
            />
          </label>
          <button type="button" disabled={!card.own_edit && !options[card.label]} onClick={() => void apply(card)}>
            {t("card.apply")}
          </button>
          {errors[card.label] && <p className="turn-error">{t(`card.error.${errors[card.label]}`)}</p>}
        </div>
      ))}
      {questions.map((q) => (
        <div key={q.label} id={`oq-${q.label}`} className="card chat-card todo">
          <h3>
            {t("oq.title")} ·{" "}
            {/* What it blocks, by title or statement, never by label (rule 7). */}
            {t("oq.blocks", {
              what: [...q.sections.map((s) => s.title || t("section.untitled")), ...q.decisions].join(", "),
            })}
          </h3>
          <p>{q.question}</p>
          {q.options.map((o, i) => (
            <label key={i} className="oq-option">
              <input
                type="radio"
                name={`option-${q.label}`}
                checked={chosen[q.label] === i}
                onChange={() => setChosen((prev) => ({ ...prev, [q.label]: i }))}
              />
              {o.keep ? t("oq.keep", { what: o.option }) : o.option}
              {o.contrast && <span className="meta"> · {t("oq.contrast")}: {o.contrast}</span>}
              {o.consequence && <span className="meta"> · {t("oq.consequence")}: {o.consequence}</span>}
              {o.risk && <span className="meta"> · {t("oq.risk")}: {o.risk}</span>}
            </label>
          ))}
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
              value={reasons[q.label] ?? ""}
              onChange={(e) => setReasons((prev) => ({ ...prev, [q.label]: e.target.value }))}
            />
          </label>
          <button type="button" disabled={chosen[q.label] === undefined} onClick={() => void applyQuestion(q)}>
            {t("card.apply")}
          </button>
          {errors[q.label] && <p className="turn-error">{t(`card.error.${errors[q.label]}`)}</p>}
        </div>
      ))}
    </div>
  );
}
