// C.19 (B-109): while the understanding is unconfirmed, its card says the agent already started.
import { test } from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { understandingUnconfirmed } from "./understandingState.ts";
import en from "./i18n/en.json" with { type: "json" };
import es from "./i18n/es.json" with { type: "json" };

const confirmed = {
  plan_confirmed: true,
  cases: [{ state: "confirmed" }, { state: "corrected" }],
  sections: [{ role_state: "confirmed" }],
};

test("C19: everything answered is not unconfirmed, so the line does not show", () => {
  assert.equal(understandingUnconfirmed(confirmed), false);
  assert.equal(understandingUnconfirmed({ plan_confirmed: false, cases: [], sections: [] }), false, "no plan to answer");
});

test("C19: an unconfirmed case, a proposed mark or an unanswered plan is unconfirmed", () => {
  assert.equal(understandingUnconfirmed({ ...confirmed, cases: [{ state: "unconfirmed" }] }), true);
  assert.equal(understandingUnconfirmed({ ...confirmed, sections: [{ role_state: "proposed" }] }), true);
  assert.equal(understandingUnconfirmed({ ...confirmed, plan_confirmed: false }), true);
});

test("C19: the line, in both languages, once each", () => {
  assert.equal(es["understanding.agent_started"], "El agente ya empezó. Si corriges el Entendí, rehace lo afectado");
  assert.equal(en["understanding.agent_started"], "The agent has already started. If you correct what it understood, it redoes what is affected");
  for (const lang of ["en", "es"]) {
    const raw = readFileSync(new URL(`./i18n/${lang}.json`, import.meta.url), "utf8");
    assert.equal(raw.split('"understanding.agent_started"').length - 1, 1, lang);
  }
});
