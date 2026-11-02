// The three panels (backlog-ui U.1): what the page remembers per browser (B-98),
// the limits of the divider, and where each destination leads (B-95, B-96).
// Nothing here has evidence value: it only places what Go already computed.

export type CenterView = "conv" | "doc";
export type RightTab = "frame" | "history" | "markdown";

export interface Prefs {
  leftOff: boolean;
  rightOff: boolean;
  rightW: number;
  rightTab: RightTab;
}

export interface Route {
  center: CenterView | null;
  right: RightTab | null;
}

interface Store {
  getItem(key: string): string | null;
  setItem(key: string, value: string): void;
}

export const DEFAULT_RIGHT_W = 420;
export const MIN_RIGHT_W = 300;
export const MAX_RIGHT_SHARE = 0.65;
export const MIN_CENTER_W = 360;
export const DIVIDER_W = 6;
export const LEFT_W = 264;
export const LEFT_OFF_W = 52;
export const ARROW_STEP = 24;
export const NARROW_W = 900;

const RIGHT_TABS: readonly string[] = ["frame", "history", "markdown"];

// The getter is itself guarded: reading localStorage can throw when the
// browser blocks site data.
const browserStore = (): Store => globalThis.localStorage;

export function readPrefs(store: () => Store = browserStore): Prefs {
  const prefs: Prefs = { leftOff: false, rightOff: false, rightW: DEFAULT_RIGHT_W, rightTab: "frame" };
  try {
    const s = store();
    prefs.leftOff = s.getItem("tq.leftOff") === "1";
    prefs.rightOff = s.getItem("tq.rightOff") === "1";
    const width = Number.parseInt(s.getItem("tq.rightW") ?? "", 10);
    if (width > 0) prefs.rightW = width;
    const tab = s.getItem("tq.rightTab");
    if (tab && RIGHT_TABS.includes(tab)) prefs.rightTab = tab as RightTab;
  } catch {
    // Defaults (B-98).
  }
  return prefs;
}

export function writePrefs(changed: Partial<Prefs>, store: () => Store = browserStore): void {
  try {
    const s = store();
    if (changed.leftOff !== undefined) s.setItem("tq.leftOff", changed.leftOff ? "1" : "0");
    if (changed.rightOff !== undefined) s.setItem("tq.rightOff", changed.rightOff ? "1" : "0");
    if (changed.rightW !== undefined) s.setItem("tq.rightW", String(changed.rightW));
    if (changed.rightTab !== undefined) s.setItem("tq.rightTab", changed.rightTab);
  } catch {
    // A preference that does not save is not an error (B-98).
  }
}

// Between 300 px and 65 % of the window, and leaving the center its 360 px
// beside the left panel's width. The minimum wins where they cannot all fit.
export function clampRightWidth(px: number, windowW: number, leftW = 0): number {
  const room = windowW - leftW - DIVIDER_W - MIN_CENTER_W;
  const most = Math.max(MIN_RIGHT_W, Math.min(windowW * MAX_RIGHT_SHARE, room));
  return Math.round(Math.min(Math.max(px, MIN_RIGHT_W), most));
}

// The width while dragging: what it was, plus what the mouse moved to the left. A
// press without movement changes nothing, so the second click of a double click
// still lands on the divider.
export function dragWidth(startW: number, startX: number, x: number): number {
  return startW + startX - x;
}

// The panel sits on the right: the left arrow widens it, as in the mockup.
export function nextWidthByKey(px: number, key: string): number {
  if (key === "ArrowLeft") return px + ARROW_STEP;
  if (key === "ArrowRight") return px - ARROW_STEP;
  return px;
}

// Ctrl/⌘+B folds the left, Ctrl/⌘+. the right.
export function shortcutPanel(e: { ctrlKey: boolean; metaKey: boolean; key: string }): "left" | "right" | null {
  if (!e.ctrlKey && !e.metaKey) return null;
  if (e.key.toLowerCase() === "b") return "left";
  if (e.key === ".") return "right";
  return null;
}

// Where each destination the page reaches lives.
export function routeTarget(target: string): Route {
  if (["document", "proposals", "proposed-sections", "sign-section"].includes(target)) {
    return { center: "doc", right: null };
  }
  if (target === "history") return { center: null, right: "history" };
  if (target === "understanding" || target === "stop-report" || /^(card|oq|round)-/.test(target)) {
    return { center: "conv", right: null };
  }
  return { center: null, right: null };
}

// [EL DISEÑO NO LO DICE] Under 900 px the right panel opens over the center, so
// it also starts folded; the backlog names only the left.
export function initialPanels(prefs: Prefs, windowW: number): { leftOff: boolean; rightOff: boolean } {
  const narrow = windowW < NARROW_W;
  return { leftOff: narrow || prefs.leftOff, rightOff: narrow || prefs.rightOff };
}

// The document exists once there are sections.
export function centerView(wanted: CenterView, hasSections: boolean): CenterView {
  return hasSections ? wanted : "conv";
}

// «Historia» shows only while a history is open (B-96).
export function effectiveTab(wanted: RightTab, historyOpen: boolean): RightTab {
  return wanted === "history" && !historyOpen ? "frame" : wanted;
}
