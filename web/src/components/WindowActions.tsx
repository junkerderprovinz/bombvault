import type { ReactNode } from "react";

/**
 * WindowActions is the one button row of a window: its last element, aligned
 * to the end of the line, with the action that goes ahead given last (GlimStone
 * rule 15 and "Button order in a pair"). Under RTL the row mirrors with the page.
 */
export function WindowActions({ children, className }: { children: ReactNode; className?: string }) {
  const row = "flex shrink-0 flex-wrap items-center justify-end gap-3 px-5 py-4";
  return <div className={className ? `${row} ${className}` : row}>{children}</div>;
}
