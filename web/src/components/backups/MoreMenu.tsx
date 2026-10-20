import { useRef, useState } from "react";
import { useT } from "../../lib/i18n";
import { Button } from "../Button";
import { IconMore } from "../navGlyphs";
import { PageAction } from "../PageActions";
import { ActionMenu, type MenuAction } from "./ActionMenu";

/**
 * MoreMenu holds the actions a place needs less often behind one More button.
 * A menu of one action is that action, shown as a button of its own.
 */
export function MoreMenu({
  label,
  actions,
  page = false,
  className,
  newId,
  busy,
  title,
}: {
  /** What the menu is about, for a screen reader. */
  label: string;
  actions: MenuAction[];
  /** Draws the button as an action of the page header. */
  page?: boolean;
  /** Classes of the button. */
  className?: string;
  /** The data-new value of what the button stands in. */
  newId?: string;
  busy?: boolean;
  title?: string;
}) {
  const { t } = useT();
  const anchor = useRef<HTMLSpanElement>(null);
  const [open, setOpen] = useState(false);

  const only = actions.length === 1 ? actions[0] : null;
  const shown = only ?? { label: t("nav.more"), glyph: <IconMore /> };
  const press = only ? only.onSelect : () => setOpen((was) => !was);
  const button = page ? (
    <PageAction label={shown.label} glyph={shown.glyph} onClick={press} busy={busy} title={title} />
  ) : (
    <Button
      className={className}
      label={shown.label}
      labelKey={only ? null : "nav.more"}
      glyph={shown.glyph}
      ariaExpanded={only ? undefined : open}
      onClick={press}
      busy={busy}
      title={title}
    />
  );

  return (
    <>
      <span ref={anchor} data-new={newId} className="flex">
        {button}
      </span>
      {!only && open && anchor.current && (
        <ActionMenu label={label} actions={actions} place={{ anchor: anchor.current }} onClose={() => setOpen(false)} />
      )}
    </>
  );
}
