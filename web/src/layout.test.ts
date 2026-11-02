// U.1: the three panels' preferences, limits and routes (B-95, B-96, B-98).
import { test } from "node:test";
import assert from "node:assert/strict";
import {
  centerView,
  clampRightWidth,
  effectiveTab,
  initialPanels,
  nextWidthByKey,
  readPrefs,
  routeTarget,
  shortcutPanel,
  writePrefs,
} from "./layout.ts";

function memoryStore(seed: Record<string, string> = {}) {
  const data = new Map(Object.entries(seed));
  return {
    data,
    getItem: (k: string) => data.get(k) ?? null,
    setItem: (k: string, v: string) => void data.set(k, v),
  };
}

const DEFAULTS = { leftOff: false, rightOff: false, rightW: 420, rightTab: "frame" };

test("B-98: without saved preferences, both panels open, 420 px, «El marco»", () => {
  assert.deepEqual(readPrefs(() => memoryStore()), DEFAULTS);
});

test("B-98: preferences come back as they were written", () => {
  const store = memoryStore();
  writePrefs({ leftOff: true, rightOff: true, rightW: 512, rightTab: "history" }, () => store);
  assert.equal(store.data.get("tq.leftOff"), "1");
  assert.equal(store.data.get("tq.rightOff"), "1");
  assert.equal(store.data.get("tq.rightW"), "512");
  assert.equal(store.data.get("tq.rightTab"), "history");
  assert.deepEqual(readPrefs(() => store), { leftOff: true, rightOff: true, rightW: 512, rightTab: "history" });
});

test("B-98: a write names only what changed", () => {
  const store = memoryStore({ "tq.rightW": "500" });
  writePrefs({ leftOff: false }, () => store);
  assert.equal(store.data.get("tq.leftOff"), "0");
  assert.equal(store.data.get("tq.rightW"), "500");
});

test("B-98: a localStorage that throws gives the defaults and breaks nothing", () => {
  const denied = () => {
    throw new Error("denied");
  };
  const throwing = {
    getItem: () => {
      throw new Error("denied");
    },
    setItem: () => {
      throw new Error("quota");
    },
  };
  assert.deepEqual(readPrefs(denied), DEFAULTS);
  assert.deepEqual(readPrefs(() => throwing), DEFAULTS);
  assert.doesNotThrow(() => writePrefs({ leftOff: true }, denied));
  assert.doesNotThrow(() => writePrefs({ leftOff: true }, () => throwing));
});

test("B-98: a damaged saved value falls back to its default", () => {
  const store = memoryStore({ "tq.rightW": "wide", "tq.rightTab": "secrets" });
  assert.deepEqual(readPrefs(() => store), DEFAULTS);
});

test("B-98: the right panel stays between 300 px and 65 % of the window", () => {
  assert.equal(clampRightWidth(100, 2000), 300);
  assert.equal(clampRightWidth(420, 2000), 420);
  assert.equal(clampRightWidth(1900, 2000), 1300);
});

test("B-98: the center never drops under 360 px beside the open panels", () => {
  // 1280 - 264 (left) - 6 (divider) - 360 (center) = 650 of room; 65 % is 832.
  assert.equal(clampRightWidth(900, 1280, 264), 650);
  assert.equal(clampRightWidth(900, 1280, 52), 832);
});

test("B-98: where 300 px no longer fits, the minimum holds", () => {
  assert.equal(clampRightWidth(420, 800, 264), 300);
  assert.equal(clampRightWidth(420, 390), 300);
});

test("U.1: the arrows move the divider 24 px, and other keys do not", () => {
  assert.equal(nextWidthByKey(420, "ArrowLeft"), 444);
  assert.equal(nextWidthByKey(420, "ArrowRight"), 396);
  assert.equal(nextWidthByKey(420, "ArrowUp"), 420);
  assert.equal(nextWidthByKey(420, "a"), 420);
});

test("U.1: Ctrl or ⌘ with B folds the left, with «.» the right, and nothing else does", () => {
  assert.equal(shortcutPanel({ ctrlKey: true, metaKey: false, key: "b" }), "left");
  assert.equal(shortcutPanel({ ctrlKey: false, metaKey: true, key: "B" }), "left");
  assert.equal(shortcutPanel({ ctrlKey: true, metaKey: false, key: "." }), "right");
  assert.equal(shortcutPanel({ ctrlKey: false, metaKey: true, key: "." }), "right");
  assert.equal(shortcutPanel({ ctrlKey: false, metaKey: false, key: "b" }), null);
  assert.equal(shortcutPanel({ ctrlKey: true, metaKey: false, key: "k" }), null);
});

test("B-95: a section, its notices, its proposals and «Firmar» lead to the document", () => {
  for (const target of ["document", "proposals", "proposed-sections", "sign-section"]) {
    assert.deepEqual(routeTarget(target), { center: "doc", right: null }, target);
  }
});

test("B-95: a card, a «por aclarar», a round, the «Entendí» and the report lead to the conversation", () => {
  for (const target of ["card-dec-1", "oq-q-2", "round-r-3", "understanding", "stop-report"]) {
    assert.deepEqual(routeTarget(target), { center: "conv", right: null }, target);
  }
});

test("B-96: a history opens its tab and leaves the center as it is", () => {
  assert.deepEqual(routeTarget("history"), { center: null, right: "history" });
});

test("U.1: anything else moves nothing", () => {
  assert.deepEqual(routeTarget("frame-todo"), { center: null, right: null });
  assert.deepEqual(routeTarget(""), { center: null, right: null });
});

test("U.1: under 900 px both panels start folded, whatever was saved", () => {
  const open = { ...DEFAULTS } as Parameters<typeof initialPanels>[0];
  assert.deepEqual(initialPanels(open, 899), { leftOff: true, rightOff: true });
  assert.deepEqual(initialPanels(open, 900), { leftOff: false, rightOff: false });
  assert.deepEqual(initialPanels({ ...open, leftOff: true, rightOff: true }, 1400), { leftOff: true, rightOff: true });
});

test("U.1: without sections there is no document to show", () => {
  assert.equal(centerView("doc", false), "conv");
  assert.equal(centerView("doc", true), "doc");
  assert.equal(centerView("conv", true), "conv");
});

test("B-96: with its history closed, the «Historia» tab falls back to the frame", () => {
  assert.equal(effectiveTab("history", false), "frame");
  assert.equal(effectiveTab("history", true), "history");
  assert.equal(effectiveTab("frame", true), "frame");
});

test("U.3: the «Markdown» tab is remembered, and it needs no open history", () => {
  const store = memoryStore({ "tq.rightTab": "markdown" });
  assert.equal(readPrefs(() => store).rightTab, "markdown");
  assert.equal(effectiveTab("markdown", false), "markdown");
});
