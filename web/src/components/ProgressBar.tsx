// A thin accent bar for live backup and restore progress. By default it is
// pinned to the bottom of a `relative overflow-hidden` card, so it clips to the
// card's rounded corners. `inline` puts it in normal flow with an optional
// caption above it, as in the restore panel; the pinned bar has no caption
// because the card's action button already names the running phase.
//
// Without a percentage a small segment loops (`glim-indeterminate` in
// index.css, RTL-aware). An inactive bar renders nothing.

interface ProgressBarProps {
  percent: number;
  active: boolean;
  /** Force the looping animation. Defaults to `active && percent <= 0`. */
  indeterminate?: boolean;
  /** Caption naming the phase or percentage, such as "Restoring… 42%". */
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
          ? "relative h-1 w-full overflow-hidden rounded-pill"
          : "absolute bottom-0 start-0 end-0 h-1 overflow-hidden"
      }
      style={{ background: "var(--carbon-border)" }}
      role="progressbar"
      aria-valuemin={0}
      aria-valuemax={100}
      aria-valuenow={isIndeterminate ? undefined : Math.round(clamped)}
    >
      {isIndeterminate ? (
        <div
          className="absolute inset-y-0 w-1/3 rounded-pill"
          style={{
            background: "var(--accent)",
            animation: "glim-indeterminate 1.2s ease-in-out infinite",
          }}
        />
      ) : (
        <div
          // `glim-progress-fill` is the travelling band of light (index.css,
          // behind the reduced-motion gate). It stops at 100, where a light
          // still sweeping the bar would contradict the number.
          className={`h-full transition-[width] duration-300 ease-out${clamped < 100 ? " glim-progress-fill" : ""}`}
          style={{ width: `${clamped}%`, background: "var(--accent)" }}
        />
      )}
    </div>
  );

  if (inline) {
    return (
      <div className="flex flex-col gap-0.5">
        {label && <span className="text-caption text-carbon-textMuted">{label}</span>}
        {track}
      </div>
    );
  }

  return track;
}
