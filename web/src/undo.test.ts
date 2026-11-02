// UI tests of undo and the alert (backlog-r4 4.2), run with `pnpm test`.
import { test } from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { alertReasonKey, alertReasonWhat, undoLineKey, undoReasonKey } from "./undo.ts";

const catalog = (lang: string): Record<string, string> =>
  JSON.parse(readFileSync(new URL(`./i18n/${lang}.json`, import.meta.url), "utf8"));
const en = catalog("en");
const es = catalog("es");

test("every reason «Deshacer» can be off has its words, in both languages", () => {
  for (const r of ["no_edit", "agent_wrote", "proposal_accepted", "later_edit"]) {
    assert.equal(undoReasonKey(r), `undo.reason.${r}`);
    assert.ok(en[undoReasonKey(r)] && es[undoReasonKey(r)], r);
  }
  assert.equal(undoReasonKey("internal"), "undo.error");
  assert.ok(en["undo.error"] && es["undo.error"]);
});

test("P-115: the button's line says why it is off, also with no edit of yours", () => {
  assert.equal(undoLineKey(true, ""), "");
  assert.equal(undoLineKey(false, "no_edit"), "undo.reason.no_edit");
  assert.equal(en["undo.reason.no_edit"], "No edit of yours to undo.");
  assert.equal(undoLineKey(false, "agent_wrote"), "undo.reason.agent_wrote");
});

test("every motive of an alert has its words, in both languages", () => {
  for (const kind of ["paragraph_deleted", "over_half", "figure_anchor", "citation_anchor", "decision_text", "signed_text", "other_section", "pasted"]) {
    const key = alertReasonKey({ rule: 1, kind });
    assert.ok(en[key] && es[key], kind);
  }
});

test("a motive names the decision's statement first, and stays literal", () => {
  assert.equal(alertReasonWhat({ rule: 2, kind: "figure_anchor", about: "60 veces", statement: "Comparar igual." }), "Comparar igual.");
  assert.equal(alertReasonWhat({ rule: 4, kind: "pasted", about: "cuenca.md" }), "cuenca.md");
  assert.equal(alertReasonWhat({ rule: 1, kind: "over_half" }), "");
});
