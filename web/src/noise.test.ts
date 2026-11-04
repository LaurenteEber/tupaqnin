// C.13: larger base type, the agent's note once per step, and a counter on the cards.
import { test } from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { cardCounter, countKey, withoutHistoryNotes } from "./noise.ts";
import en from "./i18n/en.json" with { type: "json" };
import es from "./i18n/es.json" with { type: "json" };

test("C13: a note already in the history does not show again in the stream", () => {
  const stream = [
    { field: "triage_line", text: "Construir: uno." },
    { field: "note", text: "La nota del paso uno." },
    { field: "reply", text: "Respuesta." },
    { field: "note", text: "La nota del paso dos." },
  ];
  const entries = [{ kind: "step", note: "La nota del paso uno." }];
  assert.deepEqual(withoutHistoryNotes(stream, entries), [
    { field: "triage_line", text: "Construir: uno." },
    { field: "reply", text: "Respuesta." },
    { field: "note", text: "La nota del paso dos." },
  ]);
  assert.equal(withoutHistoryNotes(stream, []).length, 4, "nothing in the history yet");
});

test("C13: the counter, in both languages", () => {
  assert.deepEqual(cardCounter(1, 5), { n: "2", total: "5" });
  assert.equal(en["card.counter"], "Card {n} of {total} in this conversation");
  assert.equal(es["card.counter"], "Tarjeta {n} de {total} en esta conversación");
});

test("C13: the base type is 15 px", () => {
  const css = readFileSync(new URL("./styles.css", import.meta.url), "utf8");
  assert.match(css, /font: 15px\/1\.5 var\(--ui\)/);
});

test("C.26: the header and the cards say what each counts", () => {
  assert.equal(es["status.yours.many"], "Te toca: {n} cosas en todo el proyecto");
  assert.equal(es["status.yours.one"], "Te toca: 1 cosa en todo el proyecto");
  assert.match(en["status.yours.many"], /across the project/);
  assert.match(en["status.yours.one"], /across the project/);
});

test("C.26: the polish of the screens stays in their source", () => {
  const read = (path: string) => readFileSync(new URL(path, import.meta.url), "utf8");
  assert.match(read("./components/SectionEditor.tsx"), /spellcheck: "false"/);
  assert.match(read("./components/FramePanel.tsx"), /onMove && active && /);
  assert.match(read("./styles.css"), /^input, textarea, select \{ font: inherit; \}/m);
  for (const panel of ["./components/RequestPanel.tsx", "./components/UnderstandingPanel.tsx"]) {
    assert.doesNotMatch(read(panel), /type="file"/);
    assert.match(read(panel), /<FileButton/);
  }
  assert.match(read("./components/CardPanel.tsx"), /oq-option-body/);
});

test("P-129: one is singular in the frame's counts, in both languages", () => {
  assert.equal(countKey("frame.unverified", 1), "frame.unverified.one");
  assert.equal(countKey("frame.unverified", 2), "frame.unverified");
  assert.equal(es["frame.unverified.one"], "1 párrafo sin verificar");
  assert.equal(en["frame.unverified.one"], "1 unverified paragraph");
  assert.equal(es["section.pendings.one"], "1 pendiente");
  assert.equal(en["section.pendings.one"], "1 pending");
  assert.equal(es["section.pendings"], "{n} pendientes");
  assert.equal(es["frame.unverified"], "{n} párrafos sin verificar");
  const read = (path: string) => readFileSync(new URL(path, import.meta.url), "utf8");
  assert.doesNotMatch(read("./components/FramePanel.tsx"), /t\("(section\.pendings|frame\.unverified)"/);
  assert.doesNotMatch(read("./App.tsx"), /t\("section\.pendings"/);
});

test("P-129: one unconfirmed case is singular too", () => {
  assert.equal(es["frame.todo.cases_n.one"], "1 caso");
  assert.equal(en["frame.todo.cases_n.one"], "1 case");
  const frame = readFileSync(new URL("./components/FramePanel.tsx", import.meta.url), "utf8");
  assert.match(frame, /countKey\("frame\.todo\.cases_n", frame\.todo\.unconfirmed_cases\)/);
});
