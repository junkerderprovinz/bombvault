import { useLayoutEffect, useRef, useState, type CSSProperties, type ReactNode } from "react";
import { Badge } from "../Badge";
import { InfoBubble } from "../InfoBubble";
import { hueVars } from "../../lib/appearance";

export type StepState = "idle" | "ok" | "warn" | "bad";

/** The grid three step cards stand in: one column on a phone, and on a wider
 *  screen as many as keep every heading on one line, so a card that would cut
 *  its title short moves to the next row instead. The number badges straddle
 *  the top edge, so stacked cards keep more room between them than a card gap.
 *  Pair it with the style useStepGrid hands out. */
export const STEP_GRID =
  "relative grid grid-cols-1 gap-10 md:grid-cols-[repeat(auto-fit,minmax(min(100%,max(var(--step-min),30%)),1fr))] md:gap-6 md:[&_h2>*]:whitespace-nowrap";

/** How wide a card has to be for its heading to stay on one line: the badges
 *  at their full width and the room StepCard leaves beside its heading. */
export function stepWidth(root: HTMLElement): number {
  const rem = parseFloat(getComputedStyle(document.documentElement).fontSize);
  const widths = Array.from(root.querySelectorAll<HTMLElement>("h2")).map((head) => {
    const badges = Array.from(head.children as HTMLCollectionOf<HTMLElement>);
    const gap = parseFloat(getComputedStyle(head).columnGap) || 0;
    return badges.reduce((sum, badge) => sum + badge.scrollWidth, 0) + gap * (badges.length - 1);
  });
  return Math.ceil(Math.max(0, ...widths) + 3.25 * rem);
}

/** useStepGrid measures the width STEP_GRID needs, again when the words or
 *  the font change. */
export function useStepGrid<E extends HTMLElement>(words: string) {
  const grid = useRef<E>(null);
  const [stepMin, setStepMin] = useState(0);
  useLayoutEffect(() => {
    const root = grid.current!;
    const measure = () => setStepMin(stepWidth(root));
    measure();
    void document.fonts?.ready.then(measure);
    const watch = new ResizeObserver(measure);
    watch.observe(root);
    return () => watch.disconnect();
  }, [words]);
  return { grid, style: { "--step-min": `${stepMin}px` } as CSSProperties };
}

export function StepCard({
  n,
  title,
  hint,
  state,
  children,
  hueIndex,
}: {
  n: number;
  title: string;
  /** Explanation shown in an InfoBubble on the heading. Steps whose body is
   *  only controls and live results leave it out. */
  hint?: string;
  /** The status dot in the corner; a step that only explains leaves it out. */
  state?: StepState;
  children: ReactNode;
  /** Rainbow position shared by the heading and every accent control in the
   *  card. Without it the card keeps the theme accent. */
  hueIndex?: number;
}) {
  const dot = state === "ok" ? "bg-statusOkSolid" : state === "bad" ? "bg-statusFailSolid" : state === "warn" ? "bg-statusWarnSolid" : "bg-carbon-surface3";
  // .glim-hue redefines the accent and focus-ring properties, and custom
  // properties inherit, so tagging the card recolours every accent control in
  // it. Status colours read --status-* tokens and keep their meaning.
  // glim-notch-card lets the reactive rainbow mode reveal the hue on hover
  // anywhere in the card.
  const hueOn = hueIndex !== undefined;
  const hueStyle = hueOn ? (hueVars(hueIndex) as CSSProperties) : undefined;
  return (
    <div
      className={`relative glim-notch-card rounded-card bg-carbon-surface p-4 ${state ? "" : "pt-7"}${hueOn ? " glim-hue" : ""}`}
      style={hueStyle}
    >
      {/* The number and the title are two heading badges in the flow of one
          positioned h2, so the pair stays centred on the card's top edge even
          when a long title wraps to two lines. Positioning each badge on its
          own would need an offset derived from an assumed height. Without
          start or end offsets the h2 sits at the card's content edge, in RTL
          too. The percentage in max-w resolves against the card's padding box,
          so 3.25rem pays for both p-4 edges plus the status dot and its gap;
          a long title wraps before it reaches the dot. */}
      <h2 className="absolute top-0 -translate-y-1/2 z-10 flex items-center gap-1.5 max-w-[calc(100%-3.25rem)]">
        <Badge tone="heading" size="heading" inFlow hueIndex={hueIndex} className="shrink-0">
          {/* On the Badge itself, tracking-normal would tie with the heading's
              tracking-widest and lose on CSS order. Letter spacing also
              follows the last character, which pushes a lone digit off
              centre. */}
          <span className="tracking-normal tabular-nums">{n}</span>
        </Badge>
        <Badge tone="heading" size="heading" inFlow wrap hueIndex={hueIndex} className="min-w-0">
          {title}
          {hint && <InfoBubble tip={hint} onAccent />}
        </Badge>
      </h2>
      {state && (
        <div className="flex mb-2">
          <span className={`ms-auto h-2.5 w-2.5 rounded-full ${dot}`} />
        </div>
      )}
      <div className="text-sm text-carbon-textMuted flex flex-col gap-2">{children}</div>
    </div>
  );
}
