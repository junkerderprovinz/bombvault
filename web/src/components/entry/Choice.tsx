import type { ReactNode } from "react";

/** Choices holds the ways a window offers, of which one is taken. */
export function Choices({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div role="radiogroup" aria-label={label} className="flex flex-col gap-2">
      {children}
    </div>
  );
}

/** Choice is one of those ways, with a line that says what it leads to. */
export function Choice({
  on,
  title,
  text,
  onPick,
}: {
  on: boolean;
  title: string;
  text?: string;
  onPick: () => void;
}) {
  return (
    <button
      type="button"
      role="radio"
      aria-checked={on}
      onClick={onPick}
      className={`flex items-start gap-3 rounded-control bg-carbon-surface2 p-3 text-start ring-2 ring-inset transition-colors hover:bg-carbon-surface3 ${
        on ? "ring-accent" : "ring-transparent"
      }`}
    >
      <span
        className={`mt-0.5 flex h-[18px] w-[18px] flex-none items-center justify-center rounded-full border-2 ${
          on ? "border-accent" : "border-carbon-textMuted"
        }`}
      >
        {on && <span className="h-2 w-2 rounded-full bg-accent" />}
      </span>
      <span className="flex min-w-0 flex-col gap-0.5">
        <span className="text-sm font-semibold text-carbon-text wrap-anywhere">{title}</span>
        {text && <span className="text-xs text-carbon-textMuted wrap-anywhere">{text}</span>}
      </span>
    </button>
  );
}
