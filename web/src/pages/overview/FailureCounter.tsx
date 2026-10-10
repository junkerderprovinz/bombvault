import type { CSSProperties } from "react";
import { hueVars } from "../../lib/appearance";
import type { useT } from "../../lib/i18n";
import { Badge } from "../../components/Badge";
import { StatCard } from "./StatCard";

/**
 * The one acknowledgeable-failure counter, two densities: the desktop stat
 * tile and the phone runs block's full-width row both render a count of
 * failedRunsNeedingAttention and both open the error detail panel, the only
 * place in the app where a failure can be read and acknowledged. The count
 * itself is the caller's (the desktop tile reads computeStatData, the phone
 * row the page's polled runs; both bottom out in the same derivation, so the
 * two surfaces cannot disagree). The status colour rides the Badge, not the
 * row.
 */
export function FailureCounter({
  t,
  count,
  dense,
  hueIndex,
  onOpen,
}: {
  t: ReturnType<typeof useT>["t"];
  count: number;
  /** Phone glance face: a full-width >=44px row instead of the stat tile. */
  dense?: boolean;
  /** The phone row's position in the block's hue rotation. */
  hueIndex?: number;
  onOpen: () => void;
}) {
  if (dense) {
    return (
      <button
        type="button"
        onClick={onOpen}
        aria-label={`${t("dashboard.statErrors")}: ${count}`}
        className="mb-1 flex min-h-[2.75rem] w-full items-center gap-2 rounded-control bg-carbon-surface2 px-2 py-2 text-start glim-hue"
        style={hueVars(hueIndex ?? 0) as CSSProperties}
      >
        <Badge tone="fail">{count}</Badge>
        <span className="min-w-0 flex-1 truncate text-sm text-carbon-text">
          {t("dashboard.statErrors")}
        </span>
      </button>
    );
  }
  return (
    <StatCard
      label={t("dashboard.statErrors")}
      value={count}
      danger
      // Clickable only when there are errors to show.
      onClick={count > 0 ? onOpen : undefined}
    />
  );
}
