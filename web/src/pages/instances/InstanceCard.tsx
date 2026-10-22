import type { CSSProperties, ReactNode } from "react";

import { hueVars } from "../../lib/appearance";

/** The box every card's mark fills, the logo or a glyph, so the marks of a
 *  grid stand the same size. */
const MARK_BOX = "h-20 w-20 shrink-0 md:h-[104px] md:w-[104px]";

/** InstanceMark is the BombVault logo on the start edge of a card, as in the sidebar. */
export function InstanceMark() {
  return <img src="/logo.svg" alt="" aria-hidden="true" draggable={false} className={`block object-contain ${MARK_BOX}`} />;
}

/** LogoImage is the same logo inside a picture, in user units. */
export function LogoImage({ x, y, size }: { x: number; y: number; size: number }) {
  return <image href="/logo.svg" x={x.toFixed(1)} y={y.toFixed(1)} width={size} height={size} className="glim-logo-image" />;
}

/** The outline of a phone on a 16 unit grid, with its screen and button cut out. */
export const PHONE_PATH =
  "M5 .8h6a1.6 1.6 0 0 1 1.6 1.6v11.2a1.6 1.6 0 0 1-1.6 1.6H5a1.6 1.6 0 0 1-1.6-1.6V2.4A1.6 1.6 0 0 1 5 .8Zm.6 2.2v8.6h4.8V3Zm2.4 9.6a.8.8 0 1 0 0 1.6.8.8 0 0 0 0-1.6Z";

/** GlyphMark stands where the logo would for something that is no BombVault:
 *  a glyph on the card's accent. */
export function GlyphMark({ children }: { children: ReactNode }) {
  return (
    <span className={`grid place-items-center rounded-card bg-accent text-accentContrast [&>svg]:h-[42%] [&>svg]:w-[42%] ${MARK_BOX}`}>
      {children}
    </span>
  );
}

export function Figure({ value, label }: { value: ReactNode; label: string }) {
  return (
    <div className="flex flex-col">
      <b className="glim-num text-sm font-semibold text-carbon-text">{value}</b>
      <span className="text-dense uppercase tracking-[.14em] text-carbon-textMuted">{label}</span>
    </div>
  );
}

/** InstanceCard is one tile of the instances grid: the mark beside the name
 *  with its badges, what kind it is, a line of facts and the figures, then
 *  what the instance does, and the actions along the foot. The grid gives
 *  every card one height, so the actions sit on one line across a row. */
export function InstanceCard({
  cardKey,
  name,
  eyebrow,
  mark,
  badges,
  facts,
  figures,
  children,
  actions,
  index,
  hot = false,
}: {
  /** What the card stands for: "self", a member id or a ZFS server's id. */
  cardKey: string;
  name: string;
  eyebrow?: string;
  mark?: ReactNode;
  badges?: ReactNode;
  facts?: ReactNode;
  figures?: ReactNode;
  children?: ReactNode;
  actions?: ReactNode;
  /** Rainbow position by place in the grid. */
  index: number;
  /** The pointer is on this instance in the net picture. */
  hot?: boolean;
}) {
  return (
    <div
      data-instance={cardKey}
      data-net={cardKey}
      style={{ ...hueVars(index), "--row-i": String(index) } as CSSProperties}
      className={`relative flex h-full flex-col overflow-hidden rounded-card bg-carbon-surface glim-hue glim-stagger-row glim-net-entry${
        hot ? " glim-net-entry-hot" : ""
      }`}
    >
      <div className="flex items-start">
        {/* Level with the name rather than centred, so the mark stands in one
            place on every card whatever the card holds. */}
        <div data-new="instances-card-logo" className="flex shrink-0 ps-4 pt-5 md:pt-7">
          {mark ?? <InstanceMark />}
        </div>
        <div className="flex min-w-0 flex-1 flex-col gap-4 p-5 md:p-7">
          <div className="flex flex-col gap-1">
            {/* The name is never cut short: the badges move under it when the
                two do not fit one line. */}
            <div data-new="instances-full-names" className="flex flex-wrap items-start gap-x-2.5 gap-y-1.5">
              <span className="min-w-0 max-w-full font-semibold text-carbon-text wrap-anywhere">{name}</span>
              {badges && <span className="ms-auto flex flex-wrap items-center gap-2">{badges}</span>}
            </div>
            {eyebrow && <span className="text-dense uppercase tracking-[.14em] text-carbon-textMuted">{eyebrow}</span>}
            {facts && <p className="text-xs text-carbon-textMuted wrap-anywhere">{facts}</p>}
          </div>
          {figures && <div className="flex flex-wrap items-baseline gap-x-7 gap-y-3 empty:hidden">{figures}</div>}
        </div>
      </div>
      {children && <div className="@container flex flex-col gap-4 px-4 pb-4 md:px-5">{children}</div>}
      {actions && (
        <div className="mt-auto flex flex-wrap items-center justify-end gap-2 px-4 pb-4 md:px-5 md:pb-5">
          {actions}
        </div>
      )}
    </div>
  );
}

/** Joins the facts of a card's second line with the app's middle dot. */
export function Facts({ items }: { items: ReactNode[] }) {
  const shown = items.filter(Boolean);
  return (
    <>
      {shown.map((fact, i) => (
        <span key={i}>
          {i > 0 && " · "}
          {fact}
        </span>
      ))}
    </>
  );
}
