// C.3: the project in the address, the language in the browser.
import { test } from "node:test";
import assert from "node:assert/strict";
import { LANG_KEY, pickLang, projectHash, readProjectHash, saveLang } from "./route.ts";

test("C.3: the hash is read", () => {
  assert.equal(readProjectHash("#/p/01J9XYZ_a-b"), "01J9XYZ_a-b");
  assert.equal(readProjectHash(""), "");
  assert.equal(readProjectHash("#"), "");
  assert.equal(readProjectHash("#/p/"), "");
  assert.equal(readProjectHash("#/p/a/b"), "", "only one project");
  assert.equal(readProjectHash("#/p/..%2Fapi"), "", "nothing that leaves the project's path");
  assert.equal(readProjectHash("#/x/abc"), "");
});

test("C.3: the hash is written, and reads back", () => {
  assert.equal(projectHash("abc"), "#/p/abc");
  assert.equal(projectHash(""), "");
  assert.equal(readProjectHash(projectHash("abc")), "abc");
});

function memory(initial: Record<string, string> = {}) {
  const saved = new Map(Object.entries(initial));
  return { getItem: (k: string) => saved.get(k) ?? null, setItem: (k: string, v: string) => void saved.set(k, v) };
}

test("C.3: the language you chose stays; the address can still ask for one", () => {
  const store = memory();
  assert.equal(pickLang("", () => store), "en");
  saveLang("es", () => store);
  assert.equal(store.getItem(LANG_KEY), "es");
  assert.equal(pickLang("", () => store), "es");
  assert.equal(pickLang("?lang=en", () => store), "en");
  assert.equal(pickLang("?lang=es", () => memory()), "es");
  assert.equal(pickLang("?lang=fr", () => memory({ [LANG_KEY]: "es" })), "es");
});

test("C.3: a blocked localStorage does not break the language", () => {
  const blocked = () => { throw new Error("blocked"); };
  assert.equal(pickLang("", blocked), "en");
  assert.doesNotThrow(() => saveLang("es", blocked));
  const full = { getItem: () => null, setItem: () => { throw new Error("quota"); } };
  assert.doesNotThrow(() => saveLang("es", () => full));
});
