// UI tests for the page's live updates (P-92, P-93, P-94), run with
// `pnpm test` (node --test, no extra dependency).
import { test } from "node:test";
import assert from "node:assert/strict";
import {
  DEFENSE_POLL_CAP_MS,
  DEFENSE_POLL_MS,
  pollDefense,
  IDLE_ACTIVITY,
  nextActivity,
  pollRound,
  roundKeepsPolling,
  statusLine,
  roundStarted,
  sendErrorCode,
  keepsAskingToSign,
  shouldReloadEditor,
  stopKey,
  turnToFollow,
  startsStep,
  textAfterSend,
} from "./live.ts";

// A fake clock: sleep advances it.
function clock() {
  let t = 0;
  return { now: () => t, sleep: async (ms: number) => void (t += ms) };
}

test("P-92: the conversation refreshes by itself until the defense's questions arrive and the preparation ends", async () => {
  const c = clock();
  const reads: number[][] = [];
  let calls = 0;
  const end = await pollDefense<number>({
    ...c,
    cancelled: () => false,
    // Preparing for two rounds; the questions are written before it ends.
    preparing: async () => ++calls < 3,
    conversation: async () => (calls < 3 ? [1] : [1, 2, 3]),
    onEntries: (e) => reads.push(e),
  });
  assert.equal(end, "ended");
  assert.deepEqual(reads, [[1], [1], [1, 2, 3]]);
  assert.equal(c.now(), 3 * DEFENSE_POLL_MS);
});

test("P-92: the polling stops at its cap when the preparation never ends", async () => {
  const c = clock();
  let reads = 0;
  const end = await pollDefense<number>({
    ...c,
    cancelled: () => false,
    preparing: async () => true,
    conversation: async () => [],
    onEntries: () => void reads++,
  });
  assert.equal(end, "cap");
  assert.equal(reads, DEFENSE_POLL_CAP_MS / DEFENSE_POLL_MS);
});

test("P-92: a failed read is asked again; leaving the project cancels the polling", async () => {
  const c = clock();
  let calls = 0;
  let left = false;
  const end = await pollDefense<number>({
    ...c,
    cancelled: () => left,
    preparing: async () => {
      calls++;
      if (calls === 1) throw new Error("http_500");
      left = calls === 2;
      return true;
    },
    conversation: async () => [],
    onEntries: () => {},
  });
  assert.equal(end, "cancelled");
  assert.equal(calls, 2);
});

test("P-94: a send refused while the defense line shows names the defense", () => {
  const conflict = new Error("http_409");
  assert.equal(sendErrorCode(conflict, true), "turn_running_defense");
  assert.equal(sendErrorCode(conflict, false), "turn_running");
  assert.equal(sendErrorCode(new Error("http_500"), true), "turn_failed");
});

test("P-94: the input keeps your text whenever the send fails", () => {
  assert.equal(textAfterSend("¿y el piloto?", false), "¿y el piloto?");
  assert.equal(textAfterSend("¿y el piloto?", true), "");
});

test("P-93: the start of a step's triage line means the step before it was applied", () => {
  assert.equal(startsStep({ field: "triage_line", start: true }), true);
  assert.equal(startsStep({ field: "triage_line" }), false);
  assert.equal(startsStep({ field: "text", start: true }), false);
});

test("P-93: the selected section's editor reloads a newer text the agent wrote, never over your typing", () => {
  const base = { version: 1, savedText: "" };
  const agent = { version: 4, text: "Incluimos los estudios con filtro." };
  assert.equal(shouldReloadEditor(base, agent, "", ""), true, "empty editor, the agent wrote");
  assert.equal(shouldReloadEditor(base, agent, "Lo mío", ""), false, "you typed since it loaded");
  assert.equal(shouldReloadEditor(base, { version: 1, text: agent.text }, "", ""), false, "not newer");
  assert.equal(shouldReloadEditor({ version: 1, savedText: agent.text }, agent, agent.text, agent.text), false, "same text");
});

test("P-95: when the defense's wait ends, the header's summary is read again; not when it is cancelled", async () => {
  for (const [preparing, left, want] of [
    [false, false, 1], // ended
    [true, false, 1], // cap
    [true, true, 0], // cancelled
  ] as const) {
    const c = clock();
    let summaries = 0;
    await pollDefense<number>({
      ...c,
      cancelled: () => left,
      preparing: async () => preparing,
      conversation: async () => [],
      onEntries: () => {},
      onEnd: () => void summaries++,
    });
    assert.equal(summaries, want);
  }
});

test("3.2 (B-64): a «round» piece of the stream names the round that started; other pieces do not", () => {
  assert.equal(roundStarted({ field: "round", text: "rnd-2" }), "rnd-2");
  assert.equal(roundStarted({ field: "triage_line", text: "rnd-2" }), "");
  assert.equal(roundStarted({ field: "text", text: "x" }), "");
});

test("3.2 (B-64): the round card asks again while its readers run, and stops when they ended", () => {
  assert.equal(roundKeepsPolling({ running: true, parts: [{ status: "returned" }] }), true);
  assert.equal(roundKeepsPolling({ running: false, parts: [{ status: "running" }] }), true);
  assert.equal(roundKeepsPolling({ running: false, parts: [{ status: "returned" }, { status: "empty" }] }), false);
});

