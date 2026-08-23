import { useCallback, useEffect, useLayoutEffect, useRef, useState, type CSSProperties, type KeyboardEvent as ReactKeyboardEvent } from "react";
import { createPortal } from "react-dom";
import { computeBubblePosition } from "../lib/bubblePosition";
import { hueVars, rainbowAt } from "../lib/appearance";
import { useRainbow } from "../lib/useRainbow";
import { useT } from "../lib/i18n";

// ---------------------------------------------------------------------------
// TimePicker — a shared hour/minute picker (GlimStone form-engine, a NEW
// standard component; see docs/design-language.md "The time picker" for the
// canonical spec). Replaces every native `<input type="time">` in the app
// (jdp, live-review: "Können wir bei den Zeitfeldern einen schönen Stunden-
// und Minuten-Picker implementieren, damit man es nicht manuell eintippen
// muss" — a nice hour/minute picker so the value never has to be typed by
// hand). As of this pass the ONE call site is CadenceBuilder.tsx's cadence
// time field; a full repo sweep for `type="time"` found no others
// (Settings.tsx/Dashboard.tsx/ItemScheduleOverride.tsx all route through
// CadenceBuilder rather than rendering their own).
//
// Interaction model: a compact field-styled trigger shows the current
// "HH:MM" (read-only text, never a typeable `<input>` — the whole point of
// this component), and clicking/activating it opens a popover holding TWO
// independently scrollable `role="listbox"` columns (hour 0-23, minute in
// configurable steps, default 5 — schedules in this app are backup cadences,
// which don't need to-the-minute precision, and no existing granularity
// convention elsewhere in the app suggested a different default; the
// `intervalDays`/"every N days" field is a free-typed integer with no fixed
// step at all). This is the SAME "custom listbox escape hatch"
// Settings.tsx's LanguageCard already established for a control that needs
// full styling control a native `<select>` can't give it — extended here to
// a scrollable NUMERIC list instead of a short fixed one, which also
// satisfies the "scrollable column" shape of the alternative design the task
// brief offered. A pair of `<select>`-replacement dropdowns was the other
// option considered; the always-visible scrollable listbox was chosen
// instead because it shows every value the same way the native
// `<input type="time">` spinner did, without a second click to open a
// nested dropdown per field.
//
// Popover chrome/positioning/dismissal is NOT reinvented: it reuses the same
// portal + measure-then-clamp-position + outside-mousedown/Escape/scroll-
// dismiss contract ColorPickerSwatch (components/ColorPickerPopover.tsx) and
// InfoBubble.tsx/IconTipButton.tsx already established, calling
// lib/bubblePosition.ts's own `computeBubblePosition` directly (unlike
// ColorPickerPopover, which predates that shared helper and still does its
// own inline clamp math — this component generalizes it instead of adding a
// third copy). See index.css's `.glim-time-popover` for why it needs
// `transform: translateX(-50%)` + `width: max-content` to match that
// helper's centred-trigger contract exactly.
//
// Shape engine: every radius here (`.glim-time-popover`'s `--radius-card`,
// `.glim-time-option`'s `--radius-control`, the trigger's own
// `rounded-control`) reads the SAME tokens lib/shape.ts's applyShape() sets
// on `<html>` — reshapes with round/soft/square exactly like every other
// control in the app, no separate mechanism.
//
// Colour engine: the trigger keeps its EXACT visual identity as the native
// `<input type="time">` it replaces (same `inputCls`-equivalent classes as
// CadenceBuilder's sibling cron/everyN fields, including
// `bv-field-focus-well`'s inset focus ring), so it fits into the existing
// well without a visual seam. The popover's selected-hour/selected-minute
// highlight reads `var(--accent)`/`var(--accent-contrast)` directly (see
// index.css's `.glim-time-option[aria-selected="true"]`) — this is a
// standalone field-replacing control, not a row in an enumerable list, the
// same reasoning ColorPickerSwatch's own trigger/popover use the flat accent
// rather than a rainbow position. An optional `hueIndex` prop is still wired
// through the SAME `hueVars(rainbowAt(i))`/`.glim-hue` mechanism
// Selector.tsx's segments use, for a future call site that DOES sit in a
// hue-indexed list (e.g. a per-row schedule override) — `[data-rainbow]
// .glim-hue` (index.css) already redefines the plain `--accent`/
// `--accent-contrast` custom properties for any element carrying that class,
// so the same selected-state rule above resolves to the item's own hue
// automatically the moment it's used; no second CSS rule was needed for that
// composition (this is the "wire it in as it's built" standing rule, not an
// afterthought — see the memory this pass is explicitly answering).
//
// RTL: forced `dir="ltr"` on both the trigger and the popover, matching
// CronEditor's own `dir="ltr"` on its raw cron-expression input a few lines
// above this component's one call site — a clock time like "14:30" reads
// left-to-right even inside an RTL page (Arabic/Hebrew), the same convention
// every other time-shaped value in this app already follows.
//
// Keyboard: ArrowUp/ArrowDown step within a column (wrapping at both ends)
// and immediately commit the new value (roving tabindex follows the
// currently selected item, matching Selector.tsx's own "arrow-selects-as-it-
// moves" convention for a `select="one"` strip); Home/End jump to the first/
// last item; ArrowLeft/ArrowRight move focus between the hour and minute
// columns (safe to hard-code left=hour/right=minute without an RTL check,
// since the whole control is forced `dir="ltr"`); Escape closes the popover
// from anywhere (document-level listener, matching ColorPickerSwatch);
// clicking/Enter/Space on any option commits it without closing the popover,
// so both hour and minute can be picked in one visit — dismissal is always
// explicit (outside click, Escape, or scroll/resize), never automatic,
// matching ColorPickerSwatch's own contract exactly.
// ---------------------------------------------------------------------------

