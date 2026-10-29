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
import { FramePanel, type TodoTarget } from "./components/FramePanel";
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
  // The frame refetches when the understanding, the cards or the
  // sections change.
  const [frameTick, setFrameTick] = useState(0);
  const bumpFrame = useCallback(() => setFrameTick((k) => k + 1), []);
  const [menuOpen, setMenuOpen] = useState(false);
  const saveRef = useRef<() => Promise<void>>(async () => {});
  const registerSave = useCallback((fn: () => Promise<void>) => {
    saveRef.current = fn;
  }, []);

  const refreshSections = useCallback(async (project: string) => {
    const list = await listSections(project);
    setSections(list);
    setFrameTick((k) => k + 1);
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
    setMenuOpen(false);
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
    bumpFrame(); // the first line may have changed (M-4)
  }

  function openProject(id: string) {
    if (id !== projectID) {
      setSections([]);
      setSelected("");
      setHistoryBlock(0);
      clearStream();
      setStep(null);
      setErrorCode("");
      setProjectID(id);
    }
    setMenuOpen(false);
  }

  // «Te toca» leads to the card, the understanding or the signoff
  // button; the frame never opens from the signoff (§9).
  function reach(id: string) {
    const el = document.getElementById(id);
    if (!el) return;
    el.scrollIntoView({ block: "center", behavior: "smooth" });
    el.classList.add("flash");
    window.setTimeout(() => el.classList.remove("flash"), 900);
  }

  function onTodo(target: TodoTarget) {
    if (target.kind === "card") reach(`card-${target.label}`);
    else if (target.kind === "cases") reach("understanding");
    else {
      setSelected(target.section);
      setHistoryBlock(0);
      window.setTimeout(() => reach("sign-section"), 0);
    }
  }

  function openSection(label: string, block = 0) {
    setSelected(label);
    setHistoryBlock(block);
    window.setTimeout(() => reach(block > 0 ? "history" : "document"), 0);
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
  // Never a label on screen (P-30, P-36): the title, or the catalog's.
  const sectionName = (s: SectionView) => s.title || t("section.untitled");

  return (
    <main className="app">
      <header className="app-header">
        <div className="brand">
          <h1>{t("app.title")}</h1>
          <span className="tagline">{t("app.tagline")}</span>
        </div>
        <div className="lang-switch" role="group" aria-label="language">
          <button type="button" className={lang === "en" ? "active" : ""} onClick={() => setLang("en")}>
            {t("language.en")}
          </button>
          <button type="button" className={lang === "es" ? "active" : ""} onClick={() => setLang("es")}>
            {t("language.es")}
          </button>
        </div>
      </header>

      <div className={`columns${projectID && sections.length > 0 ? "" : " nodoc"}`}>
        <aside className="col" aria-label={t("frame.title")}>
          <div className="hd">
            <button
              type="button"
              className="proj"
              aria-expanded={menuOpen}
              onClick={() => setMenuOpen((open) => !open)}
            >
              {projectID ? projectName(currentProject) : t("project.list")}
            </button>
            {menuOpen && (
              <div className="pmenu">
                {projects.map((p) => (
                  <button key={p.project_id} type="button" onClick={() => openProject(p.project_id)}>
                    {projectName(p)}
                  </button>
                ))}
                <button type="button" className="meta" onClick={() => void newProject("case-5")}>
                  {t("project.demo")}
                </button>
                <button type="button" className="meta" onClick={() => void newProject()}>
                  {t("project.blank")}
                </button>
              </div>
            )}
          </div>
          <div className="scroll">
            {projectID ? (
              <FramePanel
                lang={lang}
                projectID={projectID}
                refreshKey={`${understandingKey}-${frameTick}`}
                onSection={(label) => openSection(label)}
                onHistory={(label, block) => openSection(label, block)}
                onTodo={onTodo}
              />
            ) : (
              <div className="frame-empty">
                <b>{t("frame.title")}</b>
                <span>{t("frame.empty")}</span>
              </div>
            )}
            {projectID && (
              <button type="button" className="small" onClick={() => void newSection()}>
                {t("section.new")}
              </button>
            )}
          </div>
        </aside>

        <section className="col" aria-label={t("chat.title")}>
          <div className="hd">
            <span className="lbl">{t("chat.title")}</span>
            {summary && (
              <span className="meta">
                {t("summary.spent", { spent: summary.spent_usd.toFixed(2), cap: summary.cap_usd.toFixed(2) })}
              </span>
            )}
          </div>
          <div className="scroll">
            {!projectID && (
              <div className="empty">
                <b>{t("chat.empty_title")}</b>
                <span>{t("chat.empty")}</span>
              </div>
            )}
            {projectID && !editor && <p className="turn-error">{t("tab.read_only")}</p>}

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
                onChanged={bumpFrame}
              />
            )}

            {(working || stream.length > 0 || step || errorCode) && (
              <div className="turn-panel" aria-live="polite">
                {working && <p className="working">{t("turn.working")}</p>}
                {stream.map((piece, i) =>
                  piece.field === "text" ? (
                    <div key={i} className="stream-text">{piece.text}</div>
                  ) : piece.field === "triage_line" ? (
                    <p key={i} className="triage">{piece.text}</p>
                  ) : piece.field === "note" ? (
                    <details key={i} className="note">
                      <summary>{t("turn.note")}</summary>
                      <p className="meta">{piece.text}</p>
                    </details>
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
                    <p className="meta">{t(`turn.status.${step.status}`)}</p>
                    {stream.length === 0 && step.triage_line && <p className="triage">{step.triage_line}</p>}
                    {stream.length === 0 && step.note && (
                      <details className="note">
                        <summary>{t("turn.note")}</summary>
                        <p className="meta">{step.note}</p>
                      </details>
                    )}
                  </div>
                )}
                {errorCode && <p className="turn-error">{t(`stream.error.${errorCode}`)}</p>}
              </div>
            )}

            {projectID && (
              <CardPanel
                lang={lang}
                projectID={projectID}
                refreshKey={understandingKey}
                onAnswered={() => void refreshSections(projectID)}
              />
            )}
          </div>
          <div className="composer">
            <MessageInput lang={lang} disabled={!projectID || !editor} onSend={send} />
            <span className="hint">{t("chat.decisions_hint")}</span>
          </div>
        </section>

        {projectID && sections.length > 0 && (
          <section className="col doc" aria-label={t("doc.title")} id="document">
            <div className="hd">
              <span className="lbl">{t("doc.title")}</span>
              {/* Signing is piece 1.7: the button is in place, without a flow. */}
              <button
                type="button"
                id="sign-section"
                className="primary small"
                disabled
                title={t("doc.sign_unavailable")}
              >
                {t("doc.sign")}
              </button>
            </div>
            <div className="scroll">
              <ul className="section-tabs">
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
              <p className="hint">{t("doc.paragraph_hint")}</p>

              {current && historyBlock > 0 && (
                <div id="history">
                  <HistoryPanel
                    lang={lang}
                    projectID={projectID}
                    label={current.label}
                    name={sectionName(current)}
                    block={historyBlock}
                    onClose={() => setHistoryBlock(0)}
                  />
                </div>
              )}

              <details className="trace">
                <summary>{t("doc.trace")}</summary>
                <dl>
                  <dt>{t("doc.trace.last_triage")}</dt>
                  <dd>{summary?.last_triage || t("doc.trace.none")}</dd>
                  {summary && (
                    <>
                      <dt>{t("doc.trace.spent")}</dt>
                      <dd>
                        {t("summary.spent", { spent: summary.spent_usd.toFixed(2), cap: summary.cap_usd.toFixed(2) })}
                      </dd>
                    </>
                  )}
                </dl>
              </details>
            </div>
          </section>
        )}
      </div>
    </main>
  );
}
