import type { CSSProperties, ReactNode } from "react";
import { Badge } from "../Badge";
import { InfoBubble } from "../InfoBubble";
import { hueVars } from "../../lib/appearance";

export type StepState = "idle" | "ok" | "warn" | "bad";

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
  state: StepState;
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
      className={`relative glim-notch-card rounded-card bg-carbon-surface p-4${hueOn ? " glim-hue" : ""}`}
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
      <div className="flex mb-2">
        <span className={`ms-auto h-2.5 w-2.5 rounded-full ${dot}`} />
      </div>
      <div className="text-sm text-carbon-textMuted flex flex-col gap-2">{children}</div>
    </div>
  );
}
