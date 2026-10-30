import type {
  AnswerCardResponse,
  BlockHistoryView,
  DecisionHistoryView,
  CaseView,
  ConversationEntry,
  EditorResponse,
  CreateProjectResponse,
  PostMessageResponse,
  ProjectView,
  RequestView,
  SectionResponse,
  SectionRoleView,
  SectionView,
  SignoffBlocked,
  SourceView,
  SummaryView,
  TodoView,
  UnderstandingView,
} from "./types";
import type { Frame, SignoffView } from "./graph";

// Thin API client. It only transports what Go computes (AGENTS.md
// rule 5): no state, fingerprint or validity is decided here.
async function json<T>(res: Response): Promise<T> {
  if (!res.ok) {
    throw new Error(`http_${res.status}`);
  }
  return (await res.json()) as T;
}

export async function createProject(template = ""): Promise<CreateProjectResponse> {
  return json(
    await fetch("/api/projects", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ name: "", template }),
    }),
  );
}

export async function listProjects(): Promise<ProjectView[]> {
  return json(await fetch("/api/projects"));
}

// U-1: you rename the project the understanding titled; Go refuses a
// blank name (400 invalid_name, project.error.<code> in the catalog).
export async function renameProject(projectID: string, name: string): Promise<void> {
  const res = await fetch(`/api/projects/${projectID}/name`, {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ name }),
  });
  if (!res.ok) {
    const body = (await res.json().catch(() => ({}))) as { error?: string };
    throw new Error(body.error ?? "internal");
  }
}

// U-5: the whole conversation, oldest first, read from the history.
export async function getConversation(projectID: string): Promise<ConversationEntry[]> {
  return json(await fetch(`/api/projects/${projectID}/conversation`));
}

export async function getRequest(projectID: string): Promise<RequestView> {
  return json(await fetch(`/api/projects/${projectID}/request`));
}

export async function putRequest(
  projectID: string,
  req: { text: string; audience: string; criteria: string[]; premise: string; exploratory: boolean },
): Promise<RequestView> {
  return json(
    await fetch(`/api/projects/${projectID}/request`, {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(req),
    }),
  );
}

// Uploads one source; on failure throws the catalog code the API
// returned (source.error.<code> in the i18n catalog).
export async function uploadSource(
  projectID: string,
  kind: string,
  file: File,
): Promise<SourceView> {
  const form = new FormData();
  form.set("kind", kind);
  form.set("file", file);
  const res = await fetch(`/api/projects/${projectID}/sources`, { method: "POST", body: form });
  if (!res.ok) {
    const body = (await res.json().catch(() => ({}))) as { error?: string };
    throw new Error(body.error ?? "internal");
  }
  return (await res.json()) as SourceView;
}

export async function getUnderstanding(projectID: string): Promise<UnderstandingView> {
  return json(await fetch(`/api/projects/${projectID}/understanding`));
}

export async function confirmCase(projectID: string, label: string): Promise<CaseView> {
  return json(
    await fetch(`/api/projects/${projectID}/cases/${label}/confirm`, { method: "POST" }),
  );
}

export async function correctCase(
  projectID: string,
  label: string,
  text: string,
): Promise<CaseView> {
  return json(
    await fetch(`/api/projects/${projectID}/cases/${label}/correct`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ text }),
    }),
  );
}

// P4 (backlog-r1 1.4): what is yours to do, and answering a card. Go
// decides whether the «porque» counts (422 invalid_reason).
export async function getTodo(projectID: string): Promise<TodoView> {
  return json(await fetch(`/api/projects/${projectID}/todo`));
}

export async function answerCard(
  projectID: string,
  label: string,
  option: "keep" | "other",
  reason: string,
): Promise<AnswerCardResponse> {
  return json(
    await fetch(`/api/projects/${projectID}/cards/${label}/answer`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ option, reason }),
    }),
  );
}

export async function putSectionRole(
  projectID: string,
  label: string,
  role: string,
): Promise<SectionRoleView> {
  return json(
    await fetch(`/api/projects/${projectID}/sections/${label}/role`, {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ role }),
    }),
  );
}

export async function submitRequest(projectID: string): Promise<PostMessageResponse> {
  const res = await fetch(`/api/projects/${projectID}/request/submit`, { method: "POST" });
  if (!res.ok) {
    const body = (await res.json().catch(() => ({}))) as { error?: string };
    throw new Error(body.error ?? "internal");
  }
  return (await res.json()) as PostMessageResponse;
}

// The error code Go answers (title_required, focus_too_long…), or
// "internal".
async function errorCode(res: Response): Promise<string> {
  const body = (await res.json().catch(() => ({}))) as { error?: string };
  return body.error ?? "internal";
}

