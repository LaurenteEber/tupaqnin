// The editor's save (P-115, P-116). «Saved» only when what you see is what the last
// PUT that returned saved; «Unsaved changes» while a change waits; «Saving…» only
// while a PUT is in flight.

// AUTOSAVE_DELAY_MS is how long you stop typing before it saves (P-116: 2 s [inicial]).
export const AUTOSAVE_DELAY_MS = 2000;

export function editingKind(current: string, saved: string): "unsaved" | "saved" {
  return current === saved ? "saved" : "unsaved";
}

// A conflict stays on screen and is retried on blur, as before: typing does not
// resend the same refused save (B-73 d keeps its route).
export function shouldAutosave(current: string, saved: string, conflicted: boolean): boolean {
  return !conflicted && current !== saved;
}

type Schedule = (fn: () => void, ms: number) => () => void;

const realTimer: Schedule = (fn, ms) => {
  const id = setTimeout(fn, ms);
  return () => clearTimeout(id);
};

// createAutosave runs save after delayMs without changes, or at once on flush, one
// at a time: a save uses the version the last one returned, so two in flight would
// answer as a conflict with yourself.
export function createAutosave(save: () => Promise<void>, delayMs = AUTOSAVE_DELAY_MS, schedule: Schedule = realTimer) {
  let cancelTimer: (() => void) | null = null;
  let tail: Promise<void> = Promise.resolve();
  let pending = 0;
  const cancel = () => {
    cancelTimer?.();
    cancelTimer = null;
  };
  const flush = (): Promise<void> => {
    cancel();
    pending++;
    tail = tail.catch(() => {}).then(save).finally(() => { pending--; });
    return tail;
  };
  return {
    changed: () => {
      cancel();
      cancelTimer = schedule(() => {
        cancelTimer = null;
        void flush().catch(() => {});
      }, delayMs);
    },
    flush,
    cancel,
    busy: () => pending > 0,
  };
}
