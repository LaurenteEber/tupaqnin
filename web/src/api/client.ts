import type {
  CreateProjectResponse,
  PostMessageResponse,
  SectionResponse,
  SectionView,
} from "./types";

// Thin API client. It only transports what Go computes (AGENTS.md
// rule 5): no state, fingerprint or validity is decided here.
async function json<T>(res: Response): Promise<T> {
  if (!res.ok) {
    throw new Error(`http_${res.status}`);
  }
  return (await res.json()) as T;
}

export async function createProject(): Promise<CreateProjectResponse> {
  return json(await fetch("/api/projects", { method: "POST" }));
}

export async function createSection(projectID: string): Promise<SectionResponse> {
  return json(
    await fetch(`/api/projects/${projectID}/sections`, { method: "POST" }),
  );
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
export async function saveSection(
  projectID: string,
  label: string,
  baseVersion: number,
  text: string,
): Promise<SaveResult> {
  const res = await fetch(`/api/projects/${projectID}/sections/${label}`, {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ base_version: baseVersion, text }),
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
  closing_risk?: string;
  note?: string;
  deferred?: { what: string; why: string }[];
}

export interface TurnEvents {
  onDelta: (text: string) => void;
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
    const data = JSON.parse((e as MessageEvent).data) as { text?: string };
    if (data.text) events.onDelta(data.text);
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
