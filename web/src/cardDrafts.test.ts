// C.11: a card you are writing in keeps its text and its place.
import { test } from "node:test";
import assert from "node:assert/strict";
import { anchoredScroll, draftHasText, draftsOf, dropDraft, holdOrder, setDraft } from "./cardDrafts.ts";

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
