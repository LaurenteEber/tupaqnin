import { useState } from "react";
import { editQueuedMessage, removeQueuedMessage } from "../api/client";
import { canSaveEdit, isQueued } from "../queue";
import type { ReactNode } from "react";
import type { StreamDelta } from "../api/client";
import type { ConversationEntry } from "../api/types";
import { translate, type Lang } from "../i18n";
import { invitesRetry, stopKey } from "../live";
import { RoundCard } from "./RoundCard";

// U-5: the conversation as the history keeps it (Go reads it from the
// events), then the turn in flight as it streams. When a turn ends the
// page refetches the history and drops the stream, so nothing shows
// twice.
export function ConversationPanel({ lang, projectID, entries, requestText, working, stream, rawStream, errorCode, report, disabled, onAnswerAudience, defensePreparing, onRoundAct, readOnly, onQueueChanged }: {
  // B-101: a queued message stays yours to change while the agent works, unless
  // this tab is read-only; a change reads the conversation again.
  readOnly?: boolean;
  onQueueChanged?: () => void;
  lang: Lang;
  // The project, for a round's card (backlog-r3 3.2).
  projectID: string;
  entries: ConversationEntry[];
  // The sent request already shows as your first message, with its
  // sources (P-37).
  requestText: string;
  working: boolean;
  stream: StreamDelta[];
  rawStream: string;
  errorCode: string;
  // P6 (backlog-r2 2.5): the last stop report, complete, in the place of
  // its entry; earlier reports show their first line.
  report?: ReactNode;
  // P-86: «Sí» and «No» on an audience question of a defense prepared
  // after your answer, with no stop report.
  disabled?: boolean;
  onAnswerAudience?: (label: string, answer: "yes" | "no") => void;
  // P-92: a defense after your answer is being prepared.
  defensePreparing?: boolean;
  // Backlog-r3 3.4: your click on a round's digest (repeat: its turn).
  onRoundAct?: (turnID: string) => void;
}) {
  const t = (key: string) => translate(lang, key);

  // [EL DISEÑO NO LO DICE] the understanding turn keeps the request as
  // your message (human_message); the request panel already shows it,
  // so its first occurrence is not repeated here. The understanding
  // itself is the card above, so its entries are not repeated either.
  let requestSkipped = requestText === "";
  const shown = entries.filter((e) => {
    if (e.kind === "understanding") return false;
    if (!requestSkipped && e.kind === "message" && e.text === requestText) {
      requestSkipped = true;
      return false;
    }
    return true;
  });

  let lastReport = -1;
  shown.forEach((e, i) => {
    if (e.kind === "stop_report") lastReport = i;
  });

  return (
    <>
      {shown.map((e, i) =>
        i === lastReport && report ? (
          <div key={`${e.created_at}-${i}`}>{report}</div>
        ) : (
          <Entry key={`${e.created_at}-${i}`} lang={lang} projectID={projectID} entry={e} disabled={disabled} onAnswerAudience={onAnswerAudience} onRoundAct={onRoundAct} readOnly={readOnly} onQueueChanged={onQueueChanged} />
        ),
      )}

      {defensePreparing && (
        <p className="working" aria-live="polite">{t("conversation.defense_preparing")}</p>
      )}

      {(working || stream.length > 0 || errorCode) && (
        <div className="turn-panel" aria-live="polite">
          {working && <p className="working">{t("turn.working")}</p>}
          {stream.map((piece, i) =>
            piece.field === "reply" ? (
              <p key={i} className="reply">{piece.text}</p>
            ) : piece.field === "text" ? (
              <div key={i} className="stream-text">{piece.text}</div>
            ) : piece.field === "triage_line" ? (
              <p key={i} className="triage">{piece.text}</p>
            ) : piece.field === "note" ? (
              <details key={i} className="note">
                <summary>{t("turn.note")}</summary>
                <p className="meta">{piece.text}</p>
              </details>
            ) : (
              <p key={i}>{piece.text}</p>
            ),
          )}
          {rawStream && (
            <details>
              <summary>{t("turn.raw")}</summary>
              <pre>{rawStream}</pre>
            </details>
          )}
          {errorCode && (
            <p className="turn-error">
              {t(`stream.error.${errorCode}`)}
              {invitesRetry(errorCode) && <> {t("turn.write_to_agent")}</>}
            </p>
          )}
        </div>
      )}
    </>
  );
}

