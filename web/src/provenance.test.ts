// UI tests for the origin of each paragraph (backlog-r3 3.5), run with
// `pnpm test`.
import { test } from "node:test";
import assert from "node:assert/strict";
import { readFileSync, readdirSync } from "node:fs";
import { join } from "node:path";
import { fileURLToPath } from "node:url";
import { originKey, originMarkClass, originVar, paragraphHasHistory, safeLink, searchedName } from "./provenance.ts";

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
  const origins = [
    { block: 1, origin: "external" },
    { block: 2, origin: "unverified" },
    { block: 3, origin: "model_knowledge" },
    { block: 4, origin: "material" },
  ];
  assert.equal(originMarkClass(origins, 1), "origin-mark origin-mark-external");
  assert.equal(originMarkClass(origins, 2), "origin-mark origin-mark-unverified");
  assert.equal(originMarkClass(origins, 3), "origin-mark origin-mark-model-knowledge");
  assert.equal(originMarkClass(origins, 4), "origin-mark origin-mark-material");
  assert.equal(originMarkClass(origins, 5), "");
  assert.equal(originMarkClass([], 1), "");
  assert.equal(originVar("model_knowledge"), "--origin-model-knowledge");
});

test("R-1: an origin the catalog does not know gets no mark", () => {
  assert.equal(originMarkClass([{ block: 1, origin: "x\" onclick=\"y" }], 1), "");
});

test("R-1: styles.css draws each origin's mark from its custom property", () => {
  const css = readFileSync(new URL("./styles.css", import.meta.url), "utf8");
  for (const origin of ["material", "external", "model_knowledge", "unverified"]) {
    const name = origin.replace(/_/g, "-");
    assert.match(css, new RegExp(`\\.origin-mark-${name}::after \\{[^}]*content: var\\(${originVar(origin)}\\)`), origin);
  }
});

test("P-118: an empty paragraph has no history to ask for", () => {
  assert.equal(paragraphHasHistory(""), false);
  assert.equal(paragraphHasHistory(" \n\t"), false);
  assert.equal(paragraphHasHistory("Texto"), true);
  const editor = readFileSync(new URL("./components/SectionEditor.tsx", import.meta.url), "utf8");
  assert.match(editor, /paragraphHasHistory\(p\.textContent/);
});

function sources(dir: string): string[] {
  return readdirSync(dir, { withFileTypes: true }).flatMap((entry) => {
    const path = join(dir, entry.name);
    if (entry.isDirectory()) return sources(path);
    return /\.tsx?$/.test(entry.name) && !entry.name.includes(".test.") ? [path] : [];
  });
}

test("P-118 (CSP): the page has no inline <style> element", () => {
  const src = fileURLToPath(new URL(".", import.meta.url));
  const files = sources(src);
  assert.ok(files.some((f) => f.endsWith("SectionEditor.tsx")));
  const page = readFileSync(new URL("../index.html", import.meta.url), "utf8");
  assert.doesNotMatch(page, /<style/i);
  let editors = 0;
  for (const file of files) {
    const code = readFileSync(file, "utf8");
    assert.doesNotMatch(code, /<style[\s>]|createElement\(\s*["']style["']/, file);
    // TipTap adds its own <style> unless told not to.
    for (const call of code.matchAll(/useEditor\(\{/g)) {
      editors++;
      assert.match(code.slice(call.index), /^useEditor\(\{\s*injectCSS: false,/, file);
    }
  }
  assert.ok(editors > 0, "no editor found");
});
