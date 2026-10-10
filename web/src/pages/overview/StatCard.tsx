export function StatCard({
  label,
  value,
  danger,
  onClick,
}: {
  label: string;
  value: number;
  danger?: boolean;
  /** When set, the card is a real button. The errors tile opens the error
   *  detail panel with it. */
  onClick?: () => void;
}) {
  const base = "bg-carbon-surface rounded-card px-4 py-3 flex flex-col gap-1 min-w-0 overflow-hidden";
  const inner = (
    <>
      <span
        className={`text-2xl font-bold tabular-nums ${
          danger && value > 0 ? "text-statusFail" : "text-carbon-text"
        }`}
      >
        {value}
      </span>
      <span className="text-xs text-carbon-textMuted wrap-break-word leading-tight">{label}</span>
    </>
  );
  if (onClick) {
    return (
      <button
        type="button"
        onClick={onClick}
        className={`${base} text-start cursor-pointer hover:bg-carbon-hover motion-safe:transition-colors focus:outline-solid focus:outline-2 focus:outline-(--focus-ring)`}
      >
        {inner}
      </button>
    );
  }
  return <div className={base}>{inner}</div>;
}
