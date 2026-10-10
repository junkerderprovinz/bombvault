import type { CSSProperties, ReactNode } from "react";

import { hueVars } from "../../lib/appearance";

/** InstanceMark is the BombVault logo on the start edge of a card, in the
 *  variant the colour mode needs, as in the sidebar. */
export function InstanceMark() {
  const cls = "h-20 md:h-[104px] w-auto shrink-0";
  return (
    <>
      <img src="/logo.svg" alt="" aria-hidden="true" draggable={false} className={`${cls} block dark:hidden`} />
      <img src="/logo-light.svg" alt="" aria-hidden="true" draggable={false} className={`${cls} hidden dark:block`} />
    </>
  );
}

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
}) {
  return (
    <div
      data-instance={cardKey}
      style={{ ...hueVars(index), "--row-i": String(index) } as CSSProperties}
      className="relative flex h-full flex-col overflow-hidden rounded-card bg-carbon-surface2 glim-hue glim-stagger-row"
    >
      <div className="flex items-stretch">
        <div className="flex shrink-0 items-center ps-4">{mark ?? <InstanceMark />}</div>
        <div className="flex min-w-0 flex-1 flex-col gap-4 p-5 md:p-7">
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
      {children && <div className="flex flex-col gap-4 px-4 pb-4 md:px-5">{children}</div>}
      {actions && (
        <div className="mt-auto flex flex-wrap items-center gap-2 px-4 pb-4 md:px-5 md:pb-5 [&>:first-child]:flex-1">
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
