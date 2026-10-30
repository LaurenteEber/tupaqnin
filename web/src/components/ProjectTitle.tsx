import { useState } from "react";
import { renameProject } from "../api/client";
import { translate, type Lang } from "../i18n";

// U-1: the project title in the header. The understanding proposes it
// (Go); one click turns it into a field you edit. Go refuses a blank
// name (invalid_name).
export function ProjectTitle({ lang, projectID, name, onRenamed }: {
  lang: Lang;
  projectID: string;
  name: string;
  onRenamed: () => void;
}) {
  const t = (key: string) => translate(lang, key);
  const [editing, setEditing] = useState(false);
  const [value, setValue] = useState("");
  const [errorCode, setErrorCode] = useState("");

  function start() {
    setValue(name);
    setErrorCode("");
    setEditing(true);
  }

  async function save() {
    if (value.trim() === name) {
      setEditing(false);
      return;
    }
    try {
      await renameProject(projectID, value);
      setEditing(false);
      onRenamed();
    } catch (e) {
      setErrorCode(e instanceof Error ? e.message : "internal");
    }
  }

  if (!editing) {
    return (
      <button type="button" className="project-title" title={t("project.rename")} onClick={start}>
        {name || t("project.untitled")}
      </button>
    );
  }
  return (
    <span className="project-title-edit">
      <input
        autoFocus
        aria-label={t("project.rename")}
        value={value}
        onChange={(e) => setValue(e.target.value)}
        onBlur={() => void save()}
        onKeyDown={(e) => {
          if (e.key === "Enter") void save();
          if (e.key === "Escape") setEditing(false);
        }}
      />
      {errorCode && <span className="turn-error">{t(`project.error.${errorCode}`)}</span>}
    </span>
  );
}
