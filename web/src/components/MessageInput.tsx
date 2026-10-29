import { useState } from "react";
import { translate, type Lang } from "../i18n";

// The single text entry of the app (B3-14): today it only accepts
// text. Voice, when it arrives, plugs into this same component and
// the cycle still receives only text.
export function MessageInput({
  lang,
  disabled,
  onSend,
}: {
  lang: Lang;
  disabled: boolean;
  onSend: (text: string) => Promise<void>;
}) {
  const [text, setText] = useState("");
  const [sending, setSending] = useState(false);

  async function send() {
    const value = text.trim();
    if (value === "" || sending) return;
    setSending(true);
    try {
      await onSend(value);
      setText("");
    } finally {
      setSending(false);
    }
  }

  return (
    <div className="message-input">
      <textarea
        value={text}
        disabled={disabled || sending}
        placeholder={translate(lang, "message.placeholder")}
        onChange={(e) => setText(e.target.value)}
        onKeyDown={(e) => {
          if (e.key === "Enter" && (e.metaKey || e.ctrlKey)) {
            e.preventDefault();
            void send();
          }
        }}
        rows={3}
      />
      <button type="button" disabled={disabled || sending || text.trim() === ""} onClick={() => void send()}>
        {translate(lang, "message.send")}
      </button>
    </div>
  );
}
