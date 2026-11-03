// C.9 (B-103): catching up before signing.
import { test } from "node:test";
import assert from "node:assert/strict";
import { correctionMessage, oneAtATime, pendingRoute, shownAbout } from "./catchUp.ts";
import { stopKey } from "./live.ts";
import en from "./i18n/en.json" with { type: "json" };
import es from "./i18n/es.json" with { type: "json" };

test("TestB103: a card and a «por aclarar» lead to them, anything else asks the agent", () => {
  assert.deepEqual(pendingRoute({ kind: "open_card", about: "card-2" }), { to: "reach", target: "card-card-2" });
  assert.deepEqual(pendingRoute({ kind: "open_question", about: "oq-3" }), { to: "reach", target: "oq-oq-3" });
  assert.deepEqual(pendingRoute({ kind: "figure_without_decision", about: "7" }), { to: "correct" });
  assert.deepEqual(pendingRoute({ kind: "unchecked" }), { to: "correct" });
});

test("TestB103: a label routes and never shows; what the text says does", () => {
  assert.equal(shownAbout({ kind: "open_card", about: "card-2" }), "");
  assert.equal(shownAbout({ kind: "open_question", about: "oq-3" }), "");
  assert.equal(shownAbout({ kind: "figure_without_decision", about: "7" }), "7");
  assert.equal(shownAbout({ kind: "unchecked" }), "");
});

test("TestB103: the message that stands for a correction is in the page's language", () => {
  const t = (lang: Record<string, string>) => (key: string, params?: Record<string, string>) =>
    Object.entries(params ?? {}).reduce((s, [k, v]) => s.replace(`{${k}}`, v), lang[key] ?? key);
  assert.equal(
    correctionMessage(t(en), { kind: "figure_without_decision", about: "7" }, "Síntesis"),
    "Correct «A figure without a decision · 7» in «Síntesis».",
  );
  assert.equal(
    correctionMessage(t(es), { kind: "figure_without_decision", about: "7" }, "Síntesis"),
    "Corrige «" + es["pending.figure_without_decision"] + " · 7» en «Síntesis».",
  );
  assert.equal(correctionMessage(t(en), { kind: "open_change", about: "chg-1" }, "Síntesis"),
    "Correct «" + en["signoff.open_change"] + "» in «Síntesis».", "a change's label never goes in the message");
});

test("TestB103: the two sections and the buttons, in en and es", () => {
  const strings: Record<string, [string, string]> = {
    "stop_report.decided": ["What was decided and why ({n})", "Lo que se decidió y por qué ({n})"],
    "stop_report.agent_changes": ["What the agent changed on its own ({n})", "Lo que el agente cambió por su cuenta ({n})"],
    "stop_report.agent_changes.see": ["See changes", "Ver cambios"],
    "doc.read_full": ["Read in full", "Leer completo"],
    "signoff.correction.ask": ["Ask the agent to correct it", "Pedir corrección al agente"],
    "signoff.go_to_card": ["Go to the card", "Ir a la tarjeta"],
    "signoff.answer_question": ["Answer it", "Responder el «por aclarar»"],
  };
  for (const [key, [inEn, inEs]] of Object.entries(strings)) {
    assert.equal((en as Record<string, string>)[key], inEn, key);
    assert.equal((es as Record<string, string>)[key], inEs, key);
  }
  for (const key of ["stop_report.decided.who.human", "stop_report.decided.who.agent", "stop_report.decided.because",
    "stop_report.decided.instead_of", "stop_report.agent_changes.step", "signoff.error.pending_closed", "signoff.error.pending_yours"]) {
    assert.ok((en as Record<string, string>)[key], `en ${key}`);
    assert.ok((es as Record<string, string>)[key], `es ${key}`);
  }
});

test("TestB103: the correction's stop has its own reason, in en and es", () => {
  const key = stopKey("correction_applied");
  assert.equal(key, "turn.stop.correction_applied");
  assert.ok((en as Record<string, string>)[key]);
  assert.ok((es as Record<string, string>)[key]);
  assert.notEqual((en as Record<string, string>)[key], en["turn.stop.decision_applied"]);
});

test("TestB103: a double click asks once, and a later click can ask again", async () => {
  const busy = { current: false };
  let runs = 0;
  let finish = () => {};
  const run = () => new Promise<void>((resolve) => {
    runs++;
    finish = resolve;
  });
  const first = oneAtATime(busy, run);
  await oneAtATime(busy, run);
  assert.equal(runs, 1);
  finish();
  await first;
  const again = oneAtATime(busy, run);
  assert.equal(runs, 2);
  finish();
  await again;
});

test("TestB103: a correction that left the pending open says so, in en and es", () => {
  const key = stopKey("correction_still_open");
  assert.equal(key, "turn.stop.correction_still_open");
  assert.match((en as Record<string, string>)[key], /still open/);
  assert.match((es as Record<string, string>)[key], /sigue abierto/);
});

test("TestB103: every code the correction's route answers has its message, in en and es", () => {
  for (const code of ["empty_message", "not_found", "pending_closed", "pending_yours", "invalid_body", "internal"]) {
    assert.ok((en as Record<string, string>)[`signoff.error.${code}`], `en ${code}`);
    assert.ok((es as Record<string, string>)[`signoff.error.${code}`], `es ${code}`);
  }
});
