import { useState, type KeyboardEvent } from "react";
import { changeSectionDetails } from "../api/client";
import { translate, type Lang } from "../i18n";

type Field = "title" | "focus";

// P-49: under the section header, the title and «qué debe abordar» are
// edited with one click. Enter (title) or leaving the field saves;
// Escape cancels. Neither changes the state (K-17): Go records
// section_details_changed.
export function SectionDetails({ lang, projectID, label, title, focus, readOnly, onChanged }: {
  lang: Lang;
  projectID: string;
  label: string;
  title: string;
  focus: string;
  readOnly: boolean;
  onChanged: () => void;
}) {
  const t = (key: string) => translate(lang, key);
  const [editing, setEditing] = useState<Field | null>(null);
  const [value, setValue] = useState("");
  const [errorCode, setErrorCode] = useState("");
  const current = { title, focus };

  function start(field: Field) {
    if (readOnly) return;
    setValue(current[field]);
    setErrorCode("");
    setEditing(field);
  }

  async function save() {
    if (!editing) return;
    if (value === current[editing]) {
      setEditing(null);
      return;
    }
    try {
      await changeSectionDetails(projectID, label, { [editing]: value });
      setEditing(null);
      onChanged();
    } catch (e) {
      setErrorCode(e instanceof Error ? e.message : "internal");
    }
  }

  const keys = (e: KeyboardEvent) => {
    if (e.key === "Enter" && editing === "title") void save();
    if (e.key === "Escape") setEditing(null);
  };

  return (
    <div className="section-details">
      {editing === "title" ? (
        <input autoFocus aria-label={t("section.field.title")} value={value}
          onChange={(e) => setValue(e.target.value)} onBlur={() => void save()} onKeyDown={keys} />
      ) : (
        <button type="button" className="section-title" disabled={readOnly} title={t("section.edit_title")}
          onClick={() => start("title")}>
          {title || t("section.untitled")}
        </button>
      )}
      {editing === "focus" ? (
        <textarea autoFocus rows={3} aria-label={t("section.field.focus")} value={value}
          onChange={(e) => setValue(e.target.value)} onBlur={() => void save()} onKeyDown={keys} />
      ) : (
        <button type="button" className="section-focus" disabled={readOnly} title={t("section.edit_focus")}
          onClick={() => start("focus")}>
          <b>{t("section.field.focus")}:</b> {focus || <span className="no">{t("section.focus_empty")}</span>}
        </button>
      )}
      {errorCode && <p className="turn-error">{t(`section.error.${errorCode}`)}</p>}
    </div>
  );
}
