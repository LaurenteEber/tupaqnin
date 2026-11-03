import { useCallback, useEffect, useRef, useState } from "react";
import {
  claimEditor,
  releaseEditor,
  takeOverEditor,
  auditSection,
  continueFromStopReport,
  createProject,
  getRound,
  getSummary,
  answerAudienceQuestion,
  getConversation,
  getSection,
  undoEdit,
  getSession,
  logout,
  listProjects,
  listSections,
  reorderSections,
  postMessage,
  putMode,
  requestSignoff,
  startRound,
  subscribeTurn,
  type StreamDelta,
} from "./api/client";
import type { ConversationEntry, ProjectLine, SectionView, SessionView, SignoffBlocked, SummaryView } from "./api/types";
import type { EditAlert, Warning } from "./api/graph";
import { undoLineKey, undoReasonKey } from "./undo";
import { MessageInput } from "./components/MessageInput";
import { RequestPanel } from "./components/RequestPanel";
import { UnderstandingPanel } from "./components/UnderstandingPanel";
import { CardPanel } from "./components/CardPanel";
import { HistoryPanel } from "./components/HistoryPanel";
import { NewSectionForm } from "./components/NewSectionForm";
import { SectionDetails } from "./components/SectionDetails";
import { SectionEditor } from "./components/SectionEditor";
import { ProposalPanel } from "./components/ProposalPanel";
import { SectionWarnings } from "./components/SectionWarnings";
import { EditAlerts } from "./components/EditAlerts";
import { FramePanel, type TodoTarget } from "./components/FramePanel";
import { SignoffPanel } from "./components/SignoffPanel";
import { ConversationPanel } from "./components/ConversationPanel";
import { StopReportPanel } from "./components/StopReportPanel";
import { ModeBar } from "./components/ModeBar";
import { ModelLine } from "./components/ModelLine";
import { ProjectTitle } from "./components/ProjectTitle";
import { HomePage, type HomeForm } from "./components/HomePage";
import { LeftPanel } from "./components/LeftPanel";
import { RightPanel } from "./components/RightPanel";
import { MarkdownPanel } from "./components/MarkdownPanel";
import { CenterHeader } from "./components/CenterHeader";
import { Shell, useLayout } from "./components/Shell";
import { centerView, effectiveTab, routeTarget } from "./layout";
import { createErrorKey } from "./home";
import { initialLang, translate, type Lang } from "./i18n";
import { takeOverResult, type TakeOverResult } from "./tab";
import { projectHash, readProjectHash, saveLang } from "./route";
import { isQueued } from "./queue";
import {
  IDLE_ACTIVITY,
  SUMMARY_POLL_MS,
  auditErrorCode,
  keepsAskingToSign,
  nextActivity,
  pollDefense,
  roundErrorCode,
  roundStarted,
  sendErrorCode,
  startsStep,
  statusLine,
  turnToFollow,
  type AgentActivity,
} from "./live";

