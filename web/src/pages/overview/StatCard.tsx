export function StatCard({
  label,
  value,
  danger,
  onClick,
}: {
  label: string;
  value: number;
  danger?: boolean;
  /** When set, the tile is a real button. */
  onClick?: () => void;
}) {
  const base = "flex min-w-0 flex-col gap-1 overflow-hidden rounded-control bg-carbon-surface2 px-4 py-3";
  const inner = (
    <>
      <span className={`text-2xl font-bold tabular-nums ${danger && value > 0 ? "text-statusFail" : "text-carbon-text"}`}>
        {value}
      </span>
      <span className="text-xs leading-tight text-carbon-textMuted wrap-break-word">{label}</span>
    </>
  );
  if (onClick) {
    return (
      <button
        type="button"
        onClick={onClick}
        className={`${base} cursor-pointer text-start hover:bg-carbon-surface3 motion-safe:transition-colors focus:outline-solid focus:outline-2 focus:outline-(--focus-ring)`}
      >
        {inner}
      </button>
    );
  }
  return <div className={base}>{inner}</div>;
}
