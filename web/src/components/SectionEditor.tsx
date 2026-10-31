import { useEffect, useRef, type CSSProperties, type ReactNode } from "react";
import { useEditor, EditorContent, Extension } from "@tiptap/react";
import { Plugin, PluginKey } from "@tiptap/pm/state";
import { Decoration, DecorationSet } from "@tiptap/pm/view";
import type { Node as PMNode } from "@tiptap/pm/model";
import type { Warning } from "../api/graph";
import Document from "@tiptap/extension-document";
import Paragraph from "@tiptap/extension-paragraph";
import Text from "@tiptap/extension-text";
import { saveSection } from "../api/client";
import { translate, type Lang } from "../i18n";

// The minimal editor (backlog 0.10): TipTap 3 over the section's
// Markdown text. Only document/paragraph/text nodes are enabled, so
// what the person types stays literal text — no rich-text conversion
// rewrites the Markdown the app compares (T-1, B-3).
//
// Saving (specification §2): on blur, every ~30 s, when the tab is
// hidden and before sending a message (the parent calls saveNow).
// The client does not send if the text has not changed; Go still
// decides no-change (204), a new version (200) or a conflict (409).
//
// A change outside the editor that leaves the text as it was last
// saved (the role, P-27) still makes a new version: the editor takes
// it, so the next save is not a 409. A change of text does not.
export function SectionEditor({
  lang,
  projectID,
  label,
  heading,
  band,
  underlines,
  onQuestion,
  readOnly,
  onBlock,
  unverified,
  version,
  text,
  onSaved,
  registerSave,
}: {
  lang: Lang;
  projectID: string;
  label: string;
  // The section title and «qué debe abordar», editable (P-49).
  heading: ReactNode;
  // The band of warnings (backlog-r2 2.9), under the title and «qué debe
  // abordar».
  band?: ReactNode;
  // The warnings Go placed in a paragraph (block, offsets): their passage
  // is underlined; a «por aclarar»'s leads to its card.
  underlines: Warning[];
  onQuestion: (label: string) => void;
  readOnly: boolean;
  onBlock: (block: number) => void;
  // The paragraphs without a verified origin, by block from 1 (R-1):
  // each shows «sin verificar» at its end. Go computes them.
  unverified: number[];
  version: number;
  text: string;
  onSaved: (label: string, version: number, text: string) => void;
  registerSave: (fn: () => Promise<void>) => void;
}) {
  const base = useRef({ version, savedText: text });
  const status = useRef<((kind: string) => void) | null>(null);

  useEffect(() => {
    if (version > base.current.version && text === base.current.savedText) {
      base.current = { version, savedText: text };
    }
  }, [version, text]);

  const editor = useEditor({
    editable: !readOnly,
    extensions: [Document, Paragraph, Text, WarningUnderlines],
    content: text
      .split(/\n{2,}/)
      .map((p) => `<p>${escapeHTML(p)}</p>`)
      .join(""),
    editorProps: {
      attributes: { class: "section-editor-content" },
    },
  });

  const currentText = () =>
    editor ? editor.getText({ blockSeparator: "\n\n" }) : text;

  useEffect(() => {
    editor?.setEditable(!readOnly);
  }, [editor, readOnly]);

  // Go's placements go to the plugin; it keeps them mapped through what
  // the person types until Go sends new ones.
  const underlineTitle = {
    open_question: translate(lang, "warnings.underline.open_question"),
    agent_pending: translate(lang, "warnings.underline.agent_pending"),
  };
  const underlinesSig = JSON.stringify(underlines.map((w) => [w.kind, w.target, w.block, w.start, w.end]));
  useEffect(() => {
    if (!editor) return;
    const marks: Underline[] = underlines
      .filter((w) => (w.block ?? 0) > 0 && w.end > w.start)
      .map((w) => ({
        kind: w.kind,
        target: w.target,
        block: w.block ?? 0,
        start: w.start,
        end: w.end,
        title: w.kind === "open_question" ? underlineTitle.open_question : underlineTitle.agent_pending,
      }));
    editor.view.dispatch(editor.state.tr.setMeta(underlinePlugin, marks));
  }, [editor, underlinesSig, lang]);

  async function saveNow() {
    if (!editor || readOnly) return;
    const value = currentText();
    if (value === base.current.savedText) return; // the client does not send an unchanged document
    status.current?.("saving");
    const result = await saveSection(projectID, label, base.current.version, value);
    if (result.kind === "saved") {
      base.current = { version: result.version, savedText: value };
      status.current?.("saved");
      onSaved(label, result.version, value);
    } else if (result.kind === "unchanged") {
      base.current = { version: base.current.version, savedText: value };
      status.current?.("saved");
    } else if (result.kind === "conflict") {
      status.current?.("conflict");
    } else {
      status.current?.("error");
    }
  }

  const saveRef = useRef(saveNow);
  saveRef.current = saveNow;

  useEffect(() => {
    registerSave(() => saveRef.current());
  }, [registerSave]);

  useEffect(() => {
    const timer = window.setInterval(() => void saveRef.current(), 30_000);
    const onVisibility = () => {
      if (document.hidden) void saveRef.current();
    };
    document.addEventListener("visibilitychange", onVisibility);
    return () => {
      window.clearInterval(timer);
      document.removeEventListener("visibilitychange", onVisibility);
    };
  }, []);

  // The mark goes after each unverified paragraph of the editor, which is
  // block n (B-10): a rule per paragraph, with the label from the i18n
  // catalog through a custom property. Labels and block numbers only.
  // [EL DISEÑO NO LO DICE]: how the mark looks; a small note at the end
  // of the paragraph, in the colour of what is pending.
  const markRule =
    unverified.length > 0
      ? unverified
          .map((n) => `.section-editor[data-section="${label}"] .section-editor-content > p:nth-child(${n})::after`)
          .join(",\n") +
        " { content: var(--unverified-label); margin-left: 0.5em; font-size: 0.75em; color: var(--todo); background: var(--todo-soft); border-radius: 4px; padding: 0 4px; }"
      : "";
  const markLabel = { "--unverified-label": JSON.stringify(translate(lang, "provenance.unverified")) } as CSSProperties;

  return (
    <div className="section-editor" data-section={label} style={markLabel} onBlur={() => void saveRef.current()}>
      {markRule && <style>{markRule}</style>}
      <div className="section-editor-head">
        <div className="section-editor-title">{heading}</div>
        <SaveStatus lang={lang} register={(fn) => (status.current = fn)} />
      </div>
      {band}
      <EditorContent
        editor={editor}
        onClick={(ev) => {
          // An underlined passage of a «por aclarar» leads to its card
          // (pantallas, 2.9); an agent pending's opens the paragraph's
          // history, like any click on it.
          const mark = (ev.target as HTMLElement).closest("[data-warning-kind]");
          if (mark?.getAttribute("data-warning-kind") === "open_question") {
            onQuestion(mark.getAttribute("data-warning-target") ?? "");
            return;
          }
          // A click on a paragraph opens its history (P5). Each paragraph of
          // the editor is one block of the section (B-10).
          // [EL DISEÑO NO LO DICE]: the minimal editor has paragraphs only;
          // paragraph n is block n.
          const p = (ev.target as HTMLElement).closest("p");
          const root = p?.parentElement;
          if (p && root) onBlock(Array.prototype.indexOf.call(root.children, p) + 1);
        }}
      />
    </div>
  );
}

