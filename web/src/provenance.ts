// Where a paragraph comes from (backlog-r3 3.5, R-1, R-2): Go computes the
// origin; this only picks its line and writes its CSS.

export type ParagraphOrigin = { block: number; origin: string };

// The catalog's line for an origin: material · external · model_knowledge ·
// unverified.
export function originKey(origin: string): string {
  return `provenance.origin.${origin}`;
}

// A searched source as the page names it: title and, when it has one, year.
export function searchedName(s: { title: string; year?: number }): string {
  return s.year ? `${s.title} (${s.year})` : s.title;
}

// A candidate as the page names it: how the text names it, with what the
// agent says of it.
export function candidateName(c: { text: string; title?: string; year?: number }): string {
  const known = [c.title, c.year].filter(Boolean).join(", ");
  return known ? `${c.text} (${known})` : c.text;
}

// An API's link, only when it is http or https: the page never makes an
// anchor of anything else.
export function safeLink(url?: string): string {
  return url && /^https?:\/\//i.test(url) ? url : "";
}

// The custom property holding an origin's label in the editor's marks.
export function originVar(origin: string): string {
  return `--origin-${origin.replace(/_/g, "-")}`;
}

// The editor's marks (B-10): after each paragraph n, block n, its origin's
// label. Labels and block numbers only.
// [EL DISEÑO NO LO DICE]: how the mark looks; a small note at the end of the
// paragraph, green when verified, in the colour of what is pending when not.
export function originMarkRules(section: string, origins: ParagraphOrigin[]): string {
  const rules: string[] = [];
  for (const origin of ["material", "external", "model_knowledge", "unverified"]) {
    const blocks = origins.filter((o) => o.origin === origin).map((o) => o.block);
    if (blocks.length === 0) continue;
    const verified = origin === "material" || origin === "external";
    const selector = blocks
      .map((n) => `.section-editor[data-section="${section}"] .section-editor-content > p:nth-child(${n})::after`)
      .join(",\n");
    rules.push(`${selector} { content: var(${originVar(origin)}); margin-left: 0.5em; font-size: 0.75em; ` +
      `color: var(${verified ? "--ok" : "--todo"}); background: var(${verified ? "--ok-soft" : "--todo-soft"}); ` +
      "border-radius: 4px; padding: 0 4px; }");
  }
  return rules.join("\n");
}
