import type { CSSProperties, ReactNode } from "react";
import { Badge } from "../Badge";
import { InfoBubble } from "../InfoBubble";
import { hueVars, rainbowAt } from "../../lib/appearance";

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
  /** The step's own explanatory prose, folded into an `onAccent` InfoBubble
   *  on the heading badge instead of a permanent `<p>` in the card body —
   *  the same `hint` shape (and the same house convention, "explanations
   *  belong behind an inline (i)") Settings.tsx's Card(), Config.tsx's own
   *  Cards and FolderBrowser already use. jdp, live review of this tab:
   *  "Info-Texte in i Infobubbles." Optional: a step whose body is nothing
   *  but controls and live results (Discover) has no standing explanation to
   *  fold away and passes nothing. */
  hint?: string;
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
      className={`relative glim-notch-card rounded-card bg-carbon-surface p-4${hueOn ? " glim-hue" : ""}`}
      style={hueStyle}
    >
      {/* Task 5 (rule 11): structurally identical to every other converted
          Card heading — a rounded-card bg-carbon-surface panel with its own
          <h2> title, not nested inside anything already badged.
            THE NUMBER, three rounds of it, because the shape keeps mattering:
          it began as its own 24px `bg-carbon-surface2` circle sitting to the
          LEFT of the heading badge — a second, separately-coloured piece of
          chrome that never joined the colour engine. jdp then asked for it to
          become part of the title badge ("erst die Nummer und dann der Name
          der Card"), which shipped as a SPLIT pill: one badge, a shaded
          leading cell, a hard seam. He has now reversed that in turn: "Die
          Cardtitelbadges mit Nummer sollen zwei getrennte Badges sein. Der
          Badge der Nummer nicht abgedunkelt." So: two genuinely separate
          badges, side by side, BOTH taking the plain heading fill — the
          number badge is not shaded, tinted or dimmed in any way; it is the
          same `tone="heading"` badge the name is, at the same hue.
            POSITIONING, and why it moved up here rather than onto the badges:
          a heading notch normally positions ITSELF (`absolute top-0
          -translate-y-1/2`, see Badge.tsx). Two self-positioning notches
          would resolve to the same static position and land on top of each
          other, and the obvious fix — nudge the second one across by a fixed
          number of pixels — is exactly the mistake this codebase has already
          made and removed twice for this very notch (Badge.tsx's own
          REGRESSION note, and Settings.tsx's offsite-tab history). Both are
          the same defect shape: an offset derived from ONE assumed height or
          width, silently wrong the moment the real one differs — and the name
          badge's height genuinely varies here, since a long step title wraps
          to two lines at ordinary browser widths.
            So the <h2> is the positioned element and the badges ride inside
          it in ordinary flow (`inFlow`, Badge.tsx). Three consequences, all
          of them the point:
            - `-translate-y-1/2` resolves against the H2's OWN border-box
              height — the tallest badge in the row — so the PAIR's vertical
              centre sits exactly on the card's top edge at any height, wrapped
              name or not, with no number assumed anywhere.
            - `items-center` centres both badges on that same line, so the
              short number badge and a one- OR two-line name badge stay
              mutually centred instead of one drifting off the other's axis.
            - no `start-*`/`insetStart`: with left and right both `auto` the
              h2 falls back to its CSS static position, which is this p-4
              card's own content edge (the h2 is a normal-flow child of it) —
              the same self-maintaining, automatically RTL-correct mechanism
              the single badge used to rely on, just applied one level up.
          `max-w` keeps the pair clear of the status dot so a long title wraps
          instead of running under it, and `min-w-0` on the name badge is what
          lets it actually shrink to allow that. The 3.25rem is not a guess: an
          absolutely-positioned box's percentage resolves against the CARD'S
          PADDING BOX, so the subtraction has to pay for this card's own two
          p-4 edges (2rem) PLUS the dot's own 10px column and the 10px gap
          before it (the `gap-2.5` the dot used to get from the shared row).
          Measured live at 760px — the width where step 2's German title
          genuinely wraps — the first cut, which only subtracted the padding,
          put the wrapped badge's right edge at x=705, the exact pixel the
          dot's right edge sits on: a 1px-tall overlap the badge's own z-10
          would have painted straight over. With the dot's column paid for it
          lands at x=685, a clean 10px short of the dot.
            `glim-notch-card` on the card above is the other half of the
          colour wiring: index.css keys the reactive-mode card-wide hover zone
          off exactly that class, so a Recovery step reveals its hue from
          anywhere in the card like every Settings/Config/Flash card does,
          instead of only from a 22px pill. Both badges carry the same
          `hueIndex`, so they light up together as one heading. */}
      <h2 className="absolute top-0 -translate-y-1/2 z-10 flex items-center gap-1.5 max-w-[calc(100%-3.25rem)]">
        <Badge tone="heading" size="heading" inFlow hueIndex={hueIndex} className="shrink-0">
          {/* `tracking-normal` on an inner span, not on the Badge's own
              className: the heading stage's `tracking-widest` is the same CSS
              property at the same specificity, so two utilities on ONE element
              would be decided by Tailwind's generated source order (widest
              wins) rather than by what this call site asked for. On a child it
              simply overrides the inherited value. Letter-spacing is applied
              after the LAST character too, so a tracked single digit sits
              visibly left of its own badge's centre without this. */}
          <span className="tracking-normal tabular-nums">{n}</span>
        </Badge>
        <Badge tone="heading" size="heading" inFlow wrap hueIndex={hueIndex} className="min-w-0">
          {title}
          {hint && <InfoBubble tip={hint} onAccent />}
        </Badge>
      </h2>
      {/* The status dot keeps its own row and its own right-hand position. It
          used to share a `flex items-center gap-2.5` row with the <h2>, which
          claimed the leftover width via `flex-1` — now that the h2 is
          absolutely positioned it is no longer a flex item at all, so `ms-auto`
          on the dot does that job instead. The row's measured height is
          unchanged either way (the h2's absolute content never contributed to
          it), so nothing below moves. */}
      <div className="flex mb-2">
        <span className={`ms-auto h-2.5 w-2.5 rounded-full ${dot}`} />
      </div>
      <div className="text-sm text-carbon-textMuted flex flex-col gap-2">{children}</div>
    </div>
  );
}