// «+ Sección» (K-17): role is "conclusion" with the «aquí se concluye»
// mark, or empty and the principal proposes it.
export async function createSection(
  projectID: string,
  title: string,
  role: "" | "conclusion",
  focus: string,
): Promise<SectionResponse> {
  const res = await fetch(`/api/projects/${projectID}/sections`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ tab_id: TAB_ID, title, role, focus }),
  });
  if (!res.ok) throw new Error(await errorCode(res));
  return (await res.json()) as SectionResponse;
}

// P-49: the title and «qué debe abordar» of any section; a field left
// undefined stays as it is.
export async function changeSectionDetails(
  projectID: string,
  label: string,
  details: { title?: string; focus?: string },
): Promise<void> {
  const res = await fetch(`/api/projects/${projectID}/sections/${label}/details`, {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ tab_id: TAB_ID, ...details }),
  });
  if (!res.ok) throw new Error(await errorCode(res));
}

// P-49: «subir» and «bajar» send every section's label in the new order.
export async function reorderSections(projectID: string, labels: string[]): Promise<void> {
  const res = await fetch(`/api/projects/${projectID}/sections/order`, {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ tab_id: TAB_ID, labels }),
  });
  if (!res.ok) throw new Error(await errorCode(res));
}

export async function listSections(projectID: string): Promise<SectionView[]> {
  return json(await fetch(`/api/projects/${projectID}/sections`));
}

export type SaveResult =
  | { kind: "saved"; version: number }
  | { kind: "unchanged" }
  | { kind: "conflict" }
  | { kind: "error" };

// PUT one section text. 200 → new version, 204 → no change (B-3),
// 409 → the base version is stale (specification §2).
// TAB_ID names this tab for the editor lease: only one tab edits
// (specification §2, backlog-r1 1.5).
export const TAB_ID =
  typeof crypto !== "undefined" && "randomUUID" in crypto
    ? crypto.randomUUID()
    : `${Date.now()}-${Math.random()}`;

// claimEditor claims or renews this tab's editor lease; false means the
// tab is read-only.
export async function claimEditor(projectID: string): Promise<boolean> {
  const res = await json<EditorResponse>(
    await fetch(`/api/projects/${projectID}/editor`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ tab_id: TAB_ID }),
    }),
  );
  return res.editor;
}

// P5 (backlog-r1 1.6): the history of one block; opening it records
// decision_seen (history) in Go.
export async function getBlockHistory(projectID: string, label: string, block: number): Promise<BlockHistoryView> {
  return json(await fetch(`/api/projects/${projectID}/sections/${label}/history?block=${block}`));
}

// P-41 (30-oct): the history of a decision with no point in the text,
// opened from the signoff; it also records decision_seen (history).
export async function getDecisionHistory(projectID: string, decision: string): Promise<DecisionHistoryView> {
  return json(await fetch(`/api/projects/${projectID}/decisions/${decision}/history`));
}

// The frame (backlog-r1 1.5b, specification §9): computed in Go, read
// only. Expanding a decision records decision_seen (frame) in Go.
export async function getFrame(projectID: string): Promise<Frame> {
  return json(await fetch(`/api/projects/${projectID}/frame`));
}

export async function markDecisionSeen(projectID: string, label: string): Promise<void> {
  const res = await fetch(`/api/projects/${projectID}/frame/decisions/${label}/seen`, { method: "POST" });
  if (!res.ok) throw new Error(`http_${res.status}`);
}

// P7, the signoff (backlog-r1 1.7). Go decides everything: whether it
// opens, what is missing, whether it coincides and can be signed. On a
// refusal these throw the API's error code (signoff.error.<code> in
// the i18n catalog).
async function signoffJSON<T>(res: Response): Promise<T> {
  if (!res.ok) {
    const body = (await res.json().catch(() => ({}))) as { error?: string };
    throw new Error(body.error ?? "internal");
  }
  return (await res.json()) as T;
}

function post(path: string, body?: unknown): Promise<Response> {
  return fetch(path, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: body === undefined ? undefined : JSON.stringify(body),
  });
}

export type RequestSignoffResult =
  | { kind: "opened"; signoff: SignoffView }
  | { kind: "blocked"; blocked: SignoffBlocked };

// 7a: asking to sign opens the signoff, or lists what keeps it closed.
export async function requestSignoff(projectID: string, section: string): Promise<RequestSignoffResult> {
  const res = await post(`/api/projects/${projectID}/sections/${section}/signoff`);
  if (res.status === 409) {
    const blocked = (await res.json()) as SignoffBlocked;
    if (blocked.error === "pendings") return { kind: "blocked", blocked };
    throw new Error(blocked.error ?? "internal");
  }
  return { kind: "opened", signoff: await signoffJSON<SignoffView>(res) };
}

