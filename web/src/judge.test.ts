// UI tests for the judge round's card (backlog-r3 3.6, B-67), run with
// `pnpm test` (node --test, no extra dependency).
import { test } from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { flawOutcomeKey, offersNotApplicable, triageKey } from "./judge.ts";
import { correctionSummary } from "./judge.ts";
import { auditErrorCode, invitesRetry, roundErrorCode, roundKeepsPolling, sendErrorCode } from "./live.ts";

const open = { state: "open", batch_started: false };

// B-67: «No aplica» shows only until the batch starts, on a flaw you did
// not discard already.
test("B-67: «No aplica» is offered only before the batch starts", () => {
  assert.equal(offersNotApplicable(open, { discarded: false }), true);
  assert.equal(offersNotApplicable(open, { discarded: true }), false);
  assert.equal(offersNotApplicable({ state: "open", batch_started: true }, { discarded: false }), false);
  assert.equal(offersNotApplicable({ state: "closed", batch_started: false }, { discarded: false }), false);
});

// K-3: the triage line says the judge enters, with its risk, or that you
// asked for the audit.
test("K-3: the triage line names the risk or your audit", () => {
  assert.equal(triageKey({ reason: "risk" }), "judge.triage.risk");
  assert.equal(triageKey({ reason: "audit" }), "judge.triage.audit");
});

// After the recheck each flaw says how it ended; before it, what you did.
test("B-67: a flaw's outcome comes from Go's verdict", () => {
  assert.equal(flawOutcomeKey({ discarded: true }), "judge.outcome.discarded");
  assert.equal(flawOutcomeKey({ discarded: false, verdict: "resolved" }), "judge.outcome.resolved");
  assert.equal(flawOutcomeKey({ discarded: false, verdict: "still_open" }), "judge.outcome.still_open");
  assert.equal(flawOutcomeKey({ discarded: false }), "");
});

// The card keeps asking while its judge round is open: batch and recheck
// happen in the turn, with no reader in it.
test("B-67: a judge round keeps polling while open", () => {
  assert.equal(roundKeepsPolling({ running: false, parts: [], kind: "judge", state: "open" }), true);
  assert.equal(roundKeepsPolling({ running: false, parts: [], kind: "judge", state: "closed" }), false);
  assert.equal(roundKeepsPolling({ running: false, parts: [], kind: "research", state: "open" }), false);
});

// Every on-screen string comes from both catalogs (rule 3).
test("B-67: the card's strings are in both catalogs", () => {
  for (const lang of ["en", "es"]) {
    const catalog = JSON.parse(readFileSync(new URL(`./i18n/${lang}.json`, import.meta.url), "utf8")) as Record<string, string>;
    for (const key of ["doc.audit", "frame.todo.flaws", "judge.triage.risk", "judge.triage.audit", "judge.not_verified",
      "judge.not_applicable", "judge.outcome.discarded", "judge.outcome.resolved", "judge.outcome.still_open",
      "judge.risk.figures_and_calculations", "judge.risk.unverifiable", "oq.scope.redo_passage", "oq.scope.accept_risk",
      "stop_report.judge_not_verified", "warnings.flaw", "warnings.underline.flaw", "judge.error.batch_started"]) {
      assert.ok(catalog[key], `${lang}: ${key}`);
    }
  }
});

// F2: each refusal of «Auditar sección» shows its own message, and only a
// real failure invites retrying. sendErrorCode's other callers stay as
// they were.
test("B-67: an audit's refusal keeps its own code", () => {
  for (const code of ["round_open", "section_empty", "turn_running"]) {
    assert.equal(auditErrorCode(new Error(code), false), code);
  }
  assert.equal(auditErrorCode(new Error("turn_running"), true), "turn_running_defense");
  assert.equal(auditErrorCode(new Error("internal"), false), "turn_failed");
  assert.equal(sendErrorCode(new Error("section_empty"), false), "turn_failed");
  for (const code of ["round_open", "section_empty", "turn_running", "turn_running_defense"]) {
    assert.equal(invitesRetry(code), false, code);
  }
  assert.equal(invitesRetry("turn_failed"), true);
  assert.equal(invitesRetry("provider_error"), true);
  for (const lang of ["en", "es"]) {
    const catalog = JSON.parse(readFileSync(new URL(`./i18n/${lang}.json`, import.meta.url), "utf8")) as Record<string, string>;
    for (const code of ["round_open", "section_empty", "turn_running"]) {
      assert.ok(catalog[`stream.error.${code}`], `${lang}: ${code}`);
    }
  }
});

// P-100: a refused round keeps its own code, and writing again cannot help.
test("P-100: a refused round keeps its own code", () => {
  for (const code of ["request_required", "candidate_not_found"]) {
    assert.equal(roundErrorCode(new Error(code), false), code);
    assert.equal(invitesRetry(code), false);
  }
  assert.equal(roundErrorCode(new Error("turn_running"), true), "turn_running_defense");
  assert.equal(roundErrorCode(new Error("internal"), false), "turn_failed");
});

// P-100: every refusal the page can show has its text in both catalogs.
test("P-100: the refusals are in both catalogs", () => {
  for (const lang of ["en", "es"]) {
    const catalog = JSON.parse(readFileSync(new URL(`./i18n/${lang}.json`, import.meta.url), "utf8"));
    for (const key of ["stream.error.request_required", "stream.error.candidate_not_found", "digest.error.version_conflict"]) {
      assert.ok(catalog[key], `${lang}: ${key}`);
    }
  }
});

// 3.7, B-59: the card names a discard with the words of the design.
test("3.7: a discard reads «fuente inexistente» and «pasaje ausente»", () => {
  const es = JSON.parse(readFileSync(new URL("./i18n/es.json", import.meta.url), "utf8"));
  assert.equal(es["digest.reason.source_not_found"], "fuente inexistente");
  assert.equal(es["digest.reason.passage_not_found"], "pasaje ausente");
  const en = JSON.parse(readFileSync(new URL("./i18n/en.json", import.meta.url), "utf8"));
  assert.equal(en["digest.reason.source_not_found"], "source does not exist");
  assert.equal(en["digest.reason.passage_not_found"], "passage not found");
});

// P-111: the report's list of the agent's pendings has its title in both catalogs.
test("P-111: the report names the agent's pendings", () => {
  for (const lang of ["en", "es"]) {
    const catalog = JSON.parse(readFileSync(new URL(`./i18n/${lang}.json`, import.meta.url), "utf8"));
    assert.ok(catalog["stop_report.agent_pendings"], lang);
  }
});

test("B-99: the card says how many flaws were found and how many got a change", () => {
  const card = { flaws: [{ n: 1 }, { n: 2 }, { n: 3 }], changes: [{ n: 1 }, { n: 3 }, { n: 3 }, {}] };
  assert.deepEqual(correctionSummary(card), { found: 3, corrected: 2 });
  assert.deepEqual(correctionSummary({}), { found: 0, corrected: 0 });
});
