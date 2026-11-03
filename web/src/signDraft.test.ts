// C.10, B-105: your explanation survives leaving the signoff or a failed attempt.
import { test } from "node:test";
import assert from "node:assert/strict";
import { clearSignDraft, loadSignDraft, saveSignDraft, signDraftKey, SIGN_DRAFT_DELAY_MS } from "./signDraft.ts";

function memory() {
  const data = new Map<string, string>();
  return {
    data,
    getItem: (k: string) => data.get(k) ?? null,
    setItem: (k: string, v: string) => void data.set(k, v),
    removeItem: (k: string) => void data.delete(k),
  };
}

const blocked = () => {
  throw new Error("blocked");
};
const refusing = {
  getItem: blocked,
  setItem: blocked,
  removeItem: blocked,
};

test("C10: the draft goes under tq.sign.{project}.{section}, every second", () => {
  assert.equal(signDraftKey("p1", "sec-2"), "tq.sign.p1.sec-2");
  assert.equal(SIGN_DRAFT_DELAY_MS, 1000);
});

test("C10: saving, recovering and clearing", () => {
  const s = memory();
  assert.equal(loadSignDraft(() => s, "p1", "sec-2"), "");
  saveSignDraft(() => s, "p1", "sec-2", "Mi explicación.");
  assert.equal(s.data.get("tq.sign.p1.sec-2"), "Mi explicación.");
  assert.equal(loadSignDraft(() => s, "p1", "sec-2"), "Mi explicación.");
  assert.equal(loadSignDraft(() => s, "p1", "sec-3"), "", "another section's is its own");
  clearSignDraft(() => s, "p1", "sec-2");
  assert.equal(loadSignDraft(() => s, "p1", "sec-2"), "");
});

test("C10: emptying the text leaves no draft", () => {
  const s = memory();
  saveSignDraft(() => s, "p1", "sec-2", "Algo.");
  saveSignDraft(() => s, "p1", "sec-2", "");
  assert.equal(s.data.size, 0);
});

test("C10: a storage that throws changes nothing", () => {
  for (const store of [() => refusing, blocked as () => typeof refusing]) {
    assert.equal(loadSignDraft(store, "p1", "sec-2"), "");
    assert.doesNotThrow(() => saveSignDraft(store, "p1", "sec-2", "Algo."));
    assert.doesNotThrow(() => clearSignDraft(store, "p1", "sec-2"));
  }
});
