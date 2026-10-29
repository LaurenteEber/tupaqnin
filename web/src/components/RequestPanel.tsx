import { useEffect, useState } from "react";
import { getRequest, putRequest, submitRequest, uploadSource } from "../api/client";
import type { RequestView } from "../api/types";
import { translate, type Lang } from "../i18n";

// The left half of P2 (backlog-r1 1.1): the request with its
// audience, criteria and premise, plus its sources. Everything is
// kept literal in Go; this panel only transports it.
export function RequestPanel({
  lang,
  projectID,
  onSubmitted,
}: {
  lang: Lang;
  projectID: string;
  onSubmitted: (turnID: string) => void;
}) {
  const t = (key: string) => translate(lang, key);
  const [view, setView] = useState<RequestView | null>(null);
  const [text, setText] = useState("");
  const [audience, setAudience] = useState("");
  const [criteria, setCriteria] = useState("");
  const [premise, setPremise] = useState("");
  const [exploratory, setExploratory] = useState(false);
  const [kind, setKind] = useState("file");
  const [errorCode, setErrorCode] = useState("");
  const [saved, setSaved] = useState(false);

  useEffect(() => {
    if (!projectID) return;
    void getRequest(projectID).then((v) => {
      setView(v);
      setText(v.text);
      setAudience(v.audience);
      setCriteria(v.criteria.join("\n"));
      setPremise(v.premise);
      setExploratory(v.exploratory);
    });
  }, [projectID]);

  async function save() {
    const v = await putRequest(projectID, {
      text,
      audience,
      criteria: criteria.split("\n").map((c) => c.trim()).filter(Boolean),
      premise,
      exploratory,
    });
    setView(v);
    setSaved(true);
  }

  async function upload(file: File) {
    setErrorCode("");
    try {
      await uploadSource(projectID, kind, file);
      setView(await getRequest(projectID));
    } catch (e) {
      setErrorCode(e instanceof Error ? e.message : "internal");
    }
  }

  async function submit() {
    setErrorCode("");
    try {
      await save();
      const { turn_id } = await submitRequest(projectID);
      onSubmitted(turn_id);
    } catch (e) {
      setErrorCode(e instanceof Error ? e.message : "internal");
    }
  }

  if (!projectID) return null;

  return (
    <div className="request-panel">
      <h2>{t("request.title")}</h2>
      <label>
        {t("request.text")}
        <textarea value={text} rows={4} onChange={(e) => setText(e.target.value)} />
      </label>
      <label>
        {t("request.audience")}
        <input value={audience} onChange={(e) => setAudience(e.target.value)} />
      </label>
      <label>
        {t("request.criteria")}
        <textarea value={criteria} rows={3} onChange={(e) => setCriteria(e.target.value)} />
      </label>
      <label>
        {t("request.premise")}
        <input value={premise} disabled={exploratory} onChange={(e) => setPremise(e.target.value)} />
      </label>
      <label>
        <input type="checkbox" checked={exploratory} onChange={(e) => setExploratory(e.target.checked)} />
        {t("request.exploratory")}
      </label>
      <div>
        <button type="button" onClick={() => void save()}>{t("request.save")}</button>
        <button type="button" onClick={() => void submit()}>{t("request.submit")}</button>
        {saved && <span>{t("request.saved")}</span>}
      </div>

      <h3>{t("source.title")}</h3>
      {view && view.sources.length === 0 && <p>{t("source.empty")}</p>}
      <ul>
        {view?.sources.map((s) => (
          <li key={s.label}>
            {s.filename} · {s.label} · <code>{s.fingerprint.slice(0, 12)}</code>
          </li>
        ))}
      </ul>
      <label>
        {t("source.kind")}
        <select value={kind} onChange={(e) => setKind(e.target.value)}>
          <option value="file">{t("source.kind.file")}</option>
          <option value="calculation">{t("source.kind.calculation")}</option>
        </select>
      </label>
      <input
        type="file"
        accept=".md,.txt,.csv,.pdf"
        aria-label={t("source.upload")}
        onChange={(e) => {
          const f = e.target.files?.[0];
          if (f) void upload(f);
          e.target.value = "";
        }}
      />
      {errorCode && <p className="turn-error">{t(`source.error.${errorCode}`)}</p>}
    </div>
  );
}