// B-101: the mark under a message that waits, with «Edit» and «Remove».
function QueuedMark({ lang, projectID, n, text, readOnly, onChanged }: {
  lang: Lang;
  projectID: string;
  n: number;
  text: string;
  readOnly?: boolean;
  onChanged?: () => void;
}) {
  const t = (key: string) => translate(lang, key);
  const [editing, setEditing] = useState(false);
  const [draft, setDraft] = useState(text);
  const [busy, setBusy] = useState(false);
  const [late, setLate] = useState(false);

  async function act(run: () => Promise<void>) {
    setBusy(true);
    setLate(false);
    try {
      await run();
      setEditing(false);
    } catch {
      setLate(true); // it entered a step meanwhile
    } finally {
      setBusy(false);
      onChanged?.();
    }
  }

  return (
    <div className="queued" role="status">
      <span className="meta">{t("message.queued")}</span>
      {editing ? (
        <>
          <textarea aria-label={t("message.queued_edit")} value={draft} onChange={(e) => setDraft(e.target.value)} />
          <button type="button" className="small" disabled={busy || !canSaveEdit(draft)} onClick={() => void act(() => editQueuedMessage(projectID, n, draft))}>
            {t("message.queued_save")}
          </button>
          <button type="button" className="small" disabled={busy} onClick={() => setEditing(false)}>
            {t("message.queued_cancel")}
          </button>
        </>
      ) : (
        !readOnly && (
          <>
            <button type="button" className="link" disabled={busy} onClick={() => { setDraft(text); setEditing(true); }}>
              {t("message.queued_edit")}
            </button>
            <button type="button" className="link" disabled={busy} onClick={() => void act(() => removeQueuedMessage(projectID, n))}>
              {t("message.queued_remove")}
            </button>
          </>
        )
      )}
      {late && <span className="meta">{t("message.queued_late")}</span>}
    </div>
  );
}

function Entry({ lang, projectID, entry, disabled, onAnswerAudience, onRoundAct, readOnly, onQueueChanged }: {
  readOnly?: boolean;
  onQueueChanged?: () => void;
  lang: Lang;
  projectID: string;
  entry: ConversationEntry;
  disabled?: boolean;
  onAnswerAudience?: (label: string, answer: "yes" | "no") => void;
  onRoundAct?: (turnID: string) => void;
}) {
  const t = (key: string) => translate(lang, key);
  switch (entry.kind) {
    case "message":
      return (
        <>
          <div className="msg me">{entry.text}</div>
          {isQueued(entry) && (
            <QueuedMark key={entry.n} lang={lang} projectID={projectID} n={entry.n ?? 0} text={entry.text ?? ""} readOnly={readOnly} onChanged={onQueueChanged} />
          )}
          {entry.waits_signoff && <p className="meta" role="status">{t("message.waits_signoff")}</p>}
        </>
      );
    case "round":
      // Backlog-r3 3.2 (B-64): the round card, read from Go.
      return entry.label ? <RoundCard lang={lang} projectID={projectID} label={entry.label} onAct={onRoundAct} /> : null;
    case "card_answered":
      return (
        <p className="meta answered">
          {t("conversation.card_answered")} «{entry.text}»
        </p>
      );
    case "step":
      return (
        <div className="agent-step">
          {/* R-5: what the agent answers you, apart from its triage and note. */}
          {entry.reply && <p className="reply">{entry.reply}</p>}
          {entry.triage_line && <p className="triage">{entry.triage_line}</p>}
          {/* B-43: rule 0 moved a direct step to build; Go says why. */}
          {entry.reclassified && entry.reclassified.length > 0 && (
            <p className="triage">
              {translate(lang, "triage.reclassified", {
                what: entry.reclassified.map((r) => t(`triage.touches.${r}`)).join(", "),
              })}
            </p>
          )}
          {entry.note && (
            <details className="note">
              <summary>{t("turn.note")}</summary>
              <p className="meta">{entry.note}</p>
            </details>
          )}
          {entry.status === "agent_pending" && <p className="meta">{t("turn.status.agent_pending")}</p>}
          {/* Why the chain stopped is in its stop report (B-37), below; a
              stop without report says it here (P-85, B-24). */}
          {entry.stopped_without_report && entry.stop_reason && (
            <p className="meta">{t(`turn.stop.${entry.stop_reason}`)}</p>
          )}
        </div>
      );
    case "stop_report":
      // An earlier stop report: its first line and why the chain stopped.
      return (
        <div className="agent-step">
          <p className="meta">{t(stopKey(entry.stop_reason ?? "", entry.cause))}</p>
          {entry.incomplete && <p className="meta">{t("stop_report.incomplete")}</p>}
          {entry.text && <p className="reply">{entry.text}</p>}
        </div>
      );
    case "audience_question":
      // P-86: a likely question of your reader, from a defense prepared
      // when your answer left the section ready. It never blocks signing.
      return (
        <div className="note">
          <p className="meta">{translate(lang, "conversation.audience_question", { section: entry.section ?? "" })}</p>
          <p>
            {entry.not_covered ? translate(lang, "stop_report.defense.not_covered", { text: entry.text ?? "" }) : entry.text}{" "}
            {entry.state === "open" && entry.label ? (
              <>
                <button type="button" className="small" disabled={disabled}
                  onClick={() => entry.label && onAnswerAudience?.(entry.label, "yes")}>
                  {t("stop_report.defense.yes")}
                </button>{" "}
                <button type="button" className="small" disabled={disabled}
                  onClick={() => entry.label && onAnswerAudience?.(entry.label, "no")}>
                  {t("stop_report.defense.no")}
                </button>
              </>
            ) : (
              <span className="meta">{t(`conversation.audience_question.${entry.state}`)}</span>
            )}
          </p>
        </div>
      );
    default:
      return null;
  }
}