const HOURS: number[] = Array.from({ length: 24 }, (_, i) => i);

/** minutesFor is the minute column's own value set for a given step — pure
 *  and exported so it (and nearestStep below) are unit-tested directly
 *  without a renderer, matching this repo's established no-jsdom pattern for
 *  pure logic (Selector.test.ts's stepFor/nextFocusIndex). */
export function minutesFor(step: number): number[] {
  // Any non-positive, non-finite, or out-of-range step (0, negative, NaN,
  // >59) falls back to the 5-minute default rather than producing an empty,
  // infinite, or nonsensical column.
  const truncated = Math.trunc(step);
  const s = Number.isFinite(truncated) && truncated >= 1 && truncated <= 59 ? truncated : 5;
  const out: number[] = [];
  for (let m = 0; m < 60; m += s) out.push(m);
  return out;
}

/** nearestStep is which of `options` sits closest to `n` — used to highlight
 *  a real listbox option even when the stored minute predates a step change
 *  (or came from a cron-derived time never divisible by the current step).
 *  Ties resolve to the smaller/earlier option. */
export function nearestStep(options: number[], n: number): number {
  return options.reduce((best, cur) => (Math.abs(cur - n) < Math.abs(best - n) ? cur : best), options[0]);
}

function pad2(n: number): string {
  return String(n).padStart(2, "0");
}

/** formatTime renders hour/minute as the "HH:MM" string this component's
 *  callers store and pass back in as `value` — the exact format the native
 *  `<input type="time">` it replaces already used, so no data model changes
 *  anywhere else. */
export function formatTime(hour: number, minute: number): string {
  return `${pad2(hour)}:${pad2(minute)}`;
}

/** parseTime parses "HH:MM" into clamped { hour, minute }, defaulting to
 *  00:00 for anything malformed or empty — never throws, matching
 *  CadenceBuilder's own tolerant parseCadenceString/cronFromState. */
