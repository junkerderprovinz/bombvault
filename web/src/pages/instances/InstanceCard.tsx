import type { CSSProperties, ReactNode } from "react";

import { hueVars } from "../../lib/appearance";

/** InstanceMark is the BombVault logo on the start edge of a card, as in the sidebar. */
export function InstanceMark() {
  return <img src="/logo.svg" alt="" aria-hidden="true" draggable={false} className="block h-20 md:h-[104px] w-auto shrink-0" />;
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
    <span className="grid h-[72px] w-[72px] shrink-0 place-items-center rounded-card bg-accent text-accentContrast [&>svg]:h-[34px] [&>svg]:w-[34px]">
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
 *  with its badges, a line of facts and the figures, then what the instance
 *  does, and the actions along the foot. Cards of a row share their height,
 *  so the actions sit on one line across the row. */
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
      <div className="flex items-stretch">
        {/* A card with little beside the mark, such as a phone's, keeps the
            mark off its edge and its words beside the mark's middle. */}
        <div className="flex shrink-0 items-center py-5 ps-4 md:py-6">{mark ?? <InstanceMark />}</div>
        <div className="flex min-w-0 flex-1 flex-col justify-center gap-4 p-5 md:p-7">
          <div className="flex flex-col gap-1">
            <div className="flex flex-wrap items-center gap-x-2.5 gap-y-1.5">
              <span className="min-w-0 truncate font-semibold text-carbon-text max-md:whitespace-normal max-md:wrap-break-word">
                {name}
              </span>
              {eyebrow && (
                <span className="shrink-0 text-dense uppercase tracking-[.14em] text-carbon-textMuted">{eyebrow}</span>
              )}
              {badges && <span className="ms-auto flex flex-wrap items-center gap-2">{badges}</span>}
            </div>
            {facts && <p className="text-xs text-carbon-textMuted wrap-anywhere">{facts}</p>}
          </div>
          {figures && <div className="flex flex-wrap items-baseline gap-x-7 gap-y-3 empty:hidden">{figures}</div>}
        </div>
      </div>
      {children && <div className="@container flex flex-col gap-4 px-4 pb-4 md:px-5">{children}</div>}
      {actions && (
        <div className="mt-auto flex flex-wrap items-center gap-2 px-4 pb-4 md:px-5 md:pb-5 [&>:first-child]:flex-auto">
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
