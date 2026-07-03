import type { ReactNode } from "react";

export type StepState = "idle" | "ok" | "warn" | "bad";

export function StepCard({ n, title, state, children }: { n: number; title: string; state: StepState; children: ReactNode }) {
  const dot = state === "ok" ? "bg-[#6fdc8c]" : state === "bad" ? "bg-[#ff8389]" : state === "warn" ? "bg-[#f1c21b]" : "bg-carbon-surface3";
  return (
    <div className="rounded-xl border border-carbon-border bg-carbon-surface p-4">
      <div className="flex items-center gap-2.5 mb-2">
        <span className="flex h-6 w-6 shrink-0 items-center justify-center rounded-full bg-carbon-surface2 text-xs font-semibold text-carbon-textSub">{n}</span>
        <h2 className="text-sm font-semibold text-carbon-text flex-1">{title}</h2>
        <span className={`h-2.5 w-2.5 rounded-full ${dot}`} />
      </div>
      <div className="text-sm text-carbon-textMuted flex flex-col gap-2">{children}</div>
    </div>
  );
}
