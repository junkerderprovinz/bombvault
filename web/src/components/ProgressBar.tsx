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
//
// `label` adds a small caption so the bar can name its phase ("Restoring…" vs
// "Backing up…"); on the pinned card bar it sits at the bottom-right corner, on
// an `inline` bar it sits above the track. `inline` renders the bar in normal
// document flow (for use inside a restore panel) instead of pinned to a card.
// ---------------------------------------------------------------------------

interface ProgressBarProps {
  percent: number;
  active: boolean;
  /** Force the looping animation. Defaults to `active && percent <= 0`. */
  indeterminate?: boolean;
  /** Optional caption naming the phase / percentage (e.g. "Restoring… 42%"). */
  label?: string;
  /** Render in normal document flow instead of pinned to a card's bottom edge. */
  inline?: boolean;
}

export function ProgressBar({ percent, active, indeterminate, label, inline }: ProgressBarProps) {
  if (!active) return null;

  const isIndeterminate = indeterminate ?? percent <= 0;
  const clamped = Math.max(0, Math.min(100, percent));

  const track = (
    <div
      className={
        inline
          ? "relative h-1 w-full overflow-hidden rounded-full"
          : "absolute bottom-0 left-0 right-0 h-1 overflow-hidden"
      }
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

  // Inline: caption above the bar, both in normal flow.
  if (inline) {
    return (
      <div className="flex flex-col gap-0.5">
        {label && <span className="text-[11px] text-carbon-textMuted">{label}</span>}
        {track}
      </div>
    );
  }

  // Pinned card bar: caption floats in the bottom-right corner (no layout churn).
  return (
    <>
      {label && (
        <span className="absolute bottom-1.5 right-2 z-10 text-[10px] text-carbon-textMuted pointer-events-none">
          {label}
        </span>
      )}
      {track}
    </>
  );
}
