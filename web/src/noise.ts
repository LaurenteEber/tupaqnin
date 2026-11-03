// C.13: less noise on screen. Plain functions, for `node --test`.

// A chain's stream keeps its earlier steps until it ends, and each ended step is
// already in the conversation: its note shows there, once.
export function withoutHistoryNotes<T extends { field?: string; text?: string }>(stream: T[], entries: { note?: string }[]): T[] {
  const inHistory = new Set(entries.map((e) => e.note).filter(Boolean));
  return stream.filter((piece) => piece.field !== "note" || !inHistory.has(piece.text ?? ""));
}

// «Tarjeta 2 de 5», from a card's place in the list.
export function cardCounter(index: number, total: number): { n: string; total: string } {
  return { n: String(index + 1), total: String(total) };
}
