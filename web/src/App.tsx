import { useCallback, useEffect, useRef, useState } from "react";
import {
  createProject,
  createSection,
  listProjects,
  listSections,
  postMessage,
  subscribeTurn,
  type StepPayload,
} from "./api/client";
import type { ProjectView, SectionView } from "./api/types";
import { MessageInput } from "./components/MessageInput";
import { RequestPanel } from "./components/RequestPanel";
import { UnderstandingPanel } from "./components/UnderstandingPanel";
import { CardPanel } from "./components/CardPanel";
import { SectionEditor } from "./components/SectionEditor";
import { initialLang, translate, type Lang } from "./i18n";

export function App() {
  const [lang, setLang] = useState<Lang>(initialLang);
  const t = (key: string, params?: Record<string, string>) => translate(lang, key, params);

  const [projects, setProjects] = useState<ProjectView[]>([]);
  const [understandingKey, setUnderstandingKey] = useState(0);
  const [projectID, setProjectID] = useState("");
  const [sections, setSections] = useState<SectionView[]>([]);
  const [selected, setSelected] = useState("");
  const [streamText, setStreamText] = useState("");
  const [step, setStep] = useState<StepPayload | null>(null);
  const [errorCode, setErrorCode] = useState("");
  const [working, setWorking] = useState(false);
  const [understandingTurn, setUnderstandingTurn] = useState(false);
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

  const refreshProjects = useCallback(async () => {
    setProjects(await listProjects());
  }, []);

  useEffect(() => {
    void refreshProjects();
  }, [refreshProjects]);

  async function newProject(template = "") {
    const created = await createProject(template);
    setSections([]);
    setSelected("");
    setStreamText("");
    setStep(null);
    setProjectID(created.project_id);
    await refreshProjects();
  }

  function watchTurn(turn_id: string) {
    setWorking(true);
    subscribeTurn(projectID, turn_id, {
      onDelta: (delta) => setStreamText((prev) => prev + delta),
      onStep: (payload) => {
        setStep(payload);
        setWorking(false);
        if (payload.next_turn_id) {
          // The agent's chain starts on its own after the understanding.
          setUnderstandingKey((k) => k + 1);
          setUnderstandingTurn(false);
          setStreamText("");
          watchTurn(payload.next_turn_id);
          return;
        }
        // Any turn (submit or message) can create cases and sections:
        // refetch both when its stream ends (P-26).
        setUnderstandingKey((k) => k + 1);
        void refreshSections(projectID);
      },
      onError: (code) => {
        setErrorCode(code);
        setWorking(false);
      },
    });
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
    setUnderstandingTurn(false);
    setStreamText("");
    setStep(null);
    setErrorCode("");
    let turn_id: string;
    try {
      ({ turn_id } = await postMessage(projectID, text));
    } catch (e) {
      // [EL DISEÑO NO LO DICE] 409 turn_running: a turn of this
      // project is already running; the message was not sent.
      setErrorCode(e instanceof Error && e.message === "http_409" ? "turn_running" : "turn_failed");
      return;
    }
    watchTurn(turn_id);
  }

  const current = sections.find((s) => s.label === selected);
  const currentProject = projects.find((p) => p.project_id === projectID);
  // Never an internal id on screen (P-30): the name, or the catalog's.
  const projectName = (p?: ProjectView) => p?.name || t("project.untitled");
  const sectionName = (s: SectionView) => s.title || t("section.label", { label: s.label });

  return (
    <main className="app">
      <header className="app-header">
        <div>
          <h1>{t("app.title")}</h1>
          <p className="tagline">{t("app.tagline")}</p>
        </div>
        <div className="header-actions">
          <button type="button" onClick={() => void newProject("case-5")}>
            {t("project.demo")}
          </button>
          <button type="button" onClick={() => void newProject()}>
            {t("project.blank")}
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
          {t("project.current")}: {projectName(currentProject)}
        </p>
      )}

      <section className="projects-panel">
        <h2>{t("project.list")}</h2>
        <ul>
          {projects.map((p) => (
            <li key={p.project_id}>
              <button type="button" onClick={() => setProjectID(p.project_id)}>
                {projectName(p)}
              </button>
            </li>
          ))}
        </ul>
      </section>

      {projectID && (
        <UnderstandingPanel
          lang={lang}
          projectID={projectID}
          refreshKey={understandingKey}
          pending={understandingTurn && step?.status === "agent_pending"}
          onRoleChanged={() => {
            void refreshSections(projectID);
            setUnderstandingKey((k) => k + 1); // a role can open a card (K-13)
          }}
        />
      )}

      {projectID && (
        <CardPanel
          lang={lang}
          projectID={projectID}
          refreshKey={understandingKey}
          onAnswered={() => void refreshSections(projectID)}
        />
      )}

      {projectID && (
        <RequestPanel
          key={projectID}
          lang={lang}
          projectID={projectID}
          onSubmitted={(turnID) => {
            setStreamText("");
            setStep(null);
            setErrorCode("");
            setUnderstandingTurn(true);
            watchTurn(turnID);
          }}
        />
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
                  {sectionName(s)} · v{s.version}
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
              name={sectionName(current)}
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
