import type { ReactNode } from "react";
import { InfoBubble } from "../InfoBubble";

/** StepCard is one step of the window that adds a storage location. */
export function StepCard({ title, tip, children }: { title: string; tip?: string; children: ReactNode }) {
  return (
    <section className="flex flex-col gap-3 rounded-card bg-carbon-surface2 p-4">
      <h3 className="flex items-center gap-1.5 text-xs font-semibold uppercase tracking-widest text-carbon-textSub">
        {title}
        {tip && <InfoBubble tip={tip} />}
      </h3>
      {children}
    </section>
  );
}
