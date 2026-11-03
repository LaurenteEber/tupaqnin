// C.4, B-101: the queued message's mark and its actions.
import { test } from "node:test";
import assert from "node:assert/strict";
import { canSaveEdit, isQueued, queuedEditRequest, queuedMessagePath, queuedRemoveRequest } from "./queue.ts";
import en from "./i18n/en.json" with { type: "json" };
import es from "./i18n/es.json" with { type: "json" };

test("B-101: only a message still waiting carries the mark", () => {
  assert.equal(isQueued({ kind: "message", queued: true, n: 7 }), true);
  assert.equal(isQueued({ kind: "message" }), false, "a message that already entered");
  assert.equal(isQueued({ kind: "step", queued: true, n: 7 }), false);
  assert.equal(isQueued({ kind: "message", queued: true }), false, "nothing to edit it by");
});

test("B-101: removing is a DELETE of that message; editing a PUT with its new text", () => {
  assert.equal(queuedMessagePath("p1", 7), "/api/projects/p1/messages/7");
  assert.equal(queuedRemoveRequest().method, "DELETE");
  const edit = queuedEditRequest("Otro texto");
  assert.equal(edit.method, "PUT");
  assert.deepEqual(JSON.parse(edit.body), { text: "Otro texto" });
});

test("B-101: a blank edit does not go", () => {
  assert.equal(canSaveEdit("  "), false);
  assert.equal(canSaveEdit("Sí"), true);
});

test("B-101: the mark says what happens, in both languages", () => {
  assert.equal(en["message.queued"], "Queued · it enters the agent's next step");
  assert.equal(es["message.queued"], "En cola · entra al siguiente paso del agente");
});
