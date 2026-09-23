import {
  useCallback,
  useEffect,
  useLayoutEffect,
  useRef,
  useState,
  type KeyboardEvent as ReactKeyboardEvent,
} from "react";
import { createPortal } from "react-dom";
import { useT } from "../lib/i18n";

// ColorPickerSwatch is the shared custom-colour control: a swatch sized like
// the preset swatches beside it, opening a popover with a saturation/value
// square, a hue bar and a hex field. The popover is the app's own rather than a
// native <input type="color">, which would open a window outside the page.

export interface Hsv {
  h: number;
  s: number;
  v: number;
}

const DEFAULT_HSV: Hsv = { h: 220, s: 0.8, v: 0.9 };

/** hexToHsv parses "#rrggbb" or "rrggbb" in either case, or returns null. */
export function hexToHsv(hex: string): Hsv | null {
  const m = /^#?([0-9a-f]{6})$/i.exec(hex || "");
  const group = m?.[1];
  if (!group) return null;
  const n = parseInt(group, 16);
  const r = ((n >> 16) & 255) / 255;
  const g = ((n >> 8) & 255) / 255;
  const b = (n & 255) / 255;
  const mx = Math.max(r, g, b);
  const mn = Math.min(r, g, b);
  const d = mx - mn;
  let h = 0;
  if (d) {
    if (mx === r) h = 60 * (((g - b) / d) % 6);
    else if (mx === g) h = 60 * ((b - r) / d + 2);
    else h = 60 * ((r - g) / d + 4);
  }
  if (h < 0) h += 360;
  return { h, s: mx ? d / mx : 0, v: mx };
}

export function hsvToHex(h: number, s: number, v: number): string {
  const c = v * s;
  const x = c * (1 - Math.abs(((h / 60) % 2) - 1));
  const m = v - c;
  let r = 0;
  let g = 0;
  let b = 0;
  if (h < 60) {
    r = c;
    g = x;
  } else if (h < 120) {
    r = x;
    g = c;
  } else if (h < 180) {
    g = c;
    b = x;
  } else if (h < 240) {
    g = x;
    b = c;
  } else if (h < 300) {
    r = x;
    b = c;
  } else {
    r = c;
    b = x;
  }
  const f = (u: number) => Math.round((u + m) * 255).toString(16).padStart(2, "0");
  return `#${f(r)}${f(g)}${f(b)}`;
}

/** normalizeHex accepts "2f6feb" or "#2F6FEB", returns "#rrggbb" lowercase,
 * or null if invalid. */
