// Backlog-r3 3.4: what the round card shows of the digest, as pure
// functions over what Go computed (B-63; pantallas «Dónde va cada vista
// de la rebanada 3»). Go counts; these only pick the catalog line.

// A line of the catalog with its parameters.
export interface Line {
  key: string;
  params: Record<string, string>;
}

interface Coverage {
  part: number;
  status: string;
  reason?: string;
  sources: number;
  sources_unread: string[];
  hits: number;
  hits_unread: string[];
  gaps: string[];
}

// «Sin revisar» (A-5): per reader, what it was given and left unread,
// «2 de 5 resultados sin leer», and an empty reader as such. A reader
// that read everything it was given has no line.
export function coverageLines(coverage: Coverage[]): Line[] {
  const out: Line[] = [];
  for (const c of coverage) {
    const reader = String(c.part);
    if (c.status === "empty") {
      out.push({ key: "digest.reader_empty", params: { reader } });
      continue;
    }
    if (c.hits_unread.length > 0) {
      out.push({
        key: "digest.hits_unread",
        params: { reader, unread: String(c.hits_unread.length), total: String(c.hits)},
      });
    }
    if (c.sources_unread.length > 0) {
      out.push({
        key: "digest.sources_unread",
        params: { reader, unread: String(c.sources_unread.length), total: String(c.sources)},
      });
    }
  }
  return out;
}

// The reason of a discarded or unverified finding, as a catalog key.
export function findingReasonKey(f: { verification: string; reason?: string }): string {
  return `digest.reason.${f.reason || f.verification}`;
}

// «Repetir con otra pregunta» and «dejarlo por aclarar» show only when
// Go offers the repeat (the doubt was not reduced) and the round was not
// left to clarify yet; neither runs on its own (No hacer).
export function offersRepeat(card: { repeat?: { estimated_cost_usd: number } | null }): boolean {
  return !!card.repeat;
}

export function offersLeaveOpen(card: { repeat?: { estimated_cost_usd: number } | null; scope_question?: string }): boolean {
  return !!card.repeat && !card.scope_question;
}
