import type { ReactNode } from "react";
import { Badge } from "../Badge";

export type StepState = "idle" | "ok" | "warn" | "bad";

export function StepCard({ n, title, state, children }: { n: number; title: string; state: StepState; children: ReactNode }) {
  const dot = state === "ok" ? "bg-statusOkSolid" : state === "bad" ? "bg-statusFailSolid" : state === "warn" ? "bg-statusWarnSolid" : "bg-carbon-surface3";
  return (
    <div className="rounded-card bg-carbon-surface p-4">
      <div className="flex items-center gap-2.5 mb-2">
        <span className="flex h-6 w-6 shrink-0 items-center justify-center rounded-full bg-carbon-surface2 text-xs font-semibold text-carbon-textSub">{n}</span>
        {/* Task 5 (rule 11): structurally identical to every other converted
            Card heading — a rounded-card bg-carbon-surface panel with its
            own <h2> title, not nested inside anything already badged. */}
        <h2 className="flex items-center min-w-0 flex-1">
          <Badge tone="heading" size="heading" wrap className="max-w-full">{title}</Badge>
        </h2>
        <span className={`h-2.5 w-2.5 rounded-full ${dot}`} />
      </div>
      <div className="text-sm text-carbon-textMuted flex flex-col gap-2">{children}</div>
    </div>
  );
}
