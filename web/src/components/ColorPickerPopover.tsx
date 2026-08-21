import { useCallback, useEffect, useLayoutEffect, useRef, useState } from "react";
import { createPortal } from "react-dom";

// ---------------------------------------------------------------------------
// ColorPickerSwatch — the shared custom-colour trigger (design-language.md,
// "The user-owned axes" > Accent: "every custom colour value ... gets the
// SAME trigger: a flat colour swatch, same size and shape as the preset
// swatches beside it. Clicking it opens the shared popover anchored to that
// swatch, pre-synced to its current value"). React port of GlimStone's
// framework-free reference/colorPicker.ts + its openColorPickerPopover() —
// a REAL saturation/value square + hue bar the app owns, never a native
// `<input type="color">` (which hands control to a browser/OS surface
// entirely outside the page — jdp: "der Farbpicker soll... eine Blase die
// eingeblendet wird, kein eigenes Fenster welches sich öffnet").
//
// Structural reference for the portal + dismissal wiring: InfoBubble.tsx
// (createPortal(..., document.body), position measured off the trigger's own
// rect, closes on scroll) and FilterPopover.tsx (outside-mousedown + Escape
// dismissal, role="dialog"). Neither of those needs to size itself against
// its OWN rendered content, so this adds a useLayoutEffect position pass
// (reference's own `position()`, called only after the panel is already in
// the DOM) to clamp against the viewport using the popover's real measured
// width/height, matching reference/colorPicker.ts's openColorPickerPopover
// exactly — including its choice to CLOSE (not reposition) on scroll/resize,
// since a de-anchored fixed popover reads as broken either way.
// ---------------------------------------------------------------------------

export interface Hsv {
  h: number;
  s: number;
  v: number;
}

const DEFAULT_HSV: Hsv = { h: 220, s: 0.8, v: 0.9 };

/** hexToHsv/hsvToHex/normalizeHex are ported verbatim (same math, same edge
 * cases) from GlimStone's reference/colorPicker.ts — kept pure/DOM-free so
 * they're unit-tested directly, the same split lib/accent.ts's own
 * contrastOn/softTint use between "pure colour math" and "the DOM-owning
 * caller". */
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

// Only ONE popover is ever open across the whole app — mirrors reference/
// colorPicker.ts's own module-level `openPopover` singleton (opening a new
// one closes whichever was already open, the same idiom a native <select>'s
// single open dropdown already follows). React has no built-in shared slot
// for this, so a tiny module-level closer reference stands in for it.
let activeCloser: (() => void) | null = null;

export function ColorPickerSwatch({
  value,
  onChange,
  label,
  disabled,
  className,
}: {
  /** Current 6-digit hex value the swatch displays and the popover opens
   *  pre-synced to. */
  value: string;
  /** Fires with a normalized "#rrggbb" on every drag update and on every
   *  valid typed hex — same call shape the native `<input type="color">`
   *  this replaces used, so callers (setAccent, a palette-array updater)
   *  are unaffected by the swap. */
  onChange: (hex: string) => void;
  /** Accessible name + native hover title, and the popover dialog's own
   *  aria-label. */
  label: string;
  disabled?: boolean;
  /** Caller-supplied size/shape/border classes — kept out of this
   *  component so both call sites (the single accent swatch, sized like its
   *  presets; the 8 rainbow-palette swatches, sized like their own
   *  neighbours) keep owning their own visual footprint instead of this
   *  component hard-coding one. */
  className?: string;
}) {
  const [open, setOpen] = useState(false);
  const [hsv, setHsv] = useState<Hsv>(() => hexToHsv(value) ?? DEFAULT_HSV);
  const [hexDraft, setHexDraft] = useState(value);
  const [pos, setPos] = useState<{ left: number; top: number } | null>(null);

  const triggerRef = useRef<HTMLButtonElement>(null);
  const panelRef = useRef<HTMLDivElement>(null);
  const svRef = useRef<HTMLDivElement>(null);
  const hueRef = useRef<HTMLDivElement>(null);
  // hsvRef mirrors `hsv` state but is mutated synchronously during a drag —
  // the drag effect below attaches its document-level mousemove/touchmove
  // listeners once per open/close transition (re-attaching mid-drag on every
  // hsv change would drop whatever mouse/touch session was in progress), so
  // it needs a way to read/write the CURRENT value without `hsv` in its own
  // dependency array. onChangeRef is the same fix for the caller's onChange
  // identity, which is under no obligation to stay stable across renders.
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

  // Position the popover off the trigger's own rect, clamped so it never
  // overflows the viewport — same math as reference's own `position()`,
  // which is likewise only called once the panel already has real dimensions
  // to measure. A useLayoutEffect (not a plain effect) so the corrected
  // position lands before the browser's next paint — no visible jump from a
  // top-left flash to the real spot.
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

  // Dismissal: outside pointerdown, Escape, or scroll/resize — reference's
  // own documented set. Scroll/resize CLOSE rather than reposition (a fixed
  // popover de-anchored from its trigger reads as broken either way; see
  // this file's header comment).
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

  // Drag wiring for the SV square + hue bar — mouse and touch both, exactly
  // reference/colorPicker.ts's own `drag()` helper, ported to attach/detach
  // via a ref-scoped effect instead of returning plain DOM nodes.
  useEffect(() => {
    if (!open) return;
    const svEl = svRef.current;
    const hueEl = hueRef.current;
    if (!svEl || !hueEl) return;

    function apply(patch: Partial<Hsv>) {
      const next = { ...hsvRef.current, ...patch };
      hsvRef.current = next;
      setHsv(next);
      const hex = hsvToHex(next.h, next.s, next.v);
      setHexDraft(hex);
      onChangeRef.current(hex);
    }

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
  }, [open]);

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
              <div
                ref={svRef}
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
              <div ref={hueRef} className="glim-picker-hue">
                <span className="glim-picker-hdot" style={{ left: `${(hsv.h / 360) * 100}%` }} />
              </div>
            </div>
            <input
              type="text"
              value={hexDraft}
              onChange={(e) => {
                const raw = e.target.value;
                // The field's own displayed text stays exactly what was
                // typed (reference never overwrites it back to a normalized
                // form here) — only the DRAG handlers above resync it to a
                // freshly-computed hex. See this component's header comment
                // for why.
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
