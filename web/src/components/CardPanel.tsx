import { useCallback, useEffect, useState } from "react";
import { answerCard, getTodo } from "../api/client";
import type { CardView } from "../api/types";
import { translate, type Lang } from "../i18n";

// P4 (backlog-r1 1.4): the open cards, what is yours to do. Each one
// reads its decision, why it is critical and the sections that use it;
// you choose an option and write a «porque». Go decides whether the
// «porque» counts: «ok» does not (fichas §3).
export function CardPanel({ lang, projectID, refreshKey, onAnswered }: {
  lang: Lang;
  projectID: string;
  refreshKey: number;
  onAnswered: () => void;
}) {
  const t = (key: string, params?: Record<string, string>) => translate(lang, key, params);
  const [cards, setCards] = useState<CardView[]>([]);
  const [options, setOptions] = useState<Record<string, "keep" | "other">>({});
  const [reasons, setReasons] = useState<Record<string, string>>({});
  const [errors, setErrors] = useState<Record<string, string>>({});

  const refresh = useCallback(async () => {
    if (projectID) setCards((await getTodo(projectID)).cards);
  }, [projectID]);

  useEffect(() => {
    void refresh();
  }, [refresh, refreshKey]);

  async function apply(card: CardView) {
    const option = options[card.label];
    if (!option) return;
    try {
      await answerCard(projectID, card.label, option, reasons[card.label] ?? "");
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

  if (cards.length === 0) return null;

  return (
    <div className="request-panel">
      <h2>{t("card.todo")}</h2>
      {cards.map((card) => (
        <div key={card.label} className="card">
          <h3>
            {t("card.title", { label: card.decision })} · {t("card.critical")}:{" "}
            {card.reasons.map((r) => t(`card.reason.${r}`)).join(", ")}
          </h3>
          <p>{card.statement}</p>
          <p>{t("card.instead_of")}: {card.instead_of}</p>
          {card.would_choose_if && <p>{t("card.would_choose_if")}: {card.would_choose_if}</p>}
          <p>{t("card.sections")}: {card.sections.join(", ")}</p>
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
          <label>
            {t("card.reason")}
            <input
              value={reasons[card.label] ?? ""}
              onChange={(e) => setReasons((prev) => ({ ...prev, [card.label]: e.target.value }))}
            />
          </label>
          <button type="button" disabled={!options[card.label]} onClick={() => void apply(card)}>
            {t("card.apply")}
          </button>
          {errors[card.label] && <p className="turn-error">{t(`card.error.${errors[card.label]}`)}</p>}
        </div>
      ))}
    </div>
  );
}
