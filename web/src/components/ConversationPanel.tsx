import type { ReactNode } from "react";
import type { StreamDelta } from "../api/client";
import type { ConversationEntry } from "../api/types";
import { translate, type Lang } from "../i18n";

// U-5: the conversation as the history keeps it (Go reads it from the
// events), then the turn in flight as it streams. When a turn ends the
// page refetches the history and drops the stream, so nothing shows
// twice.
export function ConversationPanel({ lang, entries, requestText, working, stream, rawStream, errorCode, report }: {
  lang: Lang;
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
          <Entry key={`${e.created_at}-${i}`} lang={lang} entry={e} />
        ),
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
              {errorCode !== "turn_running" && <> {t("turn.write_to_agent")}</>}
            </p>
          )}
        </div>
      )}
    </>
  );
}

function Entry({ lang, entry }: { lang: Lang; entry: ConversationEntry }) {
  const t = (key: string) => translate(lang, key);
  switch (entry.kind) {
    case "message":
      return <div className="msg me">{entry.text}</div>;
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
          {/* Why the chain stopped is in its stop report (B-37), below. */}
          {entry.status === "agent_pending" && <p className="meta">{t("turn.status.agent_pending")}</p>}
        </div>
      );
    case "stop_report":
      // An earlier stop report: its first line and why the chain stopped.
      return (
        <div className="agent-step">
          <p className="meta">{t(`turn.stop.${entry.stop_reason}`)}</p>
          {entry.text && <p className="reply">{entry.text}</p>}
        </div>
      );
    default:
      return null;
  }
}
