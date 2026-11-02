import { useEffect, useMemo, useState } from "react";
import { exportURL, getExportText } from "../api/client";
import { renderMarkdown } from "../markdown";
import { translate, type Lang } from "../i18n";

// «Markdown» (U.3, B-47, B-97): the document exactly as it downloads, from the same
// route. It reads while its tab shows: on opening it, and when refreshKey moves
// after a save, a step of the agent or a signoff.
export function MarkdownPanel({
  lang,
  projectID,
  active,
  refreshKey,
}: {
  lang: Lang;
  projectID: string;
  active: boolean;
  refreshKey: string;
}) {
  const t = (key: string) => translate(lang, key);
  const [source, setSource] = useState("");
  const [failed, setFailed] = useState(false);
  const [showSource, setShowSource] = useState(false);
  const [copy, setCopy] = useState<"" | "copied" | "failed">("");

  useEffect(() => {
    if (!active) return;
    let live = true;
    getExportText(projectID, lang).then(
      (text) => {
        if (!live) return;
        setSource(text);
        setFailed(false);
      },
      () => {
        if (live) setFailed(true);
      },
    );
    return () => {
      live = false;
    };
  }, [active, projectID, lang, refreshKey]);

  const rendered = useMemo(() => renderMarkdown(source), [source]);

  async function copyText() {
    try {
      await navigator.clipboard.writeText(source);
      setCopy("copied");
    } catch {
      setCopy("failed");
    }
  }

  return (
    <div className="md-pane">
      <div className="md-tools">
        <span className="seg" role="group" aria-label={t("markdown.title")}>
          <button type="button" aria-pressed={!showSource} className={showSource ? "" : "on"} onClick={() => setShowSource(false)}>
            {t("markdown.view")}
          </button>
          <button type="button" aria-pressed={showSource} className={showSource ? "on" : ""} onClick={() => setShowSource(true)}>
            {t("markdown.source")}
          </button>
        </span>
        <span className="spacer" />
        <button type="button" className="small" onClick={() => void copyText()}>
          {t("markdown.copy")}
        </button>
        <a className="button-link" href={exportURL(projectID, lang)} download>
          {t("export.download")}
        </a>
      </div>
      {copy && (
        <p className={copy === "failed" ? "turn-error" : "meta"} role="status">
          {t(copy === "failed" ? "markdown.copy_failed" : "markdown.copied")}
        </p>
      )}
      {failed && <p className="turn-error">{t("markdown.error")}</p>}
      {showSource ? <pre className="md-raw">{source}</pre> : <div className="md-preview">{rendered}</div>}
      <p className="meta">{t("markdown.footer")}</p>
    </div>
  );
}
