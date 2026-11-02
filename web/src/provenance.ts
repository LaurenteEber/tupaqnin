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

// Where a finding's passage opens: its source's link (the URL an API
// gave, B-58) or, for your material, its download; "" when it has none.
export function sourceHref(projectID: string, f: { source?: string; source_url?: string }): string {
  const link = safeLink(f.source_url); // no javascript: link (B3-22 row 8)
  if (link) return link;
  if (f.source) return `/api/projects/${projectID}/sources/${f.source}/download`;
  return "";
}

// The custom property holding an origin's label in the editor's marks.
export function originVar(origin: string): string {
  return `--origin-${origin.replace(/_/g, "-")}`;
}

const MARKED_ORIGINS = ["material", "external", "model_knowledge", "unverified"];

// The classes of paragraph n's mark (B-10), block n: styles.css draws the label
// from its custom property. Only an origin the catalog knows gets a class, and no
// rule is written at run time (the CSP forbids an inline style element, P-118).
// [EL DISEÑO NO LO DICE]: how the mark looks; a small note at the end of the
// paragraph, green when verified, in the colour of what is pending when not.
export function originMarkClass(origins: ParagraphOrigin[], block: number): string {
  const origin = origins.find((o) => o.block === block)?.origin;
  if (!origin || !MARKED_ORIGINS.includes(origin)) return "";
  return `origin-mark origin-mark-${origin.replace(/_/g, "-")}`;
}

// A paragraph with no text is no block of Go's: it has no history to ask for (P-118).
export function paragraphHasHistory(text: string): boolean {
  return text.trim() !== "";
}
