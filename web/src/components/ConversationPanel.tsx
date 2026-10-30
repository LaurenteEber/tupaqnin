import type { StreamDelta } from "../api/client";
import type { ConversationEntry } from "../api/types";
import { translate, type Lang } from "../i18n";

// U-5: the conversation as the history keeps it (Go reads it from the
// events), then the turn in flight as it streams. When a turn ends the
// page refetches the history and drops the stream, so nothing shows
// twice.
export function ConversationPanel({ lang, entries, requestText, working, stream, rawStream, errorCode }: {
  lang: Lang;
  entries: ConversationEntry[];
  // The sent request already shows as your first message, with its
  // sources (P-37).
  requestText: string;
  working: boolean;
  stream: StreamDelta[];
  rawStream: string;
  errorCode: string;
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

  return (
    <>
      {shown.map((e, i) => (
        <Entry key={`${e.created_at}-${i}`} lang={lang} entry={e} />
      ))}

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
          {entry.note && (
            <details className="note">
              <summary>{t("turn.note")}</summary>
              <p className="meta">{entry.note}</p>
            </details>
          )}
          {/* B-24 (P-43): why the chain stopped, as Go stored it, and that
              you can write to the agent. */}
          {entry.stop_reason ? (
            <p className="meta">
              {t(`turn.stop.${entry.stop_reason}`)} {t("turn.write_to_agent")}
            </p>
          ) : (
            entry.status === "agent_pending" && <p className="meta">{t("turn.status.agent_pending")}</p>
          )}
        </div>
      );
    default:
      return null;
  }
}
