// B3-22 row 8: what comes from outside never runs in the site. Run with
// `pnpm test`.
import { test } from "node:test";
import assert from "node:assert/strict";
import { readFileSync, readdirSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { join } from "node:path";
import { safeLink, sourceHref } from "./provenance.ts";

const src = fileURLToPath(new URL(".", import.meta.url));

function sourceFiles(dir: string): string[] {
  return readdirSync(dir, { withFileTypes: true }).flatMap((entry) => {
    const path = join(dir, entry.name);
    if (entry.isDirectory()) return sourceFiles(path);
    return /\.tsx?$/.test(entry.name) && !/\.test\.tsx?$/.test(entry.name) ? [path] : [];
  });
}

test("B3-22 row 8: no file of web/src uses innerHTML or dangerouslySetInnerHTML", () => {
  const files = sourceFiles(src);
  assert.ok(files.length > 10, `read ${files.length} files`);
  assert.ok(files.some((f) => f.endsWith("App.tsx")), "App.tsx is among them");
  for (const file of files) {
    assert.doesNotMatch(readFileSync(file, "utf8"), /innerHTML/i, file);
  }
});

test("B3-22 row 8: safeLink cancels javascript: and keeps http and https", () => {
  for (const bad of ["javascript:alert(1)", "JaVaScRiPt:alert(1)", " javascript:alert(1)", "data:text/html,x", ""]) {
    assert.equal(safeLink(bad), "", bad);
  }
  assert.equal(safeLink(undefined), "");
  for (const good of ["https://a.example/x", "http://a.example/x"]) {
    assert.equal(safeLink(good), good);
  }
});

test("B3-22 row 8: a finding's link goes through safeLink", () => {
  assert.equal(sourceHref("p", { source_url: "javascript:alert(1)" }), "");
  assert.equal(
    sourceHref("p", { source_url: "javascript:alert(1)", source: "src-1" }),
    "/api/projects/p/sources/src-1/download",
  );
  assert.equal(sourceHref("p", { source_url: "https://a.example/x" }), "https://a.example/x");
  assert.equal(sourceHref("p", { source: "src-1" }), "/api/projects/p/sources/src-1/download");
});
