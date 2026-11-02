// U.3 (B-97): the Markdown tab's own renderer. React writes the text, so what the
// document says can never become markup (B3-22 row 8).
import { test } from "node:test";
import assert from "node:assert/strict";
import { createElement, Fragment } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { parseInline, parseMarkdown, renderMarkdown } from "./markdown.ts";

const html = (src: string) => renderToStaticMarkup(createElement(Fragment, null, renderMarkdown(src)));

test("B-97: headings # to ###, and #### stays a paragraph", () => {
  assert.equal(html("# One"), "<h1>One</h1>");
  assert.equal(html("## Two"), "<h2>Two</h2>");
  assert.equal(html("### Three"), "<h3>Three</h3>");
  assert.equal(html("#### Four"), "<p>#### Four</p>");
  assert.equal(html("#NoSpace"), "<p>#NoSpace</p>");
});

test("B-97: paragraphs split on a blank line and keep their inner line breaks", () => {
  assert.equal(html("First\nstill first\n\nSecond"), "<p>First\nstill first</p><p>Second</p>");
  assert.equal(html("Windows\r\n\r\nends"), "<p>Windows</p><p>ends</p>");
});

test("B-97: lists with - and 1.", () => {
  assert.equal(html("- a\n- b"), "<ul><li>a</li><li>b</li></ul>");
  assert.equal(html("1. a\n2. b"), "<ol><li>a</li><li>b</li></ol>");
  assert.equal(html("- a\n1. b"), "<ul><li>a</li></ul><ol><li>b</li></ol>");
});

test("B-97: a heading or a list right under a paragraph starts its own block", () => {
  assert.equal(html("text\n## Title\nmore"), "<p>text</p><h2>Title</h2><p>more</p>");
  assert.equal(html("text\n- item"), "<p>text</p><ul><li>item</li></ul>");
});

test("B-97: bold, italics and code, also inside headings and lists", () => {
  assert.equal(html("a **b** c"), "<p>a <strong>b</strong> c</p>");
  assert.equal(html("a *b* c"), "<p>a <em>b</em> c</p>");
  assert.equal(html("a `b` c"), "<p>a <code>b</code> c</p>");
  assert.equal(html("## **b**"), "<h2><strong>b</strong></h2>");
  assert.equal(html("- *b*"), "<ul><li><em>b</em></li></ul>");
});

test("B-97: code keeps its marks as text", () => {
  assert.equal(html("`**a**`"), "<p><code>**a**</code></p>");
});

test("B-97: a lone mark or a multiplication is text, not emphasis", () => {
  assert.equal(html("2 * 3 * 4"), "<p>2 * 3 * 4</p>");
  assert.equal(html("a *b"), "<p>a *b</p>");
  assert.equal(html("a **b"), "<p>a **b</p>");
  assert.equal(html("a `b"), "<p>a `b</p>");
  assert.deepEqual(parseInline("****"), [{ kind: "text", text: "****" }]);
});

test("B-97 (B3-22 row 8): a <script> in the text comes out escaped", () => {
  const out = html("<script>alert(1)</script>");
  assert.doesNotMatch(out, /<script/);
  assert.match(out, /&lt;script&gt;alert\(1\)&lt;\/script&gt;/);
});

test("B-97 (B3-22 row 8): an <img onerror> comes out escaped everywhere markdown can put it", () => {
  const bad = '<img src=x onerror="alert(1)">';
  for (const src of [bad, `# ${bad}`, `- ${bad}`, `1. ${bad}`, `**${bad}**`, `*${bad}*`, `\`${bad}\``]) {
    assert.doesNotMatch(html(src), /<img/, src);
    assert.match(html(src), /&lt;img/, src);
  }
});

test("B-97: links and raw HTML are not interpreted", () => {
  assert.doesNotMatch(html("[x](javascript:alert(1))"), /<a/);
  assert.doesNotMatch(html("<b>x</b>"), /<b>/);
});

test("B-97: an empty text does not break", () => {
  assert.deepEqual(parseMarkdown(""), []);
  assert.deepEqual(parseMarkdown("\n\n  \n"), []);
  assert.equal(html(""), "");
});

test("B-97: what the document says stays literally the same (rule 8)", () => {
  assert.equal(html("Conteos: 4 partículas por litro · «cita» ñ"), "<p>Conteos: 4 partículas por litro · «cita» ñ</p>");
});

test("P-118: a «> » line is a quote, and the export's unsigned mark shows no «>»", () => {
  assert.equal(html("> Not signed"), "<blockquote>Not signed</blockquote>");
  assert.equal(html("## Findings\n\n> Not signed\n\nText"), "<h2>Findings</h2><blockquote>Not signed</blockquote><p>Text</p>");
});

test("P-118: consecutive «> » lines are one quote, with emphasis inside", () => {
  assert.equal(html("> one\n> **two**"), "<blockquote>one\n<strong>two</strong></blockquote>");
  assert.equal(html("> one\n\n> two"), "<blockquote>one</blockquote><blockquote>two</blockquote>");
  assert.equal(html("text\n> quote\nmore"), "<p>text</p><blockquote>quote</blockquote><p>more</p>");
});

test("P-118: a «>» that is not a quote mark stays text", () => {
  assert.equal(html(">no space"), "<p>&gt;no space</p>");
  assert.equal(html("5 > 3"), "<p>5 &gt; 3</p>");
});

test("P-118 (B3-22 row 8): markup inside a quote comes out escaped", () => {
  const out = html('> <script>alert(1)</script> <img src=x onerror="a()">');
  assert.doesNotMatch(out, /<script|<img/);
  assert.match(out, /^<blockquote>&lt;script&gt;/);
});
