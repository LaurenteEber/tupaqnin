// C.1, B-102: the tab that reloads keeps editing.
import { test } from "node:test";
import assert from "node:assert/strict";
import { TAB_ID_KEY, editorReleaseRequest, readTabId, takeOverResult } from "./tab.ts";

function memory() {
  const saved = new Map<string, string>();
  return {
    getItem: (k: string) => saved.get(k) ?? null,
    setItem: (k: string, v: string) => void saved.set(k, v),
  };
}

test("B-102: the tab id is kept between two reads of sessionStorage", () => {
  const store = memory();
  let made = 0;
  const make = () => `id-${++made}`;
  const first = readTabId(() => store, make);
  assert.equal(first, "id-1");
  assert.equal(store.getItem(TAB_ID_KEY), "id-1");
  assert.equal(readTabId(() => store, make), first, "a reload reads the same id");
  assert.equal(made, 1);
});

test("B-102: a tab with its own storage gets its own id", () => {
  let made = 0;
  const make = () => `id-${++made}`;
  assert.notEqual(readTabId(memory, make), readTabId(memory, make));
});

test("B-102: when sessionStorage throws, on reading or on writing, an id is created", () => {
  assert.equal(readTabId(() => { throw new Error("blocked"); }, () => "fresh"), "fresh");
  const throwing = { getItem: () => { throw new Error("blocked"); }, setItem: () => {} };
  assert.equal(readTabId(() => throwing, () => "fresh"), "fresh");
  const full = { getItem: () => null, setItem: () => { throw new Error("quota"); } };
  assert.equal(readTabId(() => full, () => "fresh"), "fresh");
});

test("B-102: the release is a DELETE with the tab, and survives the page going", () => {
  const req = editorReleaseRequest("tab-1");
  assert.equal(req.method, "DELETE");
  assert.equal(req.keepalive, true);
  assert.deepEqual(JSON.parse(req.body), { tab_id: "tab-1" });
});

test("B-102: a refused take-over explains that another tab is editing", () => {
  assert.equal(takeOverResult(true), "");
  assert.equal(takeOverResult(false), "busy");
});
