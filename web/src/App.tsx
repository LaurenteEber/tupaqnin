import { useCallback, useEffect, useRef, useState } from "react";
import {
  createProject,
  createSection,
  listSections,
  postMessage,
  subscribeTurn,
  type StepPayload,
} from "./api/client";
import type { SectionView } from "./api/types";
import { MessageInput } from "./components/MessageInput";
import { SectionEditor } from "./components/SectionEditor";
import { initialLang, translate, type Lang } from "./i18n";

export function App() {
  const [lang, setLang] = useState<Lang>(initialLang);
  const t = (key: string, params?: Record<string, string>) => translate(lang, key, params);

  const [projectID, setProjectID] = useState("");
  const [sections, setSections] = useState<SectionView[]>([]);
  const [selected, setSelected] = useState("");
  const [streamText, setStreamText] = useState("");
  const [step, setStep] = useState<StepPayload | null>(null);
  const [errorCode, setErrorCode] = useState("");
  const [working, setWorking] = useState(false);
  const saveRef = useRef<() => Promise<void>>(async () => {});
  const registerSave = useCallback((fn: () => Promise<void>) => {
    saveRef.current = fn;
  }, []);

  const refreshSections = useCallback(async (project: string) => {
    const list = await listSections(project);
    setSections(list);
    setSelected((prev) => (list.some((s) => s.label === prev) ? prev : (list[0]?.label ?? "")));
  }, []);

  useEffect(() => {
    if (projectID) void refreshSections(projectID);
  }, [projectID, refreshSections]);

  async function newProject() {
    const created = await createProject();
    setSections([]);
    setSelected("");
    setStreamText("");
    setStep(null);
    setProjectID(created.project_id);
  }

  async function newSection() {
    if (!projectID) return;
    const created = await createSection(projectID);
    await refreshSections(projectID);
    setSelected(created.label);
  }

  function onSaved(label: string, version: number, text: string) {
    setSections((prev) =>
      prev.map((s) => (s.label === label ? { ...s, version, text } : s)),
    );
  }

  async function send(text: string) {
    if (!projectID) return;
    await saveRef.current(); // always save before sending (§2)
    setStreamText("");
    setStep(null);
    setErrorCode("");
    const { turn_id } = await postMessage(projectID, text);
    setWorking(true);
    subscribeTurn(projectID, turn_id, {
      onDelta: (delta) => setStreamText((prev) => prev + delta),
      onStep: (payload) => {
        setStep(payload);
        setWorking(false);
      },
      onError: (code) => {
        setErrorCode(code);
        setWorking(false);
      },
    });
  }

  const current = sections.find((s) => s.label === selected);

  return (
    <main className="app">
      <header className="app-header">
        <div>
          <h1>{t("app.title")}</h1>
          <p className="tagline">{t("app.tagline")}</p>
        </div>
        <div className="header-actions">
          <button type="button" onClick={() => void newProject()}>
            {t("project.new")}
          </button>
          <div className="lang-switch" role="group" aria-label="language">
            <button type="button" className={lang === "en" ? "active" : ""} onClick={() => setLang("en")}>
              {t("language.en")}
            </button>
            <button type="button" className={lang === "es" ? "active" : ""} onClick={() => setLang("es")}>
              {t("language.es")}
            </button>
          </div>
        </div>
      </header>

      {projectID && (
        <p className="project-line">
          {t("project.current")}: <code>{projectID}</code>
        </p>
      )}

      <div className="layout">
        <aside className="sections-panel">
          <button type="button" disabled={!projectID} onClick={() => void newSection()}>
            {t("section.new")}
          </button>
          {projectID && sections.length === 0 && <p>{t("section.empty")}</p>}
          <ul className="section-list">
            {sections.map((s) => (
              <li key={s.label}>
                <button
                  type="button"
                  className={s.label === selected ? "active" : ""}
                  onClick={() => setSelected(s.label)}
                >
                  {t("section.label", { label: s.label })} · v{s.version}
                </button>
              </li>
            ))}
          </ul>
        </aside>

        <section className="work-panel">
          {current && (
            <SectionEditor
              key={current.label}
              lang={lang}
              projectID={projectID}
              label={current.label}
              version={current.version}
              text={current.text}
              onSaved={onSaved}
              registerSave={registerSave}
            />
          )}

          <MessageInput lang={lang} disabled={!projectID} onSend={send} />

          {(working || streamText || step || errorCode) && (
            <div className="turn-panel" aria-live="polite">
              {working && <p className="working">{t("turn.working")}</p>}
              {streamText && <pre className="stream-text">{streamText}</pre>}
              {step && (
                <div className="step-result">
                  <p>{t(`turn.status.${step.status}`)}</p>
                  {step.triage_line && (
                    <p>
                      <strong>{t("turn.triage")}:</strong> {step.triage_line}
                    </p>
                  )}
                  {step.note && (
                    <p>
                      <strong>{t("turn.note")}:</strong> {step.note}
                    </p>
                  )}
                </div>
              )}
              {errorCode && <p className="turn-error">{t(`stream.error.${errorCode}`)}</p>}
            </div>
          )}
        </section>
      </div>
    </main>
  );
}
