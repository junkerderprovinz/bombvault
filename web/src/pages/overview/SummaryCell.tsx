export function SummaryCell({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="flex min-w-0 flex-col gap-1.5">
      <span className="text-xs text-carbon-textMuted">{label}</span>
      {/* A value that does not fit on one line drops to a second one instead
          of being cut off (#98). */}
      <div className="flex min-h-7 min-w-0 flex-wrap items-center gap-x-2 gap-y-1">{children}</div>
    </div>
  );
}
