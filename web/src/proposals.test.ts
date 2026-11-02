// UI tests of the proposals (backlog-r4 4.1), run with `pnpm test`.
import { test } from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { closedKey, proposalsTarget, proposalsToShow, turnToWatch } from "./proposals.ts";
import type { ProposalView } from "./api/graph";

const catalog = (lang: string): Record<string, string> =>
  JSON.parse(readFileSync(new URL(`./i18n/${lang}.json`, import.meta.url), "utf8"));
const en = catalog("en");
const es = catalog("es");

const proposal = (over: Partial<ProposalView>): ProposalView =>
  ({ label: "prop-1", state: "open", section: "sec-1", ...over }) as ProposalView;

test("«Propuestas (n)» leads to the first section with one, or to the proposed sections", () => {
  assert.equal(proposalsTarget([{ label: "prop-1", title: "Riesgos" }, { label: "prop-2", section: "sec-2" }]), "sec-2");
  assert.equal(proposalsTarget([{ label: "prop-1", title: "Riesgos" }]), "");
});

test("a section shows its proposals, and the document's end the open proposed sections", () => {
  const onSection = proposal({ label: "prop-1" });
  const newSection = proposal({ label: "prop-2", section: "", new_section: { title: "Riesgos", role: "other" } });
  const closed = proposal({ label: "prop-3", section: "", state: "discarded", new_section: { title: "Otra", role: "other" } });
  assert.deepEqual(proposalsToShow([onSection, newSection, closed], "sec-1"), [onSection]);
  assert.deepEqual(proposalsToShow([onSection, newSection, closed]), [newSection]);
});

test("every closing a proposal can have says why, in both languages", () => {
  for (const p of [
    proposal({ state: "accepted" }),
    proposal({ state: "discarded", why: "discarded" }),
    proposal({ state: "discarded", why: "signed_without" }),
    proposal({ state: "discarded", why: "anchor_not_unique" }),
    proposal({ state: "discarded", why: "section_changed" }),
  ]) {
    assert.ok(closedKey(p) in en, closedKey(p));
    assert.ok(closedKey(p) in es, closedKey(p));
  }
});

test("the new step's turn goes to watchTurn, like every turn the page shows (B-75)", () => {
  assert.equal(turnToWatch({ label: "prop-1", state: "discarded", why: "anchor_not_unique", turn_id: "t1" }), "t1");
  assert.equal(turnToWatch({ label: "prop-1", state: "accepted" }), "");
  const panel = readFileSync(new URL("./components/ProposalPanel.tsx", import.meta.url), "utf8");
  assert.match(panel, /onChanged\(turnToWatch\(/);
  const app = readFileSync(new URL("./App.tsx", import.meta.url), "utf8");
  assert.match(app, /function proposalActed\(turn: string\) \{[^}]*if \(turn\) watchTurn\(turn\)/s);
});
