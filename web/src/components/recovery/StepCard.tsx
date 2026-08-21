import type { ReactNode } from "react";
import { Badge } from "../Badge";

export type StepState = "idle" | "ok" | "warn" | "bad";

export function StepCard({ n, title, state, children }: { n: number; title: string; state: StepState; children: ReactNode }) {
  const dot = state === "ok" ? "bg-statusOkSolid" : state === "bad" ? "bg-statusFailSolid" : state === "warn" ? "bg-statusWarnSolid" : "bg-carbon-surface3";
  return (
    <div className="relative rounded-card bg-carbon-surface p-4">
      <div className="flex items-center gap-2.5 mb-2">
        <span className="flex h-6 w-6 shrink-0 items-center justify-center rounded-full bg-carbon-surface2 text-xs font-semibold text-carbon-textSub">{n}</span>
        {/* Task 5 (rule 11): structurally identical to every other converted
            Card heading — a rounded-card bg-carbon-surface panel with its
            own <h2> title, not nested inside anything already badged.
            GlimStone follow-up pass ("half-overlap card notch"): `relative`
            added on the outer p-4 card above — the heading Badge is now
            `position: absolute` and straddles that card's real edge (the
            step-number circle and status dot either side of the <h2> keep
            their own positions untouched: this row's <h2> already carries
            `flex-1`, which claims its share of the row's width regardless of
            whether its content renders in normal flow, so removing the
            badge from flow doesn't collapse the gap between them). */}
        <h2 className="flex items-center min-w-0 flex-1">
          <Badge tone="heading" size="heading" wrap className="max-w-full">{title}</Badge>
        </h2>
        <span className={`h-2.5 w-2.5 rounded-full ${dot}`} />
      </div>
      <div className="text-sm text-carbon-textMuted flex flex-col gap-2">{children}</div>
    </div>
  );
}
