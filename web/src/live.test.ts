// UI tests for the page's live updates (P-92, P-93, P-94), run with
// `pnpm test` (node --test, no extra dependency).
import { test } from "node:test";
import assert from "node:assert/strict";
import {
  DEFENSE_POLL_CAP_MS,
  DEFENSE_POLL_MS,
  pollDefense,
  roundKeepsPolling,
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
