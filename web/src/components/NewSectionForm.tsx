import { useState } from "react";
import { createSection } from "../api/client";
import { translate, type Lang } from "../i18n";

// FOCUS_MAX_WORDS mirrors cycle.FocusMaxWords (K-17, [inicial]); Go
// decides, this only shows the count.
const FOCUS_MAX_WORDS = 60;

// «+ Sección» (K-17, P-49): a title (required), the «aquí se concluye»
// mark (unmarked, the principal proposes the role) and «qué debe
// abordar» (optional). Go validates and answers the error code.
export function NewSectionForm({ lang, projectID, disabled, onCreated }: {
  lang: Lang;
  projectID: string;
  disabled: boolean;
  onCreated: (label: string) => void;
}) {
  const t = (key: string, params?: Record<string, string>) => translate(lang, key, params);
  const [open, setOpen] = useState(false);
  const [title, setTitle] = useState("");
  const [conclusion, setConclusion] = useState(false);
  const [focus, setFocus] = useState("");
  const [errorCode, setErrorCode] = useState("");
  const words = focus.trim() === "" ? 0 : focus.trim().split(/\s+/).length;

  function reset() {
    setOpen(false);
    setTitle("");
    setConclusion(false);
    setFocus("");
    setErrorCode("");
  }

  async function submit() {
    try {
      const created = await createSection(projectID, title, conclusion ? "conclusion" : "", focus);
      reset();
      onCreated(created.label);
    } catch (e) {
      setErrorCode(e instanceof Error ? e.message : "internal");
    }
  }

  if (!open) {
    return (
      <button type="button" className="small" disabled={disabled} onClick={() => setOpen(true)}>
        {t("section.new")}
      </button>
    );
  }
  return (
    <form
      className="new-section"
      onSubmit={(e) => {
        e.preventDefault();
        void submit();
      }}
    >
      <label>
        {t("section.field.title")}
        <input autoFocus required value={title} onChange={(e) => setTitle(e.target.value)} />
      </label>
      <label className="check">
        <input type="checkbox" checked={conclusion} onChange={(e) => setConclusion(e.target.checked)} />
        {t("section.field.conclusion")}
      </label>
      <label>
        {t("section.field.focus")}
        <textarea rows={3} value={focus} onChange={(e) => setFocus(e.target.value)} />
        <span className={`hint${words > FOCUS_MAX_WORDS ? " turn-error" : ""}`}>
          {t("section.field.focus_words", { n: String(words), max: String(FOCUS_MAX_WORDS) })}
        </span>
      </label>
      {errorCode && <p className="turn-error">{t(`section.error.${errorCode}`)}</p>}
      <div className="row">
        <button type="submit" className="primary small" disabled={title.trim() === ""}>
          {t("section.create")}
        </button>
        <button type="button" className="small" onClick={reset}>
          {t("section.cancel")}
        </button>
      </div>
    </form>
  );
}