export async function getSignoff(projectID: string, label: string): Promise<SignoffView> {
  return signoffJSON(await fetch(`/api/projects/${projectID}/signoffs/${label}`));
}

export async function postSignoffExplanation(projectID: string, label: string, text: string): Promise<PostMessageResponse> {
  return signoffJSON(await post(`/api/projects/${projectID}/signoffs/${label}/explanation`, { text }));
}

export async function postSignoffAnswers(
  projectID: string,
  label: string,
  originAnswer: string,
  caseAnswer: string,
): Promise<PostMessageResponse> {
  return signoffJSON(
    await post(`/api/projects/${projectID}/signoffs/${label}/answers`, {
      origin_answer: originAnswer,
      case_answer: caseAnswer,
    }),
  );
}

export async function signSignoff(projectID: string, label: string): Promise<SignoffView> {
  return signoffJSON(await post(`/api/projects/${projectID}/signoffs/${label}/sign`));
}

export async function leaveSignoff(projectID: string, label: string): Promise<SignoffView> {
  return signoffJSON(await post(`/api/projects/${projectID}/signoffs/${label}/leave`));
}

// «Preguntar al agente»: an answer without new text (B-14).
export async function askFromSignoff(projectID: string, label: string, text: string): Promise<PostMessageResponse> {
  return signoffJSON(await post(`/api/projects/${projectID}/signoffs/${label}/ask`, { text }));
}

export async function getSummary(projectID: string): Promise<SummaryView> {
  return json(await fetch(`/api/projects/${projectID}/summary`));
}

export async function saveSection(
  projectID: string,
  label: string,
  baseVersion: number,
  text: string,
): Promise<SaveResult> {
  const res = await fetch(`/api/projects/${projectID}/sections/${label}`, {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ tab_id: TAB_ID, base_version: baseVersion, text }),
  });
  if (res.status === 204) return { kind: "unchanged" };
  if (res.status === 409) return { kind: "conflict" };
  if (!res.ok) return { kind: "error" };
  const body = (await res.json()) as SectionResponse;
  return { kind: "saved", version: body.version };
}

export async function postMessage(
  projectID: string,
  text: string,
): Promise<PostMessageResponse> {
  return json(
    await fetch(`/api/projects/${projectID}/messages`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ text }),
    }),
  );
}

// StepPayload is the step event the stream forwards (cycle runner).
export interface StepPayload {
  status: "accepted" | "retry" | "agent_pending";
  errors?: string[];
  triage_line?: string;
  // What the agent answers you (R-5), shown in the conversation.
  reply?: string;
  closing_risk?: string;
  note?: string;
  deferred?: { what: string; why: string }[];
  // The understanding's step names the chain that starts after it (B-17).
  understood?: string;
  next_turn_id?: string;
  // Why the agent's chain stopped (P-43), computed in Go.
  stop_reason?: "no_work" | "agent_pending" | "step_limit";
  // A Wonder round of the signoff names its signoff and outcome (1.7).
  signoff?: string;
  outcome?: string;
}

// StreamDelta is one streamed piece (P-28): Go sends only the parsed
// triage line, reply, text, note or understanding, each with its field; a
// development build (-tags=devroutes) also sends the raw model text as
// field "raw".
export interface StreamDelta {
  text: string;
  field?: string;
  start?: boolean;
}

export interface TurnEvents {
  onDelta: (delta: StreamDelta) => void;
  onStep: (step: StepPayload) => void;
  onError: (code: string) => void;
}

// Subscribe to a turn's SSE stream: deltas as they arrive, then the
// step or an error code (never a raw provider error, §6).
export function subscribeTurn(
  projectID: string,
  turnID: string,
  events: TurnEvents,
): () => void {
  const source = new EventSource(
    `/api/projects/${projectID}/turns/${turnID}/stream`,
  );
  source.addEventListener("delta", (e) => {
    const data = JSON.parse((e as MessageEvent).data) as StreamDelta;
    if (data.text) events.onDelta(data);
  });
  source.addEventListener("step", (e) => {
    events.onStep(JSON.parse((e as MessageEvent).data) as StepPayload);
    source.close();
  });
  source.addEventListener("error", (e) => {
    const raw = (e as MessageEvent).data;
    if (raw) {
      const data = JSON.parse(raw) as { code?: string };
      events.onError(data.code ?? "turn_failed");
      source.close();
    }
  });
  return () => source.close();
}
