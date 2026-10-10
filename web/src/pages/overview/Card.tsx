import type { CSSProperties } from "react";
import { hueVars } from "../../lib/appearance";
import { Badge } from "../../components/Badge";
import { InfoBubble } from "../../components/InfoBubble";

export function Card({
  title,
  hint,
  children,
  hueIndex,
}: {
  title: string;
  /** One-line explanation of the whole card, shown as an (i) bubble beside
   *  the title, as on the Settings cards. */
  hint?: string;
  children: React.ReactNode;
  /** Rainbow position of the heading notch, handed out by the page in the
   *  order the cards are rendered. Omit it for a card that stands alone. */
  hueIndex?: number;
}) {
  return (
    // The heading badge straddles the card's top edge, and the surface box
    // below clips its overflow (a progress bar's square ends have to follow
    // the rounded corners). So the badge is positioned against this unpadded
    // outer div, and insetStart restates the surface box's p-5 for it.
    // glim-notch-card lets a hover anywhere on the card reveal the heading's
    // hue in reactive mode, and glim-hue hands the same hue to the controls
    // inside.
    <div
      className={`relative glim-notch-card${hueIndex !== undefined ? " glim-hue" : ""}`}
      style={hueIndex !== undefined ? (hueVars(hueIndex) as CSSProperties) : undefined}
    >
      <h2 className="flex items-center">
        <Badge tone="heading" size="heading" wrap hueIndex={hueIndex} insetStart={5}>
          {title}
          {hint && <InfoBubble tip={hint} onAccent />}
        </Badge>
      </h2>
      <div className="bg-carbon-surface rounded-card p-5 flex flex-col gap-4 overflow-hidden">
        {children}
      </div>
    </div>
  );
}
