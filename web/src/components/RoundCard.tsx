import { useCallback, useEffect, useState } from "react";
import { getRound, leaveRoundOpen, repeatRound } from "../api/client";
import type { RoundCard as RoundCardView, RoundFinding } from "../api/types";
import { coverageLines, findingReasonKey, offersLeaveOpen, offersRepeat, sourceHref } from "../digest";
import { numberLocale, translate, type Lang } from "../i18n";
import { ROUND_POLL_MS, roundKeepsPolling } from "../live";

// The round card in the conversation (backlog-r3 3.2, B-64; pantallas
// «Dónde va cada vista de la rebanada 3»): the question, what each
// reader searches and where, the expectation in literal, the estimated
// cost and what was spent, each reader's progress, and the privacy line.
// Go computes all of it; the card asks again while the readers run.
// Backlog-r3 3.4: once it closes, the digest: first the answer, the
// outline with its findings (each passage opens in its source),
// «Descartados (n)» and «Sin revisar» in sight, «¿se redujo la duda?»,
// and, when it was not reduced, «Repetir con otra pregunta» with its
// cost and «dejarlo por aclarar», each only on your click.
export function RoundCard({ lang, projectID, label, onAct }: {
  lang: Lang;
  projectID: string;
  label: string;
  // Your click repeated the round (its turn to follow) or left it to
  // clarify (no turn): the page reads the conversation and the cards.
  onAct?: (turnID: string) => void;
}) {
  const t = (key: string, params?: Record<string, string>) => translate(lang, key, params);
  const [card, setCard] = useState<RoundCardView | null>(null);
  const [question, setQuestion] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

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

  const reload = useCallback(async () => {
    try {
      setCard(await getRound(projectID, label));
    } catch {
      // The card keeps what it showed.
    }
  }, [projectID, label]);

  async function act(run: () => Promise<string>) {
    setBusy(true);
    setError("");
    try {
      const turnID = await run();
      await reload();
      onAct?.(turnID);
    } catch (e) {
      setError(e instanceof Error ? e.message : "internal");
    } finally {
      setBusy(false);
    }
  }

  if (!card) return null;
  const usd = (n: number) =>
    n.toLocaleString(numberLocale(lang), { minimumFractionDigits: 2, maximumFractionDigits: 3 });

  // A finding: its passage literal (never translated, rule 8), opening in
  // its source.
  const findingItem = (f: RoundFinding, key: string, reason = false) => {
    const href = sourceHref(projectID, f);
    return (
      <li key={key}>
        <details>
          <summary>
            «{f.passage || f.reference || "—"}»
            {reason && <span className="meta"> · {t(findingReasonKey(f))}</span>}
          </summary>
          <p className="meta">
            {f.part > 0 && t("digest.reader", { n: String(f.part) })}
            {f.kind && <> · {f.kind}</>}
            {(f.source_title || f.source) && <> · {f.source_title || f.source}</>}
            {f.note && <> · {f.note}</>}
          </p>
          {href && (
            <a href={href} target="_blank" rel="noreferrer noopener">
              {t("digest.open_source")}
            </a>
          )}
        </details>
      </li>
    );
  };
  const lines = coverageLines(card.coverage ?? []);
  const gaps = (card.coverage ?? []).flatMap((c) => c.gaps);

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
      {card.digest_status && card.digest_status !== "done" && (
        <p className="meta">{t(`digest.status.${card.digest_status}`)}</p>
      )}
      {card.digest && (
        <div className="digest">
          <p>
            <b>{t("digest.answer")}:</b> {card.digest.answer}
          </p>
          <p className="meta">{t("digest.outline")}</p>
          <ol>
            {card.digest.outline.map((line, i) => (
              <li key={i}>
                {line.text}
                {line.findings.length > 0 && <ul>{line.findings.map((f, j) => findingItem(f, `${i}-${j}`))}</ul>}
              </li>
            ))}
          </ol>
          {card.digest.expectation_contrast && (
            <p className="meta">
              {t("digest.contrast")}: {card.digest.expectation_contrast}
            </p>
          )}
          {card.digest.contradictions.length > 0 && (
            <>
              <p className="meta">{t("digest.contradictions")}</p>
              <ul>
                {card.digest.contradictions.map((c, i) => (
                  <li key={i}>
                    {c.about}
                    <ul>
                      {findingItem(c.a, "a")}
                      {findingItem(c.b, "b")}
                    </ul>
                    <span className="meta">
                      {c.open_question ? (
                        <a href={`#oq-${c.open_question}`}>{t("digest.contradiction_open")}</a>
                      ) : (
                        t("digest.contradiction_shown")
                      )}
                    </span>
                  </li>
                ))}
              </ul>
            </>
          )}
        </div>
      )}
      {card.reduced_doubt && (
        <p>
          <b>{t("digest.reduced_doubt")}</b> {t(`digest.reduced.${card.reduced_doubt}`)}
        </p>
      )}
      {card.digest_status && (
        <>
          {card.reader_contradictions.length > 0 && (
            <>
              <p className="meta">{t("digest.reader_contradictions")}</p>
              <ul>
                {card.reader_contradictions.map((f, i) => (
                  <li key={`rc${i}`}>
                    <ul>{findingItem(f, `rcf${i}`, f.verification !== "entered")}</ul>
                    {f.open_question && (
                      <a className="meta" href={`#oq-${f.open_question}`}>
                        {t("digest.contradiction_open")}
                      </a>
                    )}
                  </li>
                ))}
              </ul>
            </>
          )}
          {card.unused.length > 0 && (
            <details open>
              <summary>{t("digest.unused", { n: String(card.unused.length) })}</summary>
              <ul>{card.unused.map((f, i) => findingItem(f, `n${i}`))}</ul>
            </details>
          )}
          <details open={card.discarded.length > 0}>
            <summary>{t("digest.discarded", { n: String(card.discarded.length) })}</summary>
            <ul>{card.discarded.map((f, i) => findingItem(f, `d${i}`, true))}</ul>
          </details>
          {card.unverified.length > 0 && (
            <details>
              <summary>{t("digest.unverified", { n: String(card.unverified.length) })}</summary>
              <ul>{card.unverified.map((f, i) => findingItem(f, `u${i}`, true))}</ul>
            </details>
          )}
          <p className="meta">{t("digest.unreviewed")}</p>
          <ul>
            {lines.length === 0 && <li className="meta">{t("digest.unreviewed_none")}</li>}
            {lines.map((l, i) => (
              <li key={i} className="meta">
                {t(l.key, l.params)}
              </li>
            ))}
            {gaps.map((g, i) => (
              <li key={`g${i}`} className="meta">
                {t("digest.gaps")}: {g}
              </li>
            ))}
          </ul>
        </>
      )}
      {offersRepeat(card) && card.repeat && (
        <div className="digest-actions">
          <label>
            {t("digest.repeat_question")}
            <input value={question} onChange={(e) => setQuestion(e.target.value)} />
          </label>
          <button
            type="button"
            disabled={busy}
            onClick={() => void act(async () => (await repeatRound(projectID, card.label, question)).turn_id)}
          >
            {t("digest.repeat")}
          </button>
          <span className="meta"> {t("digest.repeat_cost", { cost: usd(card.repeat.estimated_cost_usd) })}</span>
          {offersLeaveOpen(card) && (
            <button
              type="button"
              disabled={busy}
              onClick={() =>
                void act(async () => {
                  await leaveRoundOpen(projectID, card.label);
                  return "";
                })
              }
            >
              {t("digest.leave_open")}
            </button>
          )}
          {card.scope_question && (
            <a className="meta" href={`#oq-${card.scope_question}`}>
              {t("digest.left_open")}
            </a>
          )}
          {error && <p className="turn-error">{t(`digest.error.${error}`)}</p>}
        </div>
      )}
      <p className="meta">
        {t("round.cost", { estimated: usd(card.estimated_cost_usd), spent: usd(card.spent_usd) })}
      </p>
      <p className="meta">{t("round.privacy")}</p>
    </div>
  );
}