test("B-83: the mode's last stop says it by its cause, any other by its reason", () => {
  assert.equal(stopKey("mode_finished", "human_only"), "turn.stop.mode_finished.human_only");
  assert.equal(stopKey("mode_finished"), "turn.stop.mode_finished");
  assert.equal(stopKey("section_ready"), "turn.stop.section_ready");
});

test("B-83: the page attaches to a running chain it does not follow, once", () => {
  assert.equal(turnToFollow({ running_turn: "t2" }, "t1"), "t2");
  assert.equal(turnToFollow({ running_turn: "t2" }, "t2"), "");
  assert.equal(turnToFollow({}, "t1"), "");
});

test("B-83: asking again for a queued signoff stops when the project changes", () => {
  assert.equal(keepsAskingToSign({ kind: "queued" }, "p1", "p1"), true);
  assert.equal(keepsAskingToSign({ kind: "queued" }, "p1", "p2"), false);
  assert.equal(keepsAskingToSign({ kind: "opened" }, "p1", "p1"), false);
});

test("C.2: a round whose readers have not started yet keeps asking; a closed one does not", () => {
  assert.equal(roundKeepsPolling({ running: false, parts: [{ status: "pending" }], state: "open" }), true);
  assert.equal(roundKeepsPolling({ running: false, parts: [{ status: "pending" }], state: "closed" }), false);
});

// The window Paula hit (B-64): the step wrote the round, the card read it before
// its readers started, and it never asked again.
test("C.2 (B-64): a round the principal started updates by itself, from a first read with nothing running", async () => {
  const reads = [
    { running: false, state: "open", parts: [{ status: "pending" }] },
    { running: true, state: "open", parts: [{ status: "running" }] },
    { running: true, state: "open", parts: [{ status: "returned" }] },
    { running: false, state: "closed", parts: [{ status: "returned" }] },
  ];
  const shown: string[] = [];
  let at = 0;
  await pollRound({
    read: async () => reads[at++],
    onCard: (c) => shown.push(c.state + ":" + c.parts[0].status),
    sleep: async () => {},
    cancelled: () => false,
  });
  assert.deepEqual(shown, ["open:pending", "open:running", "open:returned", "closed:returned"]);
});

test("C.2: a failed read asks again, and leaving the project stops asking", async () => {
  let reads = 0;
  let left = false;
  await pollRound({
    read: async () => {
      reads++;
      if (reads === 1) throw new Error("http_500");
      left = true;
      return { running: true, parts: [] };
    },
    onCard: () => assert.fail("a card read after leaving does not show"),
    sleep: async () => {},
    cancelled: () => left,
  });
  assert.equal(reads, 2);
});

test("C.2: what the agent is doing follows the stream's pieces", () => {
  const seq = (pieces: { field: string; text?: string; start?: boolean }[]) => pieces.reduce(nextActivity, IDLE_ACTIVITY);
  assert.equal(seq([{ field: "triage_line", text: "Build", start: true }]).kind, "working");
  assert.deepEqual(
    seq([{ field: "section_id", text: "sec", start: true }, { field: "section_id", text: "-2" }, { field: "text", text: "x", start: true }]),
    { kind: "writing", section: "sec-2" },
  );
  assert.equal(seq([{ field: "round", text: "rnd-1", start: true }]).kind, "reading");
  assert.deepEqual(
    seq([{ field: "section_id", text: "sec-2", start: true }, { field: "text", text: "x" }, { field: "triage_line", text: "Next", start: true }]),
    IDLE_ACTIVITY,
    "a new step starts over",
  );
});

test("C.2: the status line for each event", () => {
  const line = (over: Partial<Parameters<typeof statusLine>[0]>) =>
    statusLine({ working: true, defensePreparing: false, activity: IDLE_ACTIVITY, sectionTitle: () => "", todo: 0, ...over });
  assert.deepEqual(line({}), { key: "status.agent.working" });
  assert.deepEqual(
    line({ activity: { kind: "writing", section: "sec-2" }, sectionTitle: (l) => (l === "sec-2" ? "Síntesis" : "") }),
    { key: "status.agent.writing_section", params: { section: "Síntesis" } },
  );
  assert.deepEqual(line({ activity: { kind: "writing", section: "new-1" } }), { key: "status.agent.writing" });
  assert.deepEqual(line({ activity: { kind: "reading", section: "" } }), { key: "status.agent.reading" });
  assert.deepEqual(line({ activity: { kind: "judging", section: "" } }), { key: "status.agent.judging" });
  assert.deepEqual(line({ working: false, defensePreparing: true }), { key: "status.agent.defense" });
  assert.deepEqual(line({ working: false, todo: 1 }), { key: "status.yours.one" });
  assert.deepEqual(line({ working: false, todo: 3 }), { key: "status.yours.many", params: { n: "3" } });
  assert.equal(line({ working: false }), null);
  assert.deepEqual(line({ todo: 3 }), { key: "status.agent.working" }, "while it works it does not ask you");
});
