// C.26: the browser's file input writes its own text in its own language; this
// button says it in the app's. The input stays, hidden, for the keyboard and the
// file dialog.
export function FileButton({ label, disabled, onFile }: {
  label: string;
  disabled?: boolean;
  onFile: (file: File) => void;
}) {
  return (
    <label className={`file-button${disabled ? " off" : ""}`}>
      <input
        type="file"
        accept=".md,.txt,.csv,.pdf"
        disabled={disabled}
        onChange={(e) => {
          const f = e.target.files?.[0];
          if (f) onFile(f);
          e.target.value = "";
        }}
      />
      {label}
    </label>
  );
}
