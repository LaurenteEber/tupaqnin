import { useEffect, useRef, useState, type KeyboardEvent as ReactKeyboardEvent, type ReactNode } from "react";
import {
  DEFAULT_RIGHT_W,
  DIVIDER_W,
  LEFT_OFF_W,
  LEFT_W,
  MIN_RIGHT_W,
  NARROW_W,
  clampRightWidth,
  initialPanels,
  nextWidthByKey,
  readPrefs,
  shortcutPanel,
  writePrefs,
  type CenterView,
  type RightTab,
} from "../layout";
import { translate, type Lang } from "../i18n";

// What the panels remember (B-98) and what the page changes when it routes.
export function useLayout() {
  const [saved] = useState(() => readPrefs());
  const [windowW, setWindowW] = useState(() => window.innerWidth);
  const [leftOff, setLeftOffState] = useState(() => initialPanels(saved, window.innerWidth).leftOff);
  const [rightOff, setRightOffState] = useState(() => initialPanels(saved, window.innerWidth).rightOff);
  const [rightW, setRightWState] = useState(saved.rightW);
  const [rightTab, setRightTabState] = useState<RightTab>(saved.rightTab);
  const [center, setCenter] = useState<CenterView>("conv");

  useEffect(() => {
    const onResize = () => setWindowW(window.innerWidth);
    window.addEventListener("resize", onResize);
    return () => window.removeEventListener("resize", onResize);
  }, []);

  return {
    windowW,
    narrow: windowW < NARROW_W,
    leftOff,
    rightOff,
    rightW,
    rightTab,
    center,
    setCenter,
    setLeftOff(off: boolean) {
      setLeftOffState(off);
      writePrefs({ leftOff: off });
    },
    setRightOff(off: boolean) {
      setRightOffState(off);
      writePrefs({ rightOff: off });
    },
    // [EL DISEÑO NO LO DICE] Under 900 px the frame covers the center, so going
    // to the center folds it for now, without touching the saved preference.
    foldOverlay() {
      setRightOffState(true);
    },
    setRightW(px: number, save: boolean) {
      setRightWState(px);
      if (save) writePrefs({ rightW: px });
    },
    setRightTab(tab: RightTab) {
      setRightTabState(tab);
      writePrefs({ rightTab: tab });
    },
  };
}

export type Layout = ReturnType<typeof useLayout>;

// The three zones and the divider (pantallas.md «Disposición general», 2-nov).
// It unmounts during the signoff, so the shortcuts cannot reach the frame (P7).
export function Shell({
  lang,
  layout,
  left,
  center,
  right,
}: {
  lang: Lang;
  layout: Layout;
  left: ReactNode;
  center: ReactNode;
  right: ReactNode;
}) {
  const t = (key: string) => translate(lang, key);
  const showRight = right !== null;
  const leftW = layout.leftOff ? LEFT_OFF_W : LEFT_W;
  const clamp = (px: number) => clampRightWidth(px, layout.windowW, layout.narrow ? 0 : leftW);
  const width = clamp(layout.rightW);
  const [dragging, setDragging] = useState(false);

  // The handlers below read the latest render.
  const current = useRef(layout);
  current.current = layout;

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      const panel = shortcutPanel(e);
      if (!panel) return;
      e.preventDefault();
      if (panel === "left") current.current.setLeftOff(!current.current.leftOff);
      else current.current.setRightOff(!current.current.rightOff);
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, []);

  useEffect(() => {
    if (!dragging) return;
    const fit = (e: MouseEvent) => {
      const { windowW, narrow, leftOff } = current.current;
      const offset = narrow ? 0 : leftOff ? LEFT_OFF_W : LEFT_W;
      return clampRightWidth(window.innerWidth - e.clientX, windowW, offset);
    };
    const onMove = (e: MouseEvent) => current.current.setRightW(fit(e), false);
    const onUp = (e: MouseEvent) => {
      current.current.setRightW(fit(e), true);
      setDragging(false);
    };
    document.body.style.userSelect = "none";
    window.addEventListener("mousemove", onMove);
    window.addEventListener("mouseup", onUp);
    return () => {
      document.body.style.userSelect = "";
      window.removeEventListener("mousemove", onMove);
      window.removeEventListener("mouseup", onUp);
    };
  }, [dragging]);

  function onDividerKey(e: ReactKeyboardEvent) {
    const next = nextWidthByKey(width, e.key);
    if (next === width) return;
    e.preventDefault();
    layout.setRightW(clamp(next), true);
  }

  const classes = ["shell", layout.leftOff ? "left-off" : "", !showRight || layout.rightOff ? "right-off" : ""];
  return (
    <div className={classes.filter(Boolean).join(" ")}>
      {left}
      <main className="center">{center}</main>
      {showRight && !layout.rightOff && (
        <div
          className={`divider${dragging ? " drag" : ""}`}
          style={{ width: DIVIDER_W }}
          role="separator"
          aria-orientation="vertical"
          aria-label={t("layout.divider")}
          aria-valuenow={width}
          aria-valuemin={MIN_RIGHT_W}
          aria-valuemax={clamp(Number.MAX_SAFE_INTEGER)}
          tabIndex={0}
          onMouseDown={(e) => {
            e.preventDefault();
            setDragging(true);
          }}
          onDoubleClick={() => layout.setRightW(clamp(DEFAULT_RIGHT_W), true)}
          onKeyDown={onDividerKey}
        />
      )}
      {showRight && (
        <aside className="right" style={{ width }} aria-label={t("layout.right_view")} hidden={layout.rightOff}>
          {right}
        </aside>
      )}
    </div>
  );
}
