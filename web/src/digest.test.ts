// UI tests for the round's digest (backlog-r3 3.4), run with `pnpm test`.
import { test } from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { coverageLines, findingReasonKey, offersLeaveOpen, offersRepeat } from "./digest.ts";
import { sourceHref } from "./provenance.ts";

const catalog = (lang: string): Record<string, string> =>
  JSON.parse(readFileSync(new URL(`./i18n/${lang}.json`, import.meta.url), "utf8"));

function fill(text: string, params: Record<string, string>): string {
  for (const [k, v] of Object.entries(params)) text = text.replace(`{${k}}`, v);
  return text;
}

test("A-5: a reader that read 3 of its 5 results says «2 de 5 resultados sin leer»", () => {
  const lines = coverageLines([
    { part: 1, status: "returned", sources: 0, sources_unread: [], hits: 5, hits_unread: ["hit-4", "hit-5"], gaps: [] },
  ]);
  assert.equal(lines.length, 1);
  assert.equal(lines[0].key, "digest.hits_unread");
  assert.match(fill(catalog("es")[lines[0].key], lines[0].params), /2 de 5 resultados sin leer/);
  assert.match(fill(catalog("en")[lines[0].key], lines[0].params), /2 of 5 results unread/);
});

test("A-5: a reader that read everything has no line; an empty one says so", () => {
  const lines = coverageLines([
    { part: 1, status: "returned", sources: 1, sources_unread: [], hits: 3, hits_unread: [], gaps: [] },
    { part: 2, status: "empty", reason: "invalid_output", sources: 1, sources_unread: ["src-1"], hits: 0, hits_unread: [], gaps: [] },
  ]);
  assert.deepEqual(lines.map((l) => l.key), ["digest.reader_empty"]);
});

test("B-63: every reason a finding is discarded or unverified has its line in both catalogs", () => {
  for (const reason of ["passage_not_found", "source_not_found", "inaccessible"]) {
    const key = findingReasonKey({ verification: "discarded", reason });
    assert.ok(catalog("en")[key], key);
    assert.ok(catalog("es")[key], key);
  }
});

test("K-5: repeating and leaving it to clarify are offered only when Go offers the repeat", () => {
  assert.equal(offersRepeat({ repeat: null }), false);
  assert.equal(offersRepeat({ repeat: { estimated_cost_usd: 0.1 } }), true);
  assert.equal(offersLeaveOpen({ repeat: { estimated_cost_usd: 0.1 }, scope_question: "oq-1" }), false);
  assert.equal(offersLeaveOpen({ repeat: { estimated_cost_usd: 0.1 } }), true);
});

test("a passage opens in its source: the API's URL, or your material's download", () => {
  assert.equal(sourceHref("p1", { source: "src-2", source_url: "https://doi.org/10.1/x" }), "https://doi.org/10.1/x");
  assert.equal(sourceHref("p1", { source: "src-1" }), "/api/projects/p1/sources/src-1/download");
  assert.equal(sourceHref("p1", {}), "");
});

test("every digest key exists in both catalogs", () => {
  const en = catalog("en");
  const es = catalog("es");
  for (const key of Object.keys(en).filter((k) => k.startsWith("digest.") || k.startsWith("oq.scope."))) {
    assert.ok(es[key], `es lacks ${key}`);
  }
});

test("3.4 (flujos §2): unused findings and the readers' contradictions have their lines in both catalogs", () => {
  for (const lang of ["en", "es"]) {
    assert.match(fill(catalog(lang)["digest.unused"], { n: "2" }), /2/);
    assert.ok(catalog(lang)["digest.reader_contradictions"]);
  }
});
