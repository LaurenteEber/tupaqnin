import { useState } from "react";
import { discardFlaw } from "../api/client";
import type { RoundCard as RoundCardView } from "../api/types";
import { flawOutcomeKey, offersNotApplicable, triageKey } from "../judge";
import { numberLocale, translate, type Lang } from "../i18n";

// A judge round in the conversation (backlog-r3 3.6, B-67; pantallas
// «Juez (3.6)»): the triage line (K-3), the flaws, each with its risk, its
// scenario and «No aplica» until the batch starts, and afterwards what the
// recheck said. Passages stay literal (rule 8).
export function JudgeCard({ lang, projectID, card, reload }: {
  lang: Lang;
  projectID: string;
  card: RoundCardView;
  reload: () => Promise<void>;
}) {
  const t = (key: string, params?: Record<string, string>) => translate(lang, key, params);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const risks = (card.risks ?? []).map((r) => t(`judge.risk.${r}`)).join(", ");
  const flaws = card.flaws ?? [];
  const late = card.late_flaws ?? [];
  const section = card.section?.title || t("section.untitled");

  async function discard(n: number) {
    setBusy(true);
    setError("");
    try {
      await discardFlaw(projectID, card.label, n);
      await reload();
    } catch (e) {
      setError(e instanceof Error ? e.message : "internal");
      await reload();
    } finally {
      setBusy(false);
    }
  }

  return (
    <div id={`round-${card.label}`} className="agent-step round-card">
      <p className="meta">
        {t(triageKey(card), { risks, section })} · {t(card.running ? "judge.running" : `round.state.${card.state}`)}
      </p>
      {(card.judge_status === "judge_failed" || card.judge_status === "project_cap" || card.judge_status === "demo_cap") && (
        <p className="meta">{t("judge.not_verified")}</p>
      )}
      {card.judge_status === "done" && flaws.length === 0 && <p className="meta">{t("judge.no_flaws")}</p>}
      {flaws.length > 0 && (
        <>
          <p className="meta">{t("judge.flaws", { n: String(flaws.length) })}</p>
          <ul>
            {flaws.map((f) => {
              const outcome = flawOutcomeKey(f);
              return (
                <li key={f.n}>
                  «{f.passage}» <span className="meta">· {t(`judge.risk.${f.risk}`)}</span>
                  <br />
                  {f.scenario}
                  {outcome && <span className="meta"> · {t(outcome)}</span>}
                  {f.open_question && (
                    <>
                      {" "}
                      <a className="meta" href={`#oq-${f.open_question}`}>
                        {t("judge.go_question")}
                      </a>
                    </>
                  )}
                  {offersNotApplicable(card, f) && (
                    <>
                      {" "}
                      <button type="button" className="small" disabled={busy} onClick={() => void discard(f.n)}>
                        {t("judge.not_applicable")}
                      </button>
                    </>
                  )}
                </li>
              );
            })}
          </ul>
        </>
      )}
      {late.length > 0 && (
        <details>
          <summary>{t("judge.late", { n: String(late.length) })}</summary>
          <ul>
            {late.map((f) => (
              <li key={f.n}>
                «{f.passage}» <span className="meta">· {t(`judge.risk.${f.risk}`)}</span>
                <br />
                {f.scenario}
              </li>
            ))}
          </ul>
        </details>
      )}
      {error && <p className="turn-error">{t(`judge.error.${error}`)}</p>}
      <p className="meta">
        {t("judge.spent", {
          spent: card.spent_usd.toLocaleString(numberLocale(lang), { minimumFractionDigits: 2, maximumFractionDigits: 3 }),
        })}
      </p>
    </div>
  );
}
