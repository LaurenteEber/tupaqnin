import { useState } from "react";
import { translate, type Lang } from "../i18n";

// H-3 (backlog-r4 4.4, B-83): the mode in the conversation's header, with its
// one-line warning and the field of your message when you activate it. Go keeps
// the mode; this only asks for it.
export function ModeBar({ lang, mode, disabled, onSet }: {
  lang: Lang;
  mode: string;
  disabled: boolean;
  onSet: (mode: string, message: string) => Promise<void>;
}) {
  const t = (key: string) => translate(lang, key);
  const [asking, setAsking] = useState(false);
  const [message, setMessage] = useState("");

  if (mode === "run_to_completion") {
    return (
      <span className="mode-bar">
        <b>{t("mode.run_to_completion")}</b>{" "}
        <button type="button" className="small" disabled={disabled} onClick={() => void onSet("normal", "")}>
          {t("mode.turn_off")}
        </button>
      </span>
    );
  }
  if (!asking) {
    return (
      <span className="mode-bar">
        <button type="button" className="small" disabled={disabled} onClick={() => setAsking(true)}>
          {t("mode.activate")}
        </button>
      </span>
    );
  }
  return (
    <div className="mode-bar" role="group" aria-label={t("mode.activate")}>
      <p className="meta">{t("mode.warning")}</p>
      <p className="meta">{t("mode.yours")}</p>
      <textarea
        rows={2}
        value={message}
        placeholder={t("mode.message_placeholder")}
        onChange={(e) => setMessage(e.target.value)}
      />
      <button
        type="button"
        className="small primary"
        disabled={disabled}
        onClick={() => {
          setAsking(false);
          setMessage("");
          void onSet("run_to_completion", message);
        }}
      >
        {t("mode.confirm")}
      </button>{" "}
      <button type="button" className="small" onClick={() => setAsking(false)}>
        {t("mode.cancel")}
      </button>
    </div>
  );
}
