import type { CSSProperties, ReactNode } from "react";

import { InfoBubble } from "../../components/InfoBubble";
import { IconLatest } from "../../components/glyphs";
import { IconReceiver, IconZFS } from "../../components/navGlyphs";
import { hueVars } from "../../lib/appearance";
import { useT, type TranslationKey } from "../../lib/i18n";
import { useTipBubble } from "../../lib/useTipBubble";
import { ROLE_KINDS, type RoleKind, type RoleState } from "./instancesModel";

/** A role's name, glyph and the sentence that says what it does. */
export const ROLES: Record<RoleKind, { label: TranslationKey; tip: TranslationKey; glyph: ReactNode }> = {
  receiver: { label: "receiver.title", tip: "instances.role.receiverTip", glyph: <IconReceiver /> },
  fetcher: { label: "instances.role.fetcher", tip: "instances.role.fetcherTip", glyph: <IconLatest /> },
  zfs: { label: "zfs.replica.servers.title", tip: "instances.role.zfsTip", glyph: <IconZFS /> },
};

export const ROLE_STATE_KEY: Record<RoleState, TranslationKey> = {
  on: "instances.role.on",
  waiting: "instances.role.waiting",
  off: "zfs.replica.servers.off",
};

const FILL: Record<RoleState, string> = {
  on: "bg-accent text-accentContrast",
  // A request that still waits is filled like an active role and wears a
  // dashed inner line until the other side allows it.
  waiting: "bg-accent text-accentContrast outline-dashed outline-[1.5px] -outline-offset-[5px] outline-accentContrast",
  off: "bg-carbon-surface3 text-carbon-textMuted",
};

/** RoleTile is one role on an instance's card: filled while it is on or
 *  asked for, flat while it is off. With `onToggle` it is the button that
 *  asks for the role or gives it up. */
export function RoleTile({
  role,
  state,
  detail,
  hueIndex,
  onToggle,
}: {
  role: RoleKind;
  state: RoleState;
  /** The line under the name, in place of the state's own word. */
  detail?: string;
  /** Rainbow position among the tiles of one card. */
  hueIndex: number;
  onToggle?: () => void;
}) {
  const { t } = useT();
  const { label, tip, glyph } = ROLES[role];
  const tooltip = useTipBubble(onToggle ? t(tip) : undefined);
  const cls = `glim-hue flex min-w-0 items-center gap-2 rounded-control px-2 py-2 text-start md:px-2.5 md:py-[7px] [&>svg]:h-[18px] [&>svg]:w-[18px] ${FILL[state]}`;
  const body = (
    <>
      {glyph}
      <span className="flex min-w-0 flex-col">
        <span
          className={`text-dense font-semibold wrap-anywhere md:truncate md:text-[13px] ${
            state === "off" ? "text-carbon-text" : ""
          }`}
        >
          {t(label)}
        </span>
        <span className="text-dense leading-[1.3]">{detail ?? t(ROLE_STATE_KEY[state])}</span>
      </span>
    </>
  );
  const style = hueVars(hueIndex) as CSSProperties;

  if (!onToggle) {
    return (
      <div data-role={role} data-state={state} className={cls} style={style}>
        {body}
      </div>
    );
  }
  return (
    <>
      <button
        ref={tooltip.ref}
        type="button"
        data-role={role}
        data-state={state}
        aria-pressed={state !== "off"}
        aria-describedby={tooltip.describedBy}
        onClick={onToggle}
        {...tooltip.handlers}
        className={`${cls} glim-field-focus transition-[filter,background-color] ${
          state === "off" ? "hover:bg-carbon-hoverRaised" : "hover:brightness-105"
        }`}
        style={style}
      >
        {body}
      </button>
      {tooltip.bubble}
    </>
  );
}

/** RoleTiles is the row of role tiles on a card, under the question they
 *  answer. A role without an entry in `states` is not drawn. */
export function RoleTiles({
  heading,
  states,
  details,
  onToggle,
}: {
  heading: string;
  states: Partial<Record<RoleKind, RoleState>>;
  details?: Partial<Record<RoleKind, string>>;
  onToggle?: (role: RoleKind) => void;
}) {
  const { t } = useT();
  const shown = ROLE_KINDS.filter((k) => states[k] !== undefined);
  if (shown.length === 0) return null;
  return (
    <div className="flex flex-col gap-2">
      <span className="flex items-center gap-1.5 text-[13px] font-semibold text-carbon-textSub">
        {heading}
        <InfoBubble tip={t("instances.role.diff")} />
      </span>
      <div className="grid grid-cols-3 gap-1.5 md:gap-2">
        {shown.map((k) => (
          <RoleTile
            key={k}
            role={k}
            state={states[k]!}
            detail={details?.[k]}
            hueIndex={ROLE_KINDS.indexOf(k)}
            onToggle={onToggle && (() => onToggle(k))}
          />
        ))}
      </div>
    </div>
  );
}
