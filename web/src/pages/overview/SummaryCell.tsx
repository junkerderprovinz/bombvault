import type { CSSProperties } from "react";
import { hueVars } from "../../lib/appearance";
import { Badge } from "../../components/Badge";

export function SummaryCell({
  label,
  children,
  hueIndex,
}: {
  label: string;
  children: React.ReactNode;
  /** Rainbow position of the heading notch, a plain number the page has
   *  already resolved (see SummaryTier). */
  hueIndex?: number;
}) {
  return (
    // The same outer and inner split as Card: the heading badge sits on the
    // unpadded outer div so the surface box's overflow-hidden cannot clip it,
    // and insetStart restates the box's p-5. min-w-0 is on the outer div
    // because that is the grid item whose min-width the track sizing reads.
    <div
      className={`relative glim-notch-card min-w-0${hueIndex !== undefined ? " glim-hue" : ""}`}
      style={hueIndex !== undefined ? (hueVars(hueIndex) as CSSProperties) : undefined}
    >
      {/* wrap and max-w-full because the cell is one of three narrow columns. */}
      <h2 className="flex items-center min-w-0">
        <Badge tone="heading" size="heading" wrap className="max-w-full" hueIndex={hueIndex} insetStart={5}>{label}</Badge>
      </h2>
      {/* The badge pokes 11px into this box, so its top padding minus 11px is
          the clearance above the first line. p-5 matches Card and leaves 9px. */}
      <div className="bg-carbon-surface rounded-card p-5 flex flex-col gap-2 min-w-0 overflow-hidden">
        {/* flex-wrap so a value that cannot fit on one line (e.g. status chip + a
            relative time in a narrow half-width cell) drops to a second line and stays
            fully readable, instead of being hard-clipped by overflow-hidden (#98). */}
        <div className="flex flex-wrap items-center gap-x-2 gap-y-1 min-h-7 min-w-0">{children}</div>
      </div>
    </div>
  );
}
