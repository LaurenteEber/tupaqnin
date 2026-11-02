// What «Object» does when you press it (P-115): with a reason already written on
// the card it sends it; with none it asks for one. Go decides whether it counts.
export function objectPress(preset?: string): { reason: string; send: boolean } {
  const reason = (preset ?? "").trim();
  return { reason, send: reason !== "" };
}

// A pending the agent can no longer close at a cap (B3-20, P-115): what the checker
// did not see says so.
export function atCapPendingKey(kind: string): string {
  return kind === "unchecked" ? "pending.unchecked_cap" : `pending.${kind}`;
}
