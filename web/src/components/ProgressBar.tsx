// ---------------------------------------------------------------------------
// ProgressBar — a thin, full-width accent bar pinned to the bottom edge of a
// card to show live backup/restore progress.
//
// Intended use: render inside a `relative overflow-hidden` card so the
// absolutely-positioned bar clips to the card's rounded corners:
//
//   <ProgressBar percent={p.percent} active={p.active} />
//
// Determinate: the fill width tracks `percent` with a smooth transition.
// Indeterminate (active but no number yet): a small accent segment loops
// left→right (keyframes `bv-indeterminate` live in index.css).
// When inactive, it renders nothing.
// ---------------------------------------------------------------------------

interface ProgressBarProps {
  percent: number;
  active: boolean;
  /** Force the looping animation. Defaults to `active && percent <= 0`. */
  indeterminate?: boolean;
}

export function ProgressBar({ percent, active, indeterminate }: ProgressBarProps) {
  if (!active) return null;

  const isIndeterminate = indeterminate ?? percent <= 0;
  const clamped = Math.max(0, Math.min(100, percent));

  return (
    <div
      className="absolute bottom-0 left-0 right-0 h-1 overflow-hidden"
      style={{ background: "var(--carbon-border)" }}
      role="progressbar"
      aria-valuemin={0}
      aria-valuemax={100}
      aria-valuenow={isIndeterminate ? undefined : Math.round(clamped)}
    >
      {isIndeterminate ? (
        <div
          className="absolute inset-y-0 w-1/3 rounded-full"
          style={{
            background: "var(--accent)",
            animation: "bv-indeterminate 1.2s ease-in-out infinite",
          }}
        />
      ) : (
        <div
          className="h-full transition-[width] duration-300 ease-out"
          style={{ width: `${clamped}%`, background: "var(--accent)" }}
        />
      )}
    </div>
  );
}
