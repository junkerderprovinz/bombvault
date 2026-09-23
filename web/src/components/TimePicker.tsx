import { useCallback, useEffect, useLayoutEffect, useRef, useState, type CSSProperties, type KeyboardEvent as ReactKeyboardEvent } from "react";
import { createPortal } from "react-dom";
import { computeBubblePosition } from "../lib/bubblePosition";
import { hueVars } from "../lib/appearance";
import { useT } from "../lib/i18n";

const HOURS: number[] = Array.from({ length: 24 }, (_, i) => i);

/** minutesFor returns the minute column for a step. A step outside 1 to 59
 *  falls back to 5. */
export function minutesFor(step: number): number[] {
  const truncated = Math.trunc(step);
  const s = Number.isFinite(truncated) && truncated >= 1 && truncated <= 59 ? truncated : 5;
  const out: number[] = [];
  for (let m = 0; m < 60; m += s) out.push(m);
  return out;
}

/** nearestStep returns the option closest to `n`, so a stored minute that is
 *  off the step grid still highlights an option. Ties go to the earlier one. */
export function nearestStep(options: number[], n: number): number {
  return options.reduce((best, cur) => (Math.abs(cur - n) < Math.abs(best - n) ? cur : best), options[0]);
}

function pad2(n: number): string {
  return String(n).padStart(2, "0");
}

/** formatTime renders "HH:MM", the same format as a native time input. */
export function formatTime(hour: number, minute: number): string {
  return `${pad2(hour)}:${pad2(minute)}`;
}

/** parseTime parses "HH:MM" into a clamped hour and minute. Malformed input
 *  gives 00:00. */
export function parseTime(value: string): { hour: number; minute: number } {
  const m = /^(\d{1,2}):(\d{1,2})$/.exec((value ?? "").trim());
  if (!m) return { hour: 0, minute: 0 };
  const hour = Math.min(23, Math.max(0, parseInt(m[1], 10) || 0));
  const minute = Math.min(59, Math.max(0, parseInt(m[2], 10) || 0));
  return { hour, minute };
}

// Closes the one open TimePicker popover, so opening another closes it first.
let activeCloser: (() => void) | null = null;

// A filled dial with the hands cut out in the field's surface colour.
function ClockGlyph() {
  return (
    <svg viewBox="0 0 16 16" width="14" height="14" fill="currentColor" aria-hidden="true" className="flex-none">
      <circle cx="8" cy="8" r="6.5" />
      <rect x="7.4" y="4.6" width="1.2" height="3.4" rx="0.6" fill="var(--carbon-surface2, transparent)" />
      <rect x="7.325" y="8.2" width="3.95" height="1.2" rx="0.6" fill="var(--carbon-surface2, transparent)" transform="rotate(31.6 9.3 8.8)" />
    </svg>
  );
}

/**
 * TimePicker is an hour/minute picker. The trigger shows "HH:MM" and opens a
 * popover with an hour and a minute listbox. Arrow keys step and commit,
 * Home/End jump to the ends, Left/Right switch columns. Picking a value keeps
 * the popover open; an outside click, Escape, scroll or resize closes it. Both
 * trigger and popover are forced to LTR, since a clock time reads that way in
 * any language.
 */
export function TimePicker({
  value,
  onChange,
  label,
  disabled,
  minuteStep = 5,
  hueIndex,
  className,
}: {
  /** Current value as "HH:MM". */
  value: string;
  /** Called with the new "HH:MM" on every pick. */
  onChange: (v: string) => void;
  /** Accessible name for the trigger and the popover. */
  label: string;
  disabled?: boolean;
  /** Minute column granularity, default 5; backup schedules rarely need finer. */
  minuteStep?: number;
  /** Rainbow palette position, usually the enclosing card's. */
  hueIndex?: number;
  /** Replaces the default trigger classes. */
  className?: string;
}) {
  const { t } = useT();

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
    // The focused option unmounts with the portal, so focus returns to the
    // trigger instead of falling to <body>. After a click elsewhere, focus is
    // already where the user put it.
    const panel = panelRef.current;
    const returnFocus = !!panel && panel.contains(document.activeElement);
    setOpen(false);
    if (returnFocus) triggerRef.current?.focus();
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

  // Place the popover at the trigger, clamped into the viewport, before paint.
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

  // Keep the selected hour and minute in view and move focus into the popover
  // once per open. This waits for `pos`: focusing an option while the popover
  // is still parked at -9999px makes the browser scroll the page, and that
  // scroll closes the popover again. jsdom has no scrollIntoView.
  useLayoutEffect(() => {
    if (!open || !pos) return;
    hourRefs.current[hour]?.scrollIntoView?.({ block: "nearest" });
    minuteRefs.current[selectedMinute]?.scrollIntoView?.({ block: "nearest" });
    if (!focusedOnOpen.current) {
      focusedOnOpen.current = true;
      hourRefs.current[hour]?.focus();
    }
  }, [open, hour, selectedMinute, pos]);

  // Close on an outside click, Escape, scroll or resize, since a fixed popover
  // would drift away from its trigger. The capture-phase scroll listener also
  // sees the listbox columns scrolling (our own scrollIntoView included);
  // those do not move the trigger and are ignored.
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
    function onScroll(e: Event) {
      const target = e.target;
      if (target instanceof Node && panelRef.current?.contains(target)) return;
      closeSelf();
    }
    document.addEventListener("mousedown", onPointerDown);
    document.addEventListener("keydown", onKeyDown);
    window.addEventListener("scroll", onScroll, true);
    window.addEventListener("resize", closeSelf);
    return () => {
      document.removeEventListener("mousedown", onPointerDown);
      document.removeEventListener("keydown", onKeyDown);
      window.removeEventListener("scroll", onScroll, true);
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
  // Compared again here because `hue` does not narrow `hueIndex`.
  const hueStyle = hueIndex !== undefined ? (hueVars(hueIndex) as CSSProperties) : undefined;
  const optionCls = `glim-time-option${hue ? " glim-hue" : ""}`;

  const triggerCls =
    className ??
    "inline-flex items-center gap-1.5 rounded-control bg-carbon-surface3 text-carbon-text text-sm px-2.5 py-1.5 glim-field-focus-well disabled:opacity-50 disabled:cursor-not-allowed";

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
            // The portal leaves the trigger's subtree, so the panel sets the
            // hue variables again for the options to inherit. Without them
            // the selected option's background turns transparent.
            style={{ left: pos?.left ?? -9999, top: pos?.top ?? -9999, ...hueStyle }}
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
