import type { CSSProperties, ReactNode } from "react";
import { Badge } from "../Badge";
import { hueVars, rainbowAt } from "../../lib/appearance";

export type StepState = "idle" | "ok" | "warn" | "bad";

export function StepCard({
  n,
  title,
  state,
  children,
  hueIndex,
}: {
  n: number;
  title: string;
  state: StepState;
  children: ReactNode;
  /** Rainbow position for this step's own heading notch — same mechanism as
   *  every other Card's `hueIndex` (see Settings.tsx's `Card()`/Badge.tsx's
   *  own doc for the full history). Recovery.tsx assigns these via its own
   *  page-flat `nextHue()` counter (no tabs here, so one running sequence
   *  for the whole page) at every call site. Optional so a bare StepCard
   *  used without rainbow wiring still renders (Badge itself no-ops without
   *  an index).
   *
   *  Rainbow-mode completeness sweep (jdp, live review: "Es sind nicht alle
   *  Buttons in den Regenbogen-Modus eingepflegt. Nachholen."): this same
   *  value ALSO now lands on the card's own outer wrapper below — every
   *  Recheck/Restore/Discover/Connect/Reload/etc. button living in a step's
   *  body used to stay the flat theme accent forever, rainbow on or off,
   *  because only the heading Badge carried `.glim-hue`. One index, one
   *  hue, shared by the title notch AND everything inside the card — the
   *  same one-hue-per-card convention Settings.tsx's own Card() already
   *  uses. */
  hueIndex?: number;
}) {
  const dot = state === "ok" ? "bg-statusOkSolid" : state === "bad" ? "bg-statusFailSolid" : state === "warn" ? "bg-statusWarnSolid" : "bg-carbon-surface3";
  // `.glim-hue` redefines --accent/--accent-contrast/--accent-soft (plus the
  // Tailwind-mapped --color-accent* trio) AND --focus-ring/--field-focus-ring
  // on whichever element carries it (index.css's `[data-rainbow] .glim-hue`
  // block) — custom properties inherit normally, so tagging THIS card's own
  // outer div is enough for every descendant bg-accent/text-accentContrast
  // button (and any focused control's ring) to pick up the SAME hue with no
  // per-button edits at any call site, exactly the way ContainerRow/VMRow/
  // FileSetRow already colour their own restore buttons purely by tagging
  // the row (see components/RestorePanel.tsx's own comment: "this row lives
  // inside ContainerRow's own .glim-hue element, no hueIndex needed").
  // Load-bearing status signals are untouched by design: the state dot above
  // and every text-statusOk/Warn/Fail span in a step's body read --status-*
  // tokens, which this rule never redefines.
  const hueOn = hueIndex !== undefined;
  const hueStyle = hueOn ? (hueVars(rainbowAt(hueIndex)) as CSSProperties) : undefined;
  return (
    <div
      className={`relative rounded-card bg-carbon-surface p-4${hueOn ? " glim-hue" : ""}`}
      style={hueStyle}
    >
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
          <Badge tone="heading" size="heading" wrap className="max-w-full" hueIndex={hueIndex}>{title}</Badge>
        </h2>
        <span className={`h-2.5 w-2.5 rounded-full ${dot}`} />
      </div>
      <div className="text-sm text-carbon-textMuted flex flex-col gap-2">{children}</div>
    </div>
  );
}
