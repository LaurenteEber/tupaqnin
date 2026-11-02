// P-116: the editor's save, the words of the stop report, and the frame's actions.
import { test } from "node:test";
import assert from "node:assert/strict";
import { readFileSync, readdirSync, statSync } from "node:fs";
import { join } from "node:path";
import { AUTOSAVE_DELAY_MS, createAutosave, editingKind, shouldAutosave } from "./saveStatus.ts";
import { detectedLine } from "./detected.ts";
import en from "./i18n/en.json" with { type: "json" };
import es from "./i18n/es.json" with { type: "json" };

const catalogs = { en, es } as Record<string, Record<string, string>>;
const srcDir = new URL(".", import.meta.url).pathname;

// A clock the test moves: what the autosave scheduled and has not been cancelled.
function fakeTimers() {
  let pending: { fn: () => void; ms: number } | null = null;
  return {
    schedule: (fn: () => void, ms: number) => {
      const timer = { fn, ms };
      pending = timer;
      return () => { if (pending === timer) pending = null; };
    },
    pendingMs: () => pending?.ms,
    fire: () => { const p = pending; pending = null; p?.fn(); },
  };
}

test("P-116: a change waits as «Unsaved changes», and «Saved» only once it matches", () => {
  assert.equal(editingKind("texto nuevo", "texto"), "unsaved");
  assert.equal(editingKind("texto", "texto"), "saved");
  assert.equal(catalogs.en["save.unsaved"], "Unsaved changes");
  assert.ok(catalogs.es["save.unsaved"]);
});

test("P-116: it saves 2 s after the last change, restarting with each change", async () => {
  assert.equal(AUTOSAVE_DELAY_MS, 2000);
  const timers = fakeTimers();
  let saves = 0;
  const auto = createAutosave(async () => { saves++; }, AUTOSAVE_DELAY_MS, timers.schedule);
  auto.changed();
  auto.changed();
  assert.equal(timers.pendingMs(), 2000);
  assert.equal(saves, 0);
  timers.fire();
  await auto.flush();
  assert.equal(saves, 2, "the timer's save, and the flush's own");
  assert.equal(timers.pendingMs(), undefined);
});

test("P-116: saving on blur cancels the wait, so one change is one save", async () => {
  const timers = fakeTimers();
  let saves = 0;
  const auto = createAutosave(async () => { saves++; }, AUTOSAVE_DELAY_MS, timers.schedule);
  auto.changed();
  await auto.flush();
  assert.equal(timers.pendingMs(), undefined);
  assert.equal(saves, 1);
});

test("P-116: saves never overlap: a second one waits for the PUT in flight", async () => {
  const timers = fakeTimers();
  let inFlight = 0;
  let most = 0;
  const release: (() => void)[] = [];
  const auto = createAutosave(() => new Promise<void>((done) => {
    inFlight++;
    most = Math.max(most, inFlight);
    release.push(() => { inFlight--; done(); });
  }), AUTOSAVE_DELAY_MS, timers.schedule);
  const first = auto.flush();
  const second = auto.flush();
  await new Promise((r) => setImmediate(r));
  assert.equal(auto.busy(), true);
  release.shift()!();
  await first;
  await new Promise((r) => setImmediate(r));
  release.shift()!();
  await second;
  assert.equal(most, 1);
  assert.equal(auto.busy(), false);
});

test("P-116: a failed save does not stop the next one", async () => {
  const timers = fakeTimers();
  let saves = 0;
  const auto = createAutosave(async () => { saves++; if (saves === 1) throw new Error("network"); }, AUTOSAVE_DELAY_MS, timers.schedule);
  await assert.rejects(auto.flush());
  await auto.flush();
  assert.equal(saves, 2);
});

test("P-116: closing the section drops the wait", () => {
  const timers = fakeTimers();
  const auto = createAutosave(async () => {}, AUTOSAVE_DELAY_MS, timers.schedule);
  auto.changed();
  auto.cancel();
  assert.equal(timers.pendingMs(), undefined);
});

test("P-116: a version conflict stays as it was: typing does not resend it", () => {
  assert.equal(shouldAutosave("nuevo", "viejo", false), true);
  assert.equal(shouldAutosave("nuevo", "viejo", true), false);
  assert.equal(shouldAutosave("igual", "igual", false), false);
});

// Every state the stop report can name (internal/graph/reentry.go, DetectedSinceStop).
const decisionStates = ["agent", "human", "confirmed", "superseded", "revoked", "in_open_question"];
const sectionStates = ["draft", "ready", "signed", "changed_since_signoff", "stale"];
const actors = ["human", "agent", "app"];
const detectedKinds = ["edit", "usage_change", "source_new_version", "open_question", "card", "section_state", "decision_state"];
// A key left raw reads like this: lowercase words joined by dots.
const rawKey = /\b[a-z_]+\.[a-z_]+(\.[a-z_]+)*\b/;

for (const [lang, catalog] of Object.entries(catalogs)) {
  const t = (key: string, params?: Record<string, string>) => {
    let text = catalog[key] ?? key;
    for (const [name, value] of Object.entries(params ?? {})) text = text.replace(`{${name}}`, value);
    return text;
  };
  test(`P-116: every line of «Detected» reads as words, no raw key (${lang})`, () => {
    for (const actor of actors) {
      for (const kind of detectedKinds) {
        for (const to of [...decisionStates, ...sectionStates]) {
          // Each kind names only its own states.
          if (kind === "decision_state" && !decisionStates.includes(to)) continue;
          if (kind === "section_state" && !sectionStates.includes(to)) continue;
          const line = detectedLine(t, { kind, actor, section: "Modelo", text: "Cobrar", to, count: 1 });
          assert.doesNotMatch(line, rawKey, line);
        }
      }
    }
    assert.doesNotMatch(detectedLine(t, { kind: "edit", actor: "human", section: "Modelo", count: 3 }), rawKey);
  });
}

function sources(dir: string): string[] {
  return readdirSync(dir).flatMap((name) => {
    const path = join(dir, name);
    if (statSync(path).isDirectory()) return sources(path);
    return /\.tsx?$/.test(name) && !name.includes(".test.") ? [path] : [];
  });
}

test("P-116: every key the page names, literal or by prefix, exists in both catalogs", () => {
  const missing = new Set<string>();
  for (const path of sources(srcDir)) {
    const code = readFileSync(path, "utf8");
    for (const m of code.matchAll(/\b(?:t|translate\(lang,)\(?\s*"([a-z_]+(?:\.[a-z_]+)+)"/g)) {
      for (const [lang, catalog] of Object.entries(catalogs)) {
        if (!catalog[m[1]]) missing.add(`${lang}: ${m[1]}`);
      }
    }
    for (const m of code.matchAll(/\b(?:t|translate\(lang,)\(?\s*`([a-z_]+(?:\.[a-z_]+)*\.)\$\{/g)) {
      for (const [lang, catalog] of Object.entries(catalogs)) {
        if (!Object.keys(catalog).some((k) => k.startsWith(m[1]))) missing.add(`${lang}: ${m[1]}*`);
      }
    }
  }
  assert.deepEqual([...missing], []);
});

test("P-116: «Object» and «Revoke» are separated in the frame", () => {
  const css = readFileSync(join(srcDir, "styles.css"), "utf8");
  const rule = css.match(/\.decision-acts\s*\{([^}]*)\}/);
  assert.ok(rule, "no rule for .decision-acts");
  assert.match(rule![1], /display:\s*flex/);
  assert.match(rule![1], /gap:/);
  assert.match(rule![1], /flex-wrap:\s*wrap/);
});