// One underline the plugin draws: Go's place of a warning's passage.
type Underline = { kind: string; target: string; block: number; start: number; end: number; title: string };

const underlinePlugin = new PluginKey<DecorationSet>("warning-underlines");

// Paragraph n of the editor is block n (B-10); its text starts one
// position after the paragraph opens. Offsets are UTF-16 code units, as
// ProseMirror counts text. An underline that no longer fits its
// paragraph is clipped to it.
function buildUnderlines(doc: PMNode, marks: Underline[]): DecorationSet {
  const decorations: Decoration[] = [];
  for (const m of marks) {
    let from = -1;
    let size = 0;
    doc.forEach((node, offset, index) => {
      if (index === m.block - 1) {
        from = offset + 1;
        size = node.content.size;
      }
    });
    if (from < 0) continue;
    const start = from + Math.min(m.start, size);
    const end = from + Math.min(m.end, size);
    if (end <= start) continue;
    decorations.push(
      Decoration.inline(start, end, {
        class: `warning-underline ${m.kind}`,
        title: m.title,
        "data-warning-kind": m.kind,
        "data-warning-target": m.target,
      }),
    );
  }
  return DecorationSet.create(doc, decorations);
}

// The underlines are decorations, never marks: they do not enter the
// text the app compares (T-1).
const WarningUnderlines = Extension.create({
  name: "warningUnderlines",
  addProseMirrorPlugins() {
    return [
      new Plugin<DecorationSet>({
        key: underlinePlugin,
        state: {
          init: () => DecorationSet.empty,
          apply(tr, set) {
            const marks = tr.getMeta(underlinePlugin) as Underline[] | undefined;
            if (marks) return buildUnderlines(tr.doc, marks);
            return set.map(tr.mapping, tr.doc);
          },
        },
        props: {
          decorations(state) {
            return underlinePlugin.getState(state);
          },
        },
      }),
    ];
  },
});

function SaveStatus({
  lang,
  register,
}: {
  lang: Lang;
  register: (fn: (kind: string) => void) => void;
}) {
  const ref = useRef<HTMLSpanElement>(null);
  useEffect(() => {
    register((kind) => {
      if (!ref.current) return;
      ref.current.textContent =
        kind === "saving"
          ? translate(lang, "save.saving")
          : kind === "conflict"
            ? translate(lang, "save.conflict")
            : kind === "error"
              ? translate(lang, "save.error")
              : translate(lang, "save.saved");
    });
  }, [lang, register]);
  return <span className="save-status" ref={ref} />;
}

function escapeHTML(s: string): string {
  return s
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll("\n", "<br>");
}
