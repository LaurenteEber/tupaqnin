import type {
  RoundCard,
  StartRoundResponse,
  RepeatRoundResponse,
  LeaveRoundOpenResponse,
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
  AnswerOpenQuestionResponse,
  UnderstandingView,
  EvidenceRequiredView,
  StopReportView,
  AnswerAudienceQuestionResponse,
  CriterionView,
  SectionDetail,
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

// Throws the catalog code the API returned (source.error.<code>).
async function coded<T>(res: Response): Promise<T> {
  if (!res.ok) {
    const body = (await res.json().catch(() => ({}))) as { error?: string };
    throw new Error(body.error ?? "internal");
  }
  return (await res.json()) as T;
}

// Backlog-r2 2.8: after the request is sent, the audience changes with
// audience_changed and no section's state moves (K-13).
export async function putAudience(projectID: string, audience: string): Promise<RequestView> {
  return coded(
    await fetch(`/api/projects/${projectID}/request/audience`, {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ audience }),
    }),
  );
}

// Backlog-r2 2.8, B-46: a criterion's new version; Go queues its probe
// against what is written.
export async function putCriterion(projectID: string, label: string, text: string): Promise<CriterionView> {
  return coded(
    await fetch(`/api/projects/${projectID}/criteria/${label}`, {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ text }),
    }),
  );
}

// Backlog-r2 2.8, B-47: the document as .md, built in Go; the browser
// downloads it (Content-Disposition: attachment).
export function exportURL(projectID: string, lang: string): string {
  return `/api/projects/${projectID}/export.md?lang=${lang}`;
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

// option is null only on the card of your critical edit (R4, B-49).
export async function answerCard(
  projectID: string,
  label: string,
  option: "keep" | "other" | null,
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

// «Por aclarar» (backlog-r2 2.4): option is the index of the chosen
// option, from 0. Go decides whether the «porque» counts (422).
export async function answerOpenQuestion(
  projectID: string,
  label: string,
  option: number,
  reason: string,
): Promise<AnswerOpenQuestionResponse> {
  return json(
    await fetch(`/api/projects/${projectID}/open-questions/${label}/answer`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ option, reason }),
    }),
  );
}

// With plan, the mark is changed from the «Entendí»'s card and also
// answers the plan (R-6).
export async function putSectionRole(
  projectID: string,
  label: string,
  role: string,
  plan = false,
): Promise<SectionRoleView> {
  return json(
    await fetch(`/api/projects/${projectID}/sections/${label}/role`, {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(plan ? { role, plan } : { role }),
    }),
  );
}

// «Confirmar» on the plan (R-6): Go records plan_confirmed and copies
// each section's line to its «qué debe abordar» (B-53).
export async function confirmPlan(projectID: string): Promise<UnderstandingView> {
  return json(await fetch(`/api/projects/${projectID}/plan/confirm`, { method: "POST" }));
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
// undefined stays as it is. With plan, the change is a correction from
// the «Entendí»'s card and also answers the plan (R-6).
export async function changeSectionDetails(
  projectID: string,
  label: string,
  details: { title?: string; focus?: string; plan?: boolean },
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

// The section's detail (backlog-r2 2.9): its warnings in the text, placed
// by Go (kind, passage, block, offsets and the label each leads to).
export async function getSection(projectID: string, label: string): Promise<SectionDetail> {
  return json(await fetch(`/api/projects/${projectID}/sections/${label}`));
}

// Closing a change from the section's band (fichas §3 «Cierre de un
// cambio», 2.2's route): «updated» after editing, or «reviewed_unchanged»
// with a «porque»; Go checks both and answers with the error code.
export async function closeChange(
  projectID: string,
  change: string,
  result: "updated" | "reviewed_unchanged",
  reason = "",
): Promise<void> {
  const res = await fetch(`/api/projects/${projectID}/changes/${change}/close`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ result, reason }),
  });
  if (!res.ok) throw new Error(await errorCode(res));
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

// R-1 (backlog-r2 2.11): you accept a paragraph without a source as
// general knowledge with your «porque»; Go checks the «porque» and that
// the passage is a paragraph of the section, and records claim_accepted.
export async function acceptClaim(projectID: string, label: string, passage: string, reason: string): Promise<void> {
  const res = await fetch(`/api/projects/${projectID}/sections/${label}/claims/accept`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ passage, reason }),
  });
  if (!res.ok) throw new Error(await errorCode(res));
}

