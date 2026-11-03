// UI tests for the bar's spend notice (B3-20, B-89), run with `pnpm test`.
import { test } from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { auditCostText, spendNoticeKey } from "./spend.ts";

const catalog = (lang: string): Record<string, string> =>
  JSON.parse(readFileSync(new URL(`./i18n/${lang}.json`, import.meta.url), "utf8"));

const cases: [{ warn: boolean; stopped_by: string }, string | null][] = [
  [{ warn: false, stopped_by: "" }, null],
  [{ warn: true, stopped_by: "" }, "summary.spend.warn"],
  [{ warn: true, stopped_by: "project_cap" }, "summary.spend.project_cap"],
  [{ warn: false, stopped_by: "link" }, "summary.spend.link"],
  [{ warn: true, stopped_by: "day" }, "summary.spend.day"],
  [{ warn: false, stopped_by: "total" }, "summary.spend.total"],
];

test("B3-20: the notice names what stops the agent, or the 80 %, or nothing", () => {
  for (const [summary, want] of cases) {
    assert.equal(spendNoticeKey(summary), want, JSON.stringify(summary));
  }
});

test("B3-20: every notice and every stop has its text in both languages", () => {
  const keys = cases.flatMap(([, key]) => (key ? [key] : []));
  keys.push("turn.stop.project_cap", "turn.stop.demo_cap", "round.reason.demo_cap", "round.stopped.demo_cap", "digest.status.demo_cap");
  for (const lang of ["en", "es"]) {
    const texts = catalog(lang);
    for (const key of keys) assert.ok(texts[key], `${lang}: ${key}`);
  }
});

test("B3-20: the day's notice says the demo reached today's cap", () => {
  assert.match(catalog("es")["summary.spend.day"], /hoy/);
  assert.match(catalog("en")["summary.spend.day"], /today/);
});

test("B3-20: «sin comprobar» does not say the checker failed: the cap may have refused it", () => {
  assert.doesNotMatch(catalog("es")["pending.unchecked"], /falló/);
  assert.doesNotMatch(catalog("en")["pending.unchecked"], /failed/);
});

test("C20: the cost on «Audit section» comes from Go, formatted as the header's spend", () => {
  assert.deepEqual(auditCostText(0.05), { key: "doc.audit.cost", params: { cost: "0.05" } });
  assert.deepEqual(auditCostText(0.1234), { key: "doc.audit.cost", params: { cost: "0.12" } });
});

test("C20: before Go answers, the button says no cost", () => {
  assert.equal(auditCostText(undefined), null);
});

test("C20: the cost's text, in both languages, once each", () => {
  for (const [lang, want] of [["en", "about {cost} USD"], ["es", "unos {cost} USD"]] as const) {
    const raw = readFileSync(new URL(`./i18n/${lang}.json`, import.meta.url), "utf8");
    assert.equal(JSON.parse(raw)["doc.audit.cost"], want);
    assert.equal(raw.split('"doc.audit.cost"').length - 1, 1, lang);
  }
});
