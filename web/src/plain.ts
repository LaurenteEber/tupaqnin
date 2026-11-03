// C.7, rule 7: no internal id on screen. The model is told not to write them, and
// a slip still reaches the page: its words go through here before showing.

const INTERNAL_ID = /\b(?:oq|rnd|dec|card|sec|cit|src|chg|fig|aq|new)-\d+\b/;

// hasInternalId is what the tests and the smoke look for in a page's text.
export function hasInternalId(text: string): boolean {
  return INTERNAL_ID.test(text);
}

// plainAgentText takes the ids out of what the agent wrote, with the parentheses
// or quotes that only held them, and leaves the rest of its words as they are.
export function plainAgentText(text: string): string {
  if (!hasInternalId(text)) return text;
  const id = INTERNAL_ID.source;
  return text
    .replace(new RegExp(`\\s*[(«"]${id}[)»"]`, "g"), "")
    .replace(new RegExp(`\\s*${id}`, "g"), "")
    .replace(new RegExp(`^${id}\\s*`), "")
    .replace(/ +([.,;:])/g, "$1")
    .trim();
}

// B-53: the model writes the one phrase «falta material» when a section has no
// source yet; shown in the page's language (R-6). Author and year go as written.
export function evidenceFromKey(value: string): string {
  return value.trim().toLowerCase() === "falta material" ? "understanding.plan.no_material" : "";
}
