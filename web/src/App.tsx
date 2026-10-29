import { useCallback, useEffect, useRef, useState } from "react";
import {
  claimEditor,
  createProject,
  getSummary,
  createSection,
  listProjects,
  listSections,
  postMessage,
  subscribeTurn,
  type StepPayload,
  type StreamDelta,
} from "./api/client";
import type { ProjectView, SectionView, SummaryView } from "./api/types";
import { MessageInput } from "./components/MessageInput";
import { RequestPanel } from "./components/RequestPanel";
import { UnderstandingPanel } from "./components/UnderstandingPanel";
import { CardPanel } from "./components/CardPanel";
import { HistoryPanel } from "./components/HistoryPanel";
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
  // The streamed turn, by field (P-28): triage line, text, note or
  // understanding; the raw model text only in a devroutes build.
  const [stream, setStream] = useState<StreamDelta[]>([]);
  const [rawStream, setRawStream] = useState("");
  const [step, setStep] = useState<StepPayload | null>(null);
  const [errorCode, setErrorCode] = useState("");
  const [working, setWorking] = useState(false);
  // Only one tab edits (backlog-r1 1.5): this tab holds the editor
  // lease, renewed every 15 s, or stays read-only.
  const [editor, setEditor] = useState(true);
  const [summary, setSummary] = useState<SummaryView | null>(null);
  const [historyBlock, setHistoryBlock] = useState(0);
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

  useEffect(() => {
    if (!projectID) return;
    const renew = () => void claimEditor(projectID).then(setEditor, () => setEditor(false));
    renew();
    const timer = window.setInterval(renew, 15_000);
    return () => window.clearInterval(timer);
  }, [projectID]);

  useEffect(() => {
    if (projectID) void getSummary(projectID).then(setSummary);
  }, [projectID, understandingKey]);

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
    clearStream();
    setStep(null);
    setProjectID(created.project_id);
    await refreshProjects();
  }

  function clearStream() {
    setStream([]);
    setRawStream("");
  }

  function watchTurn(turn_id: string) {
    setWorking(true);
    subscribeTurn(projectID, turn_id, {
      onDelta: (delta) => {
        if (delta.field === "raw") {
          setRawStream((prev) => prev + delta.text);
          return;
        }
        setStream((prev) => {
          const last = prev[prev.length - 1];
          if (!delta.start && last && last.field === delta.field) {
            return [...prev.slice(0, -1), { ...last, text: last.text + delta.text }];
          }
          return [...prev, { field: delta.field, text: delta.text }];
        });
      },
      onStep: (payload) => {
        setStep(payload);
        setWorking(false);
        if (payload.next_turn_id) {
          // The agent's chain starts on its own after the understanding.
          setUnderstandingKey((k) => k + 1);
          setUnderstandingTurn(false);
          clearStream();
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
    clearStream();
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
          {summary && (
            <>
              {" · "}{t("summary.spent", { spent: summary.spent_usd.toFixed(2), cap: summary.cap_usd.toFixed(2) })}
              {summary.last_triage && <> · {t("turn.triage")}: {summary.last_triage}</>}
            </>
          )}
        </p>
      )}
      {projectID && !editor && <p className="turn-error">{t("tab.read_only")}</p>}

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
            clearStream();
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
                  onClick={() => {
                    setSelected(s.label);
                    setHistoryBlock(0);
                  }}
                >
                  {sectionName(s)} · {t(`section.status.${s.state}`)}
                  {s.pendings > 0 && ` · ${t("section.pendings", { n: String(s.pendings) })}`}
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
              readOnly={!editor}
              onBlock={setHistoryBlock}
              version={current.version}
              text={current.text}
              onSaved={onSaved}
              registerSave={registerSave}
            />
          )}

          {current && historyBlock > 0 && (
            <HistoryPanel
              lang={lang}
              projectID={projectID}
              label={current.label}
              name={sectionName(current)}
              block={historyBlock}
              onClose={() => setHistoryBlock(0)}
            />
          )}

          <MessageInput lang={lang} disabled={!projectID || !editor} onSend={send} />

          {(working || stream.length > 0 || step || errorCode) && (
            <div className="turn-panel" aria-live="polite">
              {working && <p className="working">{t("turn.working")}</p>}
              {stream.map((piece, i) =>
                piece.field === "text" ? (
                  <div key={i} className="stream-text">{piece.text}</div>
                ) : piece.field === "triage_line" ? (
                  <p key={i}><strong>{t("turn.triage")}:</strong> {piece.text}</p>
                ) : piece.field === "note" ? (
                  <p key={i}><strong>{t("turn.note")}:</strong> {piece.text}</p>
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
              {step && (
                <div className="step-result">
                  <p>{t(`turn.status.${step.status}`)}</p>
                  {stream.length === 0 && step.triage_line && (
                    <p>
                      <strong>{t("turn.triage")}:</strong> {step.triage_line}
                    </p>
                  )}
                  {stream.length === 0 && step.note && (
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
