import { Children, cloneElement, Fragment, isValidElement, useState, type MouseEvent, type ReactElement, type ReactNode } from "react";

import { Button } from "../Button";
import { IconMore } from "../navGlyphs";
import { useT } from "../../lib/i18n";
import { BottomSheet } from "./BottomSheet";

/** The most buttons a window's foot shows on a phone before it folds. */
const ROOM = 3;

function buttonsOf(children: ReactNode, prefix = ""): ReactElement[] {
  return Children.toArray(children).flatMap((child) => {
    if (!isValidElement(child)) return [];
    const key = `${prefix}${child.key ?? ""}`;
    if (child.type === Fragment) return buttonsOf((child.props as { children?: ReactNode }).children, key);
    return [cloneElement(child, { key })];
  });
}

/**
 * SheetFoot is the foot of a window shown as a sheet on a phone. Up to three
 * buttons stand in it as given. With more, it keeps the way out and the main
 * action, which stands last because forward is on the right, and the others
 * wait behind More in a sheet of their own.
 */
export function SheetFoot({ close, children }: {
  /** The window's own way out, after the actions. Without it the first of
   *  the children is the way out. */
  close?: ReactNode;
  children?: ReactNode;
}) {
  const { t } = useT();
  const [more, setMore] = useState(false);
  const buttons = buttonsOf(children);
  if (buttons.length + (close ? 1 : 0) <= ROOM) {
    return (
      <div className="flex flex-wrap items-center justify-end gap-2 py-3">
        {children}
        {close}
      </div>
    );
  }

  const way = close ?? buttons[0];
  const rest = close ? buttons : buttons.slice(1);
  const main = rest[rest.length - 1];
  // A press on an action closes the sheet so its result shows in the window.
  // The (i) beside an action is no action.
  const chosen = (e: MouseEvent) => {
    if (e.target instanceof Element && e.target.closest(".glim-btn")) setMore(false);
  };
  return (
    <div className="flex flex-wrap items-center justify-end gap-2 py-3">
      <span className="me-auto flex">
        <Button
          label={t("nav.more")}
          labelKey="nav.more"
          glyph={<IconMore />}
          tone="neutral"
          ariaExpanded={more}
          onClick={() => setMore(true)}
        />
      </span>
      {way}
      {main}
      <BottomSheet open={more} onClose={() => setMore(false)} title={t("nav.more")}>
        <div className="glim-fold-stack flex flex-col gap-3 pb-1" onClick={chosen}>
          {rest.slice(0, -1)}
        </div>
      </BottomSheet>
    </div>
  );
}