export function parseTime(value: string): { hour: number; minute: number } {
  const m = /^(\d{1,2}):(\d{1,2})$/.exec((value ?? "").trim());
  if (!m) return { hour: 0, minute: 0 };
  const hour = Math.min(23, Math.max(0, parseInt(m[1], 10) || 0));
  const minute = Math.min(59, Math.max(0, parseInt(m[2], 10) || 0));
  return { hour, minute };
}

// Only ONE TimePicker popover is ever open across the whole app — mirrors
// ColorPickerSwatch's own module-level `activeCloser` singleton (opening a
// new one closes whichever was already open).
let activeCloser: (() => void) | null = null;

// A small clock glyph for the trigger — thin stroke, `currentColor` (never a
// hard-coded tone), matching InfoBubble.tsx's own inline-SVG icon style.
function ClockGlyph() {
  return (
    <svg viewBox="0 0 16 16" width="14" height="14" fill="none" aria-hidden="true" className="flex-none">
      <circle cx="8" cy="8" r="6.5" stroke="currentColor" strokeWidth="1.2" />
      <path d="M8 4.6V8l2.6 1.6" stroke="currentColor" strokeWidth="1.2" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  );
}

export function TimePicker({
  value,
  onChange,
  label,
  disabled,
  minuteStep = 5,
  hueIndex,
  className,
}: {
  /** Current "HH:MM" value — same format/contract as the native
   *  `<input type="time">` this replaces. */
  value: string;
  /** Fires with a new "HH:MM" on every hour/minute pick. */
  onChange: (v: string) => void;
  /** Accessible name for the trigger AND the popover dialog, e.g.
   *  t("cadence.time"). */
  label: string;
  disabled?: boolean;
  /** Minute column granularity, default 5 (see this file's header comment
   *  for why). */
  minuteStep?: number;
  /** Optional rainbow palette position — see this file's header comment for
   *  the composition contract. Omitted by today's one call site (a single
   *  field, not a row in an enumerable list). */
  hueIndex?: number;
  /** Caller-supplied trigger classes; defaults to the exact classes
   *  CadenceBuilder's sibling cron/everyN fields already use, so the
   *  trigger keeps the native input's visual footprint. */
  className?: string;
}) {
  const { t } = useT();
  // Subscribed, not read directly — see lib/useRainbow.ts's own header for
  // why. Called unconditionally (rules of hooks) even with no `hueIndex`,
  // matching Selector.tsx's identical convention.
  useRainbow();

  const [open, setOpen] = useState(false);
  const [pos, setPos] = useState<{ left: number; top: number } | null>(null);
  const triggerRef = useRef<HTMLButtonElement>(null);
  const panelRef = useRef<HTMLDivElement>(null);
  const hourRefs = useRef<Record<number, HTMLButtonElement | null>>({});
  const minuteRefs = useRef<Record<number, HTMLButtonElement | null>>({});
  const focusedOnOpen = useRef(false);

  const { hour, minute } = parseTime(value);
  const minutes = minutesFor(minuteStep);
  const selectedMinute = nearestStep(minutes, minute);

  const closeSelf = useCallback(() => {
    setOpen(false);
    if (activeCloser === closeSelf) activeCloser = null;
  }, []);

  useEffect(() => {
    return () => {
      if (activeCloser === closeSelf) activeCloser = null;
    };
  }, [closeSelf]);

  function handleToggle() {
    if (open) {
      closeSelf();
      return;
    }
    activeCloser?.();
    activeCloser = closeSelf;
    setPos(null);
    focusedOnOpen.current = false;
    setOpen(true);
  }

  // Position the popover off the trigger's own rect, clamped into the
  // viewport — same computeBubblePosition call InfoBubble.tsx/
  // IconTipButton.tsx already make, run in a useLayoutEffect so the
  // corrected position lands before the browser's next paint.
  useLayoutEffect(() => {
    if (!open) return;
    const trigger = triggerRef.current;
    const panel = panelRef.current;
    if (!trigger || !panel) return;
    const rect = trigger.getBoundingClientRect();
    const viewport = {
      width: document.documentElement.clientWidth || window.innerWidth,
      height: document.documentElement.clientHeight || window.innerHeight,
    };
    const { left, top } = computeBubblePosition(
      rect,
      { width: panel.offsetWidth, height: panel.offsetHeight },
      viewport
    );
    setPos({ left, top });
  }, [open]);

  // Scroll the current hour/minute into view (re-runs on every value change
  // while open too, so arrow-key navigation keeps the highlighted option in
  // view once it scrolls past the visible window) and, once per open
  // transition, move focus into the dialog — a keyboard user's Enter/Space
  // on the trigger would otherwise leave focus stranded on a now-covered
  // button. `scrollIntoView` is guarded: jsdom stubs it as a no-op, real
  // browsers implement it natively.
  //
  // Gated on `pos !== null` (not just `open`), and NOT merged into the
  // positioning effect above despite running right after it: on the very
  // first render after opening, the popover is still sitting at its
  // `left:-9999px/top:-9999px` placeholder (see the JSX below) — the
  // positioning effect above only just SCHEDULED the real coordinates via
  // `setPos`, which lands in a later commit, not this one. Calling `.focus()`
  // on an option while its containing popover is still parked off-screen at
  // -9999px made the BROWSER auto-scroll the page trying to bring that
  // off-screen element into view — which then tripped this component's own
  // scroll-closes-the-popover dismissal listener a couple of milliseconds
  // later, closing the popover it had just opened (caught live: a
  // MutationObserver on the trigger's aria-expanded attribute showed
  // true→false ~2ms apart on every open). Waiting for `pos` to be non-null
  // means this only runs once the real, on-screen position has actually been
  // committed and painted, so scrollIntoView/focus never has an off-screen
  // element to react to.
  useLayoutEffect(() => {
    if (!open || !pos) return;
    hourRefs.current[hour]?.scrollIntoView?.({ block: "nearest" });
    minuteRefs.current[selectedMinute]?.scrollIntoView?.({ block: "nearest" });
    if (!focusedOnOpen.current) {
      focusedOnOpen.current = true;
      hourRefs.current[hour]?.focus();
    }
  }, [open, hour, selectedMinute, pos]);

  // Dismissal: outside pointerdown, Escape, or scroll/resize — the exact
  // same set ColorPickerSwatch documents and relies on (a fixed popover
  // de-anchored from its trigger reads as broken either way).
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

  function commit(nextHour: number, nextMinute: number) {
    onChange(formatTime(nextHour, nextMinute));
  }

  function onHourKeyDown(e: ReactKeyboardEvent<HTMLDivElement>) {
    const idx = HOURS.indexOf(hour);
    if (e.key === "ArrowDown" || e.key === "ArrowUp") {
      e.preventDefault();
      const dir = e.key === "ArrowDown" ? 1 : -1;
      const next = HOURS[(idx + dir + HOURS.length) % HOURS.length];
      commit(next, selectedMinute);
      hourRefs.current[next]?.focus();
    } else if (e.key === "Home") {
      e.preventDefault();
      commit(HOURS[0], selectedMinute);
      hourRefs.current[HOURS[0]]?.focus();
    } else if (e.key === "End") {
      e.preventDefault();
      const last = HOURS[HOURS.length - 1];
      commit(last, selectedMinute);
      hourRefs.current[last]?.focus();
    } else if (e.key === "ArrowRight") {
      e.preventDefault();
      minuteRefs.current[selectedMinute]?.focus();
    }
  }

  function onMinuteKeyDown(e: ReactKeyboardEvent<HTMLDivElement>) {
    const idx = minutes.indexOf(selectedMinute);
    if (e.key === "ArrowDown" || e.key === "ArrowUp") {
      e.preventDefault();
      const dir = e.key === "ArrowDown" ? 1 : -1;
      const next = minutes[(idx + dir + minutes.length) % minutes.length];
      commit(hour, next);
      minuteRefs.current[next]?.focus();
    } else if (e.key === "Home") {
      e.preventDefault();
      commit(hour, minutes[0]);
      minuteRefs.current[minutes[0]]?.focus();
    } else if (e.key === "End") {
      e.preventDefault();
      const last = minutes[minutes.length - 1];
      commit(hour, last);
      minuteRefs.current[last]?.focus();
    } else if (e.key === "ArrowLeft") {
      e.preventDefault();
      hourRefs.current[hour]?.focus();
    }
  }

  const hue = hueIndex !== undefined;
  // Checked inline (not via the `hue` boolean above) so TypeScript actually
  // narrows `hueIndex` to `number` here — a separately-stored boolean
  // derived from the same comparison doesn't re-narrow the original variable.
  const hueStyle = hueIndex !== undefined ? (hueVars(rainbowAt(hueIndex)) as CSSProperties) : undefined;
  const optionCls = `glim-time-option${hue ? " glim-hue" : ""}`;

  const triggerCls =
    className ??
    "inline-flex items-center gap-1.5 rounded-control bg-carbon-surface3 text-carbon-text text-sm px-2.5 py-1.5 bv-field-focus-well disabled:opacity-50 disabled:cursor-not-allowed";

  const display = formatTime(hour, minute);

  return (
    <>
      <button
        ref={triggerRef}
        type="button"
        dir="ltr"
        disabled={disabled}
        aria-haspopup="dialog"
        aria-expanded={open}
        aria-label={`${label}: ${display}`}
        title={`${label}: ${display}`}
        onClick={handleToggle}
        className={hue ? `${triggerCls} glim-hue` : triggerCls}
        style={hueStyle}
      >
        <ClockGlyph />
        <span className="tabular-nums">{display}</span>
      </button>
      {open &&
        createPortal(
          <div
            ref={panelRef}
            role="dialog"
            aria-label={label}
            dir="ltr"
            className="glim-time-popover glim-fade"
            style={{ left: pos?.left ?? -9999, top: pos?.top ?? -9999 }}
          >
            <div
              role="listbox"
              aria-label={t("timePicker.hour")}
              className="glim-time-col"
              style={{ scrollbarWidth: "thin", scrollbarColor: "var(--carbon-border) transparent" }}
              onKeyDown={onHourKeyDown}
            >
              {HOURS.map((h) => (
                <button
                  key={h}
                  ref={(el) => {
                    hourRefs.current[h] = el;
                  }}
                  type="button"
                  role="option"
                  aria-selected={h === hour}
                  tabIndex={h === hour ? 0 : -1}
                  onClick={() => commit(h, selectedMinute)}
                  className={optionCls}
                >
                  {pad2(h)}
                </button>
              ))}
            </div>
            <div className="flex items-center px-0.5 text-sm font-medium text-carbon-textMuted" aria-hidden="true">
              :
            </div>
            <div
              role="listbox"
              aria-label={t("timePicker.minute")}
              className="glim-time-col"
              style={{ scrollbarWidth: "thin", scrollbarColor: "var(--carbon-border) transparent" }}
              onKeyDown={onMinuteKeyDown}
            >
              {minutes.map((m) => (
                <button
                  key={m}
                  ref={(el) => {
                    minuteRefs.current[m] = el;
                  }}
                  type="button"
                  role="option"
                  aria-selected={m === selectedMinute}
                  tabIndex={m === selectedMinute ? 0 : -1}
                  onClick={() => commit(hour, m)}
                  className={optionCls}
                >
                  {pad2(m)}
                </button>
              ))}
            </div>
          </div>,
          document.body
        )}
    </>
  );
}
