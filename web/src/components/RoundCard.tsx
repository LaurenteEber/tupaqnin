import { useEffect, useState } from "react";
import { getRound } from "../api/client";
import type { RoundCard as RoundCardView } from "../api/types";
import { numberLocale, translate, type Lang } from "../i18n";
import { ROUND_POLL_MS, roundKeepsPolling } from "../live";

// The round card in the conversation (backlog-r3 3.2, B-64; pantallas
// «Dónde va cada vista de la rebanada 3»): the question, what each
// reader searches and where, the expectation in literal, the estimated
// cost and what was spent, each reader's progress, and the privacy line.
// Go computes all of it; the card asks again while the readers run.
export function RoundCard({ lang, projectID, label }: { lang: Lang; projectID: string; label: string }) {
  const t = (key: string, params?: Record<string, string>) => translate(lang, key, params);
  const [card, setCard] = useState<RoundCardView | null>(null);

  useEffect(() => {
    let live = true;
    let timer = 0;
    const read = async () => {
      try {
        const next = await getRound(projectID, label);
        if (!live) return;
        setCard(next);
        if (roundKeepsPolling(next)) timer = window.setTimeout(() => void read(), ROUND_POLL_MS);
      } catch {
        if (live) timer = window.setTimeout(() => void read(), ROUND_POLL_MS);
      }
    };
    void read();
    return () => {
      live = false;
      window.clearTimeout(timer);
    };
  }, [projectID, label]);

  if (!card) return null;
  const usd = (n: number) =>
    n.toLocaleString(numberLocale(lang), { minimumFractionDigits: 2, maximumFractionDigits: 3 });

  return (
    <div className="agent-step round-card">
      <p className="meta">
        {t("round.title", { label: card.label })} · {t(roundKeepsPolling(card) ? "round.running" : `round.state.${card.state}`)}
      </p>
      <p>
        <b>{card.question}</b>
      </p>
      {card.doubt && <p className="meta">{t("round.doubt")}: {card.doubt}</p>}
      <p className="meta">
        {t("round.expectation")}: {card.expectation ? <>«{card.expectation}»</> : t("round.no_expectation")}
      </p>
      <ul>
        {card.parts.map((p) => (
          <li key={p.part}>
            {t("round.reader", { n: String(p.part) })} · {p.where.map((w) => t(`round.where.${w}`)).join(", ")} ·{" "}
            <span className="meta">
              {t(`round.status.${p.status}`)}
              {p.reason && <> ({t(`round.reason.${p.reason}`)})</>}
              {p.tool_calls > 0 && <> · {t("round.tool_calls", { n: String(p.tool_calls) })}</>}
              {p.status === "returned" && <> · {t("round.findings", { n: String(p.findings) })}</>}
            </span>
            <br />
            {p.question}
            {p.sources.length > 0 && (
              <span className="meta">
                {" "}
                · {p.sources.map((s) => (s.what_for ? `${s.label} (${s.what_for})` : s.label)).join(", ")}
              </span>
            )}
          </li>
        ))}
      </ul>
      {card.stopped && <p className="meta">{t(`round.stopped.${card.stopped}`)}</p>}
      <p className="meta">
        {t("round.cost", { estimated: usd(card.estimated_cost_usd), spent: usd(card.spent_usd) })}
      </p>
      <p className="meta">{t("round.privacy")}</p>
    </div>
  );
}
