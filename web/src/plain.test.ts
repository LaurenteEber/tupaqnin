// C.7: no ids and no jargon on screen.
import { test } from "node:test";
import assert from "node:assert/strict";
import { readFileSync, readdirSync } from "node:fs";
import { evidenceFromKey, hasInternalId, plainAgentText } from "./plain.ts";
import en from "./i18n/en.json" with { type: "json" };
import es from "./i18n/es.json" with { type: "json" };

const catalogs = { en, es } as Record<string, Record<string, string>>;
const componentsDir = new URL("./components/", import.meta.url).pathname;

test("C.7: an id in a page's text is caught", () => {
  for (const id of ["oq-10", "rnd-1", "dec-12", "card-3", "sec-2", "cit-4"]) {
    assert.equal(hasInternalId(`Texto con ${id} dentro.`), true, id);
  }
  assert.equal(hasInternalId("Una sec-ción y 10-20 casos"), false);
});

test("C.7: no catalog text carries an id, and no «{label}» reaches the page", () => {
  for (const [lang, catalog] of Object.entries(catalogs)) {
    for (const [key, text] of Object.entries(catalog)) {
      assert.equal(hasInternalId(text), false, `${lang} ${key}: ${text}`);
      assert.equal(text.includes("{label}"), false, `${lang} ${key}: ${text}`);
    }
  }
});

test("C.7: no component writes a label as the text of the page", () => {
  for (const file of readdirSync(componentsDir)) {
    const source = readFileSync(componentsDir + file, "utf8");
    // JSX text or a template that prints the object's label, not an attribute or a call.
    const printed = source.match(/>\s*\{[a-zA-Z.?!]*\.label\}\s*</g) ?? [];
    assert.deepEqual(printed, [], file);
    assert.equal(/`\$\{s\.label\}|: s\.label\)/.test(source), false, file);
  }
});

test("C.7: an id the model let slip leaves its words readable", () => {
  assert.equal(plainAgentText("Aplico tu decisión (dec-4) en la sección."), "Aplico tu decisión en la sección.");
  assert.equal(plainAgentText("Reviso oq-10."), "Reviso.");
  assert.equal(plainAgentText("«sec-1» queda lista"), "queda lista");
  assert.equal(plainAgentText("Sin códigos."), "Sin códigos.");
  assert.equal(hasInternalId(plainAgentText("rnd-1 y dec-2 y card-3")), false);
});

test("C.7: «falta material» shows in the language of the page; author and year as written", () => {
  assert.equal(evidenceFromKey("falta material"), "understanding.plan.no_material");
  assert.equal(evidenceFromKey(" Falta material "), "understanding.plan.no_material");
  assert.equal(evidenceFromKey("Paredes y Núñez, 2023"), "");
  assert.equal(en["understanding.plan.no_material"], "material is missing");
  assert.equal(es["understanding.plan.no_material"], "falta material");
});

// §3 of pruebas-usuarios-2nov.md, each text as the tests found it.
const OLD: [string, string][] = [
  ["round.doubt", "Doubt it reduces"], ["round.expectation", "What you expect"], ["round.no_expectation", "no expectation"],
  ["round.title", "Round {label}"], ["decision.object", "Object"], ["oq.undeclared", "The agent did not declare it"],
  ["digest.leave_open", "Leave it to clarify"], ["card.reason.covers_figure", "figure"],
  ["card.reason.two_or_more_sections", "two or more sections"], ["turn.stop.no_work", "The agent stopped: this step left no new text or decisions."],
  ["signoff.result", "3 · Result (contrasted by {model})"], ["signoff.failed_cases", "failed cases: {n} of {max}"],
  ["signoff.origin_question", "Origin question"], ["understanding.concludes", "concludes here"],
  ["provenance.accept", "Accept as general knowledge"], ["warnings.agent_pending", "Agent pending: {pending}"],
  ["pending.unsourced_claim", "A claim without a source"], ["chat.decisions_hint", "Decisions are made in their cards, not in the chat."],
];
const OLD_ES: [string, string][] = [
  ["card.reason.two_or_more_sections", "dos o más secciones"], ["signoff.result", "3 · Resultado (contrasta {model})"],
  ["signoff.failed_cases", "casos fallidos: {n} de {max}"], ["signoff.origin_question", "Pregunta de origen"],
  ["understanding.concludes", "aquí se concluye"], ["provenance.accept", "Aceptar como conocimiento general"],
  ["warnings.agent_pending", "Pendiente del agente: {pending}"], ["pending.unsourced_claim", "Una afirmación sin fuente"],
];

test("C.7: each text of the list changed, in both languages", () => {
  for (const [key, old] of OLD) assert.notEqual(en[key as keyof typeof en], old, key);
  for (const [key, old] of OLD_ES) assert.notEqual(es[key as keyof typeof es], old, key);
  assert.equal(es["chat.decisions_hint"], "Las decisiones críticas se toman en sus tarjetas");
});

test("C.7: the agent's notes are not in a typewriter font", () => {
  const css = readFileSync(new URL("./styles.css", import.meta.url).pathname, "utf8");
  assert.equal(/\.triage \{[^}]*--mono/.test(css), false);
});
