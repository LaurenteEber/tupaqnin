import { useEffect, useRef, useState } from "react";
import { acceptClaim, getBlockHistory, getDecisionHistory, markCritical } from "../api/client";
import type { Entry, Provenance } from "../api/graph";
import { SearchedName } from "./SearchedName";
import { candidateName, originKey } from "../provenance";
import { translate, type Lang } from "../i18n";

// P5 (backlog-r1 1.6): how a block came to be, newest first: who, when,
// the decisions that hold it with their contrast, your «porque» and
// what was discarded, and each datum copied from a source (P-60).
// Go computes it from the history and the graph; no model narrates it.
// With decision set (P-41, 30-oct), it is that decision's own history:
// the signoff opens it for a decision with no point in the text.
// Each current decision can be marked or unmarked as critical with one
// click (fichas §2, Tu turno; backlog-r2 2.1).
// [EL DISEÑO NO LO DICE]: where the mark goes; here, on each decision
// of the history, the one view that lists decisions that are not
// critical yet.
export function HistoryPanel({ lang, projectID, label, name, block, decision, onClose, onMarked, onSearchCandidate }: {
  lang: Lang;
  projectID: string;
  label: string;
  name: string;
  block: number;
  decision?: string;
  onClose: () => void;
  onMarked?: () => void;
  // «Buscarla» (B-69): you start a round on the candidate.
  onSearchCandidate?: (n: number) => void;
}) {
  const t = (key: string, params?: Record<string, string>) => translate(lang, key, params);
  const [entries, setEntries] = useState<Entry[] | null>(null);
  // «¿En qué te basaste?» (R-2): the block's provenance, from the graph.
  const [provenance, setProvenance] = useState<Provenance | null>(null);
  const [reason, setReason] = useState("");
  const [acceptError, setAcceptError] = useState("");
  const panel = useRef<HTMLDivElement>(null);

  // The panel opens below the editor, inside a scrolling column: bring
  // it into view, or the click seems to do nothing (P-34).
  useEffect(() => {
    panel.current?.scrollIntoView({ behavior: "smooth", block: "nearest" });
  }, [block, label, decision]);

  const [reload, setReload] = useState(0);

  useEffect(() => {
    setEntries(null);
    setProvenance(null);
    if (decision) {
      void getDecisionHistory(projectID, decision).then((v) => setEntries(v.entries), () => setEntries([]));
      return;
    }
    void getBlockHistory(projectID, label, block).then(
      (v) => {
        setEntries(v.entries);
        setProvenance(v.provenance);
      },
      () => setEntries([]),
    );
  }, [projectID, label, block, decision, reload]);

  async function accept() {
    if (!provenance) return;
    setAcceptError("");
    try {
      await acceptClaim(projectID, label, provenance.passage, reason);
      setReason("");
      setReload((n) => n + 1);
      onMarked?.();
    } catch (e) {
      setAcceptError((e as Error).message);
    }
  }

  async function toggleCritical(decisionLabel: string, marked: boolean) {
    try {
      await markCritical(projectID, decisionLabel, marked);
    } finally {
      setReload((n) => n + 1);
      onMarked?.();
    }
  }

  return (
    <div className="request-panel" ref={panel}>
      <h2>
        {decision
          ? t("history.decision_title", { section: name })
          : t("history.title", { section: name, block: String(block) })}
      </h2>
      <button type="button" onClick={onClose}>{t("history.close")}</button>
      {provenance && (
        <div className="provenance">
          {provenance.verified ? (
            <>
              <b>{t(originKey(provenance.origin))}</b>
              <ul>
                {provenance.sources.map((src, i) => (
                  <li key={i}>
                    {t("history.source")}:{" "}
                    {src.searched ? (
                      <SearchedName lang={lang} source={src.searched} />
                    ) : (
                      src.source
                    )}{" "}
                    · «{src.passage}»
                  </li>
                ))}
              </ul>
            </>
          ) : (
            <p>
              <b>{t(originKey(provenance.origin))}</b>
              {provenance.accepted && (
                <>
                  {" · "}
                  {t("provenance.accepted")}: «{provenance.accepted}»
                </>
              )}
            </p>
          )}
          {provenance.candidates.length > 0 && (
            // R-2, B-69: what the agent named without a citation; never a
            // source, so it does not change what is said above.
            <ul className="provenance-candidates">
              {provenance.candidates.map((c) => (
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
          )}
          {!provenance.verified && !provenance.accepted && (
            <div className="provenance-accept">
              <input
                type="text"
                value={reason}
                placeholder={t("provenance.reason_placeholder")}
                aria-label={t("provenance.reason_placeholder")}
                onChange={(e) => setReason(e.target.value)}
              />
              <button type="button" className="small" disabled={!reason.trim()} onClick={() => void accept()}>
                {t("provenance.accept")}
              </button>
              {acceptError && <p className="turn-error">{t(`provenance.error.${acceptError}`)}</p>}
            </div>
          )}
        </div>
      )}
      {entries && entries.length === 0 && <p>{t("history.empty")}</p>}
      <ul>
        {entries?.map((e, i) => (
          <li key={i}>
            {e.kind === "text_change" ? (
              <span>
                {localTime(e.at, lang)} · {t(`history.text_change.${e.actor}`)}
              </span>
            ) : e.kind === "datum" ? (
              e.datum && (
                <span>
                  {/* A datum copied from a source (A-3, P-60): its source and
                      the passage that holds it, literal. */}
                  {t("history.datum")} {e.datum.value} · {t("history.source")}: {e.datum.source}
                  {e.datum.passage && <> · «{e.datum.passage}»</>}
                </span>
              )
            ) : (
              e.decision && (
                <span>
                  {e.decision.discarded && <strong>{t("history.discarded")}: </strong>}
                  {localTime(e.at, lang)} · {t(e.decision.origin === "human" ? "history.decision.human" : "history.decision.agent")}{" "}
                  «{e.decision.statement}»
                  {" · "}{t("card.instead_of")} «{e.decision.instead_of}»
                  {e.decision.would_choose_if && <> · {t("card.would_choose_if")} «{e.decision.would_choose_if}»</>}
                  {e.decision.reason && <> · {t("history.your_reason")}: «{e.decision.reason}»</>}
                  {e.decision.figure && <> · {t("history.figure")}: {e.decision.figure}</>}
                  {!e.decision.discarded && (
                    <>
                      {" "}
                      <button
                        type="button"
                        onClick={() => void toggleCritical(e.decision!.label, !e.decision!.marked)}
                      >
                        {t(e.decision.marked ? "decision.unmark_critical" : "decision.mark_critical")}
                      </button>
                    </>
                  )}
                </span>
              )
            )}
          </li>
        ))}
      </ul>
    </div>
  );
}

// localTime shows a stored UTC time in the browser's time zone (T-7).
function localTime(at: string, lang: Lang): string {
  const d = new Date(at);
  return Number.isNaN(d.getTime()) ? "" : d.toLocaleString(lang, { dateStyle: "medium", timeStyle: "short" });
}