export function normalizeHex(value: string): string | null {
  const trimmed = value.trim().replace(/^#/, "");
  return /^[0-9a-f]{6}$/i.test(trimmed) ? `#${trimmed.toLowerCase()}` : null;
}

// Only one popover is open across the app; opening one closes the other.
let activeCloser: (() => void) | null = null;

export function ColorPickerSwatch({
  value,
  onChange,
  label,
  disabled,
  className,
}: {
  /** The 6-digit hex the swatch shows and the popover opens with. */
  value: string;
  /** Called with a normalized "#rrggbb" on every drag or key step and every
   *  valid typed hex. */
  onChange: (hex: string) => void;
  /** The swatch's accessible name and hover title, and the popover's label. */
  label: string;
  disabled?: boolean;
  /** Size, shape and border of the swatch, so each call site can match its
   *  neighbours. */
  className?: string;
}) {
  const { t } = useT();
  const [open, setOpen] = useState(false);
  const [hsv, setHsv] = useState<Hsv>(() => hexToHsv(value) ?? DEFAULT_HSV);
  const [hexDraft, setHexDraft] = useState(value);
  const [pos, setPos] = useState<{ left: number; top: number } | null>(null);

  const triggerRef = useRef<HTMLButtonElement>(null);
  const panelRef = useRef<HTMLDivElement>(null);
  const svRef = useRef<HTMLDivElement>(null);
  const hueRef = useRef<HTMLDivElement>(null);
  // The drag listeners are attached once per opening, since re-attaching them
  // on every change would break a drag in progress. They reach the current
  // colour and the caller's latest onChange through these refs.
  const hsvRef = useRef<Hsv>(hsv);
  const onChangeRef = useRef(onChange);
  onChangeRef.current = onChange;

  const closeSelf = useCallback(() => {
    setOpen(false);
    if (activeCloser === closeSelf) activeCloser = null;
  }, []);

  useEffect(() => {
    return () => {
      if (activeCloser === closeSelf) activeCloser = null;
    };
  }, [closeSelf]);

  function handleOpen() {
    activeCloser?.();
    activeCloser = closeSelf;
    const parsed = hexToHsv(value) ?? DEFAULT_HSV;
    hsvRef.current = parsed;
    setHsv(parsed);
    setHexDraft(value);
    setPos(null);
    setOpen(true);
  }

  // Below the trigger, or above it when there is no room, clamped to the
  // viewport. It needs the panel's rendered size, and a layout effect places it
  // before the first paint.
  useLayoutEffect(() => {
    if (!open) return;
    const trigger = triggerRef.current;
    const panel = panelRef.current;
    if (!trigger || !panel) return;
    const rect = trigger.getBoundingClientRect();
    const vw = document.documentElement.clientWidth || window.innerWidth;
    const vh = document.documentElement.clientHeight || window.innerHeight;
    const width = panel.offsetWidth;
    const height = panel.offsetHeight;
    const left = Math.max(8, Math.min(vw - 8 - width, rect.left));
    const fitsBelow = rect.bottom + 8 + height <= vh;
    const top = fitsBelow ? rect.bottom + 8 : Math.max(8, rect.top - 8 - height);
    setPos({ left, top });
  }, [open]);

  // Scroll and resize close the popover rather than moving it, because a fixed
  // popover that drifts from its trigger looks broken.
  useEffect(() => {
    if (!open) return;
    function onPointerDown(e: MouseEvent) {
      const target = e.target as Node;
      if (panelRef.current?.contains(target)) return;
      if (triggerRef.current?.contains(target)) return;
      closeSelf();
    }
    function onKeyDown(e: KeyboardEvent) {
      if (e.key === "Escape") closeSelf();
    }
    document.addEventListener("mousedown", onPointerDown);
    document.addEventListener("keydown", onKeyDown);
    window.addEventListener("scroll", closeSelf, true);
    window.addEventListener("resize", closeSelf);
    return () => {
      document.removeEventListener("mousedown", onPointerDown);
      document.removeEventListener("keydown", onKeyDown);
      window.removeEventListener("scroll", closeSelf, true);
      window.removeEventListener("resize", closeSelf);
    };
  }, [open, closeSelf]);

  // Drag and keyboard input both end here.
  const apply = useCallback((patch: Partial<Hsv>) => {
    const next = { ...hsvRef.current, ...patch };
    hsvRef.current = next;
    setHsv(next);
    const hex = hsvToHex(next.h, next.s, next.v);
    setHexDraft(hex);
    onChangeRef.current(hex);
  }, []);

  // Keyboard equivalents of the two drags, with the usual slider steps: arrows
  // nudge, PageUp/PageDown and Shift take the coarse step, Home/End go to the
  // ends.
  const HUE_STEP = 1;
  const HUE_PAGE = 15;
  const SV_STEP = 0.01;
  const SV_PAGE = 0.1;

  function clamp01(n: number) {
    return Math.min(1, Math.max(0, n));
  }

  function onHueKeyDown(e: ReactKeyboardEvent<HTMLDivElement>) {
    const coarse = e.shiftKey || e.key === "PageUp" || e.key === "PageDown";
    const step = coarse ? HUE_PAGE : HUE_STEP;
    let h: number;
    const cur = hsvRef.current.h;
    switch (e.key) {
      case "ArrowRight":
      case "ArrowUp":
      case "PageUp":
        h = cur + step;
        break;
      case "ArrowLeft":
      case "ArrowDown":
      case "PageDown":
        h = cur - step;
        break;
      case "Home":
        h = 0;
        break;
      case "End":
        h = 359;
        break;
      default:
        return;
    }
    e.preventDefault();
    // Hue is a circle, so it wraps rather than clamping.
    apply({ h: ((h % 360) + 360) % 360 });
  }

  function onSvKeyDown(e: ReactKeyboardEvent<HTMLDivElement>) {
    const step = e.shiftKey || e.key === "PageUp" || e.key === "PageDown" ? SV_PAGE : SV_STEP;
    const { s: curS, v: curV } = hsvRef.current;
    let patch: Partial<Hsv>;
    switch (e.key) {
      case "ArrowRight":
        patch = { s: clamp01(curS + step) };
        break;
      case "ArrowLeft":
        patch = { s: clamp01(curS - step) };
        break;
      case "ArrowUp":
      case "PageUp":
        patch = { v: clamp01(curV + step) };
        break;
      case "ArrowDown":
      case "PageDown":
        patch = { v: clamp01(curV - step) };
        break;
      case "Home":
        patch = { s: 0, v: 1 };
        break;
      case "End":
        patch = { s: 1, v: 1 };
        break;
      default:
        return;
    }
    e.preventDefault();
    apply(patch);
  }

  // Mouse and touch dragging on the SV square and the hue bar.
  useEffect(() => {
    if (!open) return;
    const svEl = svRef.current;
    const hueEl = hueRef.current;
    if (!svEl || !hueEl) return;

    function clientPoint(e: MouseEvent | TouchEvent): { x: number; y: number } | null {
      if ("touches" in e) {
        const t = e.touches[0] ?? e.changedTouches[0];
        return t ? { x: t.clientX, y: t.clientY } : null;
      }
      return { x: e.clientX, y: e.clientY };
    }

    function attachDrag(target: HTMLElement, toPatch: (x: number, y: number) => Partial<Hsv>) {
      function move(e: MouseEvent | TouchEvent) {
        const rect = target.getBoundingClientRect();
        const point = clientPoint(e);
        if (!point) return;
        const x = Math.min(1, Math.max(0, (point.x - rect.left) / rect.width));
        const y = Math.min(1, Math.max(0, (point.y - rect.top) / rect.height));
        apply(toPatch(x, y));
        e.preventDefault();
      }
      function up() {
        document.removeEventListener("mousemove", move as EventListener);
        document.removeEventListener("mouseup", up);
        document.removeEventListener("touchmove", move as EventListener);
        document.removeEventListener("touchend", up);
      }
      function down(e: MouseEvent | TouchEvent) {
        move(e);
        document.addEventListener("mousemove", move as EventListener);
        document.addEventListener("mouseup", up);
        document.addEventListener("touchmove", move as EventListener, { passive: false });
        document.addEventListener("touchend", up);
      }
      target.addEventListener("mousedown", down as EventListener);
      target.addEventListener("touchstart", down as EventListener, { passive: false });
      return () => {
        target.removeEventListener("mousedown", down as EventListener);
        target.removeEventListener("touchstart", down as EventListener);
        up();
      };
    }

    const detachSv = attachDrag(svEl, (x, y) => ({ s: x, v: 1 - y }));
    const detachHue = attachDrag(hueEl, (x) => ({ h: Math.min(359.9, x * 360) }));
    return () => {
      detachSv();
      detachHue();
    };
    // `apply` is stable, so listing it does not re-attach the listeners.
  }, [open, apply]);

  return (
    <>
      <button
        ref={triggerRef}
        type="button"
        title={label}
        aria-label={label}
        aria-haspopup="dialog"
        aria-expanded={open}
        disabled={disabled}
        onClick={handleOpen}
        style={{ backgroundColor: value }}
        className={
          className ??
          "w-6 h-6 rounded-pill border-2 border-carbon-border transition-transform hover:scale-110 disabled:cursor-not-allowed disabled:opacity-50"
        }
      />
      {open &&
        createPortal(
          <div
            ref={panelRef}
            role="dialog"
            aria-label={label}
            className="glim-picker-popover glim-fade"
            style={{ left: pos?.left ?? -9999, top: pos?.top ?? -9999 }}
          >
            <div className="glim-picker">
              {/* ARIA has no two-axis slider, and two linked sliders would
                  double the tab stops. aria-valuetext carries both axes;
                  aria-valuenow tracks saturation, the axis the arrow keys move
                  first. */}
              <div
                ref={svRef}
                role="slider"
                tabIndex={0}
                aria-label={t("picker.saturationBrightness")}
                aria-valuemin={0}
                aria-valuemax={100}
                aria-valuenow={Math.round(hsv.s * 100)}
                aria-valuetext={`${Math.round(hsv.s * 100)}% / ${Math.round(hsv.v * 100)}%`}
                onKeyDown={onSvKeyDown}
                className="glim-picker-sv"
                style={{
                  background: `linear-gradient(to top, #000, rgba(0,0,0,0)), linear-gradient(to right, #fff, hsl(${Math.round(hsv.h)},100%,50%))`,
                }}
              >
                <span
                  className="glim-picker-dot"
                  style={{ left: `${hsv.s * 100}%`, top: `${(1 - hsv.v) * 100}%` }}
                />
              </div>
              <div
                ref={hueRef}
                role="slider"
                tabIndex={0}
                aria-label={t("picker.hue")}
                aria-valuemin={0}
                aria-valuemax={359}
                aria-valuenow={Math.round(hsv.h)}
                onKeyDown={onHueKeyDown}
                className="glim-picker-hue"
              >
                <span className="glim-picker-hdot" style={{ left: `${(hsv.h / 360) * 100}%` }} />
              </div>
            </div>
            <input
              type="text"
              value={hexDraft}
              onChange={(e) => {
                const raw = e.target.value;
                // The field keeps what was typed; only the sliders rewrite it.
                setHexDraft(raw);
                const normalized = normalizeHex(raw);
                if (!normalized) return;
                const parsed = hexToHsv(normalized);
                if (!parsed) return;
                hsvRef.current = parsed;
                setHsv(parsed);
                onChangeRef.current(normalized);
              }}
              maxLength={7}
              spellCheck={false}
              aria-label="Hex"
              className="glim-picker-hex"
            />
          </div>,
          document.body
        )}
    </>
  );
}
