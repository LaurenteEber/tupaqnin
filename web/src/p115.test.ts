import test from "node:test";
import assert from "node:assert/strict";
import { atCapPendingKey, objectPress } from "./decisionActs.ts";
import { modeBarKey } from "./live.ts";
import { editingKind } from "./saveStatus.ts";
import en from "./i18n/en.json" with { type: "json" };
import es from "./i18n/es.json" with { type: "json" };

const has = (key: string) => assert.ok((en as Record<string, string>)[key] && (es as Record<string, string>)[key], key);

test("P-115: «Object» sends the reason on the card, and with none it asks", () => {
  assert.deepEqual(objectPress("Porque esconde los extremos."), { reason: "Porque esconde los extremos.", send: true });
  assert.deepEqual(objectPress("   "), { reason: "", send: false });
  assert.deepEqual(objectPress(undefined), { reason: "", send: false });
  has("decision.error.empty_reason");
  has("decision.object_send");
});

test("P-115: «Saved» only when the text on screen is what was saved", () => {
  assert.equal(editingKind("texto nuevo", "texto"), "unsaved");
  assert.equal(editingKind("texto", "texto"), "saved");
});

test("P-115: an objection's question names the decision, with your reason apart", () => {
  has("oq.objection.reason");
  assert.equal((es as Record<string, string>)["oq.objection.question"], "¿Mantener «{statement}»?");
});

test("P-115: what the checker did not see at a cap says so", () => {
  assert.equal(atCapPendingKey("unchecked"), "pending.unchecked_cap");
  assert.equal(atCapPendingKey("open_card"), "pending.open_card");
  has("pending.unchecked_cap");
});

test("P-115: the bar says when the mode waits for your signoff", () => {
  assert.equal(modeBarKey(true), "mode.waits_signoff");
  assert.equal(modeBarKey(false), "mode.run_to_completion");
  has("mode.waits_signoff");
});
