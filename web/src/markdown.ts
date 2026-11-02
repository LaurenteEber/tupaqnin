// The Markdown tab's minimal renderer (B-97): headings # to ###, paragraphs, lists
// with - and 1., «> » quotes (P-118), **bold**, *italics* and `code`. It builds React elements, never an
// HTML string: React writes the text, so nothing in the document can become markup
// (B3-22 row 8).
import { createElement, type ReactNode } from "react";

export type Inline = { kind: "text" | "strong" | "em" | "code"; text: string };

export type Block =
  | { kind: "h1" | "h2" | "h3" | "p" | "blockquote"; inlines: Inline[] }
  | { kind: "ul" | "ol"; items: Inline[][] };

// Code first, so its marks stay text. An opening or closing mark does not touch a
// space, so «2 * 3 * 4» is not emphasis.
const INLINE = /`([^`]+)`|\*\*(?!\s)(.+?)(?<!\s)\*\*|\*(?![\s*])(.+?)(?<![\s*])\*/gs;

export function parseInline(text: string): Inline[] {
  const out: Inline[] = [];
  let last = 0;
  for (const m of text.matchAll(INLINE)) {
    if (m.index > last) out.push({ kind: "text", text: text.slice(last, m.index) });
    if (m[1] !== undefined) out.push({ kind: "code", text: m[1] });
    else if (m[2] !== undefined) out.push({ kind: "strong", text: m[2] });
    else out.push({ kind: "em", text: m[3] });
    last = m.index + m[0].length;
  }
  if (last < text.length) out.push({ kind: "text", text: text.slice(last) });
  return out;
}

const HEADING = /^(#{1,3}) +(.*)$/;
const BULLET = /^- +(.*)$/;
const NUMBERED = /^\d+\. +(.*)$/;
const QUOTE = /^> (.*)$/;

export function parseMarkdown(src: string): Block[] {
  const blocks: Block[] = [];
  let paragraph: string[] = [];
  let quote: string[] = [];
  const closeParagraph = () => {
    if (paragraph.length > 0) blocks.push({ kind: "p", inlines: parseInline(paragraph.join("\n")) });
    paragraph = [];
    if (quote.length > 0) blocks.push({ kind: "blockquote", inlines: parseInline(quote.join("\n")) });
    quote = [];
  };
  // An item continues the list right above it; after a blank line, a new list.
  const addItem = (kind: "ul" | "ol", text: string, continues: boolean) => {
    closeParagraph();
    const last = blocks[blocks.length - 1];
    if (continues && last && last.kind === kind) last.items.push(parseInline(text));
    else blocks.push({ kind, items: [parseInline(text)] });
  };
  let inList = false;
  for (const line of src.replaceAll("\r\n", "\n").split("\n")) {
    const heading = line.match(HEADING);
    const bullet = line.match(BULLET);
    const numbered = line.match(NUMBERED);
    const quoted = line.match(QUOTE);
    if (line.trim() === "") {
      closeParagraph();
      inList = false;
    } else if (quoted) {
      if (paragraph.length > 0) closeParagraph();
      inList = false;
      quote.push(quoted[1]);
    } else if (heading) {
      closeParagraph();
      inList = false;
      blocks.push({ kind: `h${heading[1].length}` as "h1" | "h2" | "h3", inlines: parseInline(heading[2]) });
    } else if (bullet || numbered) {
      addItem(bullet ? "ul" : "ol", (bullet ?? numbered)![1], inList);
      inList = true;
    } else {
      inList = false;
      if (quote.length > 0) closeParagraph();
      paragraph.push(line);
    }
  }
  closeParagraph();
  return blocks;
}

function inlineNodes(inlines: Inline[]): ReactNode[] {
  return inlines.map((inline, i) =>
    inline.kind === "text" ? inline.text : createElement(inline.kind, { key: i }, inline.text),
  );
}

export function renderMarkdown(src: string): ReactNode[] {
  return parseMarkdown(src).map((block, i) => {
    if ("items" in block) {
      const items = block.items.map((item, j) => createElement("li", { key: j }, ...inlineNodes(item)));
      return createElement(block.kind, { key: i }, ...items);
    }
    return createElement(block.kind, { key: i }, ...inlineNodes(block.inlines));
  });
}
