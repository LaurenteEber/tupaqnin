// The status while you edit (P-115): «Saved» only when what you see is what the
// last PUT that returned saved.
export function editingKind(current: string, saved: string): "saving" | "saved" {
  return current === saved ? "saved" : "saving";
}
