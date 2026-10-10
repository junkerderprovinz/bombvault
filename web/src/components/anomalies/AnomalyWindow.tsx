// The window a finding's details and an item's monitoring open in: a card over
// the page at desktop widths, a sheet from the bottom edge on a phone.

import { useEffect, useId, useRef, type ReactNode } from "react";
import { createPortal } from "react-dom";

import { Badge } from "../Badge";
import { Button } from "../Button";
import { BottomSheet } from "../mobile/BottomSheet";
import { useT } from "../../lib/i18n";
import { focusableElements } from "../../lib/useConfirm";
import { useIsDesktop } from "../../lib/useMediaQuery";

export function AnomalyWindow({
  title,
  onClose,
  actions,
  children,
}: {
  /** A name goes in as an element that keeps its own case. */
  title: ReactNode;
  onClose: () => void;
  /** Buttons for the foot of the window, ahead of the one that closes it. */
  actions?: ReactNode;
  children: ReactNode;
}) {
  const isDesktop = useIsDesktop();
  if (!isDesktop) {
    return (
      <BottomSheet
        open
        onClose={onClose}
        title={title}
        footer={actions ? <div className="flex flex-wrap items-center justify-end gap-2 py-3">{actions}</div> : undefined}
      >
        <div className="flex flex-col gap-4 pb-4">{children}</div>
      </BottomSheet>
    );
  }
  return (
    <DesktopWindow title={title} onClose={onClose} actions={actions}>
      {children}
    </DesktopWindow>
  );
}

function DesktopWindow({
  title,
  onClose,
  actions,
  children,
}: {
  title: ReactNode;
  onClose: () => void;
  actions?: ReactNode;
  children: ReactNode;
}) {
  const { t } = useT();
  const titleId = useId();
  const cardRef = useRef<HTMLDivElement>(null);
  const closeRef = useRef(onClose);
  closeRef.current = onClose;

  useEffect(() => {
    const opener = document.activeElement instanceof HTMLElement ? document.activeElement : null;
    const card = cardRef.current;
    card?.focus();

    function onKeyDown(e: KeyboardEvent) {
      if (!card) return;
      // A confirmation or an open list above this window holds the focus and
      // answers the key itself.
      const active = document.activeElement;
      const inside = active instanceof Node && card.contains(active);
      if (!inside && active !== document.body) return;
      if (e.key === "Escape") {
        e.preventDefault();
        closeRef.current();
        return;
      }
      if (e.key !== "Tab") return;
      const focusables = focusableElements(card);
      if (focusables.length === 0) return;
      const first = focusables[0];
      const last = focusables[focusables.length - 1];
      if (e.shiftKey && (!inside || active === first || active === card)) {
        e.preventDefault();
        last.focus();
      } else if (!e.shiftKey && (!inside || active === last)) {
        e.preventDefault();
        first.focus();
      }
    }
    document.addEventListener("keydown", onKeyDown);
    return () => {
      document.removeEventListener("keydown", onKeyDown);
      if (opener && document.contains(opener)) opener.focus();
    };
  }, []);

  return createPortal(
    <div
      className="glim-modal-backdrop fixed inset-0 z-50 flex items-center justify-center overflow-y-auto p-4"
      onMouseDown={(e) => {
        if (e.target === e.currentTarget) onClose();
      }}
    >
      <div className="glim-modal-card relative w-full max-w-[45rem]">
        <h2 id={titleId} className="flex items-center px-6">
          <Badge tone="heading" size="heading" wrap>
            {title}
          </Badge>
        </h2>
        <div
          ref={cardRef}
          tabIndex={-1}
          role="dialog"
          aria-modal="true"
          aria-labelledby={titleId}
          className="flex max-h-[90vh] w-full flex-col gap-4 overflow-y-auto rounded-card bg-carbon-surface p-6 pt-7 shadow-2xl outline-none"
        >
          {children}
          <div className="flex flex-wrap items-center justify-end gap-2.5">
            {actions}
            <Button label={t("common.close")} labelKey="common.close" onClick={onClose} />
          </div>
        </div>
      </div>
    </div>,
    document.body
  );
}
