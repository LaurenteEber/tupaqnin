// C.12: «Firmar» off, with its reason, on a section with no text.
import { test } from "node:test";
import assert from "node:assert/strict";
import { askToWriteParams, sectionHasNoText } from "./signEmpty.ts";
import en from "./i18n/en.json" with { type: "json" };
import es from "./i18n/es.json" with { type: "json" };

test("C12: a section with no text, or only spaces, has no text", () => {
  assert.equal(sectionHasNoText({ text: "" }), true);
  assert.equal(sectionHasNoText({ text: " \n " }), true);
  assert.equal(sectionHasNoText({}), true);
  assert.equal(sectionHasNoText({ text: "Algo." }), false);
  assert.equal(sectionHasNoText(undefined), false, "no section selected is not this reason");
});

test("C12: the reason and its button, in both languages", () => {
  assert.equal(en["signoff.empty.reason"], "This section has no text yet");
  assert.equal(es["signoff.empty.reason"], "Esta sección aún no tiene texto");
  assert.equal(en["signoff.empty.ask"], "Ask the agent to write it");
  assert.equal(es["signoff.empty.ask"], "Pedir al agente que la escriba");
  assert.equal(en["signoff.empty.message"], "Write the section «{section}».");
  assert.equal(es["signoff.empty.message"], "Escribe la sección «{section}».");
  assert.deepEqual(askToWriteParams({ title: "Síntesis" }), { section: "Síntesis" });
});
