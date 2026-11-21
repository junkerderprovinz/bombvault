import type { ReactNode } from "react";
import { RadioMark } from "../RadioMark";

/** pickSurface is the ground of a tile to pick from. A picked tile turns into
 *  the accent, the way every selection in the app is marked. */
export function pickSurface(on: boolean): string {
  return on ? "glim-active bg-accent text-accentContrast hover:bg-accentHover" : "bg-carbon-surface2 text-carbon-text hover:bg-hover";
}

/** pickSub is the colour of the line under a tile's title, which on the accent
 *  is the accent's own ink, a little fainter. */
export function pickSub(on: boolean, off = "text-carbon-textMuted"): string {
  return on ? "opacity-80" : off;
}

/** Choices holds the ways a window offers, of which one is taken. */
export function Choices({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div role="radiogroup" aria-label={label} className="flex flex-col gap-2">
      {children}
    </div>
  );
}

/** Choice is one of those ways, with a line that says what it leads to and,
 *  where it helps, a small picture of it at the end. */
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
      onClick={onPick}
      className={`flex items-start gap-3 rounded-control p-3 text-start transition-colors ${pickSurface(on)}`}
    >
      <RadioMark on={on} className="mt-0.5" />
      <span className="flex min-w-0 flex-col gap-0.5">
        <span className="text-sm font-semibold wrap-anywhere">{title}</span>
        {text && <span className={`text-xs wrap-anywhere ${pickSub(on)}`}>{text}</span>}
      </span>
      {picture && <span className="ms-auto flex-none self-center">{picture}</span>}
    </button>
  );
}