export function App() {
  const [lang, setLangState] = useState<Lang>(initialLang);
  const setLang = (next: Lang) => {
    setLangState(next);
    saveLang(next, () => globalThis.localStorage); // C.3
  };
  const t = (key: string, params?: Record<string, string>) => translate(lang, key, params);
  const layout = useLayout();

  // E.6: undefined until Go answers; null is no session.
  const [session, setSession] = useState<SessionView | null | undefined>(undefined);
  const [sessionFailed, setSessionFailed] = useState(false);
  // The code that refused the last new project (B3-18, B3-20).
  const [createError, setCreateError] = useState("");
  // The form the left panel opens on the home (U.2); the tick reopens it.
  const [homeForm, setHomeForm] = useState<{ form: HomeForm; tick: number }>({ form: "", tick: 0 });
  const [projects, setProjects] = useState<ProjectLine[]>([]);
  const [understandingKey, setUnderstandingKey] = useState(0);
  const [projectID, setProjectID] = useState("");
  // C.3: the project the address named on load, being checked; the home waits.
  const [resuming, setResuming] = useState(() => readProjectHash(window.location.hash) !== "");
  const [sections, setSections] = useState<SectionView[]>([]);
  const [selected, setSelected] = useState("");
  // The streamed turn, by field (P-28): triage line, reply, text, note
  // or understanding; the raw model text only in a devroutes build.
  // When the turn ends it is read back from the history (U-5).
  const [stream, setStream] = useState<StreamDelta[]>([]);
  const [rawStream, setRawStream] = useState("");
  const [conversation, setConversation] = useState<ConversationEntry[]>([]);
  // The sent request, shown by the request panel as your message (P-37).
  const [requestText, setRequestText] = useState("");
  const [errorCode, setErrorCode] = useState("");
  const [working, setWorking] = useState(false);
  // C.2: what the agent is doing, and how many things are yours (the frame's count).
  const [activity, setActivity] = useState<AgentActivity>(IDLE_ACTIVITY);
  const [todoCount, setTodoCount] = useState(0);
  // P-92: a defense after your answer is being prepared in the
  // background: the conversation says so and refreshes by itself.
  const [defensePreparing, setDefensePreparing] = useState(false);
  // Only one tab edits (backlog-r1 1.5): this tab holds the editor
  // lease, renewed every 15 s, or stays read-only.
  const [editor, setEditor] = useState(true);
  // «Editar aquí» (B-102): why taking the edit was refused.
  const [takeOver, setTakeOver] = useState<TakeOverResult>("");
  const [summary, setSummary] = useState<SummaryView | null>(null);
  const watchedTurn = useRef("");
  const activeProject = useRef("");
  const [signQueued, setSignQueued] = useState(false);
  const [historyBlockValue, setHistoryBlockValue] = useState(0);
  // A decision that covers no figure: its own history (P-41), opened
  // from the frame (backlog-r2 2.7). Showing a block clears it.
  const [historyDecision, setHistoryDecision] = useState("");
  const historyBlock = historyBlockValue;
  const setHistoryBlock = useCallback((block: number) => {
    setHistoryBlockValue(block);
    setHistoryDecision("");
  }, []);
  // The frame refetches when the understanding, the cards or the
  // sections change.
  const [frameTick, setFrameTick] = useState(0);
  const bumpFrame = useCallback(() => setFrameTick((k) => k + 1), []);
  // P7 (1.7): the signoff in progress takes the whole screen, without
  // the frame; what keeps a signoff from opening is listed here.
  const [signing, setSigning] = useState("");
  const [signBlocked, setSignBlocked] = useState<SignoffBlocked | null>(null);
  const [signError, setSignError] = useState("");
  const [undoError, setUndoError] = useState("");
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
    // B-102: a tab that goes frees the edit at once. A save fired on the same
    // event may claim it back for this tab, which then lapses by itself.
    const release = () => releaseEditor(projectID);
    window.addEventListener("pagehide", release);
    return () => {
      window.clearInterval(timer);
      window.removeEventListener("pagehide", release);
    };
  }, [projectID]);

  async function editHere() {
    setTakeOver("");
    try {
      const taken = await takeOverEditor(projectID);
      setEditor(taken);
      setTakeOver(takeOverResult(taken));
    } catch {
      setTakeOver("busy");
    }
  }

  activeProject.current = projectID;

  // B-83: with the mode on and nothing followed, the app may open a chain when a
  // holder that is no chain ends: ask until it does or the mode ends.
  // [EL DISEÑO NO LO DICE]: how the page learns it: the summary, every 3 s.
  // B-100, B-101: the same with a message or a decision of yours waiting, in the
  // normal mode: a turn may open with it, and its mark goes when it is taken.
  const modeOn = summary?.mode === "run_to_completion";
  const messageWaits = conversation.some((e) => isQueued(e) || (e.will_apply?.length ?? 0) > 0);
  useEffect(() => {
    if (!projectID || (!modeOn && !messageWaits) || working) return;
    const timer = window.setInterval(() => {
      void getSummary(projectID).then((s) => {
        if (activeProject.current === projectID) applySummary(s);
      });
      if (messageWaits) void refreshConversation(projectID);
    }, 3000);
    return () => window.clearInterval(timer);
  }, [projectID, modeOn, messageWaits, working]);

  useEffect(() => {
    if (!projectID) return;
    void getSummary(projectID).then((s) => {
      applySummary(s);
      if (s.defense_preparing) setDefensePreparing(true); // e.g. after a reload (P-92)
    });
  }, [projectID, understandingKey]);

  // C.2: the spend keeps up while a turn runs, also with no piece arriving.
  useEffect(() => {
    if (!projectID || !working) return;
    const timer = window.setInterval(() => refreshSummary(projectID), SUMMARY_POLL_MS);
    return () => window.clearInterval(timer);
  }, [projectID, working]);

  // P-92: while the defense is prepared, ask every few seconds (with a
  // cap) until it ends; leaving the project stops asking.
  useEffect(() => {
    if (!projectID || !defensePreparing) return;
    let live = true;
    void pollDefense({
      preparing: async () => (await getSummary(projectID)).defense_preparing,
      conversation: () => getConversation(projectID),
      onEntries: setConversation,
      sleep: (ms) => new Promise((resolve) => window.setTimeout(resolve, ms)),
      now: () => Date.now(),
      cancelled: () => !live,
      // P-95: the header shows the summary again, with the judge's cost.
      onEnd: () => void getSummary(projectID).then((s) => live && applySummary(s)),
    }).then(() => live && setDefensePreparing(false));
    return () => {
      live = false;
    };
  }, [projectID, defensePreparing]);

  const refreshConversation = useCallback(async (project: string) => {
    setConversation(await getConversation(project));
  }, []);

  useEffect(() => {
    if (projectID) void refreshConversation(projectID);
  }, [projectID, refreshConversation]);

  const refreshProjects = useCallback(async () => {
    setProjects(await listProjects());
  }, []);

  // The left panel's line counts sections and signed ones (U.2): it follows them,
  // whatever moved them (a section by hand, a signoff, an undo).
  const signedCount = sections.filter((s) => s.state === "signed").length;
  useEffect(() => {
    if (projectID) void refreshProjects();
  }, [projectID, sections.length, signedCount, refreshProjects]);

  // The session, and your projects only with one: a visitor with no session
  // asks for no list. It runs once on load and after anything that changes them.
  const refreshSession = useCallback(async () => {
    try {
      const next = await getSession();
      setSession(next);
      setProjects(next ? await listProjects() : []);
      setSessionFailed(false);
    } catch {
      setSessionFailed(true);
    }
  }, []);

  useEffect(() => {
    void refreshSession();
  }, [refreshSession]);

  async function newProject(template = "") {
    setCreateError("");
    let created;
    try {
      created = await createProject(template);
    } catch (e) {
      setCreateError(e instanceof Error ? e.message : "internal");
      await refreshSession();
      return;
    }
    setSections([]);
    setSelected("");
    setHistoryBlock(0);
    layout.setCenter("conv");
    clearStream();
    setConversation([]);
    setRequestText("");
    setDefensePreparing(false);
    setProjectID(created.project_id);
    await refreshSession();
  }

  function clearStream() {
    setStream([]);
    setRawStream("");
  }

  // «Objetar» and «Revocar» (E-1, B-82): the sections' states and notices moved,
  // and the frame, the cards and the bar read them again.
  function decisionActed() {
    void refreshSections(projectID);
    void refreshConversation(projectID); // the «por aclarar» it opened (U-5)
    bumpFrame();
    void getSummary(projectID).then(applySummary);
  }

  // Every summary read after an action: the bar, and a chain the app opened
  // (B-83), which the page attaches to here.
  function applySummary(s: SummaryView) {
    setSummary(s);
    const turn = turnToFollow(s, watchedTurn.current);
    if (turn) watchTurn(turn);
  }

  // C.2: the header's spend, read again; a failed read waits for the next one.
  function refreshSummary(project: string) {
    getSummary(project).then((s) => {
      if (activeProject.current === project) applySummary(s);
    }, () => undefined);
  }

  function watchTurn(turn_id: string) {
    // A turn already followed is not followed twice: its stream replays from
    // the start, and the text would show twice (B-1).
    if (watchedTurn.current === turn_id) return;
    watchedTurn.current = turn_id;
    setWorking(true);
    setActivity(IDLE_ACTIVITY);
    subscribeTurn(projectID, turn_id, {
      onDelta: (delta) => {
        setActivity((prev) => nextActivity(prev, delta));
        // It only names the section for the status line (C.2): never on screen.
        if (delta.field === "section_id") return;
        // P-93: a new step's triage line means the step before it was
        // applied: the selected section's editor takes its new text.
        if (startsStep(delta)) {
          void refreshSections(projectID);
          refreshSummary(projectID);
          void refreshConversation(projectID); // a queued message that entered (B-101)
        }
        // B-64: a round started: its card shows in the conversation.
        const round = roundStarted(delta);
        if (round) {
          void refreshConversation(projectID);
          refreshSummary(projectID);
          // A judge's round says «reviewing», a readers' round «reading» (C.2).
          void getRound(projectID, round).then(
            (card) => card.kind === "judge" && setActivity({ kind: "judging", section: "" }),
            () => undefined,
          );
          return;
        }
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
        setWorking(false);
        // The ended turn is now in the history: read it back and drop
        // the stream, so it does not show twice (U-5). The
        // understanding may have titled the project (U-1).
        void refreshConversation(projectID).then(clearStream);
        void refreshProjects();
        if (payload.next_turn_id) {
          // The agent's chain starts on its own after the understanding.
          setUnderstandingKey((k) => k + 1);
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

  async function sectionCreated(label: string) {
    if (!projectID) return;
    await refreshSections(projectID);
    bumpFrame();
    setSelected(label);
    // [EL DISEÑO NO LO DICE] A section you create shows at once, as it did beside
    // the conversation; B-95 does not name this destination.
    applyRoute("document");
  }

  // P-49: «subir» and «bajar» in the skeleton; Go keeps the order.
  async function reorder(labels: string[]) {
    if (!projectID) return;
    try {
      await reorderSections(projectID, labels);
    } finally {
      await refreshSections(projectID);
      bumpFrame();
    }
  }

  function detailsChanged() {
    if (!projectID) return;
    void refreshSections(projectID);
    bumpFrame();
  }

  function onSaved(label: string, version: number, text: string) {
    setSections((prev) =>
      prev.map((s) => (s.label === label ? { ...s, version, text } : s)),
    );
    bumpFrame(); // the first line may have changed (M-4)
    // Go's paragraphs of the saved text (P-78), for the next time the
    // editor opens this section.
    if (projectID) void refreshSections(projectID);
  }

  // 7a: only you ask to sign. The editor saves first (§2); Go opens the
  // signoff or lists what is pending.
  async function askToSign(section = current?.label) {
    if (!projectID || !section) return;
    await saveRef.current();
    setSignBlocked(null);
    setSignError("");
    try {
      const asked = projectID;
      let result = await requestSignoff(asked, section);
      // B-83: during the mode it queues and opens when the running step ends;
      // asking again reads what happened.
      // [EL DISEÑO NO LO DICE]: how the page learns it: it asks every 3 s.
      while (keepsAskingToSign(result, asked, activeProject.current)) {
        setSignQueued(true);
        await new Promise((resolve) => window.setTimeout(resolve, 3000));
        if (activeProject.current !== asked) break;
        result = await requestSignoff(asked, section);
      }
      setSignQueued(false);
      if (result.kind === "queued" || activeProject.current !== asked) return;
      if (result.kind === "blocked") setSignBlocked(result.blocked);
      else setSigning(result.signoff.label);
    } catch (e) {
      setSignQueued(false);
      setSignError(e instanceof Error ? e.message : "internal");
    }
  }

  // H-3: the mode in the conversation's header (B-83).
  async function setMode(mode: string, message: string) {
    if (!projectID) return;
    setErrorCode("");
    try {
      const { turn_id } = await putMode(projectID, mode, message);
      if (turn_id) {
        clearStream();
        watchTurn(turn_id);
      }
      applySummary(await getSummary(projectID));
    } catch (e) {
      setErrorCode(e instanceof Error ? e.message : "internal");
    }
  }

  function closeSignoff() {
    setSigning("");
    if (projectID) {
      void refreshSections(projectID);
      // Wonder's calls count in what was spent (P-61).
      void getSummary(projectID).then(applySummary);
    }
  }

  function openProject(id: string) {
    if (id !== projectID) {
      setSigning("");
      setSignBlocked(null);
      setSections([]);
      setSelected("");
      setHistoryBlock(0);
      clearStream();
      setConversation([]);
      setRequestText("");
      setErrorCode("");
      setDefensePreparing(false);
      setCreateError("");
      setTodoCount(0);
      layout.setCenter("conv");
      setProjectID(id);
    }
  }

  // Back to P1 (E.6): no project open.
  function closeProject() {
    openProject("");
    void refreshSession();
  }

  // C.3: the project the address names opens if it is yours; if not, the home,
  // with no error and the address clean.
  function openFromHash(id: string) {
    return getSummary(id).then(
      () => openProject(id),
      () => window.history.replaceState(null, "", window.location.pathname + window.location.search),
    );
  }

  useEffect(() => {
    const id = readProjectHash(window.location.hash);
    if (id) void openFromHash(id).finally(() => setResuming(false));
  }, []);

  // Opening and closing write the address; once resumed, so it is not wiped first.
  useEffect(() => {
    if (resuming) return;
    const next = projectHash(projectID);
    if (window.location.hash !== next) {
      window.history.pushState(null, "", window.location.pathname + window.location.search + next);
    }
  }, [projectID, resuming]);

  // The browser's back and forward, and an address edited by hand.
  useEffect(() => {
    const follow = () => {
      const id = readProjectHash(window.location.hash);
      if (id === projectID) return;
      if (id) void openFromHash(id);
      else closeProject();
    };
    window.addEventListener("hashchange", follow);
    return () => window.removeEventListener("hashchange", follow);
  }, [projectID]);

  // «Crear usuario» and «Entrar» from the left: the home, with that form open.
  function openHomeForm(form: HomeForm) {
    setHomeForm((prev) => ({ form, tick: prev.tick + 1 }));
    closeProject();
  }

  // The brand: the home, with no form open.
  function goHome() {
    openHomeForm("");
  }

  // «Salir» from the left: back to the home, as leaving from there. The project
  // closes first, so its panels stop reading, and the session is read once, after
  // the logout, so no read races it. A failure shows on the home, like a refused
  // project.
  async function leaveAccount() {
    setHomeForm((prev) => ({ form: "", tick: prev.tick + 1 }));
    openProject("");
    try {
      await logout();
    } catch {
      setCreateError("internal");
    }
    await refreshSession();
  }

  // «Te toca» leads to the card, the understanding or the signoff
  // button; the frame never opens from the signoff (§9).
  function reach(id: string) {
    applyRoute(id);
    // The view it lives in shows on the next frame.
    window.requestAnimationFrame(() => {
      const el = document.getElementById(id);
      if (!el) return;
      el.scrollIntoView({ block: "center", behavior: "smooth" });
      el.classList.add("flash");
      window.setTimeout(() => el.classList.remove("flash"), 900);
    });
  }

  // Each destination's panels (B-95, B-96).
  function applyRoute(id: string) {
    const route = routeTarget(id);
    if (route.center) {
      layout.setCenter(route.center);
      if (layout.narrow) layout.foldOverlay();
    }
    if (route.right) {
      layout.setRightTab(route.right);
      layout.setRightOff(false);
    }
  }

  // A click on a paragraph: its history, in the right panel (B-96).
  function openBlock(block: number) {
    setHistoryBlock(block);
    applyRoute("history");
  }

  // «Deshacer» (B-79): what you typed saves first, so it is your last edit; the
  // card or the alert names its edit, and Go refuses it under a later one. An
  // undo can close a signoff, which can reopen the mode's chain (B-86).
  async function undoLastEdit(section: string, editVersion?: number) {
    if (!projectID) return;
    setUndoError("");
    try {
      await saveRef.current();
      await undoEdit(projectID, section, editVersion);
    } catch (e) {
      setUndoError(e instanceof Error ? e.message : "internal");
    }
    setWarningsTick((k) => k + 1);
    bumpFrame();
    await refreshSections(projectID);
    applySummary(await getSummary(projectID));
  }

  // A warning leads to its «por aclarar», or a flaw's to its judge round.
  function reachWarning(target: string) {
    reach(warnings.some((w) => w.kind === "flaw" && w.target === target) ? `round-${target}` : `oq-${target}`);
  }

  // Accepting or discarding can change a section, the frame and the sections.
  // A new step's turn goes to watchTurn, like every turn the page shows (B-75).
  function proposalActed(turn: string) {
    bumpFrame();
    if (projectID) void refreshSections(projectID);
    if (turn) watchTurn(turn);
  }

  function onTodo(target: TodoTarget) {
    if (target.kind === "card") reach(`card-${target.label}`);
    else if (target.kind === "open_question") reach(`oq-${target.label}`);
    else if (target.kind === "flaws") reach(`round-${target.round}`);
    else if (target.kind === "cases") reach("understanding");
    else if (target.kind === "stop_report") reach("stop-report");
    else if (target.kind === "proposals") {
      if (target.section) {
        setSelected(target.section);
        setHistoryBlock(0);
        window.setTimeout(() => reach("proposals"), 0);
      } else reach("proposed-sections");
    } else {
      setSelected(target.section);
      setHistoryBlock(0);
      window.setTimeout(() => reach("sign-section"), 0);
    }
  }

  // P-41 from the frame (backlog-r2 2.7): the decision's own history,
  // in the right panel, with the section it is listed under.
  function openDecisionHistory(label: string, decision: string) {
    if (label !== selected) setSignBlocked(null);
    setSelected(label);
    setHistoryBlock(0);
    setHistoryDecision(decision);
    window.setTimeout(() => reach("history"), 0);
  }

  // U-3: the frame is the one navigation between sections.
  function openSection(label: string, block = 0) {
    if (label !== selected) setSignBlocked(null);
    setSelected(label);
    setHistoryBlock(block);
    window.setTimeout(() => reach(block > 0 ? "history" : "document"), 0);
  }

  // P6 (backlog-r2 2.5): «Firmar» opens P7 for the report's section,
  // in the document, like the section's own button (B-95).
  function signFromReport(section: string) {
    setSelected(section);
    setHistoryBlock(0);
    applyRoute("sign-section");
    void askToSign(section);
  }

  // fromReport: «Sigue» and «Abordémoslo» go through the report's route,
  // as messages of yours with the button's text.
  // It resolves whether the message went out (P-94: the input keeps
  // your text otherwise).
  async function send(text: string, fromReport = false): Promise<boolean> {
    if (!projectID) return false;
    await saveRef.current(); // always save before sending (§2)
    // With a turn running the message queues, and its stream stays on screen (B-83, B-101).
    if (!working) clearStream();
    setErrorCode("");
    let turn_id: string;
    let queued: boolean | undefined;
    try {
      ({ turn_id, queued } = await (fromReport ? continueFromStopReport : postMessage)(projectID, text));
    } catch (e) {
      // [EL DISEÑO NO LO DICE] 409 turn_running: a turn of this
      // project is already running; the message was not sent. With the
      // defense line in view, it is the defense's (P-94).
      setErrorCode(sendErrorCode(e, defensePreparing));
      return false;
    }
    void refreshConversation(projectID); // your message, from the history
    if (!queued) watchTurn(turn_id);
    return true;
  }

  // «Buscar evidencia» (B-72) and «Buscarla» on a candidate (B-69): you
  // start a round; it runs as a turn of its own, followed like a message's.
  async function searchEvidence(references?: number[]) {
    if (!projectID) return;
    setErrorCode("");
    try {
      const { turn_id } = await startRound(projectID, references);
      void refreshConversation(projectID);
      watchTurn(turn_id);
    } catch (e) {
      setErrorCode(roundErrorCode(e, defensePreparing));
    }
  }

  // «Auditar sección» (backlog-r3 3.6, B-67 b): the judge's round runs as a
  // turn of its own, followed like a message's.
  async function auditCurrent(section: string) {
    if (!projectID) return;
    setErrorCode("");
    try {
      await saveRef.current();
      const { turn_id } = await auditSection(projectID, section);
      void refreshConversation(projectID);
      watchTurn(turn_id);
    } catch (e) {
      setErrorCode(auditErrorCode(e, defensePreparing));
    }
  }

  const current = sections.find((s) => s.label === selected);

  // Avisos en el texto (backlog-r2 2.9): the chosen section's warnings,
  // computed and placed by Go, refetched when the section moves.
  const [warnings, setWarnings] = useState<Warning[]>([]);
  const [warningsTick, setWarningsTick] = useState(0);
  // «Deshacer» and the band of suspicious edits (B-79, B-80), from the same read.
  const [undo, setUndo] = useState({ available: false, reason: "" });
  const [alerts, setAlerts] = useState<EditAlert[]>([]);
  useEffect(() => {
    if (!projectID || !current) {
      setWarnings([]);
      setUndo({ available: false, reason: "" });
      setAlerts([]);
      return;
    }
    let live = true;
    getSection(projectID, current.label)
      .then((detail) => {
        if (!live) return;
        setWarnings(detail.warnings ?? []);
        setUndo({ available: detail.undo_available, reason: detail.undo_reason });
        setAlerts((detail.alerts ?? []) as EditAlert[]);
      })
      .catch(() => {
        if (!live) return;
        setWarnings([]);
        setUndo({ available: false, reason: "" });
        setAlerts([]);
      });
    return () => {
      live = false;
    };
  }, [projectID, current?.label, current?.version, current?.state, current?.pendings, frameTick, warningsTick]);
  const currentProject = projects.find((p) => p.project_id === projectID);
  // Never an internal id on screen (P-30): the name, or the catalog's.
  const projectName = (p?: ProjectLine) => p?.name || t("project.untitled");
  // Never a label on screen (P-30, P-36): the title, or the catalog's.
  const sectionName = (s: SectionView) => s.title || t("section.untitled");

  const hasDocument = sections.length > 0;
  const centerShown = centerView(layout.center, hasDocument);
  const historyOpen = Boolean(current && (historyBlock > 0 || historyDecision));
  const rightTab = effectiveTab(layout.rightTab, historyOpen);

  // C.2: what the agent does, what is yours, or nothing. A section the stream names
  // shows by its title, never its label.
  const status = statusLine({
    working,
    defensePreparing,
    activity,
    sectionTitle: (label) => sections.find((s) => s.label === label)?.title ?? "",
    todo: todoCount,
  });

  const langSwitch = (
    <div className="lang-switch" role="group" aria-label="language">
      <button type="button" className={lang === "en" ? "active" : ""} onClick={() => setLang("en")}>
        {t("language.en")}
      </button>
      <button type="button" className={lang === "es" ? "active" : ""} onClick={() => setLang("es")}>
        {t("language.es")}
      </button>
    </div>
  );

  // P7: the signoff takes the whole screen, without the frame or the text.
  if (projectID && signing) {
    return (
      <main className="app">
        <header className="app-header">
          <div className="brand">
            <h1>{t("app.title")}</h1>
            <ProjectTitle
              key={projectID}
              lang={lang}
              projectID={projectID}
              name={currentProject?.name ?? ""}
              onRenamed={() => void refreshProjects()}
            />
          </div>
          {langSwitch}
        </header>
        <SignoffPanel lang={lang} projectID={projectID} label={signing} onClose={closeSignoff} />
      </main>
    );
  }

  const left = (
    <LeftPanel
      lang={lang}
      off={layout.leftOff}
      onToggle={() => layout.setLeftOff(!layout.leftOff)}
      projects={projects}
      projectID={projectID}
      onOpen={openProject}
      onNew={(template) => void newProject(template)}
      session={session ?? null}
      onHome={goHome}
      onForm={openHomeForm}
      onLogout={() => void leaveAccount()}
      onDownload={() => {
        layout.setRightTab("markdown");
        layout.setRightOff(false);
      }}
      langSwitch={langSwitch}
    />
  );

  if (!projectID) {
    return (
      <Shell
        lang={lang}
        layout={layout}
        left={left}
        right={null}
        center={
          <div className="center-home">
            {(session !== undefined || sessionFailed) && !resuming && (
              <HomePage
                key={homeForm.tick}
                openForm={homeForm.form}
                lang={lang}
                session={session ?? null}
                sessionFailed={sessionFailed}
                projects={projects}
                createError={createError}
                onOpen={openProject}
                onNew={(template) => void newProject(template)}
                onSessionChanged={async () => {
                  setCreateError("");
                  await refreshSession();
                }}
              />
            )}
          </div>
        }
      />
    );
  }

  return (
    <Shell
      lang={lang}
      layout={layout}
      left={left}
      center={
        <>
          <CenterHeader
            lang={lang}
            view={centerShown}
            hasDocument={hasDocument}
            conversationNews={working || errorCode !== ""}
            onView={layout.setCenter}
            title={
              <ProjectTitle
                key={projectID}
                lang={lang}
                projectID={projectID}
                name={currentProject?.name ?? ""}
                onRenamed={() => void refreshProjects()}
              />
            }
            mode={
              summary && (
                <ModeBar lang={lang} mode={summary.mode} waitsSignoff={summary.mode_waits_signoff} disabled={!editor} onSet={setMode} />
              )
            }
            model={summary && <ModelLine lang={lang} summary={summary} />}
            status={
              status && (
                <p className="meta status-line" role="status">
                  {t(status.key, status.params)}
                </p>
              )
            }
            onToggleRight={() => layout.setRightOff(!layout.rightOff)}
          />
          {/* In both views: at a glance, it says why the document does not let you edit. */}
          {!editor && (
            <p className="turn-error center-notice">
              {t("tab.read_only")}{" "}
              <button type="button" className="small" onClick={() => void editHere()}>
                {t("tab.edit_here")}
              </button>
              {takeOver === "busy" && <> {t("tab.edit_here_busy")}</>}
            </p>
          )}
          {createErrorKey(createError, null) && (
            <p className="turn-error center-notice">{t(createErrorKey(createError, null) ?? "")}</p>
          )}
          <div className="center-body">
            <section className={`center-view${centerShown === "conv" ? "" : " off"}`} role="tabpanel" aria-label={t("chat.title")}>
              <div className="view-inner">
                {projectID && (
                  <RequestPanel
                    key={`request-${projectID}`}
                    lang={lang}
                    projectID={projectID}
                    onRequest={setRequestText}
                    onSubmitted={(turnID) => {
                      clearStream();
                      setErrorCode("");
                      watchTurn(turnID);
                    }}
                  />
                )}

                {projectID && (
                  <UnderstandingPanel
                    key={`understanding-${projectID}`}
                    lang={lang}
                    projectID={projectID}
                    refreshKey={understandingKey}
                    onRoleChanged={() => {
                      void refreshSections(projectID);
                      setUnderstandingKey((k) => k + 1); // a role can open a card (K-13)
                    }}
                    onChanged={bumpFrame}
                    onSearchEvidence={editor ? () => void searchEvidence() : undefined}
                  />
                )}

                {projectID && (
                  <ConversationPanel
                    lang={lang}
                    projectID={projectID}
                    entries={conversation}
                    requestText={requestText}
                    working={working}
                    stream={stream}
                    rawStream={rawStream}
                    errorCode={errorCode}
                    defensePreparing={defensePreparing}
                    onRoundAct={(turnID) => {
                      // Backlog-r3 3.4: a repeated round shows its card and is
                      // followed; a round left to clarify shows in the cards.
                      void refreshConversation(projectID);
                      bumpFrame();
                      if (turnID) watchTurn(turnID);
                    }}
                    disabled={!editor || working}
                    readOnly={!editor}
                    onQueueChanged={() => void refreshConversation(projectID)}
                    onAnswerAudience={(label, answer) => {
                      // P-86: the question keeps showing with its new state.
                      void answerAudienceQuestion(projectID, label, answer)
                        .catch(() => undefined)
                        .then(() => refreshConversation(projectID));
                    }}
                    report={
                      <StopReportPanel
                        lang={lang}
                        projectID={projectID}
                        refreshKey={conversation.length}
                        disabled={!editor || working}
                        onContinue={(text) => void send(text, true)}
                        onSign={signFromReport}
                        onSeen={bumpFrame}
                      />
                    }
                  />
                )}

                {projectID && (
                  <CardPanel
                    lang={lang}
                    projectID={projectID}
                    refreshKey={`${understandingKey}-${frameTick}`}
                    onAnswered={(defense, turnID) => {
                      void refreshSections(projectID);
                      void refreshConversation(projectID); // the answered card (U-5)
                      if (defense) setDefensePreparing(true); // P-92
                      if (turnID) {
                        // B-100: the one step that applies your decision.
                        clearStream();
                        setErrorCode("");
                        watchTurn(turnID);
                      }
                    }}
                    onUndo={undoLastEdit}
                    onDecisionActed={editor ? decisionActed : undefined}
                  />
                )}
              </div>
            </section>

            {hasDocument && (
              <section className={`center-view${centerShown === "doc" ? "" : " off"}`} role="tabpanel" aria-label={t("doc.title")} id="document">
                {/* U-3: the frame chooses the section; this view shows only
                    that one: its title, state, pendings and «Sign section». */}
                <div className="view-inner">
                <div className="hd doc-toolbar">
                  {current ? (
                    <span className="doc-state">
                      <span className={`st ${current.state === "signed" ? "signed" : current.pendings > 0 ? "todo" : ""}`}>
                        {t(`section.status.${current.state}`)}
                      </span>
                      {current.pendings > 0 && (
                        <span className="st todo">{t("section.pendings", { n: String(current.pendings) })}</span>
                      )}
                    </span>
                  ) : (
                    <span className="lbl">{t("doc.title")}</span>
                  )}
                  <button
                    type="button"
                    className="small"
                    disabled={!current || !editor || working}
                    onClick={() => current && void auditCurrent(current.label)}
                  >
                    {t("doc.audit")}
                  </button>
                  <button
                    type="button"
                    className="small"
                    disabled={!current || !editor || !undo.available}
                    title={undo.reason ? t(undoReasonKey(undo.reason)) : undefined}
                    onClick={() => current && void undoLastEdit(current.label)}
                  >
                    {t("undo.button")}
                  </button>
                  <button
                    type="button"
                    id="sign-section"
                    className="primary small"
                    disabled={!current || !editor || current.state === "signed"}
                    onClick={() => void askToSign()}
                  >
                    {t("doc.sign")}
                  </button>
                </div>
                  {undoLineKey(undo.available, undo.reason) && <p className="meta">{t(undoLineKey(undo.available, undo.reason))}</p>}
                  {undoError && <p className="turn-error">{t(undoReasonKey(undoError))}</p>}
                  {signQueued && <p className="meta" role="status">{t("signoff.request.queued")}</p>}
                  {signBlocked && (
                    <div className="sign-blocked" role="status">
                      <b>{t("signoff.blocked")}</b>
                      <ul>
                        {signBlocked.pendings?.map((p, i) => (
                          <li key={i}>
                            {t(`pending.${p.kind}`)}
                            {p.about && <> · «{p.about}»</>}
                          </li>
                        ))}
                        {signBlocked.open_changes?.map((c) => (
                          <li key={c}>{t("signoff.open_change")}</li>
                        ))}
                      </ul>
                      <button type="button" className="small" onClick={() => setSignBlocked(null)}>
                        {t("history.close")}
                      </button>
                    </div>
                  )}
                  {signError && (
                    <p className="turn-error">
                      {/* P-67: asking to sign while the agent works waits for the checker on your edits. */}
                      {signError === "turn_running" ? t("signoff.request.turn_running") : t(`signoff.error.${signError}`)}
                    </p>
                  )}
                  {current && (
                    <SectionEditor
                      key={current.label}
                      lang={lang}
                      projectID={projectID}
                      label={current.label}
                      heading={
                        <SectionDetails
                          key={`details-${current.label}`}
                          lang={lang}
                          projectID={projectID}
                          label={current.label}
                          title={current.title}
                          focus={current.focus}
                          readOnly={!editor}
                          onChanged={detailsChanged}
                        />
                      }
                      band={
                        <>
                          <EditAlerts
                            lang={lang}
                            projectID={projectID}
                            alerts={alerts}
                            readOnly={!editor}
                            onUndo={(version) => void undoLastEdit(current.label, version)}
                            onAnswered={() => setWarningsTick((k) => k + 1)}
                          />
                          <SectionWarnings
                            lang={lang}
                            projectID={projectID}
                            warnings={warnings}
                            readOnly={!editor}
                            onClosed={() => {
                              // Closing a change can free the section (A-1).
                              setWarningsTick((k) => k + 1);
                              bumpFrame();
                              void refreshSections(projectID);
                            }}
                            onQuestion={reachWarning}
                            onHistory={(block) => openSection(current.label, block)}
                          />
                        </>
                      }
                      underlines={warnings.filter(
                        (w) => w.kind === "open_question" || w.kind === "agent_pending" || w.kind === "flaw",
                      )}
                      onQuestion={reachWarning}
                      readOnly={!editor}
                      onBlock={openBlock}
                      origins={current.origins ?? []}
                      version={current.version}
                      text={current.text}
                      paragraphs={current.paragraphs}
                      onSaved={onSaved}
                      registerSave={registerSave}
                    />
                  )}
                  {current && (
                    <div id="proposals">
                      <ProposalPanel
                        lang={lang}
                        projectID={projectID}
                        section={current.label}
                        refreshKey={`${frameTick}-${current.version}`}
                        readOnly={!editor}
                        onChanged={proposalActed}
                      />
                    </div>
                  )}
                  <p className="hint">{t("doc.paragraph_hint")}</p>

                  <div id="proposed-sections">
                    <ProposalPanel
                      lang={lang}
                      projectID={projectID}
                      refreshKey={String(frameTick)}
                      readOnly={!editor}
                      onChanged={proposalActed}
                    />
                  </div>

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
          <div className="composer">
            <div className="composer-box">
              <MessageInput lang={lang} disabled={!projectID || !editor} onSend={send} />
              <span className="hint">{t("chat.decisions_hint")}</span>
            </div>
          </div>
        </>
      }
      right={
        <RightPanel
          lang={lang}
          tab={rightTab}
          historyOpen={historyOpen}
          onTab={layout.setRightTab}
          onClose={() => layout.setRightOff(true)}
          frame={
            <div className="frame-pane">
              <FramePanel
                lang={lang}
                projectID={projectID}
                refreshKey={`${understandingKey}-${frameTick}`}
                selected={selected}
                onSection={(label) => openSection(label)}
                onHistory={(label, block) => openSection(label, block)}
                onDecisionHistory={openDecisionHistory}
                onTodo={onTodo}
                onReorder={editor ? (labels) => void reorder(labels) : undefined}
                onSearchCandidate={editor ? (n) => void searchEvidence([n]) : undefined}
                onDecisionActed={editor ? decisionActed : undefined}
                onTodoCount={setTodoCount}
              />
              <NewSectionForm
                key={projectID}
                lang={lang}
                projectID={projectID}
                disabled={!editor}
                onCreated={(label) => void sectionCreated(label)}
              />
            </div>
          }
          markdown={
            <MarkdownPanel
              key={projectID}
              lang={lang}
              projectID={projectID}
              active={rightTab === "markdown" && !layout.rightOff}
              refreshKey={`${frameTick}-${understandingKey}`}
            />
          }
          history={
            current && (
              <div id="history">
                <HistoryPanel
                  lang={lang}
                  projectID={projectID}
                  label={current.label}
                  name={sectionName(current)}
                  block={historyBlock}
                  decision={historyDecision || undefined}
                  onClose={() => setHistoryBlock(0)}
                  onSearchCandidate={editor ? (n) => void searchEvidence([n]) : undefined}
                  onMarked={() => {
                    // Marking can open a card and change the frame (T-5).
                    bumpFrame();
                    void refreshSections(projectID);
                  }}
                />
              </div>
            )
          }
        />
      }
    />
  );
}