// B-51: «Corregir» on «¿pide evidencia?»; Go leaves your value confirmed.
export async function putEvidenceRequired(projectID: string, required: boolean): Promise<EvidenceRequiredView> {
  return json(
    await fetch(`/api/projects/${projectID}/request/evidence`, {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ required }),
    }),
  );
}

// P-41 (30-oct): the history of a decision with no point in the text,
// opened from the signoff; it also records decision_seen (history).
// Marks or unmarks a decision as critical (fichas §2, Tu turno): Go
// records marked_critical and opens the card of an agent's decision.
export async function markCritical(projectID: string, label: string, marked: boolean): Promise<void> {
  const res = await fetch(`/api/projects/${projectID}/decisions/${label}/critical`, {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ marked }),
  });
  if (!res.ok) throw new Error(`http_${res.status}`);
}

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

// P6, the stop report (backlog-r2 2.5): Go assembles it; null when the
// project has none yet.
export async function getStopReport(projectID: string): Promise<StopReportView | null> {
  const res = await fetch(`/api/projects/${projectID}/stop-report`);
  if (res.status === 404) return null;
  return json(res);
}

// Opening the report in the panel counts as seen (B-40); folded does not.
export async function markStopReportSeen(projectID: string): Promise<void> {
  const res = await post(`/api/projects/${projectID}/stop-report/seen`);
  if (!res.ok) throw new Error(`http_${res.status}`);
}

// 5b: answering an audience question of the defense (backlog-r2 2.6):
// «sí» → addressed, «no» → declined; it opens no turn.
export async function answerAudienceQuestion(
  projectID: string,
  label: string,
  answer: "yes" | "no",
): Promise<AnswerAudienceQuestionResponse> {
  return json(await post(`/api/projects/${projectID}/audience-questions/${label}/answer`, { answer }));
}

// «Sigue» and «Abordémoslo» are messages of yours with the button's text.
export async function continueFromStopReport(projectID: string, text: string): Promise<PostMessageResponse> {
  return json(await post(`/api/projects/${projectID}/stop-report/continue`, { text }));
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
  // The stretch Go settled and why rule 0 moved a direct step to build
  // (B-43).
  stretch?: string;
  reclassified?: string[];
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

// Backlog-r3 3.2: the round card (B-64) and «Buscar evidencia» (B-72),
// which answers 409 (http_409) while a turn of the project runs.
export async function getRound(projectID: string, label: string): Promise<RoundCard> {
  return json(await fetch(`/api/projects/${projectID}/rounds/${label}`));
}

// With references it is «Buscarla» on those candidates (backlog-r3 3.5, B-69).
export async function startRound(projectID: string, references?: number[]): Promise<StartRoundResponse> {
  return json(await post(`/api/projects/${projectID}/rounds`, references ? { references } : undefined));
}

// Backlog-r3 3.4: «Repetir con otra pregunta» and «dejarlo por aclarar»,
// each your click. On a refusal they throw the API's error code
// (digest.error.<code> in the i18n catalog).
export async function repeatRound(projectID: string, label: string, question: string): Promise<RepeatRoundResponse> {
  return signoffJSON(await post(`/api/projects/${projectID}/rounds/${label}/repeat`, { question }));
}

export async function leaveRoundOpen(projectID: string, label: string): Promise<LeaveRoundOpenResponse> {
  return signoffJSON(await post(`/api/projects/${projectID}/rounds/${label}/leave-open`));
}

// «Auditar sección» (backlog-r3 3.6, B-67 b): the judge's round runs as a
// turn of its own.
export async function auditSection(projectID: string, section: string): Promise<StartRoundResponse> {
  return signoffJSON(await post(`/api/projects/${projectID}/sections/${section}/audit`));
}

// «No aplica» on a flaw (B-67); it answers 409 once the batch started.
export async function discardFlaw(projectID: string, round: string, n: number): Promise<void> {
  const res = await post(`/api/projects/${projectID}/rounds/${round}/flaws/${n}/discard`);
  if (!res.ok) await signoffJSON(res);
}
