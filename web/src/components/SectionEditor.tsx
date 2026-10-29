import { useEffect, useRef } from "react";
import { useEditor, EditorContent } from "@tiptap/react";
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
  name,
  readOnly,
  version,
  text,
  onSaved,
  registerSave,
}: {
  lang: Lang;
  projectID: string;
  label: string;
  name: string;
  readOnly: boolean;
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
    extensions: [Document, Paragraph, Text],
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

  return (
    <div className="section-editor" onBlur={() => void saveRef.current()}>
      <div className="section-editor-head">
        <h2>{name}</h2>
        <SaveStatus lang={lang} register={(fn) => (status.current = fn)} />
      </div>
      <EditorContent editor={editor} />
    </div>
  );
}

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
