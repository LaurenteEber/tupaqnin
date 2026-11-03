// C.11: a card you are writing in keeps its text and its place.
import { test } from "node:test";
import assert from "node:assert/strict";
import { anchoredScroll, canApplyQuestion, draftHasText, draftsOf, dropDraft, holdOrder, OWN_OPTION, setDraft } from "./cardDrafts.ts";
import en from "./i18n/en.json" with { type: "json" };
import es from "./i18n/es.json" with { type: "json" };

test("C11: the draft lives outside the component, by project and card", () => {
  setDraft("p1", "card-1", { reason: "porque sí lo vi" });
  setDraft("p1", "card-1", { option: "other" });
  setDraft("p2", "card-1", { reason: "otro proyecto" });
  assert.deepEqual(draftsOf("p1")["card-1"], { reason: "porque sí lo vi", option: "other" });
  assert.equal(draftsOf("p2")["card-1"].reason, "otro proyecto", "a label repeats across projects");
  dropDraft("p1", "card-1");
  assert.equal(draftsOf("p1")["card-1"], undefined);
  dropDraft("p2", "card-1");
});

test("C11: only written text holds the card", () => {
  assert.equal(draftHasText(undefined), false);
  assert.equal(draftHasText({ reason: "  " }), false);
  assert.equal(draftHasText({ option: "keep" }), false);
  assert.equal(draftHasText({ reason: "x" }), true);
});

test("C11: with text written, a refresh keeps the order; new ones go last, gone ones leave", () => {
  assert.deepEqual(holdOrder(["card-1", "oq-1"], ["card-1", "card-2", "oq-1"], true), ["card-1", "oq-1", "card-2"]);
  assert.deepEqual(holdOrder(["card-1", "oq-1"], ["oq-1"], true), ["oq-1"]);
  assert.deepEqual(holdOrder(["oq-1", "card-1"], ["card-1", "oq-1"], true), ["oq-1", "card-1"]);
});

test("C11: without text written, the list is what Go sent", () => {
  assert.deepEqual(holdOrder(["card-1", "oq-1"], ["card-1", "card-2", "oq-1"], false), ["card-1", "card-2", "oq-1"]);
});

test("C11: what grows above the card moves the scroll by as much, so the card stays", () => {
  assert.equal(anchoredScroll(100, 681, 769), 188);
  assert.equal(anchoredScroll(188, 769, 681), 100);
  assert.equal(anchoredScroll(100, 681, 681), 100);
});

test("TestB107: your own option is written in the draft, by card, and holds the card", () => {
  setDraft("p1", "oq-1", { chosen: OWN_OPTION, own: "Usar los conteos de 2024." });
  setDraft("p1", "oq-1", { reason: "Porque sí consta." });
  assert.deepEqual(draftsOf("p1")["oq-1"], { chosen: OWN_OPTION, own: "Usar los conteos de 2024.", reason: "Porque sí consta." });
  dropDraft("p1", "oq-1");
  assert.equal(draftHasText({ own: "Algo" }), true);
  assert.equal(draftHasText({ own: "  " }), false);
});

test("TestB107: «Apply» needs a choice, and your own option its statement", () => {
  assert.equal(canApplyQuestion(undefined), false);
  assert.equal(canApplyQuestion({ chosen: 0 }), true);
  assert.equal(canApplyQuestion({ chosen: OWN_OPTION }), false);
  assert.equal(canApplyQuestion({ chosen: OWN_OPTION, own: "  " }), false);
  assert.equal(canApplyQuestion({ chosen: OWN_OPTION, own: "Mi opción." }), true);
});

test("TestB107: the third option, in en and es", () => {
  assert.equal(en["oq.own"], "Write yours");
  assert.equal(es["oq.own"], "Escribe la tuya");
  assert.equal(en["oq.own.statement"], "Your decision");
  assert.equal(es["oq.own.statement"], "Tu decisión");
});
