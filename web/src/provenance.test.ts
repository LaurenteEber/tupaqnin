// UI tests for the origin of each paragraph (backlog-r3 3.5), run with
// `pnpm test`.
import { test } from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { originKey, originMarkRules, originVar, safeLink, searchedName } from "./provenance.ts";

const catalog = (lang: string): Record<string, string> =>
  JSON.parse(readFileSync(new URL(`./i18n/${lang}.json`, import.meta.url), "utf8"));

test("R-1: each origin Go computes has its line and its short mark in both catalogs", () => {
  for (const lang of ["es", "en"]) {
    for (const origin of ["material", "external", "model_knowledge", "unverified"]) {
      assert.ok(catalog(lang)[originKey(origin)], `${lang}: ${origin}`);
      assert.ok(catalog(lang)[`provenance.mark.${origin}`], `${lang}: mark ${origin}`);
    }
  }
  assert.match(catalog("es")[originKey("external")], /fuente buscada/);
  assert.match(catalog("es")[originKey("model_knowledge")], /sin verificar/);
});

test("R-2, B-69: the candidate and «Buscarla» have their lines in both catalogs", () => {
  assert.equal(catalog("es")["provenance.candidate"], "candidata, sin verificar");
  assert.equal(catalog("es")["provenance.search_candidate"], "Buscarla");
  assert.ok(catalog("en")["provenance.candidate"]);
  assert.ok(catalog("en")["provenance.search_candidate"]);
});

test("R-4: the understanding's offer without material has its lines in both catalogs", () => {
  for (const lang of ["es", "en"]) {
    assert.ok(catalog(lang)["understanding.no_material"], lang);
  }
});

test("R-1: a searched source is named by title and year, and only http links are anchors", () => {
  assert.equal(searchedName({ title: "Detection limits", year: 2021 }), "Detection limits (2021)");
  assert.equal(searchedName({ title: "Detection limits" }), "Detection limits");
  assert.equal(safeLink("https://openalex.org/W1"), "https://openalex.org/W1");
  assert.equal(safeLink("javascript:alert(1)"), "");
  assert.equal(safeLink(undefined), "");
});

test("R-1: each paragraph gets its origin's mark, block by block", () => {
  const rules = originMarkRules("sec-1", [
    { block: 1, origin: "external" },
    { block: 2, origin: "unverified" },
    { block: 3, origin: "external" },
  ]);
  assert.match(rules, /section="sec-1"\] \.section-editor-content > p:nth-child\(1\)::after/);
  assert.match(rules, /p:nth-child\(3\)::after \{ content: var\(--origin-external\)/);
  assert.match(rules, /p:nth-child\(2\)::after \{ content: var\(--origin-unverified\)/);
  assert.equal(originVar("model_knowledge"), "--origin-model-knowledge");
  assert.equal(originMarkRules("sec-1", []), "");
});
