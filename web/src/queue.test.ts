// C.4, B-101: the queued message's mark and its actions.
import { test } from "node:test";
import assert from "node:assert/strict";
import { canSaveEdit, isQueued, queuedEditRequest, queuedMark, queuedMessagePath, queuedRemoveRequest, queuedRetryPath } from "./queue.ts";
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

test("B-101: a message Go says was not sent says so and offers the retry, in both languages", () => {
  assert.deepEqual(queuedMark({ not_sent: true, retry_sends: 1 }), { key: "message.not_sent", retry: "message.queued_retry", n: "1" });
  assert.deepEqual(queuedMark({}), { key: "message.queued", retry: null, n: "0" });
  assert.equal(queuedRetryPath("p1", 7), "/api/projects/p1/messages/7/retry");
  assert.equal(en["message.not_sent"], "Not sent · the agent stopped before reading it");
  assert.equal(es["message.not_sent"], "No se envió · el agente se detuvo antes de leerlo");
  assert.equal(en["message.queued_retry"], "Retry");
  assert.equal(es["message.queued_retry"], "Reintentar");
});

test("B-101: with two or more waiting the retry says how many it sends", () => {
  assert.deepEqual(queuedMark({ not_sent: true, retry_sends: 3 }), { key: "message.not_sent", retry: "message.queued_retry_many", n: "3" });
  assert.equal(en["message.queued_retry_many"], "Retry · sends the {n} waiting");
  assert.equal(es["message.queued_retry_many"], "Reintentar · envía los {n} que esperan");
});

test("B-101: at a spend cap the mark says so and offers no retry", () => {
  assert.deepEqual(queuedMark({ not_sent: true, not_sent_cap: true, retry_sends: 2 }), { key: "message.not_sent_cap", retry: null, n: "2" });
  assert.equal(en["message.not_sent_cap"], "Not sent · the spend cap was reached");
  assert.equal(es["message.not_sent_cap"], "No se envió · se alcanzó el tope de gasto");
});
