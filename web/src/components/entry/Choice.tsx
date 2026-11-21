import type { KeyboardEvent, ReactNode } from "react";

/** pickSurface is the ground of a tile to pick from. A picked tile turns into
 *  the accent, the way every selection in the app is marked, so it needs no
 *  dot or ring beside it. */
export function pickSurface(on: boolean): string {
  return on ? "glim-active bg-accent text-accentContrast hover:bg-accentHover" : "bg-carbon-surface2 text-carbon-text hover:bg-hover";
}

/** pickSub is the colour of the line under a tile's title, which on the accent
 *  is the accent's own ink, a little fainter. */
export function pickSub(on: boolean, off = "text-carbon-textMuted"): string {
  return on ? "opacity-80" : off;
}

const ARROW_STEP: Record<string, number> = { ArrowDown: 1, ArrowRight: 1, ArrowUp: -1, ArrowLeft: -1 };

/** pickWithArrows lets the arrow keys on a radio pick the next one in its
 *  group, as in any radio group, wrapping at the ends. */
export function pickWithArrows(e: KeyboardEvent<HTMLElement>) {
  const step = ARROW_STEP[e.key];
  const group = e.currentTarget.closest('[role="radiogroup"]');
  if (!step || !group) return;
  e.preventDefault();
  // Left and right follow the reading direction.
  const flip = (e.key === "ArrowLeft" || e.key === "ArrowRight") && getComputedStyle(group).direction === "rtl";
  const radios = Array.from(group.querySelectorAll<HTMLElement>('[role="radio"]:not(:disabled)'));
  const next = radios[(radios.indexOf(e.currentTarget) + (flip ? -step : step) + radios.length) % radios.length];
  next.focus();
  next.click();
}

/** Choices holds the ways a window offers, of which one is always taken. */
export function Choices({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div role="radiogroup" aria-label={label} className="flex flex-col gap-2">
      {children}
    </div>
  );
}

/** Choice is one of those ways, with a line that says what it leads to and,
 *  where it helps, a small picture of it at the start. The group is one tab
 *  stop, on the way taken. */
export function Choice({
  on,
  title,
  text,
  picture,
  onPick,
}: {
  on: boolean;
  title: string;
  text?: string;
  picture?: ReactNode;
  onPick: () => void;
}) {
  return (
    <button
      type="button"
      role="radio"
      aria-checked={on}
      tabIndex={on ? 0 : -1}
      onClick={onPick}
      onKeyDown={pickWithArrows}
      className={`flex items-center gap-3 rounded-control p-3 text-start transition-colors ${pickSurface(on)}`}
    >
      {picture && <span className="flex-none">{picture}</span>}
      <span className="flex min-w-0 flex-col gap-0.5">
        <span className="text-sm font-semibold wrap-anywhere">{title}</span>
        {text && <span className={`text-xs wrap-anywhere ${pickSub(on)}`}>{text}</span>}
      </span>
    </button>
  );
}
